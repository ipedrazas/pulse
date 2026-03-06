# Pulse Agent

The Pulse agent is a Rust daemon that runs on compute nodes. It monitors local Docker containers and reports their state to the API server over a persistent gRPC stream. It also receives and executes commands (run, stop, pull, compose up) from the control plane.

## Installation

### From source

```bash
cd agent
cargo build --release
cp target/release/pulse-agent /usr/local/bin/
```

### Docker

```bash
docker run -d \
  --name pulse-agent \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -e PULSE_API_ADDR=http://api.example.com:9090 \
  -e PULSE_NODE_NAME=node-1 \
  git.andcake.dev/ivan/pulse-agent:latest
```

## Configuration

All configuration is via environment variables.

| Variable                | Description                                     | Default                                |
| ----------------------- | ----------------------------------------------- | -------------------------------------- |
| `PULSE_API_ADDR`        | gRPC address of the API server                  | `http://localhost:9090`                |
| `PULSE_NODE_NAME`       | Name this agent registers as                    | System hostname                        |
| `PULSE_POLL_INTERVAL`   | Seconds between Docker polls                    | `30`                                   |
| `PULSE_REDACT_PATTERNS` | Comma-separated env var name patterns to redact | `PASSWORD,SECRET,KEY,TOKEN,CREDENTIAL` |
| `PULSE_TLS_CERT`        | Path to TLS client certificate                  | _(none)_                               |
| `PULSE_TLS_KEY`         | Path to TLS client key                          | _(none)_                               |
| `PULSE_TLS_CA`          | Path to TLS CA certificate                      | _(none)_                               |

## How It Works

1. **Connect** — The agent connects to the API server's gRPC `StreamLink` endpoint with automatic reconnection and exponential backoff (500ms → 30s cap).
2. **Heartbeat** — On every poll cycle it sends a heartbeat with node name, agent version, and timestamp.
3. **Container Report** — It polls the Docker socket, builds a report of all containers (running and stopped), and sends it if anything changed (SHA-256 hash of container state is used for debounce).
4. **Command Execution** — It listens for `ServerCommand` messages on the stream and executes them:
   - `RunContainer` — pull image + create + start
   - `StopContainer` — stop by container ID
   - `PullImage` — pull with streaming progress
   - `ComposeUp` — shells out to `docker compose up -d`
5. **Redaction** — Environment variables whose names match any redact pattern are replaced with `***REDACTED***` before being sent to the API.

## systemd

Create `/etc/pulse/agent.env`:

```env
PULSE_API_ADDR=http://api.example.com:9090
PULSE_NODE_NAME=node-1
PULSE_POLL_INTERVAL=30
```

Install the unit file:

```ini
# /etc/systemd/system/pulse-agent.service
[Unit]
Description=Pulse Agent
Documentation=https://github.com/ipedrazas/pulse
After=network-online.target docker.service
Wants=network-online.target
Requires=docker.service

[Service]
Type=simple
EnvironmentFile=/etc/pulse/agent.env
ExecStart=/usr/local/bin/pulse-agent
Restart=always
RestartSec=5

# Security hardening
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
ReadWritePaths=/var/run/docker.sock

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=pulse-agent

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now pulse-agent
```

Check status and logs:

```bash
sudo systemctl status pulse-agent
journalctl -u pulse-agent -f
```
