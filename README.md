# Pulse

Distributed Docker monitoring for homelabs. A lightweight heartbeat system that tracks container health across Proxmox nodes.

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                     Proxmox Cluster                     │
│                                                         │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐               │
│  │  Node 1  │  │  Node 2  │  │  Node N  │               │
│  │ ┌──────┐ │  │ ┌──────┐ │  │ ┌──────┐ │               │
│  │ │Agent │ │  │ │Agent │ │  │ │Agent │ │               │
│  │ └──┬───┘ │  │ └──┬───┘ │  │ └──┬───┘ │               │
│  └────┼─────┘  └────┼─────┘  └────┼─────┘               │
│       │              │              │                   │
│       └──────────────┼──────────────┘                   │
│                      │ gRPC (Floating IP)               │
│              ┌───────▼────────┐                         │
│              │   API Server   │                         │
│              │  (Hub)         │                         │
│              │  :50051 gRPC   │                         │
│              │  :8080  REST   │                         │
│              └───────┬────────┘                         │
│                      │                                  │
│              ┌───────▼────────┐                         │
│              │  TimescaleDB   │                         │
│              │  :5432         │                         │
│              └────────────────┘                         │
└─────────────────────────────────────────────────────────┘
```

- **Hub (API Server):** Go binary running a gRPC server (for agents) and a REST API (for querying). Backed by TimescaleDB.
- **Edge Agents:** Lightweight Go containers running on every Proxmox VM. Poll the local Docker socket every 30s, report heartbeats and metadata changes via gRPC.

## Project Structure

```
pulse/
├── proto/monitor/v1/         # Protobuf definitions + generated Go stubs
├── agent/                    # Edge agent
│   ├── cmd/agent/            # Entrypoint
│   ├── internal/             # Config, Docker poller, debounce, gRPC client
│   └── Dockerfile
├── api/                      # Hub API server
│   ├── cmd/api/              # Entrypoint
│   ├── internal/             # Config, DB migrations, gRPC service, REST handlers
│   └── Dockerfile
├── docker-compose.yml        # Hub + TimescaleDB + Agent (full stack)
├── docker-compose.agent.yml  # Agent sidecar (standalone)
├── buf.yaml / buf.gen.yaml   # Protobuf tooling
├── Taskfile.yaml             # Build tasks
└── go.work                   # Go workspace
```

## Prerequisites

- Go 1.26+
- Docker & Docker Compose
- [Task](https://taskfile.dev) (`go install github.com/go-task/task/v3/cmd/task@latest`)
- [Buf](https://buf.build) (for proto generation)

## Quick Start

### 1. Run the full stack locally

```bash
# Set required env vars
export MONITOR_TOKEN="your-secret-token"
export POSTGRES_USER="pulse"
export POSTGRES_PASSWORD="pulse"
export POSTGRES_DB="pulse"
export SERVER_ADDR="api:50051"
export PROXMOX_NODE_NAME="local-dev"

# Start everything
docker compose up --build
```

### 2. Query the REST API

```bash
# Health check
curl http://localhost:8080/healthz

# All containers with latest heartbeat
curl http://localhost:8080/status

# Single container
curl http://localhost:8080/status/<container_id>

# Containers grouped by node
curl http://localhost:8080/nodes

# Containers on a specific node
curl http://localhost:8080/nodes/<node_name>
```

## Environment Variables

### API Server (Hub)

| Variable | Required | Default | Description |
|---|---|---|---|
| `DB_URL` | Yes | — | PostgreSQL connection string |
| `MONITOR_TOKEN` | Yes | — | Shared secret for agent authentication |
| `GRPC_PORT` | No | `50051` | gRPC listen port |
| `HTTP_PORT` | No | `8080` | REST API listen port |

### Agent

| Variable | Required | Default | Description |
|---|---|---|---|
| `SERVER_ADDR` | Yes | — | Hub gRPC address (host:port) |
| `MONITOR_TOKEN` | Yes | — | Shared secret (must match hub) |
| `PROXMOX_NODE_NAME` | Yes | — | Name of this Proxmox node |

### TimescaleDB (docker-compose)

| Variable | Required | Default | Description |
|---|---|---|---|
| `POSTGRES_USER` | Yes | — | Database user |
| `POSTGRES_PASSWORD` | Yes | — | Database password |
| `POSTGRES_DB` | Yes | — | Database name |

## Development

```bash
# Generate protobuf stubs
task proto

# Build all binaries
task build

# Run all tests
task test

# Run linters
task lint

# Build Docker images
task docker

# Tidy all Go modules
task tidy
```

## REST API Reference

| Method | Path | Description |
|---|---|---|
| `GET` | `/healthz` | Liveness check (DB ping) |
| `GET` | `/status` | All containers with latest heartbeat |
| `GET` | `/status/:container` | Single container detail |
| `GET` | `/nodes` | Containers grouped by node |
| `GET` | `/nodes/:node` | Containers for a specific node |

## gRPC Service

Defined in `proto/monitor/v1/monitor.proto`:

| RPC | Description |
|---|---|
| `ReportHeartbeat` | Agent sends container_id, status, uptime every 30s |
| `SyncMetadata` | Agent sends container metadata (image, envs, mounts) only when changed |

Authentication: `x-monitor-token` metadata header on every gRPC call.

## Agent Behavior

1. Polls Docker every 30 seconds
2. Always sends `ReportHeartbeat` for each running container
3. Computes SHA256 hash of `(image + sorted_envs + mounts)` — only calls `SyncMetadata` when the hash changes or a container is new
4. Prunes tracked hashes when containers are removed
5. Uses gRPC keepalive (10s) to detect broken connections during VRRP failover
6. Retries with exponential backoff (0.5s–30s, up to 5 attempts) on `UNAVAILABLE` and `DEADLINE_EXCEEDED`

## Database

TimescaleDB with two tables:

- **`containers`** — metadata (container_id PK, node_name, name, image_tag, env_vars JSONB, mounts JSONB)
- **`container_heartbeats`** — time-series hypertable (time, container_id, status, uptime_seconds)

Policies:
- Compression after 7 days (segmented by container_id)
- Retention: drop chunks older than 30 days

Migrations run automatically on API startup.
