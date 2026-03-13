package grpcserver

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/ipedrazas/pulse/api/internal/alerts"
	"github.com/ipedrazas/pulse/api/internal/constants"
	"github.com/ipedrazas/pulse/api/internal/metrics"
	"github.com/ipedrazas/pulse/api/internal/repository"
	pulsev1 "github.com/ipedrazas/pulse/proto/gen/pulse/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// AgentService implements the bidirectional streaming AgentService.
type AgentService struct {
	pulsev1.UnimplementedAgentServiceServer
	repo     repository.Repository
	streams  *StreamRegistry
	notifier alerts.Notifier
}

func NewAgentService(repo repository.Repository, notifier alerts.Notifier) *AgentService {
	return &AgentService{
		repo:     repo,
		streams:  NewStreamRegistry(),
		notifier: notifier,
	}
}

// StreamLink handles the long-lived bidirectional stream from an agent.
func (s *AgentService) StreamLink(stream pulsev1.AgentService_StreamLinkServer) error {
	// Send response headers immediately so tonic (Rust) clients don't block
	// waiting for headers before they can start sending messages.
	if err := stream.SendHeader(metadata.MD{}); err != nil {
		return status.Errorf(codes.Internal, "send header: %v", err)
	}

	ctx := stream.Context()
	var nodeName string
	firstHeartbeat := true

	for {
		select {
		case <-ctx.Done():
			if nodeName != "" {
				s.disconnectAgent(ctx, nodeName)
			}
			return ctx.Err()
		default:
		}

		msg, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			if nodeName != "" {
				slog.Warn("stream error", "node", nodeName, "error", err)
				s.disconnectAgent(ctx, nodeName)
			}
			return status.Errorf(codes.Internal, "recv: %v", err)
		}

		switch payload := msg.Payload.(type) {
		case *pulsev1.AgentMessage_Heartbeat:
			nodeName = s.handleHeartbeat(ctx, stream, payload.Heartbeat, firstHeartbeat)
			firstHeartbeat = false

		case *pulsev1.AgentMessage_ContainerReport:
			nodeName = s.handleContainerReport(ctx, payload.ContainerReport)

		case *pulsev1.AgentMessage_CommandResult:
			s.handleCommandResult(ctx, payload.CommandResult)
		}
	}

	return nil
}

func (s *AgentService) disconnectAgent(ctx context.Context, nodeName string) {
	slog.Info("agent disconnected", "node", nodeName)
	_ = s.repo.SetAgentStatus(ctx, nodeName, constants.AgentOffline)
	s.streams.Remove(nodeName)
	metrics.ConnectedAgents.Dec()
	s.notifier.AgentOffline(nodeName)
}

func (s *AgentService) handleHeartbeat(
	ctx context.Context,
	stream pulsev1.AgentService_StreamLinkServer,
	hb *pulsev1.Heartbeat,
	firstHeartbeat bool,
) string {
	nodeName := hb.NodeName
	now := time.Now()
	agent := repository.Agent{
		Name:     nodeName,
		Status:   constants.AgentOnline,
		Version:  hb.AgentVersion,
		LastSeen: &now,
	}
	if hb.Metadata != nil {
		agent.Metadata = &repository.NodeMetadata{
			Hostname:         hb.Metadata.Hostname,
			IPAddress:        hb.Metadata.IpAddress,
			OSName:           hb.Metadata.OsName,
			OSVersion:        hb.Metadata.OsVersion,
			KernelVersion:    hb.Metadata.KernelVersion,
			UptimeSeconds:    hb.Metadata.UptimeSeconds,
			PackagesToUpdate: hb.Metadata.PackagesToUpdate,
		}
	}
	if err := s.repo.UpsertAgent(ctx, agent); err != nil {
		slog.Error("upsert agent failed", "node", nodeName, "error", err)
	}
	s.streams.Set(nodeName, stream)
	if firstHeartbeat {
		slog.Info("agent connected", "node", nodeName)
		metrics.ConnectedAgents.Inc()
		s.notifier.AgentOnline(nodeName)
	}
	s.flushPendingCommands(stream, nodeName)
	return nodeName
}

func (s *AgentService) flushPendingCommands(stream pulsev1.AgentService_StreamLinkServer, nodeName string) {
	cmds, err := s.repo.GetPendingCommands(stream.Context(), nodeName)
	if err != nil {
		slog.Error("get pending commands failed", "node", nodeName, "error", err)
		return
	}
	for _, cmd := range cmds {
		serverCmd, err := commandToProto(cmd)
		if err != nil {
			slog.Error("convert command failed", "id", cmd.ID, "error", err)
			continue
		}
		if err := stream.Send(serverCmd); err != nil {
			slog.Error("send command failed", "id", cmd.ID, "error", err)
		}
	}
}

func (s *AgentService) handleContainerReport(ctx context.Context, report *pulsev1.ContainerReport) string {
	nodeName := report.NodeName

	err := s.repo.WithTx(ctx, func(tx repository.Repository) error {
		var activeIDs []string
		for _, ci := range report.Containers {
			activeIDs = append(activeIDs, ci.Id)
			c := protoToContainer(ci, nodeName)
			if err := tx.UpsertContainer(ctx, c); err != nil {
				return err
			}
			event := repository.ContainerEvent{
				Time:          time.Now(),
				ContainerID:   ci.Id,
				AgentName:     nodeName,
				Status:        ci.Status,
				UptimeSeconds: ci.UptimeSeconds,
			}
			if err := tx.InsertContainerEvent(ctx, event); err != nil {
				return err
			}
		}
		return tx.MarkContainersRemoved(ctx, nodeName, activeIDs)
	})
	if err != nil {
		slog.Error("handle container report failed", "node", nodeName, "error", err)
	} else {
		metrics.ContainersTracked.Set(float64(len(report.Containers)))
	}
	return nodeName
}

