#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=tools/compatibility/process-groups.sh
source "${script_dir}/process-groups.sh"

test_root="$(mktemp -d "${TMPDIR:-/tmp}/mss-process-group-test.XXXXXX")"
case "${test_root}/" in
  "${TMPDIR:-/tmp}/mss-process-group-test."*/) ;;
  *)
    echo "unsafe process-group test directory: ${test_root}" >&2
    exit 1
    ;;
esac

test_pid=""
cleanup() {
  local status=$?
  trap - EXIT HUP INT TERM
  mss_stop_process_group "${test_pid}" || true
  rm -rf -- "${test_root}"
  exit "${status}"
}
trap cleanup EXIT HUP INT TERM

log_file="${test_root}/process.log"
mss_start_process_group \
  test_pid \
  "${test_root}" \
  "${log_file}" \
  "${test_root}" \
  bash -c 'trap "exit 0" TERM; while :; do sleep 0.1; done'

mss_process_group_alive "${test_pid}"
session_id="$(ps -o sid= -p "${test_pid}" | tr -d '[:space:]')"
[[ "${session_id}" == "${test_pid}" ]] || {
  echo "test process is not its session leader: pid=${test_pid} sid=${session_id}" >&2
  exit 1
}

mss_stop_process_group "${test_pid}"
if mss_process_group_alive "${test_pid}"; then
  echo "test process group survived bounded cleanup: ${test_pid}" >&2
  exit 1
fi
test_pid=""

echo "External-host process supervision passed"
