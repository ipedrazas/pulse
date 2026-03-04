package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/ipedrazas/pulse/api/internal/db"
)

func setupTestRepo(t *testing.T) *PostgresRepo {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping repository test in short mode")
	}

	ctx := context.Background()
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
	t.Cleanup(func() { pgContainer.Terminate(ctx) })

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}

	if err := db.RunMigrations(connStr); err != nil {
		t.Fatalf("migrations failed: %v", err)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	return NewPostgresRepo(pool)
}

func TestPing(t *testing.T) {
	repo := setupTestRepo(t)
	if err := repo.Ping(context.Background()); err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
}

func TestListContainers_Empty(t *testing.T) {
	repo := setupTestRepo(t)
	containers, err := repo.ListContainers(context.Background(), 0, 0)
	if err != nil {
		t.Fatalf("ListContainers failed: %v", err)
	}
	if len(containers) != 0 {
		t.Errorf("expected 0 containers, got %d", len(containers))
	}
}

func TestUpsertAndGetContainer(t *testing.T) {
	repo := setupTestRepo(t)
	ctx := context.Background()

	envsJSON, _ := json.Marshal(map[string]string{"ENV": "test"})
	mountsJSON, _ := json.Marshal([]struct{}{})
	labelsJSON, _ := json.Marshal(map[string]string{"app": "nginx"})

	m := ContainerMetadata{
		ContainerID:    "test-container-1",
		NodeName:       "node-1",
		Name:           "nginx",
		ImageTag:       "nginx:latest",
		EnvsJSON:       envsJSON,
		MountsJSON:     mountsJSON,
		LabelsJSON:     labelsJSON,
		ComposeProject: "web",
		ComposeDir:     "/opt/web",
	}

	if err := repo.UpsertMetadata(ctx, m); err != nil {
		t.Fatalf("UpsertMetadata failed: %v", err)
	}

	// Insert heartbeat
	if err := repo.InsertHeartbeat(ctx, "test-container-1", "running", 3600); err != nil {
		t.Fatalf("InsertHeartbeat failed: %v", err)
	}

	// Get single container
	cs, err := repo.GetContainer(ctx, "test-container-1")
	if err != nil {
		t.Fatalf("GetContainer failed: %v", err)
	}
	if cs.Name != "nginx" {
		t.Errorf("expected name nginx, got %s", cs.Name)
	}
	if cs.Status == nil || *cs.Status != "running" {
		t.Errorf("expected status running, got %v", cs.Status)
	}

	// List should include it
	containers, err := repo.ListContainers(ctx, 0, 0)
	if err != nil {
		t.Fatalf("ListContainers failed: %v", err)
	}
	if len(containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(containers))
	}
}

