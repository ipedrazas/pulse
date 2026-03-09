package repository

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// --- Agents ---

func (r *PostgresRepository) UpsertAgent(ctx context.Context, a Agent) error {
	var metadataJSON []byte
	if a.Metadata != nil {
		metadataJSON, _ = json.Marshal(a.Metadata)
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO agents (name, status, version, last_seen, metadata)
		VALUES ($1, $2, $3, $4, COALESCE($5::jsonb, '{}'::jsonb))
		ON CONFLICT (name) DO UPDATE SET
			status = EXCLUDED.status,
			version = EXCLUDED.version,
			last_seen = EXCLUDED.last_seen,
			metadata = CASE WHEN $5::jsonb IS NOT NULL THEN $5::jsonb ELSE agents.metadata END`,
		a.Name, a.Status, a.Version, a.LastSeen, metadataJSON)
	return err
}

func (r *PostgresRepository) GetAgent(ctx context.Context, name string) (*Agent, error) {
	var a Agent
	var metadataJSON []byte
	err := r.pool.QueryRow(ctx, `
		SELECT name, status, version, last_seen, created_at, metadata
		FROM agents WHERE name = $1`, name).
		Scan(&a.Name, &a.Status, &a.Version, &a.LastSeen, &a.CreatedAt, &metadataJSON)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(metadataJSON) > 0 {
		var m NodeMetadata
		json.Unmarshal(metadataJSON, &m)
		a.Metadata = &m
	}
	return &a, nil
}

func (r *PostgresRepository) ListAgents(ctx context.Context) ([]Agent, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT name, status, version, last_seen, created_at, metadata
		FROM agents ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []Agent
	for rows.Next() {
		var a Agent
		var metadataJSON []byte
		if err := rows.Scan(&a.Name, &a.Status, &a.Version, &a.LastSeen, &a.CreatedAt, &metadataJSON); err != nil {
			return nil, err
		}
		if len(metadataJSON) > 0 {
			var m NodeMetadata
			json.Unmarshal(metadataJSON, &m)
			a.Metadata = &m
		}
		agents = append(agents, a)
	}
	return agents, rows.Err()
}

func (r *PostgresRepository) SetAgentStatus(ctx context.Context, name, status string) error {
	_, err := r.pool.Exec(ctx, `UPDATE agents SET status = $1 WHERE name = $2`, status, name)
	return err
}

// --- Containers ---

func (r *PostgresRepository) UpsertContainer(ctx context.Context, c Container) error {
	envJSON, _ := json.Marshal(c.EnvVars)
	mountsJSON, _ := json.Marshal(c.Mounts)
	labelsJSON, _ := json.Marshal(c.Labels)
	portsJSON, _ := json.Marshal(c.Ports)

	_, err := r.pool.Exec(ctx, `
		INSERT INTO containers (container_id, agent_name, name, image, status, env_vars, mounts, labels, ports, compose_project, command, uptime_seconds)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (container_id, agent_name) DO UPDATE SET
			name = EXCLUDED.name,
			image = EXCLUDED.image,
			status = EXCLUDED.status,
			env_vars = EXCLUDED.env_vars,
			mounts = EXCLUDED.mounts,
			labels = EXCLUDED.labels,
			ports = EXCLUDED.ports,
			compose_project = EXCLUDED.compose_project,
			command = EXCLUDED.command,
			uptime_seconds = EXCLUDED.uptime_seconds,
			removed_at = NULL`,
		c.ContainerID, c.AgentName, c.Name, c.Image, c.Status,
		envJSON, mountsJSON, labelsJSON, portsJSON,
		c.ComposeProject, c.Command, c.UptimeSeconds)
	return err
}

func (r *PostgresRepository) GetContainer(ctx context.Context, containerID, agentName string) (*Container, error) {
	var c Container
	var envJSON, mountsJSON, labelsJSON, portsJSON []byte

	query := `SELECT container_id, agent_name, name, image, status, env_vars, mounts, labels, ports,
			compose_project, command, uptime_seconds, created_at, removed_at
		FROM containers WHERE container_id = $1`
	args := []any{containerID}

	if agentName != "" {
		query += " AND agent_name = $2"
		args = append(args, agentName)
	}

	err := r.pool.QueryRow(ctx, query, args...).
		Scan(&c.ContainerID, &c.AgentName, &c.Name, &c.Image, &c.Status,
			&envJSON, &mountsJSON, &labelsJSON, &portsJSON,
			&c.ComposeProject, &c.Command, &c.UptimeSeconds, &c.CreatedAt, &c.RemovedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	json.Unmarshal(envJSON, &c.EnvVars)
	json.Unmarshal(mountsJSON, &c.Mounts)
	json.Unmarshal(labelsJSON, &c.Labels)
	json.Unmarshal(portsJSON, &c.Ports)
	return &c, nil
}

func (r *PostgresRepository) ListContainers(ctx context.Context, agentName string, limit, offset int) ([]Container, int, error) {
	// Count query
	countQuery := `SELECT COUNT(*) FROM containers WHERE removed_at IS NULL`
	countArgs := []any{}
	if agentName != "" {
		countQuery += " AND agent_name = $1"
		countArgs = append(countArgs, agentName)
	}

	var total int
	if err := r.pool.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Data query
	query := `SELECT container_id, agent_name, name, image, status, env_vars, mounts, labels, ports,
		compose_project, command, uptime_seconds, created_at, removed_at
		FROM containers WHERE removed_at IS NULL`
	args := []any{}
	argIdx := 1

	if agentName != "" {
		query += " AND agent_name = $" + itoa(argIdx)
		args = append(args, agentName)
		argIdx++
	}
	query += " ORDER BY name"
	query += " LIMIT $" + itoa(argIdx) + " OFFSET $" + itoa(argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var containers []Container
	for rows.Next() {
		var c Container
		var envJSON, mountsJSON, labelsJSON, portsJSON []byte
		if err := rows.Scan(&c.ContainerID, &c.AgentName, &c.Name, &c.Image, &c.Status,
			&envJSON, &mountsJSON, &labelsJSON, &portsJSON,
			&c.ComposeProject, &c.Command, &c.UptimeSeconds, &c.CreatedAt, &c.RemovedAt); err != nil {
			return nil, 0, err
		}
		json.Unmarshal(envJSON, &c.EnvVars)
		json.Unmarshal(mountsJSON, &c.Mounts)
		json.Unmarshal(labelsJSON, &c.Labels)
		json.Unmarshal(portsJSON, &c.Ports)
		containers = append(containers, c)
	}
	return containers, total, rows.Err()
}

func (r *PostgresRepository) MarkContainersRemoved(ctx context.Context, agentName string, activeIDs []string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE containers SET removed_at = NOW()
		WHERE agent_name = $1 AND removed_at IS NULL AND container_id != ALL($2)`,
		agentName, activeIDs)
	return err
}

