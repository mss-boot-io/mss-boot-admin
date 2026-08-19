#!/usr/bin/env bash

# Start a command in a dedicated session and return the real session-leader PID
# through the named shell variable. `setsid` may fork when its caller is already
# a process-group leader, so the launcher's `$!` is not a reliable service PID.
mss_start_process_group() {
  if (($# < 5)); then
    echo "mss_start_process_group requires: output-var working-dir log-file pid-dir command..." >&2
    return 2
  fi

  local output_var="$1"
  local working_dir="$2"
  local log_file="$3"
  local pid_dir="$4"
  shift 4

  [[ "${output_var}" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]] || {
    echo "invalid process PID variable name: ${output_var}" >&2
    return 2
  }
  [[ -d "${working_dir}" ]] || {
    echo "process working directory does not exist: ${working_dir}" >&2
    return 1
  }
  [[ -d "${pid_dir}" ]] || {
    echo "process PID directory does not exist: ${pid_dir}" >&2
    return 1
  }
  command -v setsid >/dev/null 2>&1 || {
    echo "setsid is required for process supervision" >&2
    return 1
  }

  local pid_file="${pid_dir}/${output_var}.pid"
  [[ ! -e "${pid_file}" ]] || {
    echo "process PID file already exists: ${pid_file}" >&2
    return 1
  }

  (
    cd "${working_dir}" || exit 1
    # The single-quoted program is intentionally evaluated by the child Bash.
    # shellcheck disable=SC2016
    exec setsid --fork bash -c '
      pid_file="$1"
      shift
      printf "%s\n" "$$" > "${pid_file}"
      exec "$@"
    ' mss-process-group "${pid_file}" "$@"
  ) >> "${log_file}" 2>&1

  local pid=""
  for _ in {1..100}; do
    if [[ -s "${pid_file}" ]]; then
      read -r pid < "${pid_file}"
      break
    fi
    sleep 0.05
  done
  [[ "${pid}" =~ ^[1-9][0-9]*$ ]] || {
    echo "process did not publish a valid session PID: ${pid_file}" >&2
    return 1
  }

  local session_id=""
  session_id="$(ps -o sid= -p "${pid}" 2>/dev/null | tr -d '[:space:]')"
  if [[ "${session_id}" != "${pid}" ]] || ! kill -0 -- "-${pid}" 2>/dev/null; then
    echo "process did not start as session leader: pid=${pid} sid=${session_id:-missing}" >&2
    kill -TERM -- "${pid}" 2>/dev/null || true
    return 1
  fi

  printf -v "${output_var}" '%s' "${pid}"
}

mss_process_group_alive() {
  local pid="${1:-}"
  [[ "${pid}" =~ ^[1-9][0-9]*$ ]] || return 1
  kill -0 -- "-${pid}" 2>/dev/null
}

mss_stop_process_group() {
  local pid="${1:-}"
  [[ "${pid}" =~ ^[1-9][0-9]*$ ]] || return 0

  if mss_process_group_alive "${pid}"; then
    kill -TERM -- "-${pid}" 2>/dev/null || true
    for _ in {1..50}; do
      mss_process_group_alive "${pid}" || break
      sleep 0.1
    done
  fi
  if mss_process_group_alive "${pid}"; then
    kill -KILL -- "-${pid}" 2>/dev/null || true
    for _ in {1..50}; do
      mss_process_group_alive "${pid}" || break
      sleep 0.1
    done
  fi

  if mss_process_group_alive "${pid}"; then
    echo "process group did not terminate: ${pid}" >&2
    return 1
  fi
}
