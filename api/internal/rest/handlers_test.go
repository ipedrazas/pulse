package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ipedrazas/pulse/api/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- mockRepo ---

type mockRepo struct {
	agents     []repository.Agent
	agent      *repository.Agent
	containers []repository.Container
	container  *repository.Container
	total      int
	err        error
}

func (m *mockRepo) Ping(_ context.Context) error { return m.err }
func (m *mockRepo) WithTx(_ context.Context, fn func(repository.Repository) error) error {
	return fn(m)
}
func (m *mockRepo) UpsertAgent(_ context.Context, _ repository.Agent) error { return m.err }
func (m *mockRepo) GetAgent(_ context.Context, _ string) (*repository.Agent, error) {
	return m.agent, m.err
}
func (m *mockRepo) ListAgents(_ context.Context) ([]repository.Agent, error)        { return m.agents, m.err }
func (m *mockRepo) SetAgentStatus(_ context.Context, _, _ string) error             { return m.err }
func (m *mockRepo) UpsertContainer(_ context.Context, _ repository.Container) error { return m.err }
func (m *mockRepo) GetContainer(_ context.Context, _, _ string) (*repository.Container, error) {
	return m.container, m.err
}
func (m *mockRepo) ListContainers(_ context.Context, _ string, _, _ int) ([]repository.Container, int, error) {
	return m.containers, m.total, m.err
}
func (m *mockRepo) MarkContainersRemoved(_ context.Context, _ string, _ []string) error { return m.err }
func (m *mockRepo) InsertContainerEvent(_ context.Context, _ repository.ContainerEvent) error {
	return m.err
}
func (m *mockRepo) CreateCommand(_ context.Context, _ repository.Command) error { return m.err }
func (m *mockRepo) GetCommand(_ context.Context, _ string) (*repository.Command, error) {
	return nil, m.err
}
func (m *mockRepo) GetPendingCommands(_ context.Context, _ string) ([]repository.Command, error) {
	return nil, m.err
}
func (m *mockRepo) CompleteCommand(_ context.Context, _, _ string, _ bool) error { return m.err }
func (m *mockRepo) DeleteAgent(_ context.Context, _ string) error                { return m.err }
func (m *mockRepo) MarkStaleAgents(_ context.Context, _ time.Duration) (int, error) {
	return 0, m.err
}

func setupRouter(repo repository.Repository) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHandler(repo, nil)
	h.Register(r)
	return r
}

// --- healthz ---

func TestHealthz_Returns200(t *testing.T) {
	r := setupRouter(&mockRepo{})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/healthz", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "ok")
}

func TestHealthz_DBDown_Returns503(t *testing.T) {
	r := setupRouter(&mockRepo{err: errors.New("connection refused")})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/healthz", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, 503, w.Code)
	assert.Contains(t, w.Body.String(), "degraded")
}

// --- listNodes ---

func TestListNodes_Empty(t *testing.T) {
	r := setupRouter(&mockRepo{agents: []repository.Agent{}})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/nodes", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	// Empty agent list produces a nil slice which Gin serializes as "null"
	assert.Contains(t, w.Body.String(), "null")
}

func TestListNodes_WithAgents(t *testing.T) {
	repo := &mockRepo{
		agents: []repository.Agent{
			{Name: "node-1", Status: "online", Version: "0.1.0"},
		},
	}
	r := setupRouter(repo)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/nodes", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "node-1")
}

func TestListNodes_RepoError(t *testing.T) {
	repo := &mockRepo{err: errors.New("db down")}
	r := setupRouter(repo)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/nodes", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, 500, w.Code)
}

// --- getNode ---

func TestGetNode_Found(t *testing.T) {
	repo := &mockRepo{
		agent: &repository.Agent{Name: "node-1", Status: "online"},
	}
	r := setupRouter(repo)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/nodes/node-1", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "node-1")
}

func TestGetNode_NotFound(t *testing.T) {
	repo := &mockRepo{agent: nil}
	r := setupRouter(repo)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/nodes/missing", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, 404, w.Code)
}

func TestGetNode_RepoError(t *testing.T) {
	repo := &mockRepo{err: errors.New("db error")}
	r := setupRouter(repo)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/nodes/node-1", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, 500, w.Code)
}

// --- listContainers ---

func TestListContainers_Default(t *testing.T) {
	repo := &mockRepo{
		containers: []repository.Container{{ContainerID: "c1", Name: "web"}},
		total:      1,
	}
	r := setupRouter(repo)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/containers", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, float64(50), body["page_size"])
	assert.Equal(t, float64(0), body["offset"])
}

func TestListContainers_CustomParams(t *testing.T) {
	repo := &mockRepo{total: 0}
	r := setupRouter(repo)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/containers?page_size=10&offset=5&node=n1", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, float64(10), body["page_size"])
	assert.Equal(t, float64(5), body["offset"])
}

func TestListContainers_PageSizeCapped(t *testing.T) {
	repo := &mockRepo{total: 0}
	r := setupRouter(repo)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/containers?page_size=999999", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, float64(maxPageSize), body["page_size"])
}

func TestListContainers_RepoError(t *testing.T) {
	repo := &mockRepo{err: errors.New("fail")}
	r := setupRouter(repo)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/containers", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, 500, w.Code)
}

