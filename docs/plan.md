# Pulse — Implementation Plan

This plan tracks the build-out of Pulse based on [spec.md](./spec.md). Each phase produces a working, testable increment.

---

## Identified Gaps & Recommendations

Before diving into phases, these are gaps or ambiguities in the spec that should be resolved:

### 1. File Transfer (`pulse send --file`)
Add a `SendFile` message to the gRPC stream with chunked binary transfer (e.g. 64KB chunks). The proto needs a `FileChunk` message type.

### 2. Log Streaming (`pulse logs`)
Add a `LogStream` server command that triggers the agent to start sending log lines back through the bidirectional stream, or use a dedicated server-streaming RPC.

### 3. Docker Compose Support (`pulse up -d`, `pulse pull`)
The agent needs a config defined workdir. The compose file would be in there or it would be added via `pulse send`, the only exception is when doing `pulse up -f oci://...` as described in the docs (https://docs.docker.com/compose/how-tos/oci-artifact/)

### 4. Agent Permission Model
A policy config file on the agent (YAML) listing allowed actions per caller. The agent checks this before executing commands.

```

deny:
  - nodes:*

users:
  - ivan:
      read:
        - /etc/hosts
      write:
        - /home/ivan
      exec:
        - docker:*
        - tail
        - ping 
```

### 5. Authentication & TLS
Keep mTLS for agent↔API and a shared token for CLI↔API. Document cert provisioning.

### 6. Agent Registration
First connection auto-registers the agent in the `agents` table. The agent sends its node name + capabilities in the initial handshake message.
Agent has a config entry (or a flag) with the api endpoint.

### 7. Reconnection Semantics
Commands pending during disconnect are kept in the DB and retried on reconnect, with a configurable TTL.

### 8. Configuration Management
Agent config via env vars or a config file (`PULSE_API_ADDR`, `PULSE_NODE_NAME`, `PULSE_TLS_CERT`, etc.).

---

## Phase 0: Project Scaffolding
> Goal: Repository structure, tooling, and build pipeline.

- [x] Fix `.a2.yaml` typo (`source_dit` → `source_dir`)
- [x] Create directory structure:
  ```
  proto/          — Protobuf definitions
  api/            — Go API (control plane)
  cli/            — Go CLI
  agent/          — Rust agent
  ui/             — React UI
  docs/           — Documentation (exists)
  ```
- [x] Initialize Go modules (`api/go.mod`, `cli/go.mod`)
- [x] Initialize Rust project (`cargo init` in `agent/`)
- [x] Initialize React project (Vite + Tailwind in `ui/`)
- [x] Set up `buf.yaml` / `buf.gen.yaml` for proto generation
- [x] Update `Taskfile.yaml` with build/test/lint tasks
- [x] Create `compose.yml` for local dev (TimescaleDB + API)
- [x] Create `.env.example` with required env vars
- [x] Add `CLAUDE.md` with project conventions

---

## Phase 1: Protobuf & gRPC Contract
> Goal: Define the communication contract between all components.

- [x] Write `proto/dco/v1/dco.proto`:
  - `service AgentService` with `rpc Connect(stream AgentMessage) returns (stream ServerCommand)`
  - `AgentMessage`: oneof { Heartbeat, ContainerReport, CommandResult, LogLine, FileAck }
  - `ServerCommand`: oneof { RunContainer, StopContainer, PullImage, ComposeUp, SendFile, RequestLogs }
  - `Heartbeat`: node_name, agent_version, timestamp
  - `ContainerReport`: list of ContainerInfo (id, name, image, status, env_vars, mounts, labels, ports, uptime)
  - `RunContainer`: image, name, env, ports, volumes
  - `StopContainer`: container_id
- [x] Define `service CLIService` (or reuse AgentService) for CLI→API commands:
  - `rpc ListContainers(ListRequest) returns (ContainerList)`
  - `rpc ListNodes(Empty) returns (NodeList)`
  - `rpc SendCommand(CommandRequest) returns (CommandResponse)`
- [x] Generate Go code (`buf generate`)
- [x] Generate Rust code (via `tonic-build` in `agent/build.rs`)
- [x] Verify generated code compiles in both Go and Rust

---

## Phase 2: Database & API Foundation
> Goal: API server boots, connects to TimescaleDB, serves REST + gRPC.

### Database
- [x] Design schema:
  - `agents` table: name (PK), status, version, last_seen, capabilities, created_at
  - `containers` table: container_id, agent_name, name, image, status, env_vars (JSONB), mounts (JSONB), labels (JSONB), ports (JSONB), compose_project, command, created_at, removed_at
  - `container_events` hypertable: time, container_id, agent_name, status, uptime_seconds
  - `commands` table: id, agent_name, type, payload (JSONB), status, result, created_at, completed_at, ttl
- [x] Write SQL migrations (golang-migrate format)
- [x] Write migration runner

### API gRPC Server
- [x] Implement `AgentService.Connect` bidirectional stream handler:
  - Accept agent connection, register/update agent in DB
  - Receive `Heartbeat` → update agent `last_seen`
  - Receive `ContainerReport` → upsert containers + insert events into hypertable
  - Send pending `ServerCommand`s from the commands table
  - On stream close → mark agent offline
- [x] Implement `CLIService` RPCs

### API REST Server (Gin)
- [x] `GET /healthz` — health check
- [x] `GET /api/v1/nodes` — list agents with status
- [x] `GET /api/v1/nodes/:name` — agent detail + containers
- [x] `GET /api/v1/containers` — all containers (with pagination)
- [x] `GET /api/v1/containers/:id` — container detail
- [x] `POST /api/v1/commands` — submit command (routed to agent via stream)

### Repository Pattern
- [x] Define `Repository` interface
- [x] Implement `PostgresRepository`
- [x] Write tests using Testcontainers

### Infrastructure
- [x] Graceful shutdown handling
- [x] Structured logging (slog)
- [x] Configuration from env vars
- [x] Maintenance goroutine: sweep stale agents/containers

---

## Phase 3: Rust Agent
> Goal: Agent connects to API, reports containers, executes commands.

### Connection & Streaming
- [x] gRPC client with `tonic` — connect to API, establish bidirectional stream
- [x] Reconnection logic with exponential backoff (0.5s → 30s cap)
- [x] TLS support (optional, via rustls)

### Docker Integration
- [x] `bollard` client — connect to Docker socket
- [x] Poll running containers every N seconds (configurable, default 30s)
- [x] Build `ContainerReport` from Docker state
- [x] Debounce metadata: hash container state, only send full report on change
- [x] Send `Heartbeat` on every poll cycle

### Command Execution
- [x] Listen for `ServerCommand` on the incoming stream
- [x] `RunContainer` → `docker run` via bollard
- [x] `StopContainer` → `docker stop` via bollard
- [x] `PullImage` → `docker pull` via bollard
- [x] `ComposeUp` → shell out to `docker compose up -d`
- [x] Report command result back through stream

### Security
- [x] Env var redaction (`PULSE_REDACT_PATTERNS=PASSWORD,SECRET,KEY,TOKEN,CREDENTIAL`)
- [x] Permission policy: load allowed actions from config file
- [x] Drop privileges where possible

### Packaging
- [x] Dockerfile (multi-stage build)
- [x] systemd unit file template

---

## Phase 4: Go CLI
> Goal: CLI can manage the cluster via gRPC.

- [x] Set up Cobra root command with global flags (`--api-addr`, `--node`)
- [x] `pulse ps` — list all containers (table format)
- [x] `pulse ps --node <id>` — list containers on a specific node
- [x] `pulse run --image <name> --node <id>` — run a container
- [x] `pulse stop <container> --node <id>` — stop a container
- [x] `pulse pull --node <id>` — pull images
- [x] `pulse up -d --node <id>` — compose up
- [x] `pulse logs <container> --node <id>` — stream container logs
- [x] `pulse send --file <path> --node <id>` — send file to agent
- [x] `pulse nodes` — list agents and their status
- [x] Output formatting: table, JSON (`--output json`)
- [x] Error handling with user-friendly messages

---

## Phase 5: React UI
> Goal: Dashboard for monitoring agents and containers.

- [x] Project setup: Vite + React + TypeScript + Tailwind CSS
- [x] REST API client (fetch wrapper with error handling)
- [x] Layout: Header + main content area
- [x] Agent list view:
  - Cards/rows per agent
  - Online/Offline indicator (green/gray dot)
  - Last seen timestamp
  - Container count
- [x] Container table:
  - Sortable columns: name, image, status, agent, uptime
  - Status badge (running/stopped/exited with color)
  - Search/filter bar
- [x] Container detail view (click to expand):
  - Environment variables (redacted values)
  - Mounts
  - Labels
  - Ports
  - Logs (stretch goal)
- [x] Auto-refresh with polling (configurable interval)
- [x] Empty states and loading spinners
- [x] Responsive layout

---

## Phase 6: Integration & Hardening
> Goal: End-to-end tested, production-ready system.

- [ ] Integration tests: agent → API → DB round-trip (Testcontainers)
- [ ] Integration tests: CLI → API → agent command routing
- [ ] TLS/mTLS setup and documentation
- [ ] Alert webhooks (Slack/Discord) for agent online/offline transitions
- [ ] Rate limiting on REST endpoints
- [ ] API versioning strategy
- [ ] Dockerfiles for all components
- [ ] `compose.yml` for full-stack local deployment
- [ ] CI pipeline (lint, test, build)
- [ ] Documentation: README, deployment guide, API reference

---

## Dependency Graph

```
Phase 0 (Scaffolding)
  └── Phase 1 (Proto)
        ├── Phase 2 (API + DB)
        │     ├── Phase 3 (Rust Agent)  ← needs API to connect to
        │     ├── Phase 4 (CLI)         ← needs API gRPC endpoints
        │     └── Phase 5 (UI)          ← needs API REST endpoints
        └────────────────────────────────┘
              Phase 6 (Integration)      ← needs all components
```

Phases 3, 4, and 5 can be developed in parallel once Phase 2 is complete.

---

## Key Decisions to Make

| # | Decision | Options | Impact |
|---|----------|---------|--------|
| 1 | Proto package path | `dco/v1` vs `pulse/v1` | Naming across all generated code |
| 2 | CLI↔API protocol | gRPC (shared protos) vs REST | Code reuse vs simplicity |
| 3 | Compose file delivery | Pre-deployed vs transferred via `pulse send` | Affects `pulse up` UX |
| 4 | Agent config format | Env vars only vs TOML/YAML file | Deployment complexity |
| 5 | Shared Go module | Mono-module vs separate `api/` and `cli/` modules | Build independence vs code sharing |
| 6 | Log streaming | Through bidirectional stream vs dedicated RPC | Proto complexity vs connection reuse |
