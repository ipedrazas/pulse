# Pulse Improvement Plan

This document tracks the implementation plan for Pulse improvements.
Each phase is independent and can be implemented incrementally.

Legend: `[ ]` pending · `[~]` in progress · `[x]` done

---

## Phase 1: Security Hardening (P0)

### 1.1 REST API Authentication

**Goal:** Protect the REST API so that container metadata, env vars, and labels are not publicly readable.

**Why:** Currently anyone on the network can query `/status`, `/nodes` and see all container env vars (which often contain secrets).

- [x] Add a `REST_TOKEN` env var to `api/internal/config/config.go` (falls back to `MONITOR_TOKEN` if unset)
- [x] Create `api/internal/rest/middleware.go` with a Gin middleware that checks the `Authorization: Bearer <token>` header
- [x] Apply the middleware to all routes except `/healthz`
- [x] Inject token server-side via nginx `proxy_set_header` (no token in JS bundle)
- [x] Update `compose.yml` to pass the token to the API and UI containers
- [x] Add tests in `api/internal/rest/middleware_test.go` for authenticated and unauthenticated requests
- [x] Update `.env.example` with the new variable

**Files touched:**
- `api/internal/config/config.go`
- `api/internal/rest/middleware.go` (new)
- `api/internal/rest/handlers.go` (register middleware)
- `ui/nginx.conf` (inject Bearer header server-side)
- `ui/Dockerfile` (envsubst for REST_TOKEN at runtime)
- `compose.yml`
- `.env.example`

### 1.2 Environment Variable Secret Filtering

**Goal:** Redact sensitive env vars before storing or returning them.

**Why:** Agents send all env vars (including DB passwords, API keys) to the hub, which stores them in plaintext and exposes them via the REST API.

- [x] Add `ENV_REDACT_PATTERNS` env var to agent config (default: `PASSWORD,SECRET,KEY,TOKEN,CREDENTIAL`)
- [x] Create `agent/internal/redact/redact.go` with a function that replaces matching values with `[REDACTED]`
- [x] Call the redact function in `agent/cmd/agent/main.go` before sending metadata
- [x] Add unit tests for the redaction logic

**Files touched:**
- `agent/internal/config/config.go`
- `agent/internal/redact/redact.go` (new)
- `agent/internal/redact/redact_test.go` (new)
- `agent/cmd/agent/main.go`

---

## Phase 2: Compose Stack Awareness (P1 — prerequisite for Remote Actions)

### 2.1 Group Containers by Compose Project

**Goal:** Detect which containers belong to the same `docker-compose` stack and expose this grouping.

**Why:** Docker automatically sets labels `com.docker.compose.project` and `com.docker.compose.project.working_dir` on compose-managed containers. We already collect labels — we just need to use them.

- [ ] Add a `compose_project` column to the `containers` table (migration `003_add_compose_project.up.sql`)
- [ ] In `api/internal/grpcserver/service.go`, extract `com.docker.compose.project` and `com.docker.compose.project.working_dir` from the labels map and store them in the new column
- [ ] Update the `statusQuery` in `api/internal/rest/handlers.go` to include the compose project
- [ ] Add a new REST endpoint `GET /nodes/:node/stacks` that returns containers grouped by compose project
- [ ] Update `ui/src/types.ts` with the compose project field
- [ ] Update the UI to show compose-project grouping within each node (collapsible groups)

**Files touched:**
- `api/internal/db/migrations/003_add_compose_project.up.sql` (new)
- `api/internal/db/migrations/003_add_compose_project.down.sql` (new)
- `api/internal/grpcserver/service.go`
- `api/internal/rest/handlers.go`
- `ui/src/types.ts`
- `ui/src/components/NodeGrid.tsx` (or similar)

---

## Phase 3: Remote Actions — `docker compose pull` & `up -d` (P1)

This is the main feature request. It requires a **command channel** from hub to agent, since today communication is agent→hub only.

