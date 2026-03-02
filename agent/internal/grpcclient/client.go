package grpcclient

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
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
// If tlsCAFile is non-empty, the connection uses TLS with the given CA certificate;
// otherwise it falls back to an insecure (plaintext) connection.
func New(serverAddr, token, tlsCAFile string) (*Client, error) {
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

	transportCreds, err := buildTransportCredentials(tlsCAFile)
	if err != nil {
		return nil, err
	}

	conn, err := grpc.NewClient(serverAddr,
		grpc.WithTransportCredentials(transportCreds),
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

// buildTransportCredentials returns TLS credentials if a CA file is provided,
// or insecure credentials otherwise.
func buildTransportCredentials(tlsCAFile string) (credentials.TransportCredentials, error) {
	if tlsCAFile == "" {
		return insecure.NewCredentials(), nil
	}

	caCert, err := os.ReadFile(tlsCAFile)
	if err != nil {
		return nil, fmt.Errorf("read CA file: %w", err)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to parse CA certificate from %s", tlsCAFile)
	}

	return credentials.NewTLS(&tls.Config{
		RootCAs: pool,
	}), nil
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

// GetPendingCommands polls the hub for commands awaiting execution on this node.
func (c *Client) GetPendingCommands(ctx context.Context, nodeName string) ([]*monitorv1.Command, error) {
	ctx = c.withToken(ctx)
	resp, err := c.service.GetPendingCommands(ctx, &monitorv1.GetPendingCommandsRequest{
		NodeName: nodeName,
	})
	if err != nil {
		return nil, err
	}
	return resp.Commands, nil
}

// ReportCommandResult sends the outcome of a command execution back to the hub.
func (c *Client) ReportCommandResult(ctx context.Context, req *monitorv1.ReportCommandResultRequest) error {
	ctx = c.withToken(ctx)
	_, err := c.service.ReportCommandResult(ctx, req)
	return err
}

// ReportRemovedContainers tells the hub that the given containers no longer exist on the node.
func (c *Client) ReportRemovedContainers(ctx context.Context, nodeName string, containerIDs []string) error {
	ctx = c.withToken(ctx)
	_, err := c.service.ReportRemovedContainers(ctx, &monitorv1.ReportRemovedContainersRequest{
		NodeName:     nodeName,
		ContainerIds: containerIDs,
	})
	return err
}

// AgentHeartbeat sends an agent-level heartbeat to the hub.
func (c *Client) AgentHeartbeat(ctx context.Context, nodeName, agentVersion string) error {
	ctx = c.withToken(ctx)
	_, err := c.service.AgentHeartbeat(ctx, &monitorv1.AgentHeartbeatRequest{
		NodeName:     nodeName,
		AgentVersion: agentVersion,
	})
	return err
}

func (c *Client) withToken(ctx context.Context) context.Context {
	return metadata.AppendToOutgoingContext(ctx, "x-monitor-token", c.token)
}
