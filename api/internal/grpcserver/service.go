package grpcserver

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/ipedrazas/pulse/api/internal/alerts"
	"github.com/ipedrazas/pulse/api/internal/repository"
	monitorv1 "github.com/ipedrazas/pulse/proto/monitor/v1"
)

type MonitoringService struct {
	monitorv1.UnimplementedMonitoringServiceServer
	containers repository.ContainerRepository
	actions    repository.ActionRepository
	agents     repository.AgentRepository
	notifier   *alerts.Notifier
}

func NewMonitoringService(containers repository.ContainerRepository, actions repository.ActionRepository, agents repository.AgentRepository, notifier *alerts.Notifier) *MonitoringService {
	return &MonitoringService{
		containers: containers,
		actions:    actions,
		agents:     agents,
		notifier:   notifier,
	}
}

func (s *MonitoringService) ReportHeartbeat(ctx context.Context, req *monitorv1.ReportHeartbeatRequest) (*monitorv1.ReportHeartbeatResponse, error) {
	// Query previous status before inserting the new heartbeat.
	prevStatus, err := s.containers.GetPreviousStatus(ctx, req.ContainerId)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		slog.Error("failed to query previous status", "container_id", req.ContainerId, "error", err)
	}

	if err := s.containers.InsertHeartbeat(ctx, req.ContainerId, req.Status, req.UptimeSeconds); err != nil {
		slog.Error("failed to insert heartbeat", "container_id", req.ContainerId, "error", err)
		return nil, err
	}

	slog.Debug("heartbeat recorded", "container_id", req.ContainerId, "status", req.Status)

	// Detect state transitions and fire webhook.
	if s.notifier != nil && prevStatus != "" && prevStatus != req.Status {
		event := s.buildTransitionEvent(ctx, req.ContainerId, prevStatus, req.Status)
		s.notifier.Send(event)
	}

	return &monitorv1.ReportHeartbeatResponse{}, nil
}

func (s *MonitoringService) SyncMetadata(ctx context.Context, req *monitorv1.SyncMetadataRequest) (*monitorv1.SyncMetadataResponse, error) {
	mountsJSON, err := json.Marshal(req.Mounts)
	if err != nil {
		return nil, err
	}

	envsJSON, err := json.Marshal(req.Envs)
	if err != nil {
		return nil, err
	}

	labelsJSON, err := json.Marshal(req.Labels)
	if err != nil {
		return nil, err
	}

	composeProject := req.Labels["com.docker.compose.project"]
	composeDir := req.Labels["com.docker.compose.project.working_dir"]

	m := repository.ContainerMetadata{
		ContainerID:    req.ContainerId,
		NodeName:       req.NodeName,
		Name:           req.Name,
		ImageTag:       req.Image,
		EnvsJSON:       envsJSON,
		MountsJSON:     mountsJSON,
		LabelsJSON:     labelsJSON,
		ComposeProject: composeProject,
		ComposeDir:     composeDir,
	}

	if err := s.containers.UpsertMetadata(ctx, m); err != nil {
		slog.Error("failed to upsert metadata", "container_id", req.ContainerId, "error", err)
		return nil, err
	}

	slog.Debug("metadata synced", "container_id", req.ContainerId, "node", req.NodeName)
	return &monitorv1.SyncMetadataResponse{}, nil
}

func (s *MonitoringService) GetPendingCommands(ctx context.Context, req *monitorv1.GetPendingCommandsRequest) (*monitorv1.GetPendingCommandsResponse, error) {
	pending, err := s.actions.ClaimPendingCommands(ctx, req.NodeName)
	if err != nil {
		slog.Error("failed to fetch pending commands", "node", req.NodeName, "error", err)
		return nil, err
	}

	commands := make([]*monitorv1.Command, len(pending))
	for i, cmd := range pending {
		commands[i] = &monitorv1.Command{
			CommandId: cmd.CommandID,
			Action:    cmd.Action,
			Target:    cmd.Target,
			Params:    cmd.Params,
		}
	}

	if len(commands) > 0 {
		slog.Info("dispatched commands", "node", req.NodeName, "count", len(commands))
	}
	return &monitorv1.GetPendingCommandsResponse{Commands: commands}, nil
}

