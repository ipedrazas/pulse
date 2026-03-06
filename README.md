# Pulse

Pulse is a distributed system to manage container workloads across multiple remote machines.

## Architecture

```
CLI (Go/Cobra) ──gRPC──┐
                        ├── API Server (Go/Gin) ── TimescaleDB
UI (React/Tailwind) ─REST─┘        │
                              gRPC stream
                                   │
                            Agent (Rust/tonic)
                                   │
                            Docker / Compose
```

- **API** — Go control plane (gRPC + REST), connects to TimescaleDB
- **Agent** — Rust daemon on compute nodes, reports containers, executes commands
- **CLI** — Go CLI for cluster management
- **UI** — React dashboard for monitoring

## Quick Start

```bash
# Start the full stack
docker compose up -d

# Or start just the database for development
task db

# Run the API locally
task api:run

# Run the agent locally
task agent:run

# Use the CLI
task cli:build
./cli/bin/pulse nodes
./cli/bin/pulse ps
./cli/bin/pulse run --image nginx:latest --node agent-1
```

## CLI Commands

| Command | Description |
|---------|-------------|
| `pulse nodes` | List all agents |
| `pulse ps [--node NAME]` | List containers |
| `pulse run --image IMG --node NAME` | Run a container |
| `pulse stop CONTAINER --node NAME` | Stop a container |
| `pulse pull IMAGE --node NAME` | Pull an image |
| `pulse up -d --node NAME` | Docker compose up |
| `pulse logs CONTAINER --node NAME` | Request container logs |
| `pulse send --file PATH --node NAME` | Send a file to agent |

## Development

Prerequisites: Go 1.25+, Rust 1.86+, Node 22+, Docker, [Task](https://taskfile.dev), [buf](https://buf.build)

```bash
# Build all
task build

# Run all tests
task test

# Lint all
task lint

# Generate protobuf code
task proto
```

## Configuration

See [`.env.example`](.env.example) for all environment variables.

## Documentation

- [Specification](docs/spec.md)
- [Architecture](docs/architecute.md)
- [Implementation Plan](docs/plan.md)
- [TLS Setup](docs/tls.md)
