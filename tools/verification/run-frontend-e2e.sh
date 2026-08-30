#!/usr/bin/env bash

set -euo pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
evidence_root="$(mktemp -d)"
cleanup() {
  status=$?
  trap - EXIT HUP INT TERM
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
corepack pnpm@10.34.5 exec playwright test --output="${evidence_root}/test-results"
