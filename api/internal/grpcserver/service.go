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

	_, err = s.pool.Exec(ctx,
		`INSERT INTO containers (container_id, node_name, name, image_tag, env_vars, mounts, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (container_id) DO UPDATE SET
		   node_name  = EXCLUDED.node_name,
		   name       = EXCLUDED.name,
		   image_tag  = EXCLUDED.image_tag,
		   env_vars   = EXCLUDED.env_vars,
		   mounts     = EXCLUDED.mounts,
		   updated_at = EXCLUDED.updated_at`,
		req.ContainerId, req.NodeName, req.Name, req.Image,
		envsJSON, mountsJSON, time.Now(),
	)
	if err != nil {
		slog.Error("failed to upsert metadata", "container_id", req.ContainerId, "error", err)
		return nil, err
	}

	slog.Debug("metadata synced", "container_id", req.ContainerId, "node", req.NodeName)
	return &monitorv1.SyncMetadataResponse{}, nil
}
