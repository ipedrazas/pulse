FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git

WORKDIR /src

# Copy workspace and all module files for layer caching
COPY go.work go.work
COPY proto/go.mod proto/go.sum ./proto/
COPY agent/go.mod agent/go.sum ./agent/
COPY api/go.mod api/go.sum ./api/

WORKDIR /src/agent
RUN go mod download

# Copy source
WORKDIR /src
COPY proto/ ./proto/
COPY agent/ ./agent/

WORKDIR /src/agent
RUN CGO_ENABLED=0 go build -o /pulse-agent ./cmd/agent

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

# No USER directive: the agent requires access to /var/run/docker.sock,
# which is owned by root:docker on the host. Mounting the socket already
# grants host-level Docker control, so dropping to non-root adds no
# meaningful security boundary here.

COPY --from=builder /pulse-agent /usr/local/bin/pulse-agent

ENTRYPOINT ["pulse-agent"]
