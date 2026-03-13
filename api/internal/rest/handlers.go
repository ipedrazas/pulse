package rest

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/ipedrazas/pulse/api/internal/repository"
	"github.com/ipedrazas/pulse/api/internal/version"
)

// CommandSender can send commands to connected agents.
type CommandSender interface {
	SendCommand(nodeName string, cmdID string, cmdType string, payload json.RawMessage) error
}

type Handler struct {
	repo   repository.Repository
	sender CommandSender
}

func NewHandler(repo repository.Repository, sender CommandSender) *Handler {
	return &Handler{repo: repo, sender: sender}
}

func (h *Handler) Register(r *gin.Engine) {
	r.GET("/healthz", h.healthz)
	r.GET("/info", h.info)

	api := r.Group("/api/v1")
	api.GET("/nodes", h.listNodes)
	api.GET("/nodes/:name", h.getNode)
	api.DELETE("/nodes/:name", h.deleteNode)
	api.GET("/containers", h.listContainers)
	api.GET("/containers/:id", h.getContainer)
	api.POST("/containers/:id/logs", h.requestContainerLogs)
	api.POST("/containers/:id/stop", h.stopContainer)
	api.POST("/containers/:id/restart", h.restartContainer)
	api.POST("/containers/:id/pull", h.pullContainerImage)
	api.POST("/commands", h.createCommand)
	api.GET("/commands/:id", h.getCommand)
}

func (h *Handler) healthz(c *gin.Context) {
	if err := h.repo.Ping(c.Request.Context()); err != nil {
		slog.Error("healthz: database unreachable", "error", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "degraded", "error": "database unreachable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) info(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"version":    version.Version,
		"commit":     version.Commit,
		"build_date": version.BuildDate,
	})
}

func (h *Handler) listNodes(c *gin.Context) {
	agents, err := h.repo.ListAgents(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	type nodeResponse struct {
		Name           string                   `json:"name"`
		Status         string                   `json:"status"`
		Version        string                   `json:"version"`
		LastSeen       *string                  `json:"last_seen,omitempty"`
		ContainerCount int                      `json:"container_count"`
		Metadata       *repository.NodeMetadata `json:"metadata,omitempty"`
	}

	var nodes []nodeResponse
	for _, a := range agents {
		n := nodeResponse{Name: a.Name, Status: a.Status, Version: a.Version, Metadata: a.Metadata}
		if a.LastSeen != nil {
			s := a.LastSeen.Format("2006-01-02T15:04:05Z07:00")
			n.LastSeen = &s
		}
		_, count, _ := h.repo.ListContainers(c.Request.Context(), a.Name, 0, 0)
		n.ContainerCount = count
		nodes = append(nodes, n)
	}
	c.JSON(http.StatusOK, nodes)
}

func (h *Handler) getNode(c *gin.Context) {
	name := c.Param("name")
	agent, err := h.repo.GetAgent(c.Request.Context(), name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if agent == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		return
	}

	containers, _, err := h.repo.ListContainers(c.Request.Context(), name, 100, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"agent":      agent,
		"containers": containers,
	})
}

func (h *Handler) deleteNode(c *gin.Context) {
	name := c.Param("name")
	if err := h.repo.DeleteAgent(c.Request.Context(), name); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": name})
}

func (h *Handler) listContainers(c *gin.Context) {
	agentName := c.Query("node")
	pageSize := 50
	if v := c.Query("page_size"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			pageSize = n
		}
	}
	offset := 0
	if v := c.Query("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	containers, total, err := h.repo.ListContainers(c.Request.Context(), agentName, pageSize, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"containers": containers,
		"total":      total,
		"page_size":  pageSize,
		"offset":     offset,
	})
}

func (h *Handler) getContainer(c *gin.Context) {
	id := c.Param("id")
	container, err := h.repo.GetContainer(c.Request.Context(), id, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if container == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "container not found"})
		return
	}
	c.JSON(http.StatusOK, container)
}

