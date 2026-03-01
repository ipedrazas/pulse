package grpcclient

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"

	monitorv1 "github.com/ipedrazas/pulse/proto/monitor/v1"
)

// Client wraps the MonitoringService gRPC client with token injection.
type Client struct {
	conn    *grpc.ClientConn
	service monitorv1.MonitoringServiceClient
	token   string
}

// New creates a gRPC client with keepalive and retry policy.
func New(serverAddr, token string) (*Client, error) {
	kacp := keepalive.ClientParameters{
		Time:                10 * time.Second,
		Timeout:             time.Second,
		PermitWithoutStream: true,
	}

	retryPolicy := `{
		"methodConfig": [{
			"name": [{"service": "monitor.v1.MonitoringService"}],
			"retryPolicy": {
				"maxAttempts": 5,
				"initialBackoff": "0.5s",
				"maxBackoff": "30s",
				"backoffMultiplier": 2,
				"retryableStatusCodes": ["UNAVAILABLE", "DEADLINE_EXCEEDED"]
			}
		}]
	}`

	conn, err := grpc.NewClient(serverAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(kacp),
		grpc.WithDefaultServiceConfig(retryPolicy),
	)
	if err != nil {
		return nil, err
	}

	return &Client{
		conn:    conn,
		service: monitorv1.NewMonitoringServiceClient(conn),
		token:   token,
	}, nil
}

// Close closes the underlying gRPC connection.
func (c *Client) Close() error {
	return c.conn.Close()
}

// ReportHeartbeat sends a heartbeat for a container.
func (c *Client) ReportHeartbeat(ctx context.Context, containerID, status string, uptimeSeconds int64) error {
	ctx = c.withToken(ctx)
	_, err := c.service.ReportHeartbeat(ctx, &monitorv1.ReportHeartbeatRequest{
		ContainerId:   containerID,
		Status:        status,
		UptimeSeconds: uptimeSeconds,
	})
	return err
}

// SyncMetadata sends container metadata to the hub.
func (c *Client) SyncMetadata(ctx context.Context, req *monitorv1.SyncMetadataRequest) error {
	ctx = c.withToken(ctx)
	_, err := c.service.SyncMetadata(ctx, req)
	return err
}

func (c *Client) withToken(ctx context.Context) context.Context {
	return metadata.AppendToOutgoingContext(ctx, "x-monitor-token", c.token)
}
