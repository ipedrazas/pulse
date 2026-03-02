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

- [x] Add `compose_project` and `compose_dir` columns to the `containers` table (migration `003_add_compose_project`)
- [x] In `api/internal/grpcserver/service.go`, extract `com.docker.compose.project` and `com.docker.compose.project.working_dir` from the labels map and store them
- [x] Update the `statusQuery` in `api/internal/rest/handlers.go` to include compose_project; refactored scan logic into `scanContainer` helper
- [x] Add new REST endpoint `GET /nodes/:node/stacks` that returns containers grouped by compose project
- [x] Update `ui/src/types.ts` with `compose_project` field
- [x] Update `NodeCard` to group containers by compose project with section headers
- [x] Update `ContainerRow` to use dedicated `compose_project` field
- [x] Update `ContainerDetail` to show "Stack" row

**Files touched:**
- `api/internal/db/migrations/003_add_compose_project.up.sql` (new)
- `api/internal/db/migrations/003_add_compose_project.down.sql` (new)
- `api/internal/grpcserver/service.go`
- `api/internal/rest/handlers.go`
- `api/internal/rest/handlers_test.go`
- `ui/src/types.ts`
- `ui/src/components/NodeCard.tsx`
- `ui/src/components/ContainerRow.tsx`
- `ui/src/components/ContainerDetail.tsx`
- `ui/nginx.conf`

---

## Phase 3: Remote Actions — `docker compose pull` & `up -d` (P1) ✅

Command channel from hub to agent using polling-based approach.

### 3.1 Protocol — [x] Done
- [x] Added `GetPendingCommands` and `ReportCommandResult` RPCs + messages to `monitor.proto`
- [x] Generated Go code with `buf generate`

### 3.2 Database — [x] Done
- [x] Migration `004_add_commands` with command_id, node_name, action, target, params, status, output, duration_ms, timestamps

### 3.3 Hub gRPC — [x] Done
- [x] `GetPendingCommands`: atomically sets pending→running and returns commands
- [x] `ReportCommandResult`: updates status, output, duration

### 3.4 Hub REST — [x] Done
- [x] `POST /nodes/:node/actions` — creates command (validates action allowlist)
- [x] `GET /nodes/:node/actions` — lists recent 50 commands
- [x] `GET /nodes/:node/actions/:id` — single command details
- [x] All behind Bearer auth middleware

### 3.5 Agent — [x] Done
- [x] gRPC client: `GetPendingCommands` + `ReportCommandResult`
- [x] `executor` package: runs `docker compose pull && docker compose up -d` in compose working dir
- [x] Compose dir lookup via `com.docker.compose.project.working_dir` label (zero config)
- [x] `ALLOWED_ACTIONS` env var (default: `compose_update,compose_restart`)
- [x] Command polling after every poll cycle in main loop
- [x] Unit tests for executor (disallowed, unknown, missing dir, truncation)

### 3.6 UI — [x] Done
- [x] `ActionResponse` type, `createAction()` + `fetchActions()` API client
- [x] `UpdateButton` with inline confirmation per compose project group
- [x] `ActionsPanel` slide-out with status, duration, expandable output
- [x] Polls every 3s for progress updates
- [x] Wired into `NodeCard` with "Actions" link in header

---

## Phase 4: Stale Container Cleanup (P1) ✅

### 4.1 Detect Removed Containers

**Goal:** Clean up containers that no longer exist on any node.

- [x] Modified `debounce.Tracker.Prune()` to return removed container IDs
- [x] Added `ReportRemovedContainers` gRPC RPC to `monitor.proto` and generated Go code
- [x] Hub gRPC handler sets `removed_at = NOW()` for reported container IDs (soft delete)
- [x] Migration `005_add_removed_at` adds `removed_at TIMESTAMPTZ` column + partial index for active containers
- [x] `SyncMetadata` clears `removed_at` when a container reappears (handles container recreation)
- [x] Updated all REST queries to exclude containers where `removed_at IS NOT NULL`
- [x] Agent gRPC client sends `ReportRemovedContainers` after each poll cycle
- [x] Background sweeper on hub marks containers stale after 48h without heartbeat (safety net, runs hourly)

**Files touched:**
- `proto/monitor/v1/monitor.proto`
- `api/internal/grpcserver/service.go`
- `api/internal/rest/handlers.go`
- `api/internal/db/migrations/005_add_removed_at.up.sql` (new)
- `api/internal/db/migrations/005_add_removed_at.down.sql` (new)
- `api/cmd/api/main.go` (sweeper goroutine)
- `agent/internal/debounce/debounce.go` (Prune returns removed IDs)
- `agent/internal/debounce/debounce_test.go`
- `agent/internal/grpcclient/client.go` (ReportRemovedContainers method)
- `agent/cmd/agent/main.go` (report removed containers after prune)

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