func TestGetContainer_NotFound(t *testing.T) {
	repo := setupTestRepo(t)
	_, err := repo.GetContainer(context.Background(), "nonexistent")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestGetComposeDir(t *testing.T) {
	repo := setupTestRepo(t)
	ctx := context.Background()

	envsJSON, _ := json.Marshal(map[string]string{})
	mountsJSON, _ := json.Marshal([]struct{}{})
	labelsJSON, _ := json.Marshal(map[string]string{})

	m := ContainerMetadata{
		ContainerID:    "compose-dir-test",
		NodeName:       "node-1",
		Name:           "app",
		ImageTag:       "app:1",
		EnvsJSON:       envsJSON,
		MountsJSON:     mountsJSON,
		LabelsJSON:     labelsJSON,
		ComposeProject: "myproject",
		ComposeDir:     "/opt/myproject",
	}
	if err := repo.UpsertMetadata(ctx, m); err != nil {
		t.Fatalf("UpsertMetadata failed: %v", err)
	}

	dir, err := repo.GetComposeDir(ctx, "node-1", "myproject")
	if err != nil {
		t.Fatalf("GetComposeDir failed: %v", err)
	}
	if dir != "/opt/myproject" {
		t.Errorf("expected /opt/myproject, got %s", dir)
	}

	// Not found case
	_, err = repo.GetComposeDir(ctx, "node-1", "nonexistent")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestCreateAndGetAction(t *testing.T) {
	repo := setupTestRepo(t)
	ctx := context.Background()

	paramsJSON, _ := json.Marshal(map[string]string{"key": "val"})

	ar, err := repo.CreateAction(ctx, "node-1", "compose_restart", "mystack", paramsJSON)
	if err != nil {
		t.Fatalf("CreateAction failed: %v", err)
	}
	if ar.CommandID == "" {
		t.Fatal("expected non-empty CommandID")
	}
	if ar.Status != "pending" {
		t.Errorf("expected status pending, got %s", ar.Status)
	}

	// Get it back
	fetched, err := repo.GetAction(ctx, ar.CommandID, "node-1")
	if err != nil {
		t.Fatalf("GetAction failed: %v", err)
	}
	if fetched.Action != "compose_restart" {
		t.Errorf("expected action compose_restart, got %s", fetched.Action)
	}

	// Not found
	_, err = repo.GetAction(ctx, "nonexistent-id", "node-1")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestClaimPendingCommands(t *testing.T) {
	repo := setupTestRepo(t)
	ctx := context.Background()

	paramsJSON, _ := json.Marshal(map[string]string{})
	_, err := repo.CreateAction(ctx, "node-1", "container_restart", "c1", paramsJSON)
	if err != nil {
		t.Fatalf("CreateAction failed: %v", err)
	}

	commands, err := repo.ClaimPendingCommands(ctx, "node-1")
	if err != nil {
		t.Fatalf("ClaimPendingCommands failed: %v", err)
	}
	if len(commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(commands))
	}
	if commands[0].Action != "container_restart" {
		t.Errorf("expected action container_restart, got %s", commands[0].Action)
	}

	// Claiming again should return empty (already running)
	commands, err = repo.ClaimPendingCommands(ctx, "node-1")
	if err != nil {
		t.Fatalf("ClaimPendingCommands failed: %v", err)
	}
	if len(commands) != 0 {
		t.Errorf("expected 0 commands after claim, got %d", len(commands))
	}
}

func TestUpsertAndListAgents(t *testing.T) {
	repo := setupTestRepo(t)
	ctx := context.Background()

	if err := repo.UpsertAgent(ctx, "node-1", "1.0.0"); err != nil {
		t.Fatalf("UpsertAgent failed: %v", err)
	}

	agents, err := repo.ListAgents(ctx)
	if err != nil {
		t.Fatalf("ListAgents failed: %v", err)
	}
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}
	if agents[0].NodeName != "node-1" {
		t.Errorf("expected node-1, got %s", agents[0].NodeName)
	}
	if !agents[0].Online {
		t.Error("expected agent to be online")
	}

	// ListOnlineAgents
	online, err := repo.ListOnlineAgents(ctx)
	if err != nil {
		t.Fatalf("ListOnlineAgents failed: %v", err)
	}
	if len(online) != 1 || online[0] != "node-1" {
		t.Errorf("expected [node-1], got %v", online)
	}
}

func TestMarkRemovedAndSweep(t *testing.T) {
	repo := setupTestRepo(t)
	ctx := context.Background()

	envsJSON, _ := json.Marshal(map[string]string{})
	mountsJSON, _ := json.Marshal([]struct{}{})
	labelsJSON, _ := json.Marshal(map[string]string{})

	m := ContainerMetadata{
		ContainerID: "sweep-test",
		NodeName:    "node-1",
		Name:        "old-container",
		ImageTag:    "old:1",
		EnvsJSON:    envsJSON,
		MountsJSON:  mountsJSON,
		LabelsJSON:  labelsJSON,
	}
	if err := repo.UpsertMetadata(ctx, m); err != nil {
		t.Fatalf("UpsertMetadata failed: %v", err)
	}

	// MarkRemoved
	count, err := repo.MarkRemoved(ctx, "node-1", []string{"sweep-test"})
	if err != nil {
		t.Fatalf("MarkRemoved failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 removed, got %d", count)
	}

	// Should no longer appear in ListContainers
	containers, err := repo.ListContainers(ctx, 0, 0)
	if err != nil {
		t.Fatalf("ListContainers failed: %v", err)
	}
	if len(containers) != 0 {
		t.Errorf("expected 0 containers after removal, got %d", len(containers))
	}
}

func TestGetPreviousStatus(t *testing.T) {
	repo := setupTestRepo(t)
	ctx := context.Background()

	// No heartbeat yet
	_, err := repo.GetPreviousStatus(ctx, "no-heartbeat")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	// Insert container + heartbeat
	envsJSON, _ := json.Marshal(map[string]string{})
	mountsJSON, _ := json.Marshal([]struct{}{})
	labelsJSON, _ := json.Marshal(map[string]string{})
	m := ContainerMetadata{
		ContainerID: "prev-status-test",
		NodeName:    "node-1",
		Name:        "test",
		ImageTag:    "test:1",
		EnvsJSON:    envsJSON,
		MountsJSON:  mountsJSON,
		LabelsJSON:  labelsJSON,
	}
	if err := repo.UpsertMetadata(ctx, m); err != nil {
		t.Fatalf("UpsertMetadata failed: %v", err)
	}
	if err := repo.InsertHeartbeat(ctx, "prev-status-test", "running", 100); err != nil {
		t.Fatalf("InsertHeartbeat failed: %v", err)
	}

	status, err := repo.GetPreviousStatus(ctx, "prev-status-test")
	if err != nil {
		t.Fatalf("GetPreviousStatus failed: %v", err)
	}
	if status != "running" {
		t.Errorf("expected running, got %s", status)
	}
}
