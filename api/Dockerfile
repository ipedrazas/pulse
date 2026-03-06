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
RUN cd api && CGO_ENABLED=0 go build -o /pulse-api ./cmd/api

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=builder /pulse-api /usr/local/bin/pulse-api
EXPOSE 8080 9090
ENTRYPOINT ["pulse-api"]
