# Pulse — Code Improvement Proposals

## 1. Adopt the Repository Pattern

### Problem

SQL queries are scattered across two layers:

- **REST handlers** (`api/internal/rest/handlers.go`) — 6 queries embedded in handler methods
- **gRPC service** (`api/internal/grpcserver/service.go`) — 9 queries embedded in service methods

This means:

- Business logic is coupled to raw SQL and `pgx` scanning
- The same conceptual operations (e.g. "get container by ID", "list containers for a node") exist in different forms depending on whether they're called from REST or gRPC
- Unit tests require elaborate mock scaffolding (`mockDB`, `mockRows`, `mockRow`, `mockActionRow`, `mockAgentRows`) that mimics `pgx` internals — roughly **200 lines** of mock boilerplate in `handlers_test.go`
- The mocks don't validate SQL correctness at all — tests pass even if a query is syntactically wrong

### Proposal

Introduce a `repository` package (e.g. `api/internal/repository/`) with domain-oriented interfaces and a PostgreSQL implementation:

```
api/internal/
├── repository/
│   ├── repository.go          # interfaces
│   ├── postgres.go            # *pgxpool.Pool implementation
│   └── postgres_test.go       # testcontainers-based tests
```

#### Interface design

```go
package repository

type ContainerRepo interface {
    ListContainers(ctx context.Context) ([]Container, error)
    GetContainer(ctx context.Context, containerID string) (Container, error)
    ListByNode(ctx context.Context, nodeName string) ([]Container, error)
    ListByNodeGroupedByStack(ctx context.Context, nodeName string) ([]Stack, error)
    UpsertMetadata(ctx context.Context, meta ContainerMetadata) error
    MarkRemoved(ctx context.Context, nodeName string, ids []string) (int64, error)
    SweepStale(ctx context.Context, maxAge time.Duration) (int64, error)
}

type CommandRepo interface {
    Create(ctx context.Context, cmd CreateCommandParams) (Command, error)
    Get(ctx context.Context, commandID, nodeName string) (Command, error)
    ListByNode(ctx context.Context, nodeName string) ([]Command, error)
    ClaimPending(ctx context.Context, nodeName string) ([]Command, error)
    UpdateResult(ctx context.Context, commandID, status, output string, durationMs int64) error
}

type HeartbeatRepo interface {
    Insert(ctx context.Context, containerID, status string, uptimeSeconds int64) error
    GetPreviousStatus(ctx context.Context, containerID string) (string, error)
}

type AgentRepo interface {
    Upsert(ctx context.Context, nodeName, version string) error
    ListOnline(ctx context.Context) ([]string, error)
    List(ctx context.Context) ([]Agent, error)
}
```

#### Benefits

1. **Both REST handlers and gRPC service** consume the same repository interfaces — no duplicated queries
2. **Unit tests for handlers/service** become trivial: mock the repository interface (a few methods returning domain types) instead of mocking `pgx.Rows`/`pgx.Row` byte-level scanning
3. **Repository tests** run against a real TimescaleDB via testcontainers — testing actual SQL, schema compatibility, and edge cases (NULL handling, JSONB, LATERAL JOINs)
4. **Query changes** are isolated to one place

### Migration path

1. Define domain types in `repository/` (reuse existing structs or introduce clean ones)
2. Move SQL + scanning logic from handlers.go and service.go into `postgres.go`
3. Refactor `Handler` to depend on repository interfaces instead of `DB`
4. Refactor `MonitoringService` to depend on repository interfaces instead of `*pgxpool.Pool`
5. Write testcontainers-based tests in `postgres_test.go` (can reuse the existing `setupTimescaleDB` helper)
6. Simplify `handlers_test.go` — replace 200+ lines of pgx mocks with simple interface mocks

---

## 2. Testing Strategy

### Current state

