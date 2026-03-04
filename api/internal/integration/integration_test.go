package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/ipedrazas/pulse/api/internal/db"
	grpcserver "github.com/ipedrazas/pulse/api/internal/grpcserver"
	"github.com/ipedrazas/pulse/api/internal/repository"
	"github.com/ipedrazas/pulse/api/internal/rest"
	monitorv1 "github.com/ipedrazas/pulse/proto/monitor/v1"
)

const testToken = "integration-test-token"

// testEnv holds shared resources for the test suite.
type testEnv struct {
	pool       *pgxpool.Pool
	grpcAddr   string
	httpAddr   string
	grpcServer *grpc.Server
	httpServer *http.Server
}

func setupTimescaleDB(t *testing.T, ctx context.Context) string {
	t.Helper()

	pgContainer, err := postgres.Run(ctx,
		"timescale/timescaledb:latest-pg17",
		postgres.WithDatabase("pulse_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("failed to start timescaledb: %v", err)
	}
	t.Cleanup(func() {
		pgContainer.Terminate(ctx)
	})

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}
	return connStr
}

func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()
	ctx := context.Background()

	// Start TimescaleDB
	dbURL := setupTimescaleDB(t, ctx)

	// Run migrations
	if err := db.RunMigrations(dbURL); err != nil {
		t.Fatalf("migrations failed: %v", err)
	}

	// Create pool
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	repo := repository.NewPostgresRepo(pool)

	// Start gRPC server
	grpcLis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	grpcSrv := grpc.NewServer(
		grpc.UnaryInterceptor(grpcserver.TokenAuthInterceptor(testToken)),
	)
	monitorv1.RegisterMonitoringServiceServer(grpcSrv, grpcserver.NewMonitoringService(repo, repo, repo, nil))
	go grpcSrv.Serve(grpcLis)
	t.Cleanup(func() { grpcSrv.GracefulStop() })

	// Start HTTP server
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := rest.NewHandler(repo, testToken)
	handler.RegisterRoutes(router)

	httpLis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	httpSrv := &http.Server{Handler: router}
	go httpSrv.Serve(httpLis)
	t.Cleanup(func() { httpSrv.Close() })

	return &testEnv{
		pool:       pool,
		grpcAddr:   grpcLis.Addr().String(),
		httpAddr:   httpLis.Addr().String(),
		grpcServer: grpcSrv,
		httpServer: httpSrv,
	}
}

func grpcClient(t *testing.T, addr string) monitorv1.MonitoringServiceClient {
	t.Helper()
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return monitorv1.NewMonitoringServiceClient(conn)
}

func authedCtx() context.Context {
	return metadata.AppendToOutgoingContext(context.Background(), "x-monitor-token", testToken)
}

// --- Tests ---

func TestFullDataFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	env := setupTestEnv(t)
	client := grpcClient(t, env.grpcAddr)

	// 1. SyncMetadata for two containers on different nodes
	_, err := client.SyncMetadata(authedCtx(), &monitorv1.SyncMetadataRequest{
		ContainerId: "container-aaa",
		NodeName:    "node-1",
		Name:        "nginx-proxy",
		Image:       "nginx:1.25",
		Envs:        map[string]string{"ENV": "prod"},
		Mounts: []*monitorv1.MountInfo{
			{Source: "/data", Destination: "/usr/share/nginx/html", Mode: "ro"},
		},
	})
	if err != nil {
		t.Fatalf("SyncMetadata failed: %v", err)
	}

	_, err = client.SyncMetadata(authedCtx(), &monitorv1.SyncMetadataRequest{
		ContainerId: "container-bbb",
		NodeName:    "node-2",
		Name:        "redis",
		Image:       "redis:7",
		Envs:        map[string]string{},
		Mounts:      nil,
	})
	if err != nil {
		t.Fatalf("SyncMetadata failed: %v", err)
	}

	// 2. ReportHeartbeat for both
	_, err = client.ReportHeartbeat(authedCtx(), &monitorv1.ReportHeartbeatRequest{
		ContainerId: "container-aaa", Status: "running", UptimeSeconds: 3600,
	})
	if err != nil {
		t.Fatalf("ReportHeartbeat failed: %v", err)
	}

	_, err = client.ReportHeartbeat(authedCtx(), &monitorv1.ReportHeartbeatRequest{
		ContainerId: "container-bbb", Status: "running", UptimeSeconds: 1800,
	})
	if err != nil {
		t.Fatalf("ReportHeartbeat failed: %v", err)
	}

	// 3. GET /status — both containers
	resp := httpGet(t, env.httpAddr, "/status")
	var statusList []map[string]any
	jsonUnmarshal(t, resp, &statusList)

	if len(statusList) != 2 {
		t.Fatalf("expected 2 containers in /status, got %d", len(statusList))
	}

	// 4. GET /status/:container — single container
	resp = httpGet(t, env.httpAddr, "/status/container-aaa")
	var single map[string]any
	jsonUnmarshal(t, resp, &single)

	if single["name"] != "nginx-proxy" {
		t.Errorf("expected name=nginx-proxy, got %v", single["name"])
	}
	if single["status"] != "running" {
		t.Errorf("expected status=running, got %v", single["status"])
	}

	// 5. GET /nodes — two nodes
	resp = httpGet(t, env.httpAddr, "/nodes")
	var nodes []map[string]any
	jsonUnmarshal(t, resp, &nodes)

	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}

	// 6. GET /nodes/:node — specific node
	resp = httpGet(t, env.httpAddr, "/nodes/node-1")
	var nodeDetail map[string]any
	jsonUnmarshal(t, resp, &nodeDetail)

	containers := nodeDetail["containers"].([]any)
	if len(containers) != 1 {
		t.Fatalf("expected 1 container on node-1, got %d", len(containers))
	}

	// 7. GET /healthz
	resp = httpGet(t, env.httpAddr, "/healthz")
	var health map[string]any
	jsonUnmarshal(t, resp, &health)
	if health["status"] != "healthy" {
		t.Errorf("expected healthy, got %v", health["status"])
	}
}

