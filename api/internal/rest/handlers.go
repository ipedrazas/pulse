package rest

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/ipedrazas/pulse/api/internal/repository"
	"github.com/ipedrazas/pulse/api/internal/version"
)

type Handler struct {
	repo repository.Repository
}

func NewHandler(repo repository.Repository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) Register(r *gin.Engine) {
	r.GET("/healthz", h.healthz)
	r.GET("/info", h.info)

	api := r.Group("/api/v1")
	api.GET("/nodes", h.listNodes)
	api.GET("/nodes/:name", h.getNode)
	api.GET("/containers", h.listContainers)
	api.GET("/containers/:id", h.getContainer)
	api.POST("/commands", h.createCommand)
}

func (h *Handler) healthz(c *gin.Context) {
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

	c.JSON(http.StatusAccepted, gin.H{
		"command_id": cmd.ID,
		"status":     "pending",
	})
}