// --- getContainer ---

func TestGetContainer_Found(t *testing.T) {
	repo := &mockRepo{
		container: &repository.Container{ContainerID: "c1", Name: "web"},
	}
	r := setupRouter(repo)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/containers/c1", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "c1")
}

func TestGetContainer_NotFound(t *testing.T) {
	repo := &mockRepo{container: nil}
	r := setupRouter(repo)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/containers/missing", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, 404, w.Code)
}

func TestGetContainer_RepoError(t *testing.T) {
	repo := &mockRepo{err: errors.New("fail")}
	r := setupRouter(repo)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/containers/c1", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, 500, w.Code)
}

// --- createCommand ---

func TestCreateCommand_Valid(t *testing.T) {
	repo := &mockRepo{}
	r := setupRouter(repo)
	w := httptest.NewRecorder()
	body := `{"node_name":"node-1","type":"run_container","payload":{"image":"nginx"}}`
	req, _ := http.NewRequest("POST", "/api/v1/commands", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, 202, w.Code)
	assert.Contains(t, w.Body.String(), "command_id")
}

func TestCreateCommand_InvalidJSON(t *testing.T) {
	r := setupRouter(&mockRepo{})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/commands", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)
}

func TestCreateCommand_MissingFields(t *testing.T) {
	r := setupRouter(&mockRepo{})
	w := httptest.NewRecorder()
	body := `{"node_name":"node-1"}`
	req, _ := http.NewRequest("POST", "/api/v1/commands", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)
}

func TestCreateCommand_RepoError(t *testing.T) {
	repo := &mockRepo{err: errors.New("fail")}
	r := setupRouter(repo)
	w := httptest.NewRecorder()
	body := `{"node_name":"node-1","type":"run_container","payload":{}}`
	req, _ := http.NewRequest("POST", "/api/v1/commands", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, 500, w.Code)
}

// --- Edge-case and negative tests ---

func TestCreateCommand_EmptyBody(t *testing.T) {
	r := setupRouter(&mockRepo{})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/commands", nil)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)
}

func TestCreateCommand_MissingNodeName(t *testing.T) {
	r := setupRouter(&mockRepo{})
	w := httptest.NewRecorder()
	body := `{"type":"run_container","payload":{}}`
	req, _ := http.NewRequest("POST", "/api/v1/commands", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)
}

func TestGetNode_EmptyName(t *testing.T) {
	repo := &mockRepo{agent: nil}
	r := setupRouter(repo)
	w := httptest.NewRecorder()
	// Gin treats empty param as 404 since route doesn't match
	req, _ := http.NewRequest("GET", "/api/v1/nodes/", nil)
	r.ServeHTTP(w, req)

	// No route matches /api/v1/nodes/ with trailing slash
	assert.NotEqual(t, 200, w.Code)
}

func TestGetContainer_EmptyID(t *testing.T) {
	r := setupRouter(&mockRepo{container: nil})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/containers/", nil)
	r.ServeHTTP(w, req)

	assert.NotEqual(t, 200, w.Code)
}

func TestDeleteNode_NotFound(t *testing.T) {
	repo := &mockRepo{err: errors.New("not found")}
	r := setupRouter(repo)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/nodes/nonexistent", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, 404, w.Code)
}

func TestListContainers_NegativeOffset(t *testing.T) {
	repo := &mockRepo{total: 0}
	r := setupRouter(repo)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/containers?offset=-1", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	// Negative offset should be treated as 0
	assert.Equal(t, float64(0), body["offset"])
}

func TestListContainers_InvalidPageSize(t *testing.T) {
	repo := &mockRepo{total: 0}
	r := setupRouter(repo)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/containers?page_size=abc", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	// Invalid page_size should fall back to default
	assert.Equal(t, float64(50), body["page_size"])
}

func TestGetCommand_NotFound(t *testing.T) {
	repo := &mockRepo{}
	r := setupRouter(repo)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/commands/nonexistent-id", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, 404, w.Code)
}

func TestStopContainer_NotFound(t *testing.T) {
	repo := &mockRepo{container: nil}
	r := setupRouter(repo)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/containers/missing/stop", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, 404, w.Code)
}

func TestRestartContainer_NotFound(t *testing.T) {
	repo := &mockRepo{container: nil}
	r := setupRouter(repo)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/containers/missing/restart", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, 404, w.Code)
}

func TestRequestLogs_NotFound(t *testing.T) {
	repo := &mockRepo{container: nil}
	r := setupRouter(repo)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/containers/missing/logs", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, 404, w.Code)
}

func TestListNodes_NilMetadata(t *testing.T) {
	repo := &mockRepo{
		agents: []repository.Agent{
			{Name: "node-1", Status: "online", Version: "0.1.0", Metadata: nil},
		},
	}
	r := setupRouter(repo)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/nodes", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "node-1")
}

func TestHealthz_ReturnsJSON(t *testing.T) {
	r := setupRouter(&mockRepo{})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/healthz", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
}

func TestInfo_ReturnsJSON(t *testing.T) {
	r := setupRouter(&mockRepo{})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/info", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Contains(t, body, "version")
	assert.Contains(t, body, "commit")
}
