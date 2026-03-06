FROM golang:1.25-alpine AS builder
RUN apk add --no-cache git
WORKDIR /src
COPY go.work ./
COPY api/go.mod api/go.sum ./api/
COPY cli/go.mod cli/go.sum ./cli/
COPY proto/go.mod proto/go.sum ./proto/
RUN cd api && go mod download
COPY proto/ ./proto/
COPY api/ ./api/
RUN cd api && CGO_ENABLED=0 go build -o /pulse-api ./cmd/api

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=builder /pulse-api /usr/local/bin/pulse-api
EXPOSE 8080 9090
ENTRYPOINT ["pulse-api"]
