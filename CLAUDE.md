# Pulse — Project Conventions

## Architecture

Pulse is a distributed container management system with 4 components:

- **API** (`api/`) — Go control plane (Gin REST + gRPC server), connects to TimescaleDB
- **Agent** (`agent/`) — Rust agent on compute nodes (tonic gRPC + bollard Docker)
- **CLI** (`cli/`) — Go CLI (Cobra) for cluster management
- **UI** (`ui/`) — React dashboard (Vite + Tailwind CSS)
- **Proto** (`proto/`) — Protobuf definitions shared across components

## Build & Run

- `task <component>:build` — build a component
- `task <component>:test` — run tests
- `task <component>:lint` — lint code
- `task proto` — generate protobuf code
- `task up` / `task down` — start/stop local dev stack (TimescaleDB + API)
- `task db` — start only the database

## Code Style

- **Go:** `gofmt` compliant. Use `slog` for logging. Repository pattern for DB access.
- **Rust:** `cargo fmt` compliant. `cargo clippy` clean.
- **TypeScript:** Biome for formatting/linting.
- **Proto:** buf lint compliant.

## Testing

- Go API DB tests: use Testcontainers (no mocks for DB)
- Go unit tests: standard `testing` package
- Rust: `#[cfg(test)]` modules
- UI: Vitest

## Key Patterns

- gRPC bidirectional streaming for Agent↔API communication
- REST (JSON) for UI↔API
- gRPC for CLI↔API
- Database migrations via golang-migrate
- Structured logging throughout
- Configuration via environment variables

## Registry

Container images push to `git.andcake.dev/ivan/pulse-{api,agent,ui}`