func (h *Handler) createCommand(c *gin.Context) {
	var req struct {
		NodeName string          `json:"node_name" binding:"required"`
		Type     string          `json:"type" binding:"required"`
		Payload  json.RawMessage `json:"payload"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cmd := repository.Command{
		ID:        uuid.New().String(),
		AgentName: req.NodeName,
		Type:      req.Type,
		Payload:   req.Payload,
		Status:    "pending",
	}
	if err := h.repo.CreateCommand(c.Request.Context(), cmd); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if h.sender != nil {
		if err := h.sender.SendCommand(req.NodeName, cmd.ID, cmd.Type, cmd.Payload); err != nil {
			slog.Info("agent not connected, command queued", "node", req.NodeName, "id", cmd.ID)
		}
	}

	c.JSON(http.StatusAccepted, gin.H{
		"command_id": cmd.ID,
		"status":     "pending",
	})
}

func (h *Handler) getCommand(c *gin.Context) {
	id := c.Param("id")
	cmd, err := h.repo.GetCommand(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if cmd == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "command not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"command_id": cmd.ID,
		"status":     cmd.Status,
		"result":     cmd.Result,
	})
}

// sendAgentCommand creates a command for the given agent, sends it immediately if possible, and returns 202.
func (h *Handler) sendAgentCommand(c *gin.Context, agentName, cmdType string, payload json.RawMessage) {
	cmd := repository.Command{
		ID:        uuid.New().String(),
		AgentName: agentName,
		Type:      cmdType,
		Payload:   payload,
		Status:    "pending",
	}
	if err := h.repo.CreateCommand(c.Request.Context(), cmd); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if h.sender != nil {
		if err := h.sender.SendCommand(agentName, cmd.ID, cmd.Type, payload); err != nil {
			slog.Info("agent not connected, command queued", "node", agentName, "id", cmd.ID)
		}
	}

	c.JSON(http.StatusAccepted, gin.H{
		"command_id": cmd.ID,
		"status":     "pending",
	})
}

// getContainerOrFail looks up the container by the :id param, returning nil and writing an error response on failure.
func (h *Handler) getContainerOrFail(c *gin.Context) *repository.Container {
	containerID := c.Param("id")
	container, err := h.repo.GetContainer(c.Request.Context(), containerID, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return nil
	}
	if container == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "container not found"})
		return nil
	}
	return container
}

func (h *Handler) requestContainerLogs(c *gin.Context) {
	container := h.getContainerOrFail(c)
	if container == nil {
		return
	}

	var req struct {
		Tail int `json:"tail"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.Tail <= 0 {
		req.Tail = 100
	}

	payload, _ := json.Marshal(map[string]any{
		"container_id": container.ContainerID,
		"tail":         req.Tail,
		"follow":       false,
	})
	h.sendAgentCommand(c, container.AgentName, "request_logs", payload)
}

func (h *Handler) stopContainer(c *gin.Context) {
	container := h.getContainerOrFail(c)
	if container == nil {
		return
	}
	payload, _ := json.Marshal(map[string]any{
		"container_id":    container.ContainerID,
		"timeout_seconds": 10,
	})
	h.sendAgentCommand(c, container.AgentName, "stop_container", payload)
}

func (h *Handler) restartContainer(c *gin.Context) {
	container := h.getContainerOrFail(c)
	if container == nil {
		return
	}

	// Compose containers: docker compose up -d --pull=always
	if isComposeContainer(container) {
		payload, _ := json.Marshal(composePayload(container))
		h.sendAgentCommand(c, container.AgentName, "compose_up", payload)
		return
	}

	payload, _ := json.Marshal(map[string]any{
		"container_id":    container.ContainerID,
		"timeout_seconds": 10,
	})
	h.sendAgentCommand(c, container.AgentName, "restart_container", payload)
}

func (h *Handler) pullContainerImage(c *gin.Context) {
	container := h.getContainerOrFail(c)
	if container == nil {
		return
	}

	// Compose containers: docker compose up -d --pull=always
	if isComposeContainer(container) {
		payload, _ := json.Marshal(composePayload(container))
		h.sendAgentCommand(c, container.AgentName, "compose_up", payload)
		return
	}

	payload, _ := json.Marshal(map[string]any{
		"image": container.Image,
	})
	h.sendAgentCommand(c, container.AgentName, "pull_image", payload)
}

func isComposeContainer(c *repository.Container) bool {
	return c.Labels["com.docker.compose.project.working_dir"] != "" &&
		c.Labels["com.docker.compose.project.config_files"] != ""
}

func composePayload(c *repository.Container) map[string]any {
	return map[string]any{
		"project_dir": c.Labels["com.docker.compose.project.working_dir"],
		"file":        c.Labels["com.docker.compose.project.config_files"],
		"detach":      true,
		"pull":        true,
	}
}
