# Pulse

Pulse is a distributed system to manage container workloads across multiple remote machines.

## 1. System Overview

- **CLI:** Go (Cobra) for administrative commands.
- **API (Control Plane):** Go (Gin) for orchestration and state management.
- **UI:** React (Tailwind CSS) for monitoring and visualization.
- **Agent:** Rust for low-level system interaction and container management.
- **Database:** TimescaleDB (PostgreSQL) for storing agent state, metadata, and time-series metrics.

[Architecture Diagram](./architecute.md)

## 2. Communication Strategy
- **Agent <-> API:** Use **gRPC with Bidirectional Streaming**. The agent should maintain a long-lived connection. The API uses this stream to "push" commands to the agent. Presence (Online/Offline) must be handled by the API monitoring the stream's lifecycle (Context cancellation).
- **CLI <-> API:** Use gRPC for command execution and status queries to share Protobuf definitions with the agent.
- **UI <-> API:** Use standard REST (JSON) for dashboarding.

## 3. Component Requirements

### A. The Protobuf Definition (`/proto/dco.proto`)
- Define a `service AgentService`.
- Implement a bidirectional `rpc Connect(stream AgentMessage) returns (stream ServerCommand)`.
- `AgentMessage` should include `Heartbeat` and `ContainerReport` (list of running containers).
- `ServerCommand` should include `RunContainer` (image, env, ports) and `StopContainer`.

### B. The Rust Agent (Compute Node)
- Use `tonic` for gRPC.
- Use `bollard` for Docker/Containerd interaction via Unix Sockets.
- **Logic:** On startup, connect to the API. Every X seconds, scan the local container runtime and send a `ContainerReport`. Listen to the incoming stream for `RunContainer` commands and execute them locally.
- Each agent defines which actions are allowed to execute (docker pull, docker run, etc) by which user or agent in another node
- Will run as a systemd service.
- **Redact passwords**: create and env var that allows you define which strings needs redacting `ENV_REDACT_PATTERNS: PASSWORD,SECRET,KEY,TOKEN,CREDENTIAL`

### C. The Go API (Control Plane)
- Use `google.golang.org/grpc` for the server and `gin-gonic/gin` for the REST endpoints. Use the repository pattern.
- **Presence Logic:** When an agent connects, mark `status = 'online'` in TimescaleDB. When the gRPC `stream.Context().Done()` triggers, mark the agent `offline`.
- **Database:** Use `pgx` to interface with TimescaleDB. Create a hypertable for `container_events` (timestamp, agent_id, container_id, status).
- **Testing:** Use Testcontainers to test any database interaction.
- **Migrations:** Use github.com/golang-migrate/migrate for database migrations.


### D. The Go CLI
- Use `spf13/cobra`.
- **Command:** `pulse run --image <name> --node <agent_id>`. This should call the API gRPC endpoint, which then routes the command to the specific agent's active stream.
- **Command:** `pulse ps`. This should call the API gRPC endpoint, and return the status of all containers.
- **Command:** `pulse ps --node <agent_id>`. This should call the API gRPC endpoint, and return the status of all containers.
- **Command:** `pulse send --file file.txt --node <agent_id>`. This commands sends a file to the agent via the API.
- **Command:** `pulse logs container_name/container_id --node <agent_id>`. This should call the API gRPC endpoint, and return the logs of the specific container.
- **Command:** `pulse up -d --node <agent_id>`. This should call the API gRPC endpoint, which then routes the command to the specific agent's active stream.
- **Command:** `pulse pull --node <agent_id>`. This should call the API gRPC endpoint, which then routes the command to the specific agent's active stream. When no
container is specified, pulse aent will pull all the images from the Docker compose.


### E. The UI (React)
- Create a dashboard showing a list of Agents.
- Use an indicator (Green/Gray) for Online/Offline status based on the database state.
- Display a table of containers reported by the agents.
- Containers can be clicked to display more details (ENV VARs)

## 4. Technical Constraints
- **Error Handling:** Implement robust reconnection logic in the Rust agent using a backoff strategy.
- **Code Style:** Use clean architecture. Separate the "Domain" logic from the "Transport" layer (gRPC/REST).
- **Formatting:** Ensure Go code is `gofmt` compliant and Rust code is `cargo fmt` compliant.
- **Latex/Math:** For any performance calculations, use \( \text{math} \) for inline and $$ \text{math} $$ for blocks.

## 5. Execution Steps
1. Generate the `.proto` file first.
2. Implement the Go API gRPC server and Database schema.
3. Implement the Rust Agent connection logic and Docker integration.
4. Implement the CLI to send a test command.
5. Implement the UI to visualize the state.
