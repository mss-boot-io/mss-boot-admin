# Common Consul server configuration for production

server = true
bootstrap_expect = 3
datacenter = "dc1"
data_dir = "/consul/data"
log_level = "INFO"

# Bind to all interfaces inside container; advertise is auto on container IP
bind_addr = "0.0.0.0"
client_addr = "0.0.0.0"

# Join peers by DNS names (services within the same docker network)
retry_join = [
  "consul-server-1",
  "consul-server-2",
  "consul-server-3",
]

# Security hardening
disable_remote_exec = true
enable_script_checks = false

# Enable service mesh (Connect)
connect {
  enabled = true
}

# Autopilot for Raft
autopilot {
  cleanup_dead_servers      = true
  last_contact_threshold    = "200ms"
  max_trailing_logs         = 250
  server_stabilization_time = "10s"
}

# ACLs: enabled with deny-by-default. Bootstrap a management token after cluster is healthy.
acl {
  enabled                   = true
  default_policy            = "deny"
  down_policy               = "extend-cache"
  enable_token_persistence  = true
}

# TLS for HTTPS, gRPC, and internal RPC. Provide certs in ./certs.
ca_file  = "/consul/certs/ca.pem"
cert_file = "/consul/certs/server.pem"
key_file  = "/consul/certs/server-key.pem"
verify_incoming = true
verify_outgoing = true
verify_server_hostname = true

ports {
  http = -1      # disable plaintext HTTP
  https = 8501   # enable HTTPS
  grpc = 8502    # gRPC (TLS)
}

# UI over HTTPS only
ui_config {
  enabled = true
}