// --- Container Events ---

func (r *PostgresRepository) InsertContainerEvent(ctx context.Context, e ContainerEvent) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO container_events (time, container_id, agent_name, status, uptime_seconds)
		VALUES ($1, $2, $3, $4, $5)`,
		e.Time, e.ContainerID, e.AgentName, e.Status, e.UptimeSeconds)
	return err
}

// --- Commands ---

func (r *PostgresRepository) CreateCommand(ctx context.Context, cmd Command) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO commands (id, agent_name, type, payload, status)
		VALUES ($1, $2, $3, $4, $5)`,
		cmd.ID, cmd.AgentName, cmd.Type, cmd.Payload, cmd.Status)
	return err
}

func (r *PostgresRepository) GetCommand(ctx context.Context, id string) (*Command, error) {
	var c Command
	err := r.pool.QueryRow(ctx, `
		SELECT id, agent_name, type, payload, status, result, created_at, completed_at
		FROM commands WHERE id = $1`, id).
		Scan(&c.ID, &c.AgentName, &c.Type, &c.Payload, &c.Status, &c.Result, &c.CreatedAt, &c.CompletedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *PostgresRepository) GetPendingCommands(ctx context.Context, agentName string) ([]Command, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, agent_name, type, payload, status, result, created_at, completed_at
		FROM commands WHERE agent_name = $1 AND status = 'pending'
		ORDER BY created_at`, agentName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cmds []Command
	for rows.Next() {
		var c Command
		if err := rows.Scan(&c.ID, &c.AgentName, &c.Type, &c.Payload, &c.Status, &c.Result, &c.CreatedAt, &c.CompletedAt); err != nil {
			return nil, err
		}
		cmds = append(cmds, c)
	}
	return cmds, rows.Err()
}

func (r *PostgresRepository) CompleteCommand(ctx context.Context, id, result string, success bool) error {
	status := "failed"
	if success {
		status = "completed"
	}
	now := time.Now()
	_, err := r.pool.Exec(ctx, `
		UPDATE commands SET status = $1, result = $2, completed_at = $3
		WHERE id = $4`,
		status, result, now, id)
	return err
}

func itoa(i int) string {
	return strconv.Itoa(i)
}