| Layer | Approach | Coverage |
|-------|----------|----------|
| REST handlers | Mock `pgx` interfaces (~200 LOC of mocks) | Good breadth, but doesn't validate SQL |
| gRPC service | No unit tests; only integration tests | Gaps for error paths |
| Integration | Testcontainers (real DB + gRPC + HTTP) | Good happy-path coverage |
| Executor | Mock `DockerOps` interface | Good |

### Improvements

#### a) Repository-level tests with testcontainers

Once the repository pattern is in place, write focused tests per repository method against a real TimescaleDB. This is where SQL correctness gets validated:

```go
func TestContainerRepo_ListByNode(t *testing.T) {
    if testing.Short() { t.Skip("requires docker") }
    pool := setupTestDB(t)
    repo := repository.NewPostgres(pool)

    // seed data
    repo.UpsertMetadata(ctx, ...)

    // test
    containers, err := repo.ListByNode(ctx, "node-1")
    assert.NoError(t, err)
    assert.Len(t, containers, 2)
}
```

#### b) Simplify handler tests

With repositories, handler mocks become ~20 lines instead of ~200:

```go
type mockContainerRepo struct {
    containers []repository.Container
    err        error
}

func (m *mockContainerRepo) ListContainers(ctx context.Context) ([]repository.Container, error) {
    return m.containers, m.err
}
```

#### c) Add gRPC service unit tests

The gRPC `MonitoringService` currently has no unit tests. With repository interfaces injected, each RPC method can be tested in isolation.

#### d) Consider a test helper package

Both `integration_test.go` and future repository tests need a `setupTimescaleDB` helper. Extract into a shared `testutil` package:

```
api/internal/testutil/
└── testdb.go   // SetupTimescaleDB(t) *pgxpool.Pool
```

---

## 3. Logging

### Current state

Logging is already well set up:
- Uses `log/slog` (stdlib structured logging) — good choice
- JSON output to stdout — container-friendly
- Debug level set as default
- Consistent key-value pattern: `slog.Error("message", "key", value, ...)`

### Improvements

#### a) Make log level configurable

Both `api/cmd/api/main.go:103` and `agent/cmd/agent/main.go:28` hardcode `slog.LevelDebug`:

```go
slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})))
```

Add a `LOG_LEVEL` environment variable (default `info` for production):

```go
var level slog.Level
level.UnmarshalText([]byte(os.Getenv("LOG_LEVEL"))) // defaults to INFO if empty/invalid
```

Debug-level logging in production creates noise and can impact performance with high-frequency heartbeats.

#### b) Add request logging middleware for REST

The Gin router uses `gin.New()` with only `gin.Recovery()` — no request logging. Add a `slog`-based access log middleware:

```go
func SlogMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        start := time.Now()
        c.Next()
        slog.Info("http request",
            "method", c.Request.Method,
            "path", c.Request.URL.Path,
            "status", c.Writer.Status(),
            "duration_ms", time.Since(start).Milliseconds(),
        )
    }
}
```

This gives visibility into API traffic without relying on external proxies.

#### c) Add request ID / correlation

For debugging in production, propagate a request ID through both gRPC and HTTP layers. This helps correlate logs from a single user request across handler → repository → webhook calls.

---

## 4. Error Handling

### Current state

Error handling is functional but inconsistent in a few areas.

### Improvements

#### a) Distinguish "not found" from other errors in handlers

`GetContainerStatus` treats *all* scan errors as "not found":

```go
// handlers.go:143
cs, err := scanContainer(row)
if err != nil {
    c.JSON(http.StatusNotFound, gin.H{"error": "container not found"})
    return
}
```

If the DB is down or the query is malformed, the client gets a 404 instead of a 500. Fix:

```go
if errors.Is(err, pgx.ErrNoRows) {
    c.JSON(http.StatusNotFound, gin.H{"error": "container not found"})
} else {
    c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
}
```

Same issue exists in `GetAction` (line 385).

#### b) Avoid leaking internal errors to clients

Several handlers expose raw error messages:

```go
c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
```

In production, return a generic message and log the real error server-side:

```go
slog.Error("query failed", "error", err, "handler", "GetStatus")
c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
```

