package grpcserver

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	monitorv1 "github.com/ipedrazas/pulse/proto/monitor/v1"
)

type MonitoringService struct {
	monitorv1.UnimplementedMonitoringServiceServer
	pool *pgxpool.Pool
}

func NewMonitoringService(pool *pgxpool.Pool) *MonitoringService {
	return &MonitoringService{pool: pool}
}

func (s *MonitoringService) ReportHeartbeat(ctx context.Context, req *monitorv1.ReportHeartbeatRequest) (*monitorv1.ReportHeartbeatResponse, error) {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO container_heartbeats (time, container_id, status, uptime_seconds)
		 VALUES ($1, $2, $3, $4)`,
		time.Now(), req.ContainerId, req.Status, req.UptimeSeconds,
	)
	if err != nil {
		slog.Error("failed to insert heartbeat", "container_id", req.ContainerId, "error", err)
		return nil, err
	}

	slog.Debug("heartbeat recorded", "container_id", req.ContainerId, "status", req.Status)
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
	return &monitorv1.ReportRemovedContainersResponse{RemovedCount: count}, nil
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