func (s *AgentService) handleCommandResult(ctx context.Context, result *pulsev1.CommandResult) {
	output := result.Output
	if result.Error != "" {
		output = result.Error
	}
	cmdStatus := constants.StatusFailed
	if result.Success {
		cmdStatus = constants.StatusCompleted
	}
	slog.Debug("command result received", "command_id", result.CommandId, "node", result.NodeName, "success", result.Success)
	metrics.CommandsTotal.WithLabelValues("result", cmdStatus).Inc()
	if err := s.repo.CompleteCommand(ctx, result.CommandId, output, result.Success); err != nil {
		slog.Error("complete command failed", "id", result.CommandId, "error", err)
	}
}

// SendToAgent sends a command to a connected agent via its stream.
func (s *AgentService) SendToAgent(nodeName string, cmd *pulsev1.ServerCommand) error {
	stream, ok := s.streams.Get(nodeName)
	if !ok {
		return status.Errorf(codes.NotFound, "agent %q not connected", nodeName)
	}
	return stream.Send(cmd)
}

func protoToContainer(ci *pulsev1.ContainerInfo, agentName string) repository.Container {
	envVars := ci.EnvVars
	if envVars == nil {
		envVars = map[string]string{}
	}
	labels := ci.Labels
	if labels == nil {
		labels = map[string]string{}
	}
	var ports []repository.Port
	for _, p := range ci.Ports {
		ports = append(ports, repository.Port{
			HostIP:        p.HostIp,
			HostPort:      p.HostPort,
			ContainerPort: p.ContainerPort,
			Protocol:      p.Protocol,
		})
	}
	return repository.Container{
		ContainerID:    ci.Id,
		AgentName:      agentName,
		Name:           ci.Name,
		Image:          ci.Image,
		Status:         ci.Status,
		EnvVars:        envVars,
		Mounts:         ci.Mounts,
		Labels:         labels,
		Ports:          ports,
		ComposeProject: ci.ComposeProject,
		Command:        ci.Command,
		UptimeSeconds:  ci.UptimeSeconds,
	}
}

func commandToProto(cmd repository.Command) (*pulsev1.ServerCommand, error) {
	sc := &pulsev1.ServerCommand{CommandId: cmd.ID}
	switch cmd.Type {
	case constants.CmdRunContainer:
		var rc pulsev1.RunContainer
		if err := json.Unmarshal(cmd.Payload, &rc); err != nil {
			return nil, err
		}
		sc.Payload = &pulsev1.ServerCommand_RunContainer{RunContainer: &rc}
	case constants.CmdStopContainer:
		var sc2 pulsev1.StopContainer
		if err := json.Unmarshal(cmd.Payload, &sc2); err != nil {
			return nil, err
		}
		sc.Payload = &pulsev1.ServerCommand_StopContainer{StopContainer: &sc2}
	case constants.CmdPullImage:
		var pi pulsev1.PullImage
		if err := json.Unmarshal(cmd.Payload, &pi); err != nil {
			return nil, err
		}
		sc.Payload = &pulsev1.ServerCommand_PullImage{PullImage: &pi}
	case constants.CmdComposeUp:
		var cu pulsev1.ComposeUp
		if err := json.Unmarshal(cmd.Payload, &cu); err != nil {
			return nil, err
		}
		sc.Payload = &pulsev1.ServerCommand_ComposeUp{ComposeUp: &cu}
	case constants.CmdRequestLogs:
		var rl pulsev1.RequestLogs
		if err := json.Unmarshal(cmd.Payload, &rl); err != nil {
			return nil, err
		}
		sc.Payload = &pulsev1.ServerCommand_RequestLogs{RequestLogs: &rl}
	case constants.CmdRestartContainer:
		var rc pulsev1.RestartContainer
		if err := json.Unmarshal(cmd.Payload, &rc); err != nil {
			return nil, err
		}
		sc.Payload = &pulsev1.ServerCommand_RestartContainer{RestartContainer: &rc}
	}
	return sc, nil
}

// SendCommand sends a command to a connected agent, used by the REST handler.
func (s *AgentService) SendCommand(nodeName string, cmdID string, cmdType string, payload json.RawMessage) error {
	cmd := repository.Command{
		ID:      cmdID,
		Type:    cmdType,
		Payload: payload,
	}
	serverCmd, err := commandToProto(cmd)
	if err != nil {
		return err
	}
	return s.SendToAgent(nodeName, serverCmd)
}

// StreamRegistry tracks active agent streams by node name.
type StreamRegistry struct {
	mu      sync.RWMutex
	streams map[string]pulsev1.AgentService_StreamLinkServer
}

func NewStreamRegistry() *StreamRegistry {
	return &StreamRegistry{streams: make(map[string]pulsev1.AgentService_StreamLinkServer)}
}

func (r *StreamRegistry) Set(name string, stream pulsev1.AgentService_StreamLinkServer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.streams[name] = stream
}

func (r *StreamRegistry) Get(name string) (pulsev1.AgentService_StreamLinkServer, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.streams[name]
	return s, ok
}

func (r *StreamRegistry) Remove(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.streams, name)
}
