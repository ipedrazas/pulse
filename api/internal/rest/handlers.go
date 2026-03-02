package rest

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DB abstracts the database operations needed by the REST handlers.
type DB interface {
	Ping(ctx context.Context) error
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type Handler struct {
	db    DB
	token string
}

func NewHandler(pool *pgxpool.Pool, token string) *Handler {
	return &Handler{db: pool, token: token}
}

// NewHandlerWithDB creates a handler with an explicit DB interface (useful for testing).
func NewHandlerWithDB(db DB, token string) *Handler {
	return &Handler{db: db, token: token}
}

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	r.GET("/healthz", h.Healthz)

	auth := r.Group("/")
	auth.Use(BearerAuthMiddleware(h.token))
	auth.GET("/status", h.GetStatus)
	auth.GET("/status/:container", h.GetContainerStatus)
	auth.GET("/nodes", h.GetNodes)
	auth.GET("/nodes/:node", h.GetNode)
	auth.GET("/nodes/:node/stacks", h.GetNodeStacks)
	auth.POST("/nodes/:node/actions", h.CreateAction)
	auth.GET("/nodes/:node/actions", h.ListActions)
	auth.GET("/nodes/:node/actions/:id", h.GetAction)
}

// scannable is satisfied by both pgx.Rows (current row) and pgx.Row.
type scannable interface {
	Scan(dest ...any) error
}

func scanContainer(s scannable) (containerStatus, error) {
	var cs containerStatus
	var labelsJSON, envVarsJSON []byte
	err := s.Scan(
		&cs.ContainerID, &cs.NodeName, &cs.Name, &cs.ImageTag,
		&cs.Status, &cs.UptimeSeconds, &cs.LastSeen,
		&labelsJSON, &envVarsJSON, &cs.ComposeProject,
	)
	if err != nil {
		return cs, err
	}
	_ = json.Unmarshal(labelsJSON, &cs.Labels)
	_ = json.Unmarshal(envVarsJSON, &cs.EnvVars)
	return cs, nil
}

func (h *Handler) Healthz(c *gin.Context) {
	if err := h.db.Ping(c.Request.Context()); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "healthy"})
}

type containerStatus struct {
	ContainerID    string            `json:"container_id"`
	NodeName       string            `json:"node_name"`
	Name           string            `json:"name"`
	ImageTag       string            `json:"image_tag"`
	Status         *string           `json:"status"`
	UptimeSeconds  *int64            `json:"uptime_seconds"`
	LastSeen       *string           `json:"last_seen"`
	Labels         map[string]string `json:"labels"`
	EnvVars        map[string]string `json:"env_vars"`
	ComposeProject string            `json:"compose_project"`
}

const statusQuery = `
SELECT
  c.container_id,
  c.node_name,
  c.name,
  c.image_tag,
  h.status,
  h.uptime_seconds,
  h.time::text AS last_seen,
  c.labels,
  c.env_vars,
  c.compose_project
FROM containers c
LEFT JOIN LATERAL (
  SELECT status, uptime_seconds, time
  FROM container_heartbeats
  WHERE container_id = c.container_id
  ORDER BY time DESC
  LIMIT 1
) h ON true
WHERE c.removed_at IS NULL`

func (h *Handler) GetStatus(c *gin.Context) {
	rows, err := h.db.Query(c.Request.Context(), statusQuery)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	results := []containerStatus{}
	for rows.Next() {
		cs, err := scanContainer(rows)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		results = append(results, cs)
	}

	c.JSON(http.StatusOK, results)
}

func (h *Handler) GetContainerStatus(c *gin.Context) {
	containerID := c.Param("container")

	row := h.db.QueryRow(c.Request.Context(), statusQuery+" AND c.container_id = $1", containerID)

	cs, err := scanContainer(row)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "container not found"})
		return
	}

	c.JSON(http.StatusOK, cs)
}

type nodeContainers struct {
	NodeName   string            `json:"node_name"`
	Containers []containerStatus `json:"containers"`
}

func (h *Handler) GetNodes(c *gin.Context) {
	rows, err := h.db.Query(c.Request.Context(), statusQuery+" ORDER BY c.node_name, c.name")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	grouped := map[string][]containerStatus{}
	var order []string
	for rows.Next() {
		cs, err := scanContainer(rows)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if _, exists := grouped[cs.NodeName]; !exists {
			order = append(order, cs.NodeName)
		}
		grouped[cs.NodeName] = append(grouped[cs.NodeName], cs)
	}

	results := make([]nodeContainers, 0, len(grouped))
	for _, node := range order {
		results = append(results, nodeContainers{NodeName: node, Containers: grouped[node]})
	}

	c.JSON(http.StatusOK, results)
}

