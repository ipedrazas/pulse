package integration_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/ipedrazas/pulse/api/internal/alerts"
	"github.com/ipedrazas/pulse/api/internal/db"
	"github.com/ipedrazas/pulse/api/internal/grpcserver"
	"github.com/ipedrazas/pulse/api/internal/repository"
	pulsev1 "github.com/ipedrazas/pulse/proto/gen/pulse/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func setupGRPCServer(t *testing.T) (pulsev1.AgentServiceClient, pulsev1.CLIServiceClient, func()) {
	t.Helper()
	ctx := context.Background()

	// Start TimescaleDB
	pgContainer, err := postgres.Run(ctx,
		"timescale/timescaledb:latest-pg17",
		postgres.WithDatabase("pulse_test"),
		postgres.WithUsername("pulse"),
		postgres.WithPassword("pulse"),
		testcontainers.WithWaitStrategy(
			wait.ForListeningPort("5432/tcp").WithStartupTimeout(60*time.Second),
		),
	)
	require.NoError(t, err)

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	err = db.RunMigrations(connStr)
	require.NoError(t, err)

	pool, err := db.NewPool(ctx, connStr, nil)
	require.NoError(t, err)

	repo := repository.NewPostgresRepository(pool)
	notifier := alerts.NewNotifier("") // no webhook in tests

	// Start gRPC server
	agentSvc := grpcserver.NewAgentService(repo, notifier)
	cliSvc := grpcserver.NewCLIService(repo, agentSvc)

	grpcSrv := grpc.NewServer()
	pulsev1.RegisterAgentServiceServer(grpcSrv, agentSvc)
	pulsev1.RegisterCLIServiceServer(grpcSrv, cliSvc)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	go func() { _ = grpcSrv.Serve(lis) }()

	// Connect client
	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	agentClient := pulsev1.NewAgentServiceClient(conn)
	cliClient := pulsev1.NewCLIServiceClient(conn)

	cleanup := func() {
		conn.Close()
		grpcSrv.GracefulStop()
		pool.Close()
		_ = pgContainer.Terminate(ctx)
	}

	return agentClient, cliClient, cleanup
}

func TestStreamLinkHeartbeatRegistersAgent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	agentClient, cliClient, cleanup := setupGRPCServer(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Open stream and send heartbeat
	stream, err := agentClient.StreamLink(ctx)
	require.NoError(t, err)

	err = stream.Send(&pulsev1.AgentMessage{
		Payload: &pulsev1.AgentMessage_Heartbeat{
			Heartbeat: &pulsev1.Heartbeat{
				NodeName:     "test-node",
				AgentVersion: "0.1.0",
				Timestamp:    timestamppb.Now(),
			},
		},
	})
	require.NoError(t, err)

	// Give the server a moment to process
	time.Sleep(200 * time.Millisecond)

	// Verify agent appears via CLI service
	resp, err := cliClient.ListNodes(ctx, &pulsev1.ListNodesRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Nodes, 1)
	assert.Equal(t, "test-node", resp.Nodes[0].Name)
	assert.Equal(t, "online", resp.Nodes[0].Status)
	assert.Equal(t, "0.1.0", resp.Nodes[0].AgentVersion)
}

func TestStreamLinkContainerReport(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	agentClient, cliClient, cleanup := setupGRPCServer(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stream, err := agentClient.StreamLink(ctx)
	require.NoError(t, err)

	// Send heartbeat first to register agent
	err = stream.Send(&pulsev1.AgentMessage{
		Payload: &pulsev1.AgentMessage_Heartbeat{
			Heartbeat: &pulsev1.Heartbeat{
				NodeName:     "test-node",
				AgentVersion: "0.1.0",
				Timestamp:    timestamppb.Now(),
			},
		},
	})
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	// Send container report
	err = stream.Send(&pulsev1.AgentMessage{
		Payload: &pulsev1.AgentMessage_ContainerReport{
			ContainerReport: &pulsev1.ContainerReport{
				NodeName: "test-node",
				Containers: []*pulsev1.ContainerInfo{
					{
						Id:            "c1",
						Name:          "web",
						Image:         "nginx:latest",
						Status:        "running",
						EnvVars:       map[string]string{"ENV": "prod"},
						Mounts:        []string{"/data:/data"},
						Labels:        map[string]string{"app": "web"},
						UptimeSeconds: 3600,
					},
					{
						Id:            "c2",
						Name:          "db",
						Image:         "postgres:17",
						Status:        "running",
						EnvVars:       map[string]string{},
						Labels:        map[string]string{},
						UptimeSeconds: 7200,
					},
				},
			},
		},
	})
	require.NoError(t, err)
	time.Sleep(200 * time.Millisecond)

	// Verify containers via CLI service
	resp, err := cliClient.ListContainers(ctx, &pulsev1.ListContainersRequest{
		NodeName: "test-node",
		PageSize: 50,
	})
	require.NoError(t, err)
	assert.Len(t, resp.Containers, 2)

	// Verify node has container count
	nodeResp, err := cliClient.GetNode(ctx, &pulsev1.GetNodeRequest{Name: "test-node"})
	require.NoError(t, err)
	assert.Equal(t, int32(2), nodeResp.Node.ContainerCount)
	assert.Len(t, nodeResp.Containers, 2)
}

func TestSendCommandQueuesForAgent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	agentClient, cliClient, cleanup := setupGRPCServer(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Connect agent
	stream, err := agentClient.StreamLink(ctx)
	require.NoError(t, err)

	err = stream.Send(&pulsev1.AgentMessage{
		Payload: &pulsev1.AgentMessage_Heartbeat{
			Heartbeat: &pulsev1.Heartbeat{
				NodeName:     "test-node",
				AgentVersion: "0.1.0",
				Timestamp:    timestamppb.Now(),
			},
		},
	})
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	// Send command via CLI service
	cmdResp, err := cliClient.SendCommand(ctx, &pulsev1.SendCommandRequest{
		NodeName: "test-node",
		Command: &pulsev1.SendCommandRequest_PullImage{
			PullImage: &pulsev1.PullImage{Image: "nginx:latest"},
		},
	})
	require.NoError(t, err)
	assert.True(t, cmdResp.Accepted)
	assert.NotEmpty(t, cmdResp.CommandId)

	// Agent should receive the command
	cmd, err := stream.Recv()
	require.NoError(t, err)
	assert.Equal(t, cmdResp.CommandId, cmd.CommandId)
	pullCmd := cmd.GetPullImage()
	require.NotNil(t, pullCmd)
	assert.Equal(t, "nginx:latest", pullCmd.Image)
}

func TestGetContainerNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, cliClient, cleanup := setupGRPCServer(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := cliClient.GetContainer(ctx, &pulsev1.GetContainerRequest{ContainerId: "nonexistent"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
