#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT}"

export CI="${CI:-true}"
export COREPACK_ENABLE_DOWNLOAD_PROMPT=0

mkdir -p .mss/cache

printf 'mss Codex setup\n'
printf 'root: %s\n' "${ROOT}"
printf 'go: %s\n' "$(go version)"
printf 'node: %s\n' "$(node --version)"

if command -v corepack >/dev/null 2>&1; then
  corepack enable
fi

# The repository CLI owns the setup sequence. It is non-interactive,
# idempotent, and does not require production credentials.
go run ./cmd/mss setup

# Fail the environment build early when required tools or repository contracts
# are still unavailable. Optional external services remain advisory.
go run ./cmd/mss doctor --format json > .mss/cache/codex-doctor.json

go run ./cmd/mss skills validate --format json > .mss/cache/codex-skills.json

go run ./cmd/mss context --format json > .mss/cache/codex-context.json

printf 'mss Codex setup complete\n'
