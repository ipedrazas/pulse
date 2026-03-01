FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git

WORKDIR /src

# Copy workspace and all module files for layer caching
COPY go.work go.work
COPY proto/go.mod proto/go.sum ./proto/
COPY api/go.mod api/go.sum ./api/
COPY agent/go.mod agent/go.sum ./agent/

WORKDIR /src/api
RUN go mod download

# Copy source
WORKDIR /src
COPY proto/ ./proto/
COPY api/ ./api/

WORKDIR /src/api
RUN CGO_ENABLED=0 go build -o /pulse-api ./cmd/api

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S pulse && adduser -S pulse -G pulse

USER pulse

COPY --from=builder /pulse-api /usr/local/bin/pulse-api

EXPOSE 8080 50051

HEALTHCHECK --interval=30s --timeout=3s --retries=3 \
  CMD wget -qO- http://localhost:8080/healthz || exit 1

ENTRYPOINT ["pulse-api"]
