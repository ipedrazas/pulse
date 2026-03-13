package repository_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ipedrazas/pulse/api/internal/db"
	"github.com/ipedrazas/pulse/api/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupTestDB(t *testing.T) (repository.Repository, func()) {
	t.Helper()
	ctx := context.Background()

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

	// Run migrations
	err = db.RunMigrations(connStr)
	require.NoError(t, err)

	pool, err := db.NewPool(ctx, connStr, nil)
	require.NoError(t, err)

	repo := repository.NewPostgresRepository(pool)

	cleanup := func() {
		pool.Close()
		_ = pgContainer.Terminate(ctx)
	}

	return repo, cleanup
}

func TestAgentCRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now().Truncate(time.Microsecond)

	// Upsert agent
	agent := repository.Agent{
		Name:     "node-1",
		Status:   "online",
		Version:  "0.1.0",
		LastSeen: &now,
	}
	err := repo.UpsertAgent(ctx, agent)
	require.NoError(t, err)

	// Get agent
	got, err := repo.GetAgent(ctx, "node-1")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "node-1", got.Name)
	assert.Equal(t, "online", got.Status)
	assert.Equal(t, "0.1.0", got.Version)

	// List agents
	agents, err := repo.ListAgents(ctx)
	require.NoError(t, err)
	assert.Len(t, agents, 1)

	// Set status
	err = repo.SetAgentStatus(ctx, "node-1", "offline")
	require.NoError(t, err)

	got, err = repo.GetAgent(ctx, "node-1")
	require.NoError(t, err)
	assert.Equal(t, "offline", got.Status)

	// Get non-existent
	got, err = repo.GetAgent(ctx, "nonexistent")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestContainerCRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now()

	// Create agent first (FK constraint)
	err := repo.UpsertAgent(ctx, repository.Agent{
		Name: "node-1", Status: "online", Version: "0.1.0", LastSeen: &now,
	})
	require.NoError(t, err)

	// Upsert container
	c := repository.Container{
		ContainerID:    "abc123",
		AgentName:      "node-1",
		Name:           "web",
		Image:          "nginx:latest",
		Status:         "running",
		EnvVars:        map[string]string{"ENV": "prod"},
		Mounts:         []string{"/data:/data"},
		Labels:         map[string]string{"app": "web"},
		Ports:          []repository.Port{{HostPort: 8080, ContainerPort: 80, Protocol: "tcp"}},
		ComposeProject: "myapp",
		UptimeSeconds:  3600,
	}
	err = repo.UpsertContainer(ctx, c)
	require.NoError(t, err)

	// Get container
	got, err := repo.GetContainer(ctx, "abc123", "")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "web", got.Name)
	assert.Equal(t, "nginx:latest", got.Image)
	assert.Equal(t, "running", got.Status)
	assert.Equal(t, map[string]string{"ENV": "prod"}, got.EnvVars)
	assert.Len(t, got.Ports, 1)
	assert.Equal(t, uint32(80), got.Ports[0].ContainerPort)

	// List containers
	containers, total, err := repo.ListContainers(ctx, "", 50, 0)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, containers, 1)

	// List by agent
	containers, total, err = repo.ListContainers(ctx, "node-1", 50, 0)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, containers, 1)

	// Mark removed
	err = repo.MarkContainersRemoved(ctx, "node-1", []string{})
	require.NoError(t, err)

	containers, total, err = repo.ListContainers(ctx, "", 50, 0)
	require.NoError(t, err)
	assert.Equal(t, 0, total) // removed containers excluded
}

func TestContainerEvents(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now()

	err := repo.UpsertAgent(ctx, repository.Agent{
		Name: "node-1", Status: "online", Version: "0.1.0", LastSeen: &now,
	})
	require.NoError(t, err)

	event := repository.ContainerEvent{
		Time:          now,
		ContainerID:   "abc123",
		AgentName:     "node-1",
		Status:        "running",
		UptimeSeconds: 100,
	}
	err = repo.InsertContainerEvent(ctx, event)
	require.NoError(t, err)
}

func TestCommandLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now()

	err := repo.UpsertAgent(ctx, repository.Agent{
		Name: "node-1", Status: "online", Version: "0.1.0", LastSeen: &now,
	})
	require.NoError(t, err)

	// Create command
	cmd := repository.Command{
		ID:        "cmd-1",
		AgentName: "node-1",
		Type:      "run_container",
		Payload:   []byte(`{"image":"nginx:latest"}`),
		Status:    "pending",
	}
	err = repo.CreateCommand(ctx, cmd)
	require.NoError(t, err)

	// Get pending
	cmds, err := repo.GetPendingCommands(ctx, "node-1")
	require.NoError(t, err)
	assert.Len(t, cmds, 1)
	assert.Equal(t, "cmd-1", cmds[0].ID)

	// Complete command
	err = repo.CompleteCommand(ctx, "cmd-1", "container started", true)
	require.NoError(t, err)

	// No more pending
	cmds, err = repo.GetPendingCommands(ctx, "node-1")
	require.NoError(t, err)
	assert.Empty(t, cmds)
}

func TestContainerPagination(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now()

	err := repo.UpsertAgent(ctx, repository.Agent{
		Name: "node-1", Status: "online", Version: "0.1.0", LastSeen: &now,
	})
	require.NoError(t, err)

	// Create 5 containers
	for i := 0; i < 5; i++ {
		err = repo.UpsertContainer(ctx, repository.Container{
			ContainerID: fmt.Sprintf("c-%d", i),
			AgentName:   "node-1",
			Name:        fmt.Sprintf("container-%d", i),
			Image:       "nginx:latest",
			Status:      "running",
			EnvVars:     map[string]string{},
			Labels:      map[string]string{},
		})
		require.NoError(t, err)
	}

	// Page 1
	containers, total, err := repo.ListContainers(ctx, "", 2, 0)
	require.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Len(t, containers, 2)

	// Page 2
	containers, _, err = repo.ListContainers(ctx, "", 2, 2)
	require.NoError(t, err)
	assert.Len(t, containers, 2)

	// Page 3
	containers, _, err = repo.ListContainers(ctx, "", 2, 4)
	require.NoError(t, err)
	assert.Len(t, containers, 1)
}