### 3.1 Protocol: Add Command RPCs to gRPC

**Goal:** Extend the protobuf definition to support hub→agent command dispatch.

**Design decision — polling vs streaming:**
A polling approach (`GetPendingCommands`) is simpler and more resilient to network issues than server-streaming. The agent already runs a 30s loop — we can piggyback command polling onto that cycle with minimal overhead.

- [ ] Add new messages and RPCs to `proto/monitor/v1/monitor.proto`:

```protobuf
// Command dispatch — agent polls hub for pending commands
message GetPendingCommandsRequest {
  string node_name = 1;
}

message Command {
  string command_id  = 1;
  string action      = 2;   // "compose_update" | "compose_restart" | ...
  string target      = 3;   // compose project name or container ID
  map<string, string> params = 4;
}

message GetPendingCommandsResponse {
  repeated Command commands = 1;
}

// Agent reports command execution result
message ReportCommandResultRequest {
  string command_id = 1;
  string node_name  = 2;
  bool   success    = 3;
  string output     = 4;   // stdout+stderr, truncated to 10KB
  int64  duration_ms = 5;
}

message ReportCommandResultResponse {}
```

- [ ] Add RPCs to the `MonitoringService`:

```protobuf
rpc GetPendingCommands(GetPendingCommandsRequest) returns (GetPendingCommandsResponse);
rpc ReportCommandResult(ReportCommandResultRequest) returns (ReportCommandResultResponse);
```

- [ ] Run `buf generate` to regenerate Go code

**Files touched:**
- `proto/monitor/v1/monitor.proto`
- `proto/monitor/v1/monitor.pb.go` (generated)
- `proto/monitor/v1/monitor_grpc.pb.go` (generated)

### 3.2 Database: Commands Table

**Goal:** Store pending and completed commands.

- [ ] Create migration `004_add_commands.up.sql`:

```sql
CREATE TABLE commands (
    command_id  TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    node_name   TEXT        NOT NULL,
    action      TEXT        NOT NULL,
    target      TEXT        NOT NULL DEFAULT '',
    params      JSONB       NOT NULL DEFAULT '{}',
    status      TEXT        NOT NULL DEFAULT 'pending',  -- pending | running | success | failed
    output      TEXT        NOT NULL DEFAULT '',
    duration_ms BIGINT      NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_commands_node_status ON commands (node_name, status);
```

- [ ] Create the corresponding down migration

**Files touched:**
- `api/internal/db/migrations/004_add_commands.up.sql` (new)
- `api/internal/db/migrations/004_add_commands.down.sql` (new)

### 3.3 Hub: Implement Command RPCs

**Goal:** Hub can enqueue commands and serve them to agents.

- [ ] In `api/internal/grpcserver/service.go`, implement `GetPendingCommands`:
  - Query `commands` where `node_name = $1 AND status = 'pending'`
  - Set matched commands to `status = 'running'`
  - Return the list
- [ ] Implement `ReportCommandResult`:
  - Update the command row with `success`, `output`, `duration_ms`, `status`
  - Set `updated_at = NOW()`
- [ ] Add a timeout sweeper: commands stuck in `running` for > 5 minutes revert to `failed` (can be a goroutine in main or a DB cronjob)

**Files touched:**
- `api/internal/grpcserver/service.go`

### 3.4 Hub: REST Endpoints for Commands

**Goal:** The UI (or curl) can dispatch commands and track their status.

- [ ] `POST /nodes/:node/actions` — create a new command
  - Body: `{"action": "compose_update", "target": "mystack"}`
  - Validates that `action` is in an allowlist
  - Returns the created command with its ID
- [ ] `GET /nodes/:node/actions` — list recent commands for a node (last 50)
- [ ] `GET /nodes/:node/actions/:id` — get a single command's status and output
- [ ] All action endpoints require authentication (Phase 1 middleware)

**Files touched:**
- `api/internal/rest/handlers.go`

