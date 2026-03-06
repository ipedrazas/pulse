# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [0.1.0] - 2026-03-01

### Added

- Protocol Buffers definition (`proto/monitor/v1/monitor.proto`) with `MonitoringService` (ReportHeartbeat, SyncMetadata)
- API server (Hub) with dual gRPC + REST server and graceful shutdown
- gRPC token authentication interceptor (`x-monitor-token`)
- REST endpoints: `/healthz`, `/status`, `/status/:container`, `/nodes`, `/nodes/:node`
- Edge agent with Docker polling (30s interval), metadata debounce (SHA256), and hash pruning
- gRPC client with keepalive (10s), exponential backoff retry policy
- TimescaleDB schema with hypertable, compression policy (7d), and retention policy (30d)
- Embedded database migrations via `golang-migrate` (auto-run on startup)
- Multi-stage Dockerfiles for both API and agent
- `docker-compose.yml` for full stack (API + TimescaleDB + Agent)
- `docker-compose.agent.yml` for standalone agent sidecar
- Integration tests using testcontainers (data flow, upsert, auth, reconnection, TimescaleDB policies)
- Unit tests for auth interceptor, config loader, debounce tracker, and Docker helpers
- Go workspace (`go.work`) with three modules: `proto/`, `api/`, `agent/`
- Buf-based protobuf generation (`buf.yaml`, `buf.gen.yaml`)
- Taskfile with targets: `proto`, `build`, `test`, `lint`, `docker`, `tidy`, `clean`
