FROM rust:1.86-alpine AS builder
RUN apk add --no-cache musl-dev protobuf-dev
WORKDIR /src
COPY agent/Cargo.toml agent/Cargo.lock agent/build.rs ./agent/
COPY proto/ ./proto/
# Cache dependencies (real build.rs so tonic_build generates proto code)
RUN mkdir -p agent/src && echo 'fn main() {}' > agent/src/main.rs \
    && cd agent && cargo build --release
COPY agent/ ./agent/
RUN cd agent && cargo build --release

FROM alpine:3.21
RUN apk add --no-cache ca-certificates docker-cli docker-cli-compose
COPY --from=builder /src/agent/target/release/pulse-agent /usr/local/bin/pulse-agent
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD pgrep pulse-agent || exit 1
ENTRYPOINT ["pulse-agent"]
