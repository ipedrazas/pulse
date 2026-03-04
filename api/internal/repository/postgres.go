package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

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

// scannable is satisfied by both pgx.Rows (current row) and pgx.Row.
type scannable interface {
	Scan(dest ...any) error
}

// PostgresRepo implements all repository interfaces using a pgx connection pool.
type PostgresRepo struct {
	pool *pgxpool.Pool
}

// NewPostgresRepo creates a new PostgresRepo.
func NewPostgresRepo(pool *pgxpool.Pool) *PostgresRepo {
	return &PostgresRepo{pool: pool}
}

// Ping checks the database connection.
func (r *PostgresRepo) Ping(ctx context.Context) error {
	return r.pool.Ping(ctx)
}

func scanContainerStatus(s scannable) (ContainerStatus, error) {
	var cs ContainerStatus
	var labelsJSON, envVarsJSON []byte
	err := s.Scan(
		&cs.ContainerID, &cs.NodeName, &cs.Name, &cs.ImageTag,
		&cs.Status, &cs.UptimeSeconds, &cs.LastSeen,
		&labelsJSON, &envVarsJSON, &cs.ComposeProject,
	)
	if err != nil {
		return cs, err
	}
	if err := json.Unmarshal(labelsJSON, &cs.Labels); err != nil && len(labelsJSON) > 0 {
		slog.Warn("failed to unmarshal container labels", "container_id", cs.ContainerID, "error", err)
	}
	if err := json.Unmarshal(envVarsJSON, &cs.EnvVars); err != nil && len(envVarsJSON) > 0 {
		slog.Warn("failed to unmarshal container env_vars", "container_id", cs.ContainerID, "error", err)
	}
	return cs, nil
}

