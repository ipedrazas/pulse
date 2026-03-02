package rest

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func marshalOrEmpty(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

const testToken = "test-secret"

func init() {
	gin.SetMode(gin.TestMode)
}

// --- mock DB ---

type mockDB struct {
	pingErr  error
	queryErr error
	rows     *mockRows
	row      pgx.Row
}

func (m *mockDB) Ping(_ context.Context) error { return m.pingErr }
func (m *mockDB) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	if m.queryErr != nil {
		return nil, m.queryErr
	}
	return m.rows, nil
}
func (m *mockDB) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return m.row
}
func (m *mockDB) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

// --- mock rows ---

type mockRows struct {
	data    []containerStatus
	cursor  int
	scanErr error
	closed  bool
}

func (r *mockRows) Next() bool {
	if r.cursor < len(r.data) {
		r.cursor++
		return true
	}
	return false
}

func (r *mockRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	cs := r.data[r.cursor-1]
	*dest[0].(*string) = cs.ContainerID
	*dest[1].(*string) = cs.NodeName
	*dest[2].(*string) = cs.Name
	*dest[3].(*string) = cs.ImageTag
	*dest[4].(**string) = cs.Status
	*dest[5].(**int64) = cs.UptimeSeconds
	*dest[6].(**string) = cs.LastSeen
	*dest[7].(*[]byte) = marshalOrEmpty(cs.Labels)
	*dest[8].(*[]byte) = marshalOrEmpty(cs.EnvVars)
	*dest[9].(*string) = cs.ComposeProject
	return nil
}

func (r *mockRows) Close()                                       { r.closed = true }
func (r *mockRows) Err() error                                   { return nil }
func (r *mockRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *mockRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *mockRows) Values() ([]any, error)                       { return nil, nil }
func (r *mockRows) RawValues() [][]byte                          { return nil }
func (r *mockRows) Conn() *pgx.Conn                              { return nil }

// --- mock row ---

type mockRow struct {
	cs  *containerStatus
	err error
}

func (r *mockRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	*dest[0].(*string) = r.cs.ContainerID
	*dest[1].(*string) = r.cs.NodeName
	*dest[2].(*string) = r.cs.Name
	*dest[3].(*string) = r.cs.ImageTag
	*dest[4].(**string) = r.cs.Status
	*dest[5].(**int64) = r.cs.UptimeSeconds
	*dest[6].(**string) = r.cs.LastSeen
	*dest[7].(*[]byte) = marshalOrEmpty(r.cs.Labels)
	*dest[8].(*[]byte) = marshalOrEmpty(r.cs.EnvVars)
	*dest[9].(*string) = r.cs.ComposeProject
	return nil
}

// --- helpers ---

func ptr[T any](v T) *T { return &v }

func doRequest(r *gin.Engine, method, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	r.ServeHTTP(w, req)
	return w
}

// --- tests ---

func TestNewHandler(t *testing.T) {
	h := NewHandler(nil, testToken)
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
}

func TestNewHandlerWithDB(t *testing.T) {
	db := &mockDB{}
	h := NewHandlerWithDB(db, testToken)
	if h == nil || h.db != db {
		t.Fatal("expected handler with mock DB")
	}
}

