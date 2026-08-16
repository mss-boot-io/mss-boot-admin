#!/usr/bin/env bash

set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
web_dir="$(cd "${script_dir}/.." && pwd)"
repo_dir="$(realpath "${web_dir}/../..")"
run_dir="$(realpath -m "${repo_dir}/.mss/run/antd-v6-e2e")"

case "${run_dir}/" in
  "${repo_dir}/.mss/run/"*) ;;
  *)
    echo "Refusing to use an E2E run directory outside the repository" >&2
    exit 1
    ;;
esac

mkdir -p "${run_dir}"
database_path="${run_dir}/admin.db"
binary_path="${run_dir}/mss-boot-admin-e2e"

rm -f -- \
  "${database_path}" \
  "${database_path}-shm" \
  "${database_path}-wal" \
  "${binary_path}"

export STAGE=e2e
export GOTOOLCHAIN=auto
export GIN_MODE=release

cd "${repo_dir}/admin"
go build -o "${binary_path}" .
"${binary_path}" migrate \
  --username "${MSS_E2E_USERNAME:-admin}" \
  --password "${MSS_E2E_PASSWORD:-123456}" \
  --domain 127.0.0.1:8001
exec "${binary_path}" server