func TestMetadataUpsert(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	env := setupTestEnv(t)
	client := grpcClient(t, env.grpcAddr)

	// Insert
	_, err := client.SyncMetadata(authedCtx(), &monitorv1.SyncMetadataRequest{
		ContainerId: "container-upsert",
		NodeName:    "node-1",
		Name:        "app",
		Image:       "app:v1",
		Envs:        map[string]string{"VERSION": "1"},
	})
	if err != nil {
		t.Fatalf("SyncMetadata insert failed: %v", err)
	}

	// Update (same container_id, different image)
	_, err = client.SyncMetadata(authedCtx(), &monitorv1.SyncMetadataRequest{
		ContainerId: "container-upsert",
		NodeName:    "node-1",
		Name:        "app",
		Image:       "app:v2",
		Envs:        map[string]string{"VERSION": "2"},
	})
	if err != nil {
		t.Fatalf("SyncMetadata update failed: %v", err)
	}

	// Verify latest values
	resp := httpGet(t, env.httpAddr, "/status/container-upsert")
	var cs map[string]any
	jsonUnmarshal(t, resp, &cs)

	if cs["image_tag"] != "app:v2" {
		t.Errorf("expected image_tag=app:v2, got %v", cs["image_tag"])
	}
}

func TestAuthRejectsInvalidToken(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	env := setupTestEnv(t)
	client := grpcClient(t, env.grpcAddr)

	// No token
	_, err := client.ReportHeartbeat(context.Background(), &monitorv1.ReportHeartbeatRequest{
		ContainerId: "x", Status: "running", UptimeSeconds: 0,
	})
	if s, ok := status.FromError(err); !ok || s.Code() != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}

	// Wrong token
	badCtx := metadata.AppendToOutgoingContext(context.Background(), "x-monitor-token", "wrong")
	_, err = client.ReportHeartbeat(badCtx, &monitorv1.ReportHeartbeatRequest{
		ContainerId: "x", Status: "running", UptimeSeconds: 0,
	})
	if s, ok := status.FromError(err); !ok || s.Code() != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}
}

func TestNotFoundResponses(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	env := setupTestEnv(t)

	// Non-existent container
	code := httpGetCode(t, env.httpAddr, "/status/does-not-exist")
	if code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown container, got %d", code)
	}

	// Non-existent node
	code = httpGetCode(t, env.httpAddr, "/nodes/ghost-node")
	if code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown node, got %d", code)
	}
}

// --- Helpers ---

func httpGet(t *testing.T, addr, path string) []byte {
	t.Helper()
	req, err := http.NewRequest("GET", fmt.Sprintf("http://%s%s", addr, path), nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+testToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s failed: %v", path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	return body
}

func httpGetCode(t *testing.T, addr, path string) int {
	t.Helper()
	req, err := http.NewRequest("GET", fmt.Sprintf("http://%s%s", addr, path), nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+testToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s failed: %v", path, err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

func jsonUnmarshal(t *testing.T, data []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("json unmarshal failed: %v\nbody: %s", err, string(data))
	}
}