#### c) Check `rows.Err()` after iteration

After `for rows.Next()` loops, the code never calls `rows.Err()` to check for iteration errors. This is a `pgx` best practice:

```go
for rows.Next() { ... }
if err := rows.Err(); err != nil {
    c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
    return
}
```

This affects `GetStatus`, `GetNodes`, `GetNode`, `GetNodeStacks`, `ListActions`, and `GetAgents`.

---

## 5. Domain Types Location

### Problem

Domain types (`containerStatus`, `nodeContainers`, `composeStack`, `actionResponse`, `agentStatus`) are defined inside `handlers.go` and are unexported. This makes them:

- Unusable from other packages (e.g. the gRPC service that could benefit from shared types)
- Mixed with HTTP handler logic

### Proposal

Move domain types to a `models` or `domain` package, or at minimum into a separate file within `rest/`:

```
api/internal/
├── domain/
│   ├── container.go   // Container, Stack, NodeContainers
│   ├── command.go     // Command, ActionResponse
│   └── agent.go       // Agent
```

This also supports the repository pattern — repositories return domain types, handlers convert them to API responses if needed.

---

## 6. REST API Improvements

### a) Pagination

`GetStatus` and `GetNodes` return all records with no pagination. As the fleet grows, these endpoints will become slow and produce large payloads. Add `?limit=` and `?offset=` query parameters (or cursor-based pagination).

`ListActions` already has `LIMIT 50` but it's hardcoded — make it configurable via query params.

### b) Consistent error response format

Standardize error responses across all endpoints. Currently the shape is `{"error": "message"}` which is fine, but consider adding a status code field or error code for machine consumers:

```json
{"error": "container not found", "code": "NOT_FOUND"}
```

---

## 7. gRPC Service Responsibilities

### Problem

`MonitoringService` in `service.go` handles both data persistence and business logic (state transition detection, webhook dispatch, agent liveness). The `CheckAgentStatus` and `SweepStaleContainers` methods are background maintenance tasks that don't belong on the gRPC service type.

### Proposal

Extract background tasks into a separate `maintenance` or `scheduler` package:

```go
type Maintenance struct {
    containers ContainerRepo
    agents     AgentRepo
    notifier   *alerts.Notifier
}

func (m *Maintenance) SweepStaleContainers(ctx context.Context, maxAge time.Duration) (int64, error)
func (m *Maintenance) CheckAgentStatus(ctx context.Context)
```

This keeps `MonitoringService` focused on handling RPCs and makes maintenance logic independently testable.

---

## 8. Minor Code Quality

| Issue | Location | Suggestion |
|-------|----------|------------|
| Magic number `5*1e9` for timeout | `api/cmd/api/main.go:174` | Use `5 * time.Second` |
| `httpGet` helper reads body manually | `integration_test.go:343-352` | Use `io.ReadAll(resp.Body)` |
| `json.Unmarshal` errors silently ignored | `handlers.go:68-69` | At minimum log when unmarshal fails for labels/env_vars |
| Hardcoded `LIMIT 50` | `handlers.go:350` | Make configurable or add pagination params |
| No `rows.Err()` check after iteration | Multiple handlers | Add check after all `for rows.Next()` loops |

---

## Summary of priorities

| Priority | Improvement | Impact | Effort |
|----------|-------------|--------|--------|
| 1 | Repository pattern | High — fixes SQL scattering, simplifies tests, enables real DB testing | Medium |
| 2 | Error handling fixes (not-found vs 500, rows.Err()) | High — prevents silent bugs and incorrect status codes | Low |
| 3 | Configurable log level | Medium — essential for production readiness | Low |
| 4 | HTTP request logging middleware | Medium — observability | Low |
| 5 | Domain types extraction | Medium — cleaner architecture | Low |
| 6 | Pagination | Medium — scales with fleet size | Low |
| 7 | gRPC service decomposition | Low-Medium — cleaner separation of concerns | Medium |
| 8 | Request ID / correlation | Low — nice-to-have for debugging | Low |
