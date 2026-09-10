# Abandoned: do not deploy ops into the `litellm` namespace

This directory is a leftover from the 2026-09-08 attempt to colocate
litellm-ops with LiteLLM. That path is **not** the live deployment.

Live layout:

- Workload namespace: `litellm-ops`
- Manifests: `../k8s/`
- Ingress: `https://litellm-ops.flypool.io`
- Database access: NetworkPolicy
  `allow-litellm-ops-to-timescaledb` in `/root/workspace/cliproxy/k8s/litellm/network-policies.yaml`

Why this failed:

1. `litellm` already has `default-deny-ingress`. New pods in that
   namespace are unreachable from ingress-nginx unless you add an
   allow rule for those pod labels. That showed up as 503/504 and was
   misread as a Calico/cross-namespace cluster outage.
2. An egress deny-all on every pod in `litellm` would also break
   LiteLLM itself (upstream CLIProxy, Kvrocks, DNS). Never apply that.
3. Host `127.0.0.1:8080` is not LiteLLM or litellm-ops; another process
   has been bound there since 2026-09-01. Use a free port-forward.

Keep this directory only as a warning. Do not `kubectl apply -f` it.
