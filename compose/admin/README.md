# Primary Ant Design V6 local delivery

This compose profile serves the independently built `web/antd-v6` application
as the primary Admin client on `http://localhost:8001`. It proxies same-origin
`/admin/` HTTP and WebSocket traffic to the Go backend running on host port
`8080`.

From the repository root:

```shell
make web-build
docker compose -f compose/admin/docker-compose.yml up --detach --build
curl --fail http://localhost:8001/healthz
```

`MSS_FRONTEND_V6_IMAGE` may instead select an already qualified immutable V6
image digest. Production ingress, secrets, TLS, and image promotion remain
environment-owned; this local profile never injects production credentials.

Rollback always selects the preceding qualified V6 image and matching backend
release. The retired V5 image is not a deployment or recovery input.
