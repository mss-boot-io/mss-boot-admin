#!/usr/bin/env bash

set -euo pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
evidence_root="$(mktemp -d)"
cleanup() {
  rm -rf -- "${evidence_root}"
}
trap cleanup EXIT HUP INT TERM

umask 077
export MSS_E2E_PASSWORD="MssE2E-A1-$(openssl rand -hex 24)"
export PLAYWRIGHT_HTML_OUTPUT_DIR="${evidence_root}/playwright-report"

cd "${repository_root}/web/antd-v6"
corepack pnpm@10.34.5 exec playwright install chromium
corepack pnpm@10.34.5 exec playwright test --output="${evidence_root}/test-results"
