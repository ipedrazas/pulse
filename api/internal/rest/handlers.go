package rest

import (
	"context"
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
	db DB
}

func NewHandler(pool *pgxpool.Pool) *Handler {
	return &Handler{db: pool}
}

// NewHandlerWithDB creates a handler with an explicit DB interface (useful for testing).
func NewHandlerWithDB(db DB) *Handler {
	return &Handler{db: db}
}

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	r.GET("/healthz", h.Healthz)
	r.GET("/status", h.GetStatus)
	r.GET("/status/:container", h.GetContainerStatus)
	r.GET("/nodes", h.GetNodes)
	r.GET("/nodes/:node", h.GetNode)
}

func (h *Handler) Healthz(c *gin.Context) {
	if err := h.db.Ping(c.Request.Context()); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "healthy"})
}

type containerStatus struct {
	ContainerID   string  `json:"container_id"`
	NodeName      string  `json:"node_name"`
	Name          string  `json:"name"`
	ImageTag      string  `json:"image_tag"`
	Status        *string `json:"status"`
	UptimeSeconds *int64  `json:"uptime_seconds"`
	LastSeen      *string `json:"last_seen"`
}

const statusQuery = `
SELECT
  c.container_id,
  c.node_name,
  c.name,
  c.image_tag,
  h.status,
  h.uptime_seconds,
  h.time::text AS last_seen
FROM containers c
LEFT JOIN LATERAL (
  SELECT status, uptime_seconds, time
  FROM container_heartbeats
  WHERE container_id = c.container_id
  ORDER BY time DESC
  LIMIT 1
) h ON true`

func (h *Handler) GetStatus(c *gin.Context) {
	rows, err := h.db.Query(c.Request.Context(), statusQuery)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	results := []containerStatus{}
	for rows.Next() {
		var cs containerStatus
		if err := rows.Scan(&cs.ContainerID, &cs.NodeName, &cs.Name, &cs.ImageTag, &cs.Status, &cs.UptimeSeconds, &cs.LastSeen); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		results = append(results, cs)
	}

	c.JSON(http.StatusOK, results)
}

func (h *Handler) GetContainerStatus(c *gin.Context) {
	containerID := c.Param("container")

	row := h.db.QueryRow(c.Request.Context(), statusQuery+" WHERE c.container_id = $1", containerID)

	var cs containerStatus
	if err := row.Scan(&cs.ContainerID, &cs.NodeName, &cs.Name, &cs.ImageTag, &cs.Status, &cs.UptimeSeconds, &cs.LastSeen); err != nil {
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
		var cs containerStatus
		if err := rows.Scan(&cs.ContainerID, &cs.NodeName, &cs.Name, &cs.ImageTag, &cs.Status, &cs.UptimeSeconds, &cs.LastSeen); err != nil {
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

	rows, err := h.db.Query(c.Request.Context(), statusQuery+" WHERE c.node_name = $1 ORDER BY c.name", node)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	containers := []containerStatus{}
	for rows.Next() {
		var cs containerStatus
		if err := rows.Scan(&cs.ContainerID, &cs.NodeName, &cs.Name, &cs.ImageTag, &cs.Status, &cs.UptimeSeconds, &cs.LastSeen); err != nil {
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
