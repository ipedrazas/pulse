# TLS for gRPC in Pulse

This guide covers creating, configuring, and managing TLS certificates to encrypt the gRPC channel between agents and the hub.

## Why TLS?

The gRPC channel carries container metadata, environment variables (even after redaction), and remote action commands (`compose_update`, `compose_restart`). TLS ensures this traffic is encrypted in transit.

## Overview

Pulse uses **server-side TLS** (not mutual TLS):

- The **hub** (API) presents a certificate to prove its identity.
- Each **agent** verifies the hub's certificate using a trusted CA certificate.
- Agents do not present certificates — authentication is handled by the `MONITOR_TOKEN` gRPC metadata header.

TLS is **fully optional**. When the env vars are not set, both hub and agents fall back to plaintext gRPC.

## 1. Create Certificates

### Option A: Self-signed CA (recommended for homelabs)

Generate a CA key and certificate, then sign a server certificate with it.

```bash
mkdir -p certs && cd certs

# 1. Create a CA key and certificate (valid 10 years)
openssl genrsa -out ca.key 4096
openssl req -x509 -new -nodes -key ca.key -sha256 -days 3650 \
  -out ca.crt -subj "/CN=Pulse CA"

# 2. Create a server key and CSR
openssl genrsa -out server.key 2048
openssl req -new -key server.key -out server.csr \
  -subj "/CN=pulse-api"

# 3. Create a SAN config (important — Go requires SANs)
cat > san.cnf <<EOF
[req]
distinguished_name = req_distinguished_name
[req_distinguished_name]
[v3_ext]
subjectAltName = @alt_names
[alt_names]
DNS.1 = api
DNS.2 = pulse-api
DNS.3 = localhost
IP.1  = 127.0.0.1
EOF

# 4. Sign the server certificate with the CA (valid 1 year)
openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key \
  -CAcreateserial -out server.crt -days 365 -sha256 \
  -extfile san.cnf -extensions v3_ext
```

> **Important:** The SAN (Subject Alternative Name) entries must match the hostname the agent uses to connect. In Docker Compose, the service name is `api`, so `DNS.1 = api` is required. Add any other hostnames or IPs agents might use.

This produces three files you need:

| File         | Used by   | Purpose                    |
|------------- |-----------|----------------------------|
| `ca.crt`     | Agent     | Verify the hub's identity  |
| `server.crt` | Hub (API) | Server certificate         |
| `server.key` | Hub (API) | Server private key         |

You can discard `server.csr`, `san.cnf`, and `ca.srl`.

### Option B: mkcert (quick local dev)

[mkcert](https://github.com/FiloSottile/mkcert) creates locally-trusted certificates with zero config:

```bash
mkcert -install
mkcert -cert-file certs/server.crt -key-file certs/server.key \
  api pulse-api localhost 127.0.0.1

# The CA cert is at:
cp "$(mkcert -CAROOT)/rootCA.pem" certs/ca.crt
```

## 2. Configure the Hub

Set two env vars for the API service:

| Variable        | Description              | Required |
|-----------------|--------------------------|----------|
| `TLS_CERT_FILE` | Path to `server.crt`     | Both or neither |
| `TLS_KEY_FILE`  | Path to `server.key`     | Both or neither |

If only one is set, the hub will refuse to start.

## 3. Configure the Agent

Set one env var for each agent:

| Variable      | Description          | Required |
|---------------|----------------------|----------|
| `TLS_CA_FILE` | Path to `ca.crt`     | No       |

When `TLS_CA_FILE` is not set, the agent connects in plaintext (backwards compatible).

## 4. Docker Compose Setup

Add volume mounts and env vars to your `compose.yml`:

```yaml
services:
  api:
    # ... existing config ...
    environment:
      TLS_CERT_FILE: /certs/server.crt
      TLS_KEY_FILE: /certs/server.key
    volumes:
      - ./certs/server.crt:/certs/server.crt:ro
      - ./certs/server.key:/certs/server.key:ro

  agent:
    # ... existing config ...
    environment:
      TLS_CA_FILE: /certs/ca.crt
    volumes:
      - ./certs/ca.crt:/certs/ca.crt:ro
```

The `:ro` suffix mounts certificates read-only, which is good practice.

## 5. Verify TLS is Working

After restarting, check the hub logs for:

```
{"level":"INFO","msg":"gRPC TLS enabled","cert":"/certs/server.crt"}
```

Without TLS you'll see:

```
{"level":"INFO","msg":"gRPC TLS disabled (no TLS_CERT_FILE set)"}
```

You can also test from the host with `grpcurl`:

```bash
# With TLS (using your CA)
grpcurl -cacert certs/ca.crt localhost:50051 list

# Without TLS (plaintext)
grpcurl -plaintext localhost:50051 list
```

## 6. Certificate Renewal

Certificates expire. When you need to renew:

1. Generate a new `server.crt` signed by the same CA (repeat step 3 from Option A).
2. Replace the file on disk.
3. Restart the API service — the certificate is loaded at startup.
4. Agents do **not** need to restart (they already trust the CA).

If you need to rotate the CA itself:

1. Generate a new CA key and certificate.
2. Sign a new server certificate with the new CA.
3. Distribute the new `ca.crt` to all agents.
4. Restart all services.

## 7. Remote Agents (outside Docker Compose)

When agents run on different VMs (the typical Proxmox homelab setup):

1. Copy `ca.crt` to each agent VM (e.g., `/etc/pulse/ca.crt`).
2. Set `TLS_CA_FILE=/etc/pulse/ca.crt` in the agent's environment.
3. Ensure the server certificate's SAN includes the hostname or IP the agent uses to reach the hub.

For example, if agents connect to `192.168.1.50:50051`, add `IP.2 = 192.168.1.50` to the SAN config when generating the certificate.

## 8. Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| Hub exits: `failed to load TLS certificate` | Cert or key file not found / wrong format | Check file paths and that files are PEM-encoded |
| Hub exits: `TLS_CERT_FILE and TLS_KEY_FILE must both be set or both be empty` | Only one of the two is set | Set both or remove both |
| Agent: `connection refused` or `transport: authentication handshake failed` | Agent using plaintext but hub expects TLS (or vice versa) | Ensure both sides match: either both use TLS or both use plaintext |
| Agent: `certificate signed by unknown authority` | `TLS_CA_FILE` doesn't match the CA that signed the server cert | Use the correct `ca.crt` |
| Agent: `certificate is not valid for any names` | Server cert SAN doesn't include the hostname the agent connects to | Regenerate server cert with the correct SAN entries |

## File Layout Reference

```
pulse/
  certs/
    ca.crt          # CA certificate (distribute to agents)
    ca.key          # CA private key (keep secure, never distribute)
    server.crt      # Server certificate (hub only)
    server.key      # Server private key (hub only)
  compose.yml
  .env
```

> **Security note:** Never commit `ca.key` or `server.key` to version control. Add `certs/*.key` to your `.gitignore`.
