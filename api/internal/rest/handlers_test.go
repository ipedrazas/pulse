package rest

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ipedrazas/pulse/api/internal/repository"
)

const testToken = "test-secret"

func init() {
	gin.SetMode(gin.TestMode)
}

// --- mock repositories ---

type mockContainerRepo struct {
	containers []repository.ContainerStatus
	container  repository.ContainerStatus
	composeDir string
	err        error
}

func (m *mockContainerRepo) ListContainers(_ context.Context, _, _ int) ([]repository.ContainerStatus, error) {
	return m.containers, m.err
}
func (m *mockContainerRepo) GetContainer(_ context.Context, _ string) (repository.ContainerStatus, error) {
	return m.container, m.err
}
func (m *mockContainerRepo) ListContainersByNode(_ context.Context, _ string, _, _ int) ([]repository.ContainerStatus, error) {
	return m.containers, m.err
}
func (m *mockContainerRepo) ListContainersByNodeForStacks(_ context.Context, _ string, _, _ int) ([]repository.ContainerStatus, error) {
	return m.containers, m.err
}
func (m *mockContainerRepo) GetComposeDir(_ context.Context, _, _ string) (string, error) {
	return m.composeDir, m.err
}
func (m *mockContainerRepo) UpsertMetadata(_ context.Context, _ repository.ContainerMetadata) error {
	return m.err
}
func (m *mockContainerRepo) InsertHeartbeat(_ context.Context, _, _ string, _ int64) error {
	return m.err
}
func (m *mockContainerRepo) GetPreviousStatus(_ context.Context, _ string) (string, error) {
	return "", m.err
}
func (m *mockContainerRepo) GetContainerInfoForRemoval(_ context.Context, _ string, _ []string) ([]repository.ContainerInfo, error) {
	return nil, m.err
}
func (m *mockContainerRepo) GetContainerMetadataForEvent(_ context.Context, _ string) (repository.ContainerInfo, string, error) {
	return repository.ContainerInfo{}, "", m.err
}
func (m *mockContainerRepo) MarkRemoved(_ context.Context, _ string, _ []string) (int64, error) {
	return 0, m.err
}
func (m *mockContainerRepo) SweepStale(_ context.Context, _ time.Duration) (int64, error) {
	return 0, m.err
}

type mockActionRepo struct {
	actions []repository.ActionResponse
	action  repository.ActionResponse
	err     error
}

func (m *mockActionRepo) CreateAction(_ context.Context, _, _, _ string, _ []byte) (repository.ActionResponse, error) {
	return m.action, m.err
}
func (m *mockActionRepo) ListActions(_ context.Context, _ string, _, _ int) ([]repository.ActionResponse, error) {
	return m.actions, m.err
}
func (m *mockActionRepo) GetAction(_ context.Context, _, _ string) (repository.ActionResponse, error) {
	return m.action, m.err
}
func (m *mockActionRepo) ClaimPendingCommands(_ context.Context, _ string) ([]repository.PendingCommand, error) {
	return nil, m.err
}
func (m *mockActionRepo) UpdateCommandResult(_ context.Context, _, _, _ string, _ int64) error {
	return m.err
}

type mockAgentRepo struct {
	agents []repository.AgentStatus
	err    error
}

func (m *mockAgentRepo) UpsertAgent(_ context.Context, _, _ string) error { return m.err }
func (m *mockAgentRepo) ListOnlineAgents(_ context.Context) ([]string, error) {
	return nil, m.err
}
func (m *mockAgentRepo) ListAgents(_ context.Context) ([]repository.AgentStatus, error) {
	return m.agents, m.err
}

type mockHealthChecker struct {
	err error
}

func (m *mockHealthChecker) Ping(_ context.Context) error { return m.err }

// --- helpers ---