### 3.5 Agent: Command Polling & Execution

**Goal:** Agent fetches pending commands and executes them.

- [ ] Add `GetPendingCommands` and `ReportCommandResult` to `agent/internal/grpcclient/client.go`
- [ ] Create `agent/internal/executor/executor.go`:
  - Receives a `Command` and dispatches based on `action`
  - For `compose_update`: runs `docker compose -f <path> pull && docker compose -f <path> up -d`
  - For `compose_restart`: runs `docker compose -f <path> restart`
  - Captures combined stdout+stderr, truncates to 10KB
  - Returns success/failure and output
- [ ] Determine compose file path using one of:
  - **Option A (label-based):** Read `com.docker.compose.project.working_dir` from any container in the target project — this is where the compose file lives. Agent already has this data from Docker inspect.
  - **Option B (config-based):** New env var `COMPOSE_DIRS` mapping project names to paths
  - **Recommendation: Option A** — zero config, works automatically
- [ ] Add `ALLOWED_ACTIONS` env var to agent config (default: `compose_update,compose_restart`). Agent refuses any action not in the allowlist.
- [ ] In `agent/cmd/agent/main.go`, add command polling to the main loop:
  - After each `pollOnce()`, call `GetPendingCommands`
  - Execute each command sequentially
  - Report results back to hub
- [ ] Add unit tests for executor (mock exec)

**Files touched:**
- `agent/internal/grpcclient/client.go`
- `agent/internal/executor/executor.go` (new)
- `agent/internal/config/config.go`
- `agent/cmd/agent/main.go`

### 3.6 UI: Actions Panel

**Goal:** Users can trigger and monitor remote actions from the web UI.

- [ ] Add an "Update Stack" button to each compose-project group in the node view
- [ ] Clicking the button calls `POST /nodes/:node/actions` with `action: "compose_update"` and `target: <project>`
- [ ] Add a slide-out panel or modal showing recent actions for the node (`GET /nodes/:node/actions`)
- [ ] Each action row shows: status (pending/running/success/failed), target, timestamp, duration
- [ ] Clicking an action shows its output in a terminal-style log viewer
- [ ] Poll the action status every 3 seconds while any action is `pending` or `running`
- [ ] Add a confirmation dialog before triggering an action ("This will pull new images and restart containers on node X. Continue?")

**Files touched:**
- `ui/src/types.ts`
- `ui/src/api/client.ts`
- `ui/src/components/ActionButton.tsx` (new)
- `ui/src/components/ActionsPanel.tsx` (new)
- `ui/src/components/ActionOutput.tsx` (new)
- `ui/src/hooks/useActions.ts` (new)

---

## Phase 4: Stale Container Cleanup (P1)

### 4.1 Detect Removed Containers

**Goal:** Clean up containers that no longer exist on any node.

- [ ] In the agent's `pollOnce()`, after `tracker.Prune()`, build a list of container IDs that were previously known but are now gone
- [ ] Add a new gRPC RPC `ReportRemovedContainers` that accepts a list of container IDs and a node name
- [ ] Hub marks these containers as removed (soft delete: add `removed_at TIMESTAMPTZ` column) or hard-deletes them
- [ ] Migration `005_add_removed_at.up.sql`
- [ ] Update REST queries to exclude containers where `removed_at IS NOT NULL`
- [ ] Alternatively (simpler): add a scheduled DB cleanup that deletes containers with no heartbeat in the last 48 hours

**Files touched:**
- `proto/monitor/v1/monitor.proto`
- `api/internal/grpcserver/service.go`
- `api/internal/rest/handlers.go`
- `api/internal/db/migrations/005_add_removed_at.up.sql` (new)
- `agent/cmd/agent/main.go`

---

## Phase 5: TLS for gRPC (P1)

### 5.1 Add TLS Support

**Goal:** Encrypt the gRPC channel between agents and hub.

