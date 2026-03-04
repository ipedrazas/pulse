package rest

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/ipedrazas/pulse/api/internal/repository"
)

// apiError returns a JSON error response with a machine-readable code.
func apiError(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"error": message, "code": code})
}

// parsePagination reads ?limit= and ?offset= query parameters.
// Returns 0 for unset/invalid values (meaning "no limit" / "no offset").
func parsePagination(c *gin.Context) (limit, offset int) {
	if v := c.Query("limit"); v != "" {
		limit, _ = strconv.Atoi(v)
	}
	if v := c.Query("offset"); v != "" {
		offset, _ = strconv.Atoi(v)
	}
	return
}

type Handler struct {
	containers repository.ContainerRepository
	actions    repository.ActionRepository
	agents     repository.AgentRepository
	health     repository.HealthChecker
	token      string
}

func NewHandler(repo *repository.PostgresRepo, token string) *Handler {
	return &Handler{
		containers: repo,
		actions:    repo,
		agents:     repo,
		health:     repo,
		token:      token,
	}
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
	auth.GET("/agents", h.GetAgents)
}

func (h *Handler) Healthz(c *gin.Context) {
	if err := h.health.Ping(c.Request.Context()); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "healthy"})
}

func (h *Handler) GetStatus(c *gin.Context) {
	limit, offset := parsePagination(c)
	results, err := h.containers.ListContainers(c.Request.Context(), limit, offset)
	if err != nil {
		slog.Error("failed to list containers", "error", err)
		apiError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		return
	}
	if results == nil {
		results = []repository.ContainerStatus{}
	}
	c.JSON(http.StatusOK, results)
}

func (h *Handler) GetContainerStatus(c *gin.Context) {
	containerID := c.Param("container")

	cs, err := h.containers.GetContainer(c.Request.Context(), containerID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			apiError(c, http.StatusNotFound, "NOT_FOUND", "container not found")
			return
		}
		slog.Error("failed to get container", "container_id", containerID, "error", err)
		apiError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		return
	}

	c.JSON(http.StatusOK, cs)
}

type nodeContainers struct {
	NodeName   string                       `json:"node_name"`
	Containers []repository.ContainerStatus `json:"containers"`
}

func (h *Handler) GetNodes(c *gin.Context) {
	limit, offset := parsePagination(c)
	results, err := h.containers.ListContainers(c.Request.Context(), limit, offset)
	if err != nil {
		slog.Error("failed to list containers", "error", err)
		apiError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		return
	}

	grouped := map[string][]repository.ContainerStatus{}
	var order []string
	for _, cs := range results {
		if _, exists := grouped[cs.NodeName]; !exists {
			order = append(order, cs.NodeName)
		}
		grouped[cs.NodeName] = append(grouped[cs.NodeName], cs)
	}

	nodes := make([]nodeContainers, 0, len(grouped))
	for _, node := range order {
		nodes = append(nodes, nodeContainers{NodeName: node, Containers: grouped[node]})
	}

	c.JSON(http.StatusOK, nodes)
}

func (h *Handler) GetNode(c *gin.Context) {
	node := c.Param("node")
	limit, offset := parsePagination(c)

	containers, err := h.containers.ListContainersByNode(c.Request.Context(), node, limit, offset)
	if err != nil {
		slog.Error("failed to list containers by node", "node", node, "error", err)
		apiError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		return
	}

	if len(containers) == 0 {
		apiError(c, http.StatusNotFound, "NOT_FOUND", "node not found")
		return
	}

	c.JSON(http.StatusOK, nodeContainers{NodeName: node, Containers: containers})
}

type composeStack struct {
	Project    string                       `json:"project"`
	Containers []repository.ContainerStatus `json:"containers"`
}

func (h *Handler) GetNodeStacks(c *gin.Context) {
	node := c.Param("node")
	limit, offset := parsePagination(c)

	containers, err := h.containers.ListContainersByNodeForStacks(c.Request.Context(), node, limit, offset)
	if err != nil {
		slog.Error("failed to list containers for stacks", "node", node, "error", err)
		apiError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		return
	}

	grouped := map[string][]repository.ContainerStatus{}
	var order []string
	for _, cs := range containers {
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
		apiError(c, http.StatusNotFound, "NOT_FOUND", "node not found")
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
	"compose_update":    true,
	"compose_restart":   true,
	"container_stop":    true,
	"container_start":   true,
	"container_restart": true,
	"container_logs":    true,
	"container_inspect": true,
}

type createActionRequest struct {
	Action string            `json:"action" binding:"required"`
	Target string            `json:"target"`
	Params map[string]string `json:"params"`
}

func (h *Handler) CreateAction(c *gin.Context) {
	node := c.Param("node")

	var req createActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiError(c, http.StatusBadRequest, "BAD_REQUEST", "action is required")
		return
	}

	if !allowedActions[req.Action] {
		apiError(c, http.StatusBadRequest, "BAD_REQUEST", "unknown action: "+req.Action)
		return
	}

	// For compose actions, look up the working directory from containers in the project.
	if req.Action == "compose_update" || req.Action == "compose_restart" {
		composeDir, err := h.containers.GetComposeDir(c.Request.Context(), node, req.Target)
		if err == nil && composeDir != "" {
			if req.Params == nil {
				req.Params = make(map[string]string)
			}
			req.Params["compose_dir"] = composeDir
		}
		slog.Info("CreateAction: looked up compose_dir", "node", node, "project", req.Target, "dir", composeDir, "error", err)
	}

	paramsJSON, _ := json.Marshal(req.Params)

	ar, err := h.actions.CreateAction(c.Request.Context(), node, req.Action, req.Target, paramsJSON)
	if err != nil {
		slog.Error("failed to create action", "error", err)
		apiError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		return
	}

	c.JSON(http.StatusCreated, ar)
}

func (h *Handler) ListActions(c *gin.Context) {
	node := c.Param("node")
	limit, offset := parsePagination(c)

	results, err := h.actions.ListActions(c.Request.Context(), node, limit, offset)
	if err != nil {
		slog.Error("failed to list actions", "node", node, "error", err)
		apiError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		return
	}
	if results == nil {
		results = []repository.ActionResponse{}
	}
	c.JSON(http.StatusOK, results)
}

func (h *Handler) GetAction(c *gin.Context) {
	node := c.Param("node")
	id := c.Param("id")

	ar, err := h.actions.GetAction(c.Request.Context(), id, node)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			apiError(c, http.StatusNotFound, "NOT_FOUND", "action not found")
			return
		}
		slog.Error("failed to get action", "command_id", id, "error", err)
		apiError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		return
	}

	c.JSON(http.StatusOK, ar)
}

// --- Agents ---

func (h *Handler) GetAgents(c *gin.Context) {
	results, err := h.agents.ListAgents(c.Request.Context())
	if err != nil {
		slog.Error("failed to list agents", "error", err)
		apiError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		return
	}
	if results == nil {
		results = []repository.AgentStatus{}
	}
	c.JSON(http.StatusOK, results)
}