func (s *MonitoringService) ReportCommandResult(ctx context.Context, req *monitorv1.ReportCommandResultRequest) (*monitorv1.ReportCommandResultResponse, error) {
	status := "failed"
	if req.Success {
		status = "success"
	}

	if err := s.actions.UpdateCommandResult(ctx, req.CommandId, status, req.Output, req.DurationMs); err != nil {
		slog.Error("failed to update command result", "command_id", req.CommandId, "error", err)
		return nil, err
	}

	slog.Info("command result recorded", "command_id", req.CommandId, "status", status)
	return &monitorv1.ReportCommandResultResponse{}, nil
}

func (s *MonitoringService) ReportRemovedContainers(ctx context.Context, req *monitorv1.ReportRemovedContainersRequest) (*monitorv1.ReportRemovedContainersResponse, error) {
	if len(req.ContainerIds) == 0 {
		return &monitorv1.ReportRemovedContainersResponse{RemovedCount: 0}, nil
	}

	// Query container metadata before marking removed, for webhook notifications.
	var removedContainers []repository.ContainerInfo
	if s.notifier != nil {
		infos, err := s.containers.GetContainerInfoForRemoval(ctx, req.NodeName, req.ContainerIds)
		if err != nil {
			slog.Error("failed to query containers for webhook", "error", err)
		} else {
			removedContainers = infos
		}
	}

	count, err := s.containers.MarkRemoved(ctx, req.NodeName, req.ContainerIds)
	if err != nil {
		slog.Error("failed to mark containers removed", "node", req.NodeName, "error", err)
		return nil, err
	}

	if count > 0 {
		slog.Info("containers marked removed", "node", req.NodeName, "count", count)
	}

	// Fire webhook events for removed containers.
	for _, ci := range removedContainers {
		s.notifier.Send(alerts.Event{
			EventType:      alerts.EventContainerRemoved,
			ContainerID:    ci.ContainerID,
			ContainerName:  ci.Name,
			NodeName:       req.NodeName,
			Image:          ci.ImageTag,
			ComposeProject: ci.ComposeProject,
		})
	}

	return &monitorv1.ReportRemovedContainersResponse{RemovedCount: int32(count)}, nil
}

func (s *MonitoringService) buildTransitionEvent(ctx context.Context, containerID, prevStatus, newStatus string) alerts.Event {
	eventType := alerts.EventContainerStarted
	if newStatus == "exited" {
		eventType = alerts.EventContainerDied
	}

	event := alerts.Event{
		EventType:      eventType,
		ContainerID:    containerID,
		PreviousStatus: prevStatus,
		CurrentStatus:  newStatus,
	}

	// Enrich with container metadata.
	ci, nodeName, err := s.containers.GetContainerMetadataForEvent(ctx, containerID)
	if err != nil {
		slog.Warn("webhook: could not enrich event with metadata", "container_id", containerID, "error", err)
	} else {
		event.ContainerName = ci.Name
		event.NodeName = nodeName
		event.Image = ci.ImageTag
		event.ComposeProject = ci.ComposeProject
	}

	return event
}

func (s *MonitoringService) AgentHeartbeat(ctx context.Context, req *monitorv1.AgentHeartbeatRequest) (*monitorv1.AgentHeartbeatResponse, error) {
	if err := s.agents.UpsertAgent(ctx, req.NodeName, req.AgentVersion); err != nil {
		slog.Error("failed to upsert agent heartbeat", "node", req.NodeName, "error", err)
		return nil, err
	}

	slog.Debug("agent heartbeat recorded", "node", req.NodeName, "version", req.AgentVersion)
	return &monitorv1.AgentHeartbeatResponse{}, nil
}