func scanActionResponse(s scannable) (ActionResponse, error) {
	var ar ActionResponse
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

// paginationClause returns a SQL LIMIT/OFFSET suffix.
// A limit of 0 means no limit.
func paginationClause(limit, offset int) string {
	if limit <= 0 && offset <= 0 {
		return ""
	}
	if limit <= 0 {
		return fmt.Sprintf(" OFFSET %d", offset)
	}
	if offset <= 0 {
		return fmt.Sprintf(" LIMIT %d", limit)
	}
	return fmt.Sprintf(" LIMIT %d OFFSET %d", limit, offset)
}

// --- ContainerRepository ---

func (r *PostgresRepo) ListContainers(ctx context.Context, limit, offset int) ([]ContainerStatus, error) {
	q := statusQuery + " ORDER BY c.node_name, c.name" + paginationClause(limit, offset)
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []ContainerStatus
	for rows.Next() {
		cs, err := scanContainerStatus(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, cs)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

func (r *PostgresRepo) GetContainer(ctx context.Context, containerID string) (ContainerStatus, error) {
	row := r.pool.QueryRow(ctx, statusQuery+" AND c.container_id = $1", containerID)
	cs, err := scanContainerStatus(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return cs, ErrNotFound
		}
		return cs, err
	}
	return cs, nil
}

func (r *PostgresRepo) ListContainersByNode(ctx context.Context, nodeName string, limit, offset int) ([]ContainerStatus, error) {
	q := statusQuery + " AND c.node_name = $1 ORDER BY c.name" + paginationClause(limit, offset)
	rows, err := r.pool.Query(ctx, q, nodeName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []ContainerStatus
	for rows.Next() {
		cs, err := scanContainerStatus(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, cs)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

func (r *PostgresRepo) ListContainersByNodeForStacks(ctx context.Context, nodeName string, limit, offset int) ([]ContainerStatus, error) {
	q := statusQuery + " AND c.node_name = $1 ORDER BY c.compose_project, c.name" + paginationClause(limit, offset)
	rows, err := r.pool.Query(ctx, q, nodeName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []ContainerStatus
	for rows.Next() {
		cs, err := scanContainerStatus(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, cs)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

func (r *PostgresRepo) GetComposeDir(ctx context.Context, nodeName, project string) (string, error) {
	var composeDir string
	err := r.pool.QueryRow(ctx,
		`SELECT compose_dir FROM containers
		 WHERE node_name = $1 AND compose_project = $2 AND compose_dir != ''
		 LIMIT 1`,
		nodeName, project,
	).Scan(&composeDir)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	return composeDir, nil
}

func (r *PostgresRepo) UpsertMetadata(ctx context.Context, m ContainerMetadata) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO containers (container_id, node_name, name, image_tag, env_vars, mounts, labels, compose_project, compose_dir, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 ON CONFLICT (container_id) DO UPDATE SET
		   node_name        = EXCLUDED.node_name,
		   name             = EXCLUDED.name,
		   image_tag        = EXCLUDED.image_tag,
		   env_vars         = EXCLUDED.env_vars,
		   mounts           = EXCLUDED.mounts,
		   labels           = EXCLUDED.labels,
		   compose_project  = EXCLUDED.compose_project,
		   compose_dir      = EXCLUDED.compose_dir,
		   updated_at       = EXCLUDED.updated_at,
		   removed_at       = NULL`,
		m.ContainerID, m.NodeName, m.Name, m.ImageTag,
		m.EnvsJSON, m.MountsJSON, m.LabelsJSON, m.ComposeProject, m.ComposeDir, time.Now(),
	)
	return err
}

func (r *PostgresRepo) InsertHeartbeat(ctx context.Context, containerID, status string, uptimeSeconds int64) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO container_heartbeats (time, container_id, status, uptime_seconds)
		 VALUES ($1, $2, $3, $4)`,
		time.Now(), containerID, status, uptimeSeconds,
	)
	return err
}

func (r *PostgresRepo) GetPreviousStatus(ctx context.Context, containerID string) (string, error) {
	var prevStatus string
	err := r.pool.QueryRow(ctx,
		`SELECT status FROM container_heartbeats
		 WHERE container_id = $1
		 ORDER BY time DESC LIMIT 1`,
		containerID,
	).Scan(&prevStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	return prevStatus, nil
}

func (r *PostgresRepo) GetContainerInfoForRemoval(ctx context.Context, nodeName string, containerIDs []string) ([]ContainerInfo, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT container_id, name, image_tag, COALESCE(compose_project, '')
		 FROM containers
		 WHERE node_name = $1
		   AND container_id = ANY($2)
		   AND removed_at IS NULL`,
		nodeName, containerIDs,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []ContainerInfo
	for rows.Next() {
		var ci ContainerInfo
		if err := rows.Scan(&ci.ContainerID, &ci.Name, &ci.ImageTag, &ci.ComposeProject); err != nil {
			return nil, err
		}
		results = append(results, ci)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

func (r *PostgresRepo) GetContainerMetadataForEvent(ctx context.Context, containerID string) (ContainerInfo, string, error) {
	var ci ContainerInfo
	var nodeName string
	err := r.pool.QueryRow(ctx,
		`SELECT name, node_name, image_tag, COALESCE(compose_project, '')
		 FROM containers WHERE container_id = $1`,
		containerID,
	).Scan(&ci.Name, &nodeName, &ci.ImageTag, &ci.ComposeProject)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ci, "", ErrNotFound
		}
		return ci, "", err
	}
	ci.ContainerID = containerID
	return ci, nodeName, nil
}

func (r *PostgresRepo) MarkRemoved(ctx context.Context, nodeName string, containerIDs []string) (int64, error) {
	tag, err := r.pool.Exec(ctx,
		`UPDATE containers
		 SET removed_at = NOW()
		 WHERE node_name = $1
		   AND container_id = ANY($2)
		   AND removed_at IS NULL`,
		nodeName, containerIDs,
	)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (r *PostgresRepo) SweepStale(ctx context.Context, maxAge time.Duration) (int64, error) {
	tag, err := r.pool.Exec(ctx,
		`UPDATE containers c
		 SET removed_at = NOW()
		 WHERE removed_at IS NULL
		   AND NOT EXISTS (
		     SELECT 1 FROM container_heartbeats h
		     WHERE h.container_id = c.container_id
		       AND h.time > NOW() - $1::interval
		   )`,
		maxAge.String(),
	)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// --- ActionRepository ---

func (r *PostgresRepo) CreateAction(ctx context.Context, nodeName, action, target string, paramsJSON []byte) (ActionResponse, error) {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO commands (node_name, action, target, params)
		 VALUES ($1, $2, $3, $4)
		 RETURNING command_id, node_name, action, target, params, status, output, duration_ms,
		           created_at::text, updated_at::text`,
		nodeName, action, target, paramsJSON,
	)
	ar, err := scanActionResponse(row)
	if err != nil {
		return ar, err
	}
	return ar, nil
}

func (r *PostgresRepo) ListActions(ctx context.Context, nodeName string, limit, offset int) ([]ActionResponse, error) {
	if limit <= 0 {
		limit = 50
	}
	q := `SELECT command_id, node_name, action, target, params, status, output, duration_ms,
		        created_at::text, updated_at::text
		 FROM commands
		 WHERE node_name = $1
		 ORDER BY created_at DESC` + paginationClause(limit, offset)
	rows, err := r.pool.Query(ctx, q, nodeName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []ActionResponse
	for rows.Next() {
		ar, err := scanActionResponse(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, ar)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

func (r *PostgresRepo) GetAction(ctx context.Context, commandID, nodeName string) (ActionResponse, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT command_id, node_name, action, target, params, status, output, duration_ms,
		        created_at::text, updated_at::text
		 FROM commands
		 WHERE command_id = $1 AND node_name = $2`,
		commandID, nodeName,
	)
	ar, err := scanActionResponse(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ar, ErrNotFound
		}
		return ar, err
	}
	return ar, nil
}

func (r *PostgresRepo) ClaimPendingCommands(ctx context.Context, nodeName string) ([]PendingCommand, error) {
	rows, err := r.pool.Query(ctx,
		`UPDATE commands
		 SET status = 'running', updated_at = NOW()
		 WHERE node_name = $1 AND status = 'pending'
		 RETURNING command_id, action, target, params`,
		nodeName,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var commands []PendingCommand
	for rows.Next() {
		var cmd PendingCommand
		var paramsJSON []byte
		if err := rows.Scan(&cmd.CommandID, &cmd.Action, &cmd.Target, &paramsJSON); err != nil {
			return nil, err
		}
		params := map[string]string{}
		_ = json.Unmarshal(paramsJSON, &params)
		cmd.Params = params
		commands = append(commands, cmd)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return commands, nil
}

func (r *PostgresRepo) UpdateCommandResult(ctx context.Context, commandID, status, output string, durationMs int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE commands
		 SET status = $1, output = $2, duration_ms = $3, updated_at = NOW()
		 WHERE command_id = $4`,
		status, output, durationMs, commandID,
	)
	return err
}

// --- AgentRepository ---

func (r *PostgresRepo) UpsertAgent(ctx context.Context, nodeName, agentVersion string) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO agents (node_name, agent_version, last_seen)
		 VALUES ($1, $2, NOW())
		 ON CONFLICT (node_name) DO UPDATE SET
		   agent_version = EXCLUDED.agent_version,
		   last_seen     = NOW()`,
		nodeName, agentVersion,
	)
	return err
}

func (r *PostgresRepo) ListOnlineAgents(ctx context.Context) ([]string, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT node_name FROM agents WHERE last_seen > NOW() - INTERVAL '2 minutes'`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []string
	for rows.Next() {
		var nodeName string
		if err := rows.Scan(&nodeName); err != nil {
			return nil, err
		}
		nodes = append(nodes, nodeName)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return nodes, nil
}

func (r *PostgresRepo) ListAgents(ctx context.Context) ([]AgentStatus, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT node_name, agent_version, first_seen::text, last_seen::text,
		        (last_seen > NOW() - INTERVAL '2 minutes') AS online
		 FROM agents ORDER BY node_name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []AgentStatus
	for rows.Next() {
		var a AgentStatus
		if err := rows.Scan(&a.NodeName, &a.AgentVersion, &a.FirstSeen, &a.LastSeen, &a.Online); err != nil {
			return nil, err
		}
		results = append(results, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}
