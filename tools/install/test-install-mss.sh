#!/usr/bin/env bash
set -euo pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
test_root="$(mktemp -d "${TMPDIR:-/tmp}/mss-installer-test.XXXXXX")"
case "${test_root}" in
  "${TMPDIR:-/tmp}"/mss-installer-test.*) ;;
  *) echo "unsafe test directory" >&2; exit 1 ;;
esac
trap 'rm -rf -- "${test_root}"' EXIT

fixture="${test_root}/fixture"
fake_bin="${test_root}/fake-bin"
install_dir="${test_root}/installed"
mkdir -p "${fixture}/archive" "${fake_bin}" "${install_dir}"

commit="0123456789abcdef0123456789abcdef01234567"
timestamp="2026-08-25T00:00:00Z"
case "$(uname -s)" in
  Linux) platform="linux" ;;
  Darwin) platform="darwin" ;;
  *) echo "unsupported installer test operating system: $(uname -s)" >&2; exit 1 ;;
esac
case "$(uname -m)" in
  x86_64|amd64) architecture="amd64" ;;
  arm64|aarch64) architecture="arm64" ;;
  *) echo "unsupported installer test architecture: $(uname -m)" >&2; exit 1 ;;
esac
if command -v sha256sum >/dev/null 2>&1; then
  checksum_command=(sha256sum)
elif command -v shasum >/dev/null 2>&1; then
  checksum_command=(shasum -a 256)
else
  echo "installer test requires sha256sum or shasum" >&2
  exit 1
fi
for command_name in mss mss-mcp; do
  command_path="${fixture}/archive/${command_name}"
  cp /dev/null "${command_path}"
  {
    echo '#!/usr/bin/env bash'
    # This is source for the generated fixture, so its positional parameter
    # must remain literal until that fixture is executed.
    # shellcheck disable=SC2016
    printf 'if [[ "${1:-}" == "--version" ]]; then echo "%s version v1.3.3 (commit %s, timestamp %s)"; exit 0; fi\n' "${command_name}" "${commit}" "${timestamp}"
    echo 'exit 0'
  } > "${command_path}"
  chmod 0755 "${command_path}"
done
printf 'version=v1.3.3\ncommit=%s\ntimestamp=%s\n' "${commit}" "${timestamp}" > "${fixture}/archive/BUILD-INFO"
printf 'test-only license fixture\n' > "${fixture}/archive/LICENSE"
asset="mss-tools-v1.3.3-${platform}-${architecture}.tar.gz"
tar -czf "${fixture}/${asset}" -C "${fixture}/archive" BUILD-INFO LICENSE mss mss-mcp
"${checksum_command[@]}" "${fixture}/${asset}" | sed "s#${fixture}/##" > "${fixture}/SHA256SUMS.tools-v1.3.3"

cat > "${fake_bin}/curl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
output=""
url=""
while [[ "$#" -gt 0 ]]; do
  case "$1" in
    --output) output="$2"; shift 2 ;;
    --fail|--location|--silent|--show-error) shift ;;
    --retry) shift 2 ;;
    *) url="$1"; shift ;;
  esac
done
cp -- "${MSS_INSTALL_FIXTURE}/$(basename "${url}")" "${output}"
EOF
chmod 0755 "${fake_bin}/curl"

cat > "${fake_bin}/mv" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
previous=""
current=""
for argument in "$@"; do
  previous="${current}"
  current="${argument}"
done
if [[ "${MSS_INSTALL_FAIL_SECOND_MOVE:-}" = 1 &&
      "${previous}" == */.mss-mcp.install.* &&
      "${current}" == */mss-mcp &&
      ! -e "${MSS_INSTALL_FAILURE_MARKER}" ]]; then
  : > "${MSS_INSTALL_FAILURE_MARKER}"
  exit 73
fi
command -p mv "$@"
EOF
chmod 0755 "${fake_bin}/mv"

PATH="${fake_bin}:${PATH}" MSS_INSTALL_FIXTURE="${fixture}" \
  bash "${repository_root}/tools/install/install-mss.sh" \
    --version v1.3.3 \
    --install-dir "${install_dir}"

for command_name in mss mss-mcp; do
  test -x "${install_dir}/${command_name}"
  output="$("${install_dir}/${command_name}" --version)"
  [[ "${output}" == *v1.3.3* ]]
  [[ "${output}" == *"${commit}"* ]]
  [[ "${output}" == *"${timestamp}"* ]]
done

printf 'old mss marker\n' >> "${install_dir}/mss"
printf 'old mss-mcp marker\n' >> "${install_dir}/mss-mcp"
before_atomic_mss="$("${checksum_command[@]}" "${install_dir}/mss" | awk '{print $1}')"
before_atomic_mcp="$("${checksum_command[@]}" "${install_dir}/mss-mcp" | awk '{print $1}')"
failure_marker="${test_root}/second-move-failed"
if PATH="${fake_bin}:${PATH}" MSS_INSTALL_FIXTURE="${fixture}" \
  MSS_INSTALL_FAIL_SECOND_MOVE=1 MSS_INSTALL_FAILURE_MARKER="${failure_marker}" \
  bash "${repository_root}/tools/install/install-mss.sh" \
    --version v1.3.3 \
    --install-dir "${install_dir}" >/dev/null 2>&1; then
  echo "installer unexpectedly succeeded after the second replacement failed" >&2
  exit 1
fi
test -f "${failure_marker}"
after_atomic_mss="$("${checksum_command[@]}" "${install_dir}/mss" | awk '{print $1}')"
after_atomic_mcp="$("${checksum_command[@]}" "${install_dir}/mss-mcp" | awk '{print $1}')"
test "${before_atomic_mss}" = "${after_atomic_mss}"
test "${before_atomic_mcp}" = "${after_atomic_mcp}"
for command_name in mss mss-mcp; do
  output="$("${install_dir}/${command_name}" --version)"
  [[ "${output}" == *v1.3.3* ]]
  [[ "${output}" == *"${commit}"* ]]
done

before_mss="$("${checksum_command[@]}" "${install_dir}/mss" | awk '{print $1}')"
printf 'tamper\n' >> "${fixture}/${asset}"
if PATH="${fake_bin}:${PATH}" MSS_INSTALL_FIXTURE="${fixture}" \
  bash "${repository_root}/tools/install/install-mss.sh" \
    --version v1.3.3 \
    --install-dir "${install_dir}" >/dev/null 2>&1; then
  echo "installer accepted a tampered archive" >&2
  exit 1
fi
after_mss="$("${checksum_command[@]}" "${install_dir}/mss" | awk '{print $1}')"
test "${before_mss}" = "${after_mss}"

if bash "${repository_root}/tools/install/install-mss.sh" --version latest >/dev/null 2>&1; then
  echo "installer accepted a moving version" >&2
  exit 1
fi

echo "PASS install-mss checksum, provenance, atomic rollback, replacement, and tamper rejection"
