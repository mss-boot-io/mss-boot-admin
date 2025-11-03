# Consul production deployment (Docker Compose)

This directory contains a production-ready Consul cluster configuration using Docker Compose.
It runs a 3-node server cluster with TLS and ACLs enabled.

> Important: For cross-host production, prefer Kubernetes (Helm) or systemd-managed binaries.
> This Compose setup is appropriate for a single host or as a reference baseline.

## What’s included

- 3 Consul servers (`consul-server-1..3`) with persistent volumes
- TLS-only API/UI (HTTPS 8501) and gRPC (8502)
- Gossip encryption (serf) via environment variable
- ACLs enabled with deny-by-default
- Minimal port exposure (only on server-1 by default)

## Prerequisites

- Docker and Docker Compose v2
- A CA and server certificate for Consul
- A gossip encryption key

## 1) Generate a gossip key

```bash
# On any machine with consul installed
consul keygen
# Copy the output into .env (GOSSIP_KEY)
```

## 2) Generate TLS certs

You can use Consul’s built-in TLS helper, your PKI, or Vault. Example with Consul helper:

```bash
# Create a local CA
dir=compose/consul/certs
mkdir -p "$dir"
consul tls ca create -days=3650 -domain=consul
mv consul-agent-ca.pem "$dir"/ca.pem

# Create a server cert that matches all three server hostnames
consul tls cert create -server -dc=dc1 -domain=consul \
  -additional-dnsname=consul-server-1 \
  -additional-dnsname=consul-server-2 \
  -additional-dnsname=consul-server-3
mv dc1-server-consul-0.pem "$dir"/server.pem
mv dc1-server-consul-0-key.pem "$dir"/server-key.pem
```

Notes:
- If you change `CONSUL_DATACENTER`, adjust the `-dc` value.
- Use your organization’s PKI for real production. Ensure the cert SubjectAltName covers the server DNS names.

## 3) Configure environment

```bash
cp compose/consul/.env.example compose/consul/.env
# Edit compose/consul/.env to set CONSUL_DATACENTER and GOSSIP_KEY
```

## 4) Start the cluster

```bash
# From repository root
docker compose -f compose/consul/docker-compose.prod.yml --env-file compose/consul/.env up -d

# Check cluster health
docker logs -f consul-server-1 | sed -n '1,200p'
```

Once one node is elected leader, the cluster is ready.

## 5) Bootstrap ACLs and save the management token

With ACLs enabled, bootstrap to get the initial management token:

```bash
# Exec into one server (uses TLS env vars set by the service)
docker exec -it consul-server-1 consul acl bootstrap
# Example output contains "SecretID": "<MGMT_TOKEN>"
```

Store this token securely. You can then create policies, roles, and tokens as needed.

Optionally, set the HTTP token for the server container to make subsequent CLI calls simpler:

```bash
docker exec -it consul-server-1 sh -lc 'export CONSUL_HTTP_TOKEN=<MGMT_TOKEN>; consul members'
```

## 6) Access the UI

- HTTPS: https://localhost:8501 (presented with your server cert)
- Authenticate using the Management Token when prompted (ACLs enabled)

For external access, place a reverse proxy (e.g., Nginx/Traefik) in front of port 8501.

## Ports and networking

- Exposed (server-1): 8501/tcp (HTTPS), 8502/tcp (gRPC), 8600/udp (DNS)
- Internal (all servers on `consul` network): 8300, 8301/8302 (serf), 8501, 8502

If you need WAN federation or multi-host clustering, use `retry_join` with routable IPs or cloud auto-join providers.

## Files

- docker-compose.prod.yml — Production Compose file for 3 servers
- config/server.hcl — Shared server configuration (TLS, ACLs, retry_join)
- .env.example — Template for environment variables (copy to .env)
- certs/ — Place your CA and server certificates here:
  - ca.pem
  - server.pem
  - server-key.pem

## Hardening tips

- Use distinct certs/keys per server and client; pin SANs tightly
- Lock down who can read the cert/key files on the host
- Run on separate hosts or VMs for real HA
- Place the HTTPS/UI behind a WAF/reverse proxy with mTLS if possible
- Regularly rotate gossip key and ACL tokens

## Troubleshooting

- Gossip not encrypting: ensure `GOSSIP_KEY` is set and all servers use the same value
- TLS errors: verify CA/cert paths and that SANs include the container hostnames
- ACL permission denied: ensure you’ve bootstrapped and provided CONSUL_HTTP_TOKEN when using the CLI
