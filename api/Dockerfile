FROM golang:1.25-alpine AS builder
RUN apk add --no-cache git protobuf-dev
WORKDIR /src
COPY api/go.mod api/go.sum ./api/
COPY proto/go.mod proto/go.sum ./proto/
RUN go work init ./api ./proto && cd api && go mod download
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest \
    && go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
COPY proto/ ./proto/
RUN mkdir -p proto/gen/pulse/v1 \
    && protoc --go_out=proto/gen --go_opt=paths=source_relative \
           --go-grpc_out=proto/gen --go-grpc_opt=paths=source_relative \
           -I proto proto/pulse/v1/pulse.proto
COPY api/ ./api/
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
RUN cd api && CGO_ENABLED=0 go build \
    -ldflags="-X github.com/ipedrazas/pulse/api/internal/version.Version=${VERSION} \
    -X github.com/ipedrazas/pulse/api/internal/version.Commit=${COMMIT} \
    -X github.com/ipedrazas/pulse/api/internal/version.BuildDate=${BUILD_DATE}" \
    -o /pulse-api ./cmd/api

FROM alpine:3.21
RUN apk add --no-cache ca-certificates curl
COPY --from=builder /pulse-api /usr/local/bin/pulse-api
EXPOSE 8080 9090
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
  CMD curl -sf http://localhost:8080/healthz || exit 1
ENTRYPOINT ["pulse-api"]