func TestRegisterRoutes(t *testing.T) {
	h := NewHandlerWithDB(&mockDB{}, testToken)
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
	h := NewHandlerWithDB(&mockDB{}, testToken)
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
	h := NewHandlerWithDB(&mockDB{pingErr: errors.New("db down")}, testToken)
	r := gin.New()
	r.GET("/healthz", h.Healthz)

	w := doRequest(r, "GET", "/healthz")

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestGetStatus_Success(t *testing.T) {
	status := "running"
	uptime := int64(3600)
	rows := &mockRows{data: []containerStatus{
		{ContainerID: "c1", NodeName: "n1", Name: "nginx", ImageTag: "nginx:latest", Status: &status, UptimeSeconds: &uptime},
	}}

	h := NewHandlerWithDB(&mockDB{rows: rows}, testToken)
	r := gin.New()
	r.GET("/status", h.GetStatus)

	w := doRequest(r, "GET", "/status")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result []containerStatus
	json.Unmarshal(w.Body.Bytes(), &result)
	if len(result) != 1 || result[0].ContainerID != "c1" {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestGetStatus_QueryError(t *testing.T) {
	h := NewHandlerWithDB(&mockDB{queryErr: errors.New("query failed")}, testToken)
	r := gin.New()
	r.GET("/status", h.GetStatus)

	w := doRequest(r, "GET", "/status")

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestGetStatus_Empty(t *testing.T) {
	h := NewHandlerWithDB(&mockDB{rows: &mockRows{}}, testToken)
	r := gin.New()
	r.GET("/status", h.GetStatus)

	w := doRequest(r, "GET", "/status")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result []containerStatus
	json.Unmarshal(w.Body.Bytes(), &result)
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d items", len(result))
	}
}

func TestGetContainerStatus_Found(t *testing.T) {
	cs := containerStatus{ContainerID: "c1", NodeName: "n1", Name: "test", ImageTag: "img:v1", Status: ptr("running"), UptimeSeconds: ptr(int64(100))}

	h := NewHandlerWithDB(&mockDB{row: &mockRow{cs: &cs}}, testToken)
	r := gin.New()
	r.GET("/status/:container", h.GetContainerStatus)

	w := doRequest(r, "GET", "/status/c1")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result containerStatus
	json.Unmarshal(w.Body.Bytes(), &result)
	if result.ContainerID != "c1" {
		t.Errorf("expected c1, got %s", result.ContainerID)
	}
}

func TestGetContainerStatus_NotFound(t *testing.T) {
	h := NewHandlerWithDB(&mockDB{row: &mockRow{err: pgx.ErrNoRows}}, testToken)
	r := gin.New()
	r.GET("/status/:container", h.GetContainerStatus)

	w := doRequest(r, "GET", "/status/nonexistent")

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestGetNodes_Success(t *testing.T) {
	rows := &mockRows{data: []containerStatus{
		{ContainerID: "c1", NodeName: "node-a", Name: "nginx", ImageTag: "nginx:1", Status: ptr("running")},
		{ContainerID: "c2", NodeName: "node-a", Name: "redis", ImageTag: "redis:7", Status: ptr("running")},
		{ContainerID: "c3", NodeName: "node-b", Name: "postgres", ImageTag: "pg:16", Status: ptr("running")},
	}}

	h := NewHandlerWithDB(&mockDB{rows: rows}, testToken)
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
	h := NewHandlerWithDB(&mockDB{queryErr: errors.New("db error")}, testToken)
	r := gin.New()
	r.GET("/nodes", h.GetNodes)

	w := doRequest(r, "GET", "/nodes")

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestGetNode_Found(t *testing.T) {
	rows := &mockRows{data: []containerStatus{
		{ContainerID: "c1", NodeName: "pve1", Name: "nginx", ImageTag: "nginx:1", Status: ptr("running")},
	}}

	h := NewHandlerWithDB(&mockDB{rows: rows}, testToken)
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
	h := NewHandlerWithDB(&mockDB{rows: &mockRows{}}, testToken)
	r := gin.New()
	r.GET("/nodes/:node", h.GetNode)

	w := doRequest(r, "GET", "/nodes/nonexistent")

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestGetNode_QueryError(t *testing.T) {
	h := NewHandlerWithDB(&mockDB{queryErr: errors.New("db error")}, testToken)
	r := gin.New()
	r.GET("/nodes/:node", h.GetNode)

	w := doRequest(r, "GET", "/nodes/pve1")

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestContainerStatusJSON(t *testing.T) {
	cs := containerStatus{
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
	cs := containerStatus{
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
	rows := &mockRows{data: []containerStatus{
		{ContainerID: "c1", NodeName: "pve1", Name: "nginx", ImageTag: "nginx:1", Status: ptr("running"), ComposeProject: "web"},
		{ContainerID: "c2", NodeName: "pve1", Name: "redis", ImageTag: "redis:7", Status: ptr("running"), ComposeProject: "web"},
		{ContainerID: "c3", NodeName: "pve1", Name: "postgres", ImageTag: "pg:16", Status: ptr("running"), ComposeProject: "db"},
	}}

	h := NewHandlerWithDB(&mockDB{rows: rows}, testToken)
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
	rows := &mockRows{data: []containerStatus{
		{ContainerID: "c1", NodeName: "pve1", Name: "nginx", ImageTag: "nginx:1", Status: ptr("running"), ComposeProject: ""},
	}}

	h := NewHandlerWithDB(&mockDB{rows: rows}, testToken)
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
	h := NewHandlerWithDB(&mockDB{rows: &mockRows{}}, testToken)
	r := gin.New()
	r.GET("/nodes/:node/stacks", h.GetNodeStacks)

	w := doRequest(r, "GET", "/nodes/ghost/stacks")

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// --- mock action row ---

type mockActionRow struct {
	ar  actionResponse
	err error
}

func (r *mockActionRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	*dest[0].(*string) = r.ar.CommandID
	*dest[1].(*string) = r.ar.NodeName
	*dest[2].(*string) = r.ar.Action
	*dest[3].(*string) = r.ar.Target
	*dest[4].(*[]byte) = marshalOrEmpty(r.ar.Params)
	*dest[5].(*string) = r.ar.Status
	*dest[6].(*string) = r.ar.Output
	*dest[7].(*int64) = r.ar.DurationMs
	*dest[8].(*string) = r.ar.CreatedAt
	*dest[9].(*string) = r.ar.UpdatedAt
	return nil
}

// --- helpers ---

func doJSONRequest(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

// --- action tests ---

func TestCreateAction_BadJSON(t *testing.T) {
	h := NewHandlerWithDB(&mockDB{}, testToken)
	r := gin.New()
	r.POST("/nodes/:node/actions", h.CreateAction)

	w := doJSONRequest(r, "POST", "/nodes/test/actions", "")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreateAction_UnknownAction(t *testing.T) {
	h := NewHandlerWithDB(&mockDB{}, testToken)
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
	ar := actionResponse{
		CommandID: "cmd-1",
		NodeName:  "test-node",
		Action:    "compose_update",
		Target:    "my-stack",
		Status:    "pending",
		CreatedAt: "2025-01-15 10:00:00+00",
		UpdatedAt: "2025-01-15 10:00:00+00",
	}

	h := NewHandlerWithDB(&mockDB{row: &mockActionRow{ar: ar}}, testToken)
	r := gin.New()
	r.POST("/nodes/:node/actions", h.CreateAction)

	w := doJSONRequest(r, "POST", "/nodes/test-node/actions", `{"action":"compose_update","target":"my-stack"}`)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}

	var result actionResponse
	json.Unmarshal(w.Body.Bytes(), &result)
	if result.CommandID != "cmd-1" {
		t.Errorf("expected cmd-1, got %s", result.CommandID)
	}
}

func TestCreateAction_DBError(t *testing.T) {
	h := NewHandlerWithDB(&mockDB{row: &mockRow{err: errors.New("db error")}}, testToken)
	r := gin.New()
	r.POST("/nodes/:node/actions", h.CreateAction)

	w := doJSONRequest(r, "POST", "/nodes/test/actions", `{"action":"compose_update"}`)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestListActions_QueryError(t *testing.T) {
	h := NewHandlerWithDB(&mockDB{queryErr: errors.New("db error")}, testToken)
	r := gin.New()
	r.GET("/nodes/:node/actions", h.ListActions)

	w := doRequest(r, "GET", "/nodes/test/actions")

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestListActions_Empty(t *testing.T) {
	h := NewHandlerWithDB(&mockDB{rows: &mockRows{}}, testToken)
	r := gin.New()
	r.GET("/nodes/:node/actions", h.ListActions)

	w := doRequest(r, "GET", "/nodes/test/actions")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result []actionResponse
	json.Unmarshal(w.Body.Bytes(), &result)
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d items", len(result))
	}
}

func TestGetAction_NotFound(t *testing.T) {
	h := NewHandlerWithDB(&mockDB{row: &mockRow{err: pgx.ErrNoRows}}, testToken)
	r := gin.New()
	r.GET("/nodes/:node/actions/:id", h.GetAction)

	w := doRequest(r, "GET", "/nodes/test/actions/abc123")

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestGetAction_Success(t *testing.T) {
	ar := actionResponse{
		CommandID: "cmd-1",
		NodeName:  "test-node",
		Action:    "compose_restart",
		Target:    "my-stack",
		Status:    "success",
		Output:    "done",
		CreatedAt: "2025-01-15 10:00:00+00",
		UpdatedAt: "2025-01-15 10:05:00+00",
	}

	h := NewHandlerWithDB(&mockDB{row: &mockActionRow{ar: ar}}, testToken)
	r := gin.New()
	r.GET("/nodes/:node/actions/:id", h.GetAction)

	w := doRequest(r, "GET", "/nodes/test-node/actions/cmd-1")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result actionResponse
	json.Unmarshal(w.Body.Bytes(), &result)
	if result.CommandID != "cmd-1" || result.Status != "success" {
		t.Errorf("unexpected result: %+v", result)
	}
}
