# litellm-ops live manifests

Apply only this directory into namespace `litellm-ops`.

TimescaleDB and LiteLLM allow rules live in the cliproxy repo, because
they are Ingress policies on the *destination* namespace `litellm`:

`/root/workspace/cliproxy/k8s/litellm/network-policies.yaml`

Do not apply `../k8s-litellm-ns/`.
