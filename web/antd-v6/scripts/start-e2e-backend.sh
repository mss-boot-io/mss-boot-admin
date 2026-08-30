#!/usr/bin/env bash

set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
web_dir="$(cd "${script_dir}/.." && pwd)"
repo_dir="$(realpath "${web_dir}/../..")"
run_dir="$(realpath -m "${repo_dir}/.mss/run/antd-v6-e2e")"
backend_port="${MSS_V6_BACKEND_PORT:-18080}"
web_port="${MSS_V6_WEB_PORT:-18001}"
[[ "${backend_port}" =~ ^[0-9]+$ && "${web_port}" =~ ^[0-9]+$ ]] || {
  echo "MSS_V6_BACKEND_PORT and MSS_V6_WEB_PORT must be numeric" >&2
  exit 2
}
((backend_port >= 1 && backend_port <= 65535 && web_port >= 1 && web_port <= 65535)) || {
  echo "MSS_V6_BACKEND_PORT and MSS_V6_WEB_PORT must be within 1..65535" >&2
  exit 2
}
[[ "${backend_port}" != "${web_port}" ]] || {
  echo "frontend E2E backend and web ports must differ" >&2
  exit 2
}

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
runtime_dir="${run_dir}/runtime"

rm -f -- \
  "${database_path}" \
  "${database_path}-shm" \
  "${database_path}-wal" \
  "${binary_path}"
mkdir -p -- "${runtime_dir}/config"
install -m 0600 \
  "${repo_dir}/admin/config/application.yml" \
  "${runtime_dir}/config/application.yml"
python3 - \
  "${repo_dir}/admin/config/application-e2e.yml" \
  "${runtime_dir}/config/application-e2e.yml" \
  "${backend_port}" \
  "${web_port}" \
  "${database_path}" <<'PY'
import json
import sys
from pathlib import Path

source = Path(sys.argv[1])
target = Path(sys.argv[2])
backend_port = sys.argv[3]
web_port = sys.argv[4]
database_path = sys.argv[5]
content = source.read_text(encoding='utf-8')
if content.count('127.0.0.1:18080') != 1:
    raise SystemExit('expected exactly one backend address in the E2E configuration')
if content.count('http://127.0.0.1:18001') != 2:
    raise SystemExit('expected exactly two frontend origins in the E2E configuration')
database_line = "  source: '../.mss/run/antd-v6-e2e/admin.db?_busy_timeout=30000&_journal_mode=WAL'"
if content.count(database_line) != 1:
    raise SystemExit('expected exactly one E2E database source')
content = content.replace('127.0.0.1:18080', f'127.0.0.1:{backend_port}')
content = content.replace('http://127.0.0.1:18001', f'http://127.0.0.1:{web_port}')
database_dsn = f'{database_path}?_busy_timeout=30000&_journal_mode=WAL'
content = content.replace(database_line, f'  source: {json.dumps(database_dsn)}')
target.write_text(content, encoding='utf-8')
PY

export STAGE=e2e
export GOTOOLCHAIN=auto
export GIN_MODE=release

if [[ -z "${MSS_E2E_PASSWORD:-}" ]]; then
  echo "MSS_E2E_PASSWORD is required for the isolated browser qualification" >&2
  exit 1
fi

cd "${repo_dir}/admin"
go build -o "${binary_path}" .
cd "${runtime_dir}"
MSS_ADMIN_INITIAL_PASSWORD="${MSS_E2E_PASSWORD}" "${binary_path}" migrate \
  --username "${MSS_E2E_USERNAME:-admin}" \
  --domain "${MSS_E2E_DOMAIN:-127.0.0.1:${web_port}}"
unset MSS_ADMIN_INITIAL_PASSWORD MSS_E2E_PASSWORD
exec "${binary_path}" server