func newTestHandler(opts ...func(*Handler)) *Handler {
	h := &Handler{
		containers: &mockContainerRepo{},
		actions:    &mockActionRepo{},
		agents:     &mockAgentRepo{},
		health:     &mockHealthChecker{},
		token:      testToken,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

func ptr[T any](v T) *T { return &v }

func doRequest(r *gin.Engine, method, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	r.ServeHTTP(w, req)
	return w
}

func doJSONRequest(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

// --- tests ---

func TestRegisterRoutes(t *testing.T) {
	h := newTestHandler()
	r := gin.New()
	h.RegisterRoutes(r)

	routes := r.Routes()
	type routeKey struct {
		method, path string
	}
	expected := []routeKey{
		{"GET", "/healthz"},
		{"GET", "/status"},
		{"GET", "/status/:container"},
		{"GET", "/nodes"},
		{"GET", "/nodes/:node"},
		{"GET", "/nodes/:node/stacks"},
		{"POST", "/nodes/:node/actions"},
		{"GET", "/nodes/:node/actions"},
		{"GET", "/nodes/:node/actions/:id"},
		{"GET", "/agents"},
	}

	registered := make(map[routeKey]bool)
	for _, route := range routes {
		registered[routeKey{route.Method, route.Path}] = true
	}

	for _, rk := range expected {
		if !registered[rk] {
			t.Errorf("route %s %s not registered", rk.method, rk.path)
		}
	}
}

func TestHealthz_Healthy(t *testing.T) {
	h := newTestHandler()
	r := gin.New()
	r.GET("/healthz", h.Healthz)

	w := doRequest(r, "GET", "/healthz")

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["status"] != "healthy" {
		t.Errorf("expected healthy, got %s", body["status"])
	}
}

func TestHealthz_Unhealthy(t *testing.T) {
	h := newTestHandler(func(h *Handler) {
		h.health = &mockHealthChecker{err: errors.New("db down")}
	})
	r := gin.New()
	r.GET("/healthz", h.Healthz)

	w := doRequest(r, "GET", "/healthz")

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestGetStatus_Success(t *testing.T) {
	h := newTestHandler(func(h *Handler) {
		h.containers = &mockContainerRepo{
			containers: []repository.ContainerStatus{
				{ContainerID: "c1", NodeName: "n1", Name: "nginx", ImageTag: "nginx:latest", Status: ptr("running"), UptimeSeconds: ptr(int64(3600))},
			},
		}
	})
	r := gin.New()
	r.GET("/status", h.GetStatus)

	w := doRequest(r, "GET", "/status")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result []repository.ContainerStatus
	json.Unmarshal(w.Body.Bytes(), &result)
	if len(result) != 1 || result[0].ContainerID != "c1" {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestGetStatus_QueryError(t *testing.T) {
	h := newTestHandler(func(h *Handler) {
		h.containers = &mockContainerRepo{err: errors.New("query failed")}
	})
	r := gin.New()
	r.GET("/status", h.GetStatus)

	w := doRequest(r, "GET", "/status")

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}

	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["error"] != "internal server error" {
		t.Errorf("expected generic error message, got %q", body["error"])
	}
}

func TestGetStatus_Empty(t *testing.T) {
	h := newTestHandler()
	r := gin.New()
	r.GET("/status", h.GetStatus)

	w := doRequest(r, "GET", "/status")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result []repository.ContainerStatus
	json.Unmarshal(w.Body.Bytes(), &result)
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d items", len(result))
	}
}

func TestGetContainerStatus_Found(t *testing.T) {
	cs := repository.ContainerStatus{ContainerID: "c1", NodeName: "n1", Name: "test", ImageTag: "img:v1", Status: ptr("running"), UptimeSeconds: ptr(int64(100))}

	h := newTestHandler(func(h *Handler) {
		h.containers = &mockContainerRepo{container: cs}
	})
	r := gin.New()
	r.GET("/status/:container", h.GetContainerStatus)

	w := doRequest(r, "GET", "/status/c1")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result repository.ContainerStatus
	json.Unmarshal(w.Body.Bytes(), &result)
	if result.ContainerID != "c1" {
		t.Errorf("expected c1, got %s", result.ContainerID)
	}
}

func TestGetContainerStatus_NotFound(t *testing.T) {
	h := newTestHandler(func(h *Handler) {
		h.containers = &mockContainerRepo{err: repository.ErrNotFound}
	})
	r := gin.New()
	r.GET("/status/:container", h.GetContainerStatus)

	w := doRequest(r, "GET", "/status/nonexistent")

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestGetContainerStatus_InternalError(t *testing.T) {
	h := newTestHandler(func(h *Handler) {
		h.containers = &mockContainerRepo{err: errors.New("db error")}
	})
	r := gin.New()
	r.GET("/status/:container", h.GetContainerStatus)

	w := doRequest(r, "GET", "/status/c1")

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}

	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["error"] != "internal server error" {
		t.Errorf("expected generic error message, got %q", body["error"])
	}
}

func TestGetNodes_Success(t *testing.T) {
	h := newTestHandler(func(h *Handler) {
		h.containers = &mockContainerRepo{
			containers: []repository.ContainerStatus{
				{ContainerID: "c1", NodeName: "node-a", Name: "nginx", ImageTag: "nginx:1", Status: ptr("running")},
				{ContainerID: "c2", NodeName: "node-a", Name: "redis", ImageTag: "redis:7", Status: ptr("running")},
				{ContainerID: "c3", NodeName: "node-b", Name: "postgres", ImageTag: "pg:16", Status: ptr("running")},
			},
		}
	})
	r := gin.New()
	r.GET("/nodes", h.GetNodes)

	w := doRequest(r, "GET", "/nodes")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result []nodeContainers
	json.Unmarshal(w.Body.Bytes(), &result)
	if len(result) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(result))
	}
	if result[0].NodeName != "node-a" || len(result[0].Containers) != 2 {
		t.Errorf("node-a: expected 2 containers, got %d", len(result[0].Containers))
	}
	if result[1].NodeName != "node-b" || len(result[1].Containers) != 1 {
		t.Errorf("node-b: expected 1 container, got %d", len(result[1].Containers))
	}
}

func TestGetNodes_QueryError(t *testing.T) {
	h := newTestHandler(func(h *Handler) {
		h.containers = &mockContainerRepo{err: errors.New("db error")}
	})
	r := gin.New()
	r.GET("/nodes", h.GetNodes)

	w := doRequest(r, "GET", "/nodes")

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestGetNode_Found(t *testing.T) {
	h := newTestHandler(func(h *Handler) {
		h.containers = &mockContainerRepo{
			containers: []repository.ContainerStatus{
				{ContainerID: "c1", NodeName: "pve1", Name: "nginx", ImageTag: "nginx:1", Status: ptr("running")},
			},
		}
	})
	r := gin.New()
	r.GET("/nodes/:node", h.GetNode)

	w := doRequest(r, "GET", "/nodes/pve1")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result nodeContainers
	json.Unmarshal(w.Body.Bytes(), &result)
	if result.NodeName != "pve1" || len(result.Containers) != 1 {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestGetNode_NotFound(t *testing.T) {
	h := newTestHandler()
	r := gin.New()
	r.GET("/nodes/:node", h.GetNode)

	w := doRequest(r, "GET", "/nodes/nonexistent")

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestGetNode_QueryError(t *testing.T) {
	h := newTestHandler(func(h *Handler) {
		h.containers = &mockContainerRepo{err: errors.New("db error")}
	})
	r := gin.New()
	r.GET("/nodes/:node", h.GetNode)

	w := doRequest(r, "GET", "/nodes/pve1")

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestContainerStatusJSON(t *testing.T) {
	cs := repository.ContainerStatus{
		ContainerID:   "abc123",
		NodeName:      "node1",
		Name:          "nginx",
		ImageTag:      "nginx:latest",
		Status:        ptr("running"),
		UptimeSeconds: ptr(int64(3600)),
		LastSeen:      ptr("2025-01-15 10:00:00+00"),
	}

	data, err := json.Marshal(cs)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded map[string]any
	json.Unmarshal(data, &decoded)

	if decoded["container_id"] != "abc123" {
		t.Errorf("expected container_id 'abc123', got %v", decoded["container_id"])
	}
}

func TestContainerStatusJSON_NullFields(t *testing.T) {
	cs := repository.ContainerStatus{
		ContainerID: "abc123",
		NodeName:    "node1",
		Name:        "nginx",
		ImageTag:    "nginx:latest",
	}

	data, err := json.Marshal(cs)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded map[string]any
	json.Unmarshal(data, &decoded)

	if decoded["status"] != nil {
		t.Errorf("expected null status, got %v", decoded["status"])
	}
}

func TestGetNodeStacks_GroupsByProject(t *testing.T) {
	h := newTestHandler(func(h *Handler) {
		h.containers = &mockContainerRepo{
			containers: []repository.ContainerStatus{
				{ContainerID: "c1", NodeName: "pve1", Name: "nginx", ImageTag: "nginx:1", Status: ptr("running"), ComposeProject: "web"},
				{ContainerID: "c2", NodeName: "pve1", Name: "redis", ImageTag: "redis:7", Status: ptr("running"), ComposeProject: "web"},
				{ContainerID: "c3", NodeName: "pve1", Name: "postgres", ImageTag: "pg:16", Status: ptr("running"), ComposeProject: "db"},
			},
		}
	})
	r := gin.New()
	r.GET("/nodes/:node/stacks", h.GetNodeStacks)

	w := doRequest(r, "GET", "/nodes/pve1/stacks")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result []composeStack
	json.Unmarshal(w.Body.Bytes(), &result)
	if len(result) != 2 {
		t.Fatalf("expected 2 stacks, got %d", len(result))
	}
	if result[0].Project != "web" || len(result[0].Containers) != 2 {
		t.Errorf("stack 'web': expected 2 containers, got %d", len(result[0].Containers))
	}
	if result[1].Project != "db" || len(result[1].Containers) != 1 {
		t.Errorf("stack 'db': expected 1 container, got %d", len(result[1].Containers))
	}
}

func TestGetNodeStacks_StandaloneContainers(t *testing.T) {
	h := newTestHandler(func(h *Handler) {
		h.containers = &mockContainerRepo{
			containers: []repository.ContainerStatus{
				{ContainerID: "c1", NodeName: "pve1", Name: "nginx", ImageTag: "nginx:1", Status: ptr("running"), ComposeProject: ""},
			},
		}
	})
	r := gin.New()
	r.GET("/nodes/:node/stacks", h.GetNodeStacks)

	w := doRequest(r, "GET", "/nodes/pve1/stacks")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result []composeStack
	json.Unmarshal(w.Body.Bytes(), &result)
	if len(result) != 1 {
		t.Fatalf("expected 1 stack, got %d", len(result))
	}
	if result[0].Project != "(standalone)" {
		t.Errorf("expected project '(standalone)', got %q", result[0].Project)
	}
}

func TestGetNodeStacks_NotFound(t *testing.T) {
	h := newTestHandler()
	r := gin.New()
	r.GET("/nodes/:node/stacks", h.GetNodeStacks)

	w := doRequest(r, "GET", "/nodes/ghost/stacks")

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// --- action tests ---

func TestCreateAction_BadJSON(t *testing.T) {
	h := newTestHandler()
	r := gin.New()
	r.POST("/nodes/:node/actions", h.CreateAction)

	w := doJSONRequest(r, "POST", "/nodes/test/actions", "")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreateAction_UnknownAction(t *testing.T) {
	h := newTestHandler()
	r := gin.New()
	r.POST("/nodes/:node/actions", h.CreateAction)

	w := doJSONRequest(r, "POST", "/nodes/test/actions", `{"action":"unknown_action"}`)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}

	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["error"] != "unknown action: unknown_action" {
		t.Errorf("unexpected error: %s", body["error"])
	}
}

func TestCreateAction_Success(t *testing.T) {
	ar := repository.ActionResponse{
		CommandID: "cmd-1",
		NodeName:  "test-node",
		Action:    "compose_update",
		Target:    "my-stack",
		Status:    "pending",
		CreatedAt: "2025-01-15 10:00:00+00",
		UpdatedAt: "2025-01-15 10:00:00+00",
	}

	h := newTestHandler(func(h *Handler) {
		h.containers = &mockContainerRepo{err: repository.ErrNotFound}
		h.actions = &mockActionRepo{action: ar}
	})
	r := gin.New()
	r.POST("/nodes/:node/actions", h.CreateAction)

	w := doJSONRequest(r, "POST", "/nodes/test-node/actions", `{"action":"compose_update","target":"my-stack"}`)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}

	var result repository.ActionResponse
	json.Unmarshal(w.Body.Bytes(), &result)
	if result.CommandID != "cmd-1" {
		t.Errorf("expected cmd-1, got %s", result.CommandID)
	}
}

func TestCreateAction_DBError(t *testing.T) {
	h := newTestHandler(func(h *Handler) {
		h.containers = &mockContainerRepo{err: repository.ErrNotFound}
		h.actions = &mockActionRepo{err: errors.New("db error")}
	})
	r := gin.New()
	r.POST("/nodes/:node/actions", h.CreateAction)

	w := doJSONRequest(r, "POST", "/nodes/test/actions", `{"action":"compose_update"}`)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestListActions_QueryError(t *testing.T) {
	h := newTestHandler(func(h *Handler) {
		h.actions = &mockActionRepo{err: errors.New("db error")}
	})
	r := gin.New()
	r.GET("/nodes/:node/actions", h.ListActions)

	w := doRequest(r, "GET", "/nodes/test/actions")

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestListActions_Empty(t *testing.T) {
	h := newTestHandler()
	r := gin.New()
	r.GET("/nodes/:node/actions", h.ListActions)

	w := doRequest(r, "GET", "/nodes/test/actions")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result []repository.ActionResponse
	json.Unmarshal(w.Body.Bytes(), &result)
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d items", len(result))
	}
}

func TestGetAction_NotFound(t *testing.T) {
	h := newTestHandler(func(h *Handler) {
		h.actions = &mockActionRepo{err: repository.ErrNotFound}
	})
	r := gin.New()
	r.GET("/nodes/:node/actions/:id", h.GetAction)

	w := doRequest(r, "GET", "/nodes/test/actions/abc123")

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestGetAction_Success(t *testing.T) {
	ar := repository.ActionResponse{
		CommandID: "cmd-1",
		NodeName:  "test-node",
		Action:    "compose_restart",
		Target:    "my-stack",
		Status:    "success",
		Output:    "done",
		CreatedAt: "2025-01-15 10:00:00+00",
		UpdatedAt: "2025-01-15 10:05:00+00",
	}

	h := newTestHandler(func(h *Handler) {
		h.actions = &mockActionRepo{action: ar}
	})
	r := gin.New()
	r.GET("/nodes/:node/actions/:id", h.GetAction)

	w := doRequest(r, "GET", "/nodes/test-node/actions/cmd-1")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result repository.ActionResponse
	json.Unmarshal(w.Body.Bytes(), &result)
	if result.CommandID != "cmd-1" || result.Status != "success" {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestGetAction_InternalError(t *testing.T) {
	h := newTestHandler(func(h *Handler) {
		h.actions = &mockActionRepo{err: errors.New("db error")}
	})
	r := gin.New()
	r.GET("/nodes/:node/actions/:id", h.GetAction)

	w := doRequest(r, "GET", "/nodes/test/actions/cmd-1")

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}

	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["error"] != "internal server error" {
		t.Errorf("expected generic error message, got %q", body["error"])
	}
}

// --- agent tests ---

func TestGetAgents_Success(t *testing.T) {
	h := newTestHandler(func(h *Handler) {
		h.agents = &mockAgentRepo{
			agents: []repository.AgentStatus{
				{NodeName: "node-a", AgentVersion: "1.0.0", FirstSeen: "2025-01-15 10:00:00+00", LastSeen: "2025-01-15 12:00:00+00", Online: true},
				{NodeName: "node-b", AgentVersion: "1.0.1", FirstSeen: "2025-01-14 08:00:00+00", LastSeen: "2025-01-14 09:00:00+00", Online: false},
			},
		}
	})
	r := gin.New()
	r.GET("/agents", h.GetAgents)

	w := doRequest(r, "GET", "/agents")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result []repository.AgentStatus
	json.Unmarshal(w.Body.Bytes(), &result)
	if len(result) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(result))
	}
	if result[0].NodeName != "node-a" || !result[0].Online {
		t.Errorf("unexpected first agent: %+v", result[0])
	}
	if result[1].NodeName != "node-b" || result[1].Online {
		t.Errorf("unexpected second agent: %+v", result[1])
	}
}

func TestGetAgents_Empty(t *testing.T) {
	h := newTestHandler()
	r := gin.New()
	r.GET("/agents", h.GetAgents)

	w := doRequest(r, "GET", "/agents")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result []repository.AgentStatus
	json.Unmarshal(w.Body.Bytes(), &result)
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d items", len(result))
	}
}
