package integration

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	grpcserver "github.com/ipedrazas/pulse/api/internal/grpcserver"
	monitorv1 "github.com/ipedrazas/pulse/proto/monitor/v1"
)

func TestGRPCReconnectAfterServerRestart(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	env := setupTestEnv(t)

	// Create client with keepalive (mimics agent config)
	kacp := keepalive.ClientParameters{
		Time:                1 * time.Second, // aggressive for test speed
		Timeout:             500 * time.Millisecond,
		PermitWithoutStream: true,
	}

	conn, err := grpc.NewClient(env.grpcAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(kacp),
	)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	client := monitorv1.NewMonitoringServiceClient(conn)

	// First call should succeed — need metadata in containers table first
	_, err = client.SyncMetadata(authedCtx(), &monitorv1.SyncMetadataRequest{
		ContainerId: "reconnect-test",
		NodeName:    "node-1",
		Name:        "test-container",
		Image:       "test:1",
	})
	if err != nil {
		t.Fatalf("initial SyncMetadata failed: %v", err)
	}

	_, err = client.ReportHeartbeat(authedCtx(), &monitorv1.ReportHeartbeatRequest{
		ContainerId: "reconnect-test", Status: "running", UptimeSeconds: 100,
	})
	if err != nil {
		t.Fatalf("initial ReportHeartbeat failed: %v", err)
	}

	// Stop the gRPC server (simulates VRRP failover)
	env.grpcServer.GracefulStop()

	// Start a NEW gRPC server on the SAME address
	lis, err := net.Listen("tcp", env.grpcAddr)
	if err != nil {
		t.Fatalf("failed to re-listen: %v", err)
	}
	newSrv := grpc.NewServer(
		grpc.UnaryInterceptor(grpcserver.TokenAuthInterceptor(testToken)),
	)
	monitorv1.RegisterMonitoringServiceServer(newSrv, grpcserver.NewMonitoringService(env.pool, nil))
	go newSrv.Serve(lis)
	t.Cleanup(func() { newSrv.GracefulStop() })

	// Wait for client to detect the broken connection and reconnect
	// The client with keepalive should recover within a few seconds
	var lastErr error
	for range 20 {
		time.Sleep(250 * time.Millisecond)
		ctx, cancel := context.WithTimeout(authedCtx(), 2*time.Second)
		_, lastErr = client.ReportHeartbeat(ctx, &monitorv1.ReportHeartbeatRequest{
			ContainerId: "reconnect-test", Status: "running", UptimeSeconds: 200,
		})
		cancel()
		if lastErr == nil {
			return // success — reconnected
		}
	}
	t.Fatalf("client did not reconnect within 5s: %v", lastErr)
}
