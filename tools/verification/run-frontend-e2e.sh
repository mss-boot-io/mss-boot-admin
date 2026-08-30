#!/usr/bin/env bash

set -euo pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
evidence_root="$(mktemp -d)"
port_lock_fd=""
release_port_lock() {
  if [[ -n "${port_lock_fd:-}" ]]; then
    flock -u "${port_lock_fd}" >/dev/null 2>&1 || true
    exec {port_lock_fd}>&-
    port_lock_fd=""
  fi
}
cleanup() {
  status=$?
  trap - EXIT HUP INT TERM
  release_port_lock
  if [[ "${status}" -eq 0 ]]; then
    rm -rf -- "${evidence_root}"
  else
    echo "Frontend E2E evidence preserved at ${evidence_root}" >&2
  fi
  exit "${status}"
}
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

umask 077
export MSS_E2E_PASSWORD="MssE2E-A1-$(openssl rand -hex 24)"
export PLAYWRIGHT_HTML_OUTPUT_DIR="${evidence_root}/playwright-report"

cd "${repository_root}/web/antd-v6"
corepack pnpm@10.34.5 exec playwright install chromium

command -v flock >/dev/null 2>&1 || {
  echo "flock is required for collision-safe frontend E2E startup" >&2
  exit 1
}
port_lock_path="${TMPDIR:-/tmp}/mss-frontend-e2e.$(id -u).port-start.lock"
exec {port_lock_fd}>"${port_lock_path}"
if ! flock -w 600 "${port_lock_fd}"; then
  echo "timed out waiting 600s for frontend E2E port-start lock: ${port_lock_path}" >&2
  exit 1
fi
read -r backend_port web_port < <(python3 - <<'PY'
import socket

sockets = []
try:
    for _ in range(2):
        current = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        current.bind(('127.0.0.1', 0))
        sockets.append(current)
    print(*(current.getsockname()[1] for current in sockets))
finally:
    for current in sockets:
        current.close()
PY
)
[[ "${backend_port}" =~ ^[0-9]+$ && "${web_port}" =~ ^[0-9]+$ ]] || {
  echo "frontend E2E could not allocate loopback ports" >&2
  exit 1
}
[[ "${backend_port}" != "${web_port}" ]] || {
  echo "frontend E2E allocated the same backend and frontend port" >&2
  exit 1
}
export MSS_V6_BACKEND_PORT="${backend_port}"
export MSS_V6_WEB_PORT="${web_port}"
export MSS_V6_BACKEND_ORIGIN="http://127.0.0.1:${backend_port}"
export MSS_V6_BASE_URL="http://127.0.0.1:${web_port}"
export MSS_E2E_BACKEND_API_URL="${MSS_V6_BACKEND_ORIGIN}/admin/api"
export MSS_E2E_DOMAIN="127.0.0.1:${web_port}"
printf '%s\n' \
  "frontend E2E origins: backend=${MSS_V6_BACKEND_ORIGIN} frontend=${MSS_V6_BASE_URL}"
corepack pnpm@10.34.5 exec playwright test --output="${evidence_root}/test-results"