func (h *Handler) GetNode(c *gin.Context) {
	node := c.Param("node")

	rows, err := h.db.Query(c.Request.Context(), statusQuery+" AND c.node_name = $1 ORDER BY c.name", node)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	containers := []containerStatus{}
	for rows.Next() {
		cs, err := scanContainer(rows)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		containers = append(containers, cs)
	}

	if len(containers) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
		return
	}

	c.JSON(http.StatusOK, nodeContainers{NodeName: node, Containers: containers})
}

type composeStack struct {
	Project    string            `json:"project"`
	Containers []containerStatus `json:"containers"`
}

func (h *Handler) GetNodeStacks(c *gin.Context) {
	node := c.Param("node")

	rows, err := h.db.Query(c.Request.Context(), statusQuery+" AND c.node_name = $1 ORDER BY c.compose_project, c.name", node)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	grouped := map[string][]containerStatus{}
	var order []string
	for rows.Next() {
		cs, err := scanContainer(rows)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		project := cs.ComposeProject
		if project == "" {
			project = "(standalone)"
		}
		if _, exists := grouped[project]; !exists {
			order = append(order, project)
		}
		grouped[project] = append(grouped[project], cs)
	}

	if len(grouped) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
		return
	}

	results := make([]composeStack, 0, len(grouped))
	for _, project := range order {
		results = append(results, composeStack{Project: project, Containers: grouped[project]})
	}

	c.JSON(http.StatusOK, results)
}

// --- Actions (command dispatch) ---

var allowedActions = map[string]bool{
	"compose_update":  true,
	"compose_restart": true,
}

type createActionRequest struct {
	Action string            `json:"action" binding:"required"`
	Target string            `json:"target"`
	Params map[string]string `json:"params"`
}

type actionResponse struct {
	CommandID  string            `json:"command_id"`
	NodeName   string            `json:"node_name"`
	Action     string            `json:"action"`
	Target     string            `json:"target"`
	Params     map[string]string `json:"params"`
	Status     string            `json:"status"`
	Output     string            `json:"output"`
	DurationMs int64             `json:"duration_ms"`
	CreatedAt  string            `json:"created_at"`
	UpdatedAt  string            `json:"updated_at"`
}

func (h *Handler) CreateAction(c *gin.Context) {
	node := c.Param("node")

	var req createActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "action is required"})
		return
	}

	if !allowedActions[req.Action] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown action: " + req.Action})
		return
	}

	paramsJSON, _ := json.Marshal(req.Params)

	row := h.db.QueryRow(c.Request.Context(),
		`INSERT INTO commands (node_name, action, target, params)
		 VALUES ($1, $2, $3, $4)
		 RETURNING command_id, node_name, action, target, params, status, output, duration_ms,
		           created_at::text, updated_at::text`,
		node, req.Action, req.Target, paramsJSON,
	)

	ar, err := scanAction(row)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, ar)
}

func (h *Handler) ListActions(c *gin.Context) {
	node := c.Param("node")

	rows, err := h.db.Query(c.Request.Context(),
		`SELECT command_id, node_name, action, target, params, status, output, duration_ms,
		        created_at::text, updated_at::text
		 FROM commands
		 WHERE node_name = $1
		 ORDER BY created_at DESC
		 LIMIT 50`,
		node,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	results := []actionResponse{}
	for rows.Next() {
		ar, err := scanAction(rows)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		results = append(results, ar)
	}

	c.JSON(http.StatusOK, results)
}

func (h *Handler) GetAction(c *gin.Context) {
	node := c.Param("node")
	id := c.Param("id")

	row := h.db.QueryRow(c.Request.Context(),
		`SELECT command_id, node_name, action, target, params, status, output, duration_ms,
		        created_at::text, updated_at::text
		 FROM commands
		 WHERE command_id = $1 AND node_name = $2`,
		id, node,
	)

	ar, err := scanAction(row)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "action not found"})
		return
	}

	c.JSON(http.StatusOK, ar)
}

func scanAction(s scannable) (actionResponse, error) {
	var ar actionResponse
	var paramsJSON []byte
	err := s.Scan(
		&ar.CommandID, &ar.NodeName, &ar.Action, &ar.Target,
		&paramsJSON, &ar.Status, &ar.Output, &ar.DurationMs,
		&ar.CreatedAt, &ar.UpdatedAt,
	)
	if err != nil {
		return ar, err
	}
	_ = json.Unmarshal(paramsJSON, &ar.Params)
	return ar, nil
}
