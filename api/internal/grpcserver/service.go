package grpcserver

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ipedrazas/pulse/api/internal/alerts"
	monitorv1 "github.com/ipedrazas/pulse/proto/monitor/v1"
)

type MonitoringService struct {
	monitorv1.UnimplementedMonitoringServiceServer
	pool     *pgxpool.Pool
	notifier *alerts.Notifier
}

func NewMonitoringService(pool *pgxpool.Pool, notifier *alerts.Notifier) *MonitoringService {
	return &MonitoringService{pool: pool, notifier: notifier}
}

func (s *MonitoringService) ReportHeartbeat(ctx context.Context, req *monitorv1.ReportHeartbeatRequest) (*monitorv1.ReportHeartbeatResponse, error) {
	// Query previous status before inserting the new heartbeat.
	var prevStatus string
	err := s.pool.QueryRow(ctx,
		`SELECT status FROM container_heartbeats
		 WHERE container_id = $1
		 ORDER BY time DESC LIMIT 1`,
		req.ContainerId,
	).Scan(&prevStatus)
	if err != nil && err != pgx.ErrNoRows {
		slog.Error("failed to query previous status", "container_id", req.ContainerId, "error", err)
	}

	_, err = s.pool.Exec(ctx,
		`INSERT INTO container_heartbeats (time, container_id, status, uptime_seconds)
		 VALUES ($1, $2, $3, $4)`,
		time.Now(), req.ContainerId, req.Status, req.UptimeSeconds,
	)
	if err != nil {
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

	_, err = s.pool.Exec(ctx,
		`INSERT INTO containers (container_id, node_name, name, image_tag, env_vars, mounts, labels, compose_project, compose_dir, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 ON CONFLICT (container_id) DO UPDATE SET
		   node_name        = EXCLUDED.node_name,
		   name             = EXCLUDED.name,
		   image_tag        = EXCLUDED.image_tag,
		   env_vars         = EXCLUDED.env_vars,
		   mounts           = EXCLUDED.mounts,
		   labels           = EXCLUDED.labels,
		   compose_project  = EXCLUDED.compose_project,
		   compose_dir      = EXCLUDED.compose_dir,
		   updated_at       = EXCLUDED.updated_at,
		   removed_at       = NULL`,
		req.ContainerId, req.NodeName, req.Name, req.Image,
		envsJSON, mountsJSON, labelsJSON, composeProject, composeDir, time.Now(),
	)
	if err != nil {
		slog.Error("failed to upsert metadata", "container_id", req.ContainerId, "error", err)
		return nil, err
	}

	slog.Debug("metadata synced", "container_id", req.ContainerId, "node", req.NodeName)
	return &monitorv1.SyncMetadataResponse{}, nil
}

func (s *MonitoringService) GetPendingCommands(ctx context.Context, req *monitorv1.GetPendingCommandsRequest) (*monitorv1.GetPendingCommandsResponse, error) {
	rows, err := s.pool.Query(ctx,
		`UPDATE commands
		 SET status = 'running', updated_at = NOW()
		 WHERE node_name = $1 AND status = 'pending'
		 RETURNING command_id, action, target, params`,
		req.NodeName,
	)
	if err != nil {
		slog.Error("failed to fetch pending commands", "node", req.NodeName, "error", err)
		return nil, err
	}
	defer rows.Close()

	var commands []*monitorv1.Command
	for rows.Next() {
		var cmd monitorv1.Command
		var paramsJSON []byte
		if err := rows.Scan(&cmd.CommandId, &cmd.Action, &cmd.Target, &paramsJSON); err != nil {
			return nil, err
		}
		params := map[string]string{}
		_ = json.Unmarshal(paramsJSON, &params)
		cmd.Params = params
		commands = append(commands, &cmd)
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

	_, err := s.pool.Exec(ctx,
		`UPDATE commands
		 SET status = $1, output = $2, duration_ms = $3, updated_at = NOW()
		 WHERE command_id = $4`,
		status, req.Output, req.DurationMs, req.CommandId,
	)
	if err != nil {
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
	type containerInfo struct {
		containerID    string
		name           string
		image          string
		composeProject string
	}
	var removedContainers []containerInfo
	if s.notifier != nil {
		rows, err := s.pool.Query(ctx,
			`SELECT container_id, name, image_tag, COALESCE(compose_project, '')
			 FROM containers
			 WHERE node_name = $1
			   AND container_id = ANY($2)
			   AND removed_at IS NULL`,
			req.NodeName, req.ContainerIds,
		)
		if err != nil {
			slog.Error("failed to query containers for webhook", "error", err)
		} else {
			defer rows.Close()
			for rows.Next() {
				var ci containerInfo
				if err := rows.Scan(&ci.containerID, &ci.name, &ci.image, &ci.composeProject); err != nil {
					slog.Error("failed to scan container info", "error", err)
					continue
				}
				removedContainers = append(removedContainers, ci)
			}
			rows.Close()
		}
	}

	tag, err := s.pool.Exec(ctx,
		`UPDATE containers
		 SET removed_at = NOW()
		 WHERE node_name = $1
		   AND container_id = ANY($2)
		   AND removed_at IS NULL`,
		req.NodeName, req.ContainerIds,
	)
	if err != nil {
		slog.Error("failed to mark containers removed", "node", req.NodeName, "error", err)
		return nil, err
	}

	count := int32(tag.RowsAffected())
	if count > 0 {
		slog.Info("containers marked removed", "node", req.NodeName, "count", count)
	}

	// Fire webhook events for removed containers.
	for _, ci := range removedContainers {
		s.notifier.Send(alerts.Event{
			EventType:      alerts.EventContainerRemoved,
			ContainerID:    ci.containerID,
			ContainerName:  ci.name,
			NodeName:       req.NodeName,
			Image:          ci.image,
			ComposeProject: ci.composeProject,
		})
	}

	return &monitorv1.ReportRemovedContainersResponse{RemovedCount: count}, nil
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
	var name, nodeName, image, composeProject string
	err := s.pool.QueryRow(ctx,
		`SELECT name, node_name, image_tag, COALESCE(compose_project, '')
		 FROM containers WHERE container_id = $1`,
		containerID,
	).Scan(&name, &nodeName, &image, &composeProject)
	if err != nil {
		slog.Warn("webhook: could not enrich event with metadata", "container_id", containerID, "error", err)
	} else {
		event.ContainerName = name
		event.NodeName = nodeName
		event.Image = image
		event.ComposeProject = composeProject
	}

	return event
}

// SweepStaleContainers marks containers as removed if they haven't had a
// heartbeat in the given duration. Returns the number of containers swept.
func (s *MonitoringService) SweepStaleContainers(ctx context.Context, maxAge time.Duration) (int64, error) {
	tag, err := s.pool.Exec(ctx,
		`UPDATE containers c
		 SET removed_at = NOW()
		 WHERE removed_at IS NULL
		   AND NOT EXISTS (
		     SELECT 1 FROM container_heartbeats h
		     WHERE h.container_id = c.container_id
		       AND h.time > NOW() - $1::interval
		   )`,
		maxAge.String(),
	)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