**Why:** With remote command execution, the channel carries action commands — encryption is essential.

- [ ] Add `TLS_CERT_FILE` and `TLS_KEY_FILE` env vars to API config
- [ ] Add `TLS_CA_FILE` env var to agent config (for verifying the server cert)
- [ ] In API `main.go`, if cert/key are provided, create a `tls.Config` and pass `grpc.Creds(credentials.NewTLS(...))` to the gRPC server
- [ ] In agent `grpcclient`, if CA file is provided, use `grpc.WithTransportCredentials` instead of `grpc.WithInsecure()`
- [ ] If no TLS files are set, fall back to current insecure mode (backwards compatible)
- [ ] Document certificate generation with `openssl` or `mkcert` in README

**Files touched:**
- `api/internal/config/config.go`
- `api/cmd/api/main.go`
- `agent/internal/config/config.go`
- `agent/internal/grpcclient/client.go`

---

## Phase 6: Alerting & Notifications (P2)

### 6.1 Webhook Notifications

**Goal:** Notify external systems when containers go down or come back up.

- [ ] Add `WEBHOOK_URL` and `WEBHOOK_EVENTS` env vars to API config
- [ ] Create `api/internal/alerts/webhook.go` that POSTs JSON payloads to the webhook URL
- [ ] In the heartbeat handler, detect transitions (running → exited, exited → running) by comparing with the previous heartbeat
- [ ] Fire webhook on state transitions
- [ ] Support Slack-compatible webhook format (works with Slack, Discord, Mattermost)

**Files touched:**
- `api/internal/config/config.go`
- `api/internal/alerts/webhook.go` (new)
- `api/internal/grpcserver/service.go`

---

## Phase 7: Agent Health & Observability (P2)

### 7.1 Agent Health Tracking

**Goal:** Track agent liveness separately from container liveness.

- [ ] Add a new gRPC RPC `AgentHeartbeat` that agents call every poll cycle with their node name, agent version, and uptime
- [ ] Store in a new `agent_heartbeats` table (or reuse a simple `agents` table with `last_seen`)
- [ ] Add REST endpoint `GET /agents` returning agent status per node
- [ ] Add an agent health indicator to the UI header per node

**Files touched:**
- `proto/monitor/v1/monitor.proto`
- `api/internal/grpcserver/service.go`
- `api/internal/rest/handlers.go`
- `api/internal/db/migrations/` (new migration)
- `agent/cmd/agent/main.go`
- `ui/src/components/Header.tsx`

---

## Implementation Order

Recommended sequence (each phase is a PR):

```
Phase 1 (Security)      ← do first, low effort, high impact
  ↓
Phase 2 (Compose Stacks) ← prerequisite for Phase 3
  ↓
Phase 3 (Remote Actions) ← main feature, largest effort
  ↓
Phase 4 (Stale Cleanup)  ← data hygiene, independent
  ↓
Phase 5 (TLS)            ← important once Phase 3 lands
  ↓
Phase 6 (Alerting)       ← nice to have
  ↓
Phase 7 (Agent Health)   ← nice to have
```

Phases 4–7 are independent of each other and can be done in any order.

---

## Architecture Diagram After All Phases

```
                         ┌──────────────────┐
                         │    Web UI :3000   │
                         │  (React + Vite)   │
                         └────────┬─────────┘
                             REST │ (Bearer token auth)
                         ┌────────▼─────────┐
                         │  API Hub :8080    │
                         │  (Gin + gRPC)     │◄──── Webhook alerts
                         └──┬────────────┬──┘
                   gRPC+TLS │            │ gRPC+TLS
               ┌────────────▼──┐   ┌─────▼───────────┐
               │  Agent (VM1)  │   │  Agent (VM2)     │
               │  - Docker poll│   │  - Docker poll   │
               │  - Cmd exec   │   │  - Cmd exec      │
               │  - Compose    │   │  - Compose        │
               └───────────────┘   └──────────────────┘
```
