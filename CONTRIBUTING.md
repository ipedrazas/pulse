# Contributing to Pulse

## Development Setup

### Prerequisites

- Go 1.26+
- Docker & Docker Compose
- [Task](https://taskfile.dev) — `go install github.com/go-task/task/v3/cmd/task@latest`
- [Buf](https://buf.build) — `brew install bufbuild/buf/buf`
- `protoc-gen-go` and `protoc-gen-go-grpc`:
  ```bash
  go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
  go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
  ```

### Getting Started

```bash
git clone https://github.com/ipedrazas/pulse.git
cd pulse

# Generate protobuf stubs
task proto

# Build everything
task build

# Run tests (unit tests only, no Docker required)
task test

# Run integration tests (requires Docker)
go test github.com/ipedrazas/pulse/api/internal/integration -v -timeout 120s
```

### Project Layout

This is a Go workspace with three modules:

| Module   | Purpose                                    |
| -------- | ------------------------------------------ |
| `proto/` | Protobuf definitions and generated Go code |
| `api/`   | Hub API server (gRPC + REST + migrations)  |
| `agent/` | Edge agent (Docker poller + gRPC client)   |

### Workflow

1. Make changes
2. Run `task lint` to check for issues
3. Run `task test` to run unit tests
4. Run integration tests if touching gRPC handlers, REST endpoints, or migrations
5. Run `task docker` to verify Docker builds

### Proto Changes

When modifying `proto/monitor/v1/monitor.proto`:

1. Run `task proto` to regenerate Go stubs
2. Update both `api/` and `agent/` code to match the new definitions
3. Run `task test` to verify nothing breaks

### Code Style

- Use `log/slog` for structured logging
- Keep packages small and focused under `internal/`
- Follow existing patterns for error handling and context propagation
- Tests go next to the code they test (`_test.go` suffix)
