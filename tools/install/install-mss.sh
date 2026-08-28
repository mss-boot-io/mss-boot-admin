#!/usr/bin/env bash
set -euo pipefail

readonly DEFAULT_VERSION="v1.3.7"
readonly REPOSITORY="mss-boot-io/mss-boot-admin"

version="${DEFAULT_VERSION}"
install_dir="${HOME}/.local/bin"
stage_mss=""
stage_mcp=""
backup_mss=""
backup_mcp=""
temporary_dir=""

usage() {
  cat <<'EOF'
Install the versioned mss and mss-mcp release tools.

Usage:
  install-mss.sh [--version vX.Y.Z] [--install-dir PATH]

The installer verifies the release checksum before replacing either command.
It never uses sudo and never edits a shell profile.
EOF
}

while [[ "$#" -gt 0 ]]; do
  case "$1" in
    --version)
      [[ "$#" -ge 2 ]] || { echo "--version requires a value" >&2; exit 2; }
      version="$2"
      shift 2
      ;;
    --install-dir)
      [[ "$#" -ge 2 ]] || { echo "--install-dir requires a value" >&2; exit 2; }
      install_dir="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ ! "${version}" =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?$ ]]; then
  echo "invalid release version: ${version}" >&2
  exit 2
fi
if [[ -z "${install_dir}" ]]; then
  echo "install directory must not be empty" >&2
  exit 2
fi

case "$(uname -s)" in
  Linux) platform="linux" ;;
  Darwin) platform="darwin" ;;
  *) echo "unsupported operating system: $(uname -s)" >&2; exit 1 ;;
esac
case "$(uname -m)" in
  x86_64|amd64) architecture="amd64" ;;
  arm64|aarch64) architecture="arm64" ;;
  *) echo "unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

for command_name in cat curl tar awk sort mktemp mkdir install mv uname sed chmod rm; do
  command -v "${command_name}" >/dev/null 2>&1 || {
    echo "required command is missing: ${command_name}" >&2
    exit 1
  }
done
if command -v sha256sum >/dev/null 2>&1; then
  checksum_command=(sha256sum)
elif command -v shasum >/dev/null 2>&1; then
  checksum_command=(shasum -a 256)
else
  echo "required SHA-256 command is missing (sha256sum or shasum)" >&2
  exit 1
fi

cleanup() {
  if [[ -n "${stage_mss}" ]]; then rm -f -- "${stage_mss}"; fi
  if [[ -n "${stage_mcp}" ]]; then rm -f -- "${stage_mcp}"; fi
  if [[ -n "${backup_mss}" ]]; then rm -f -- "${backup_mss}"; fi
  if [[ -n "${backup_mcp}" ]]; then rm -f -- "${backup_mcp}"; fi
  if [[ -n "${temporary_dir}" ]]; then
    case "${temporary_dir}" in
      "${TMPDIR:-/tmp}"/mss-install.*) rm -rf -- "${temporary_dir}" ;;
    esac
  fi
}
trap cleanup EXIT

temporary_dir="$(mktemp -d "${TMPDIR:-/tmp}/mss-install.XXXXXX")"
case "${temporary_dir}" in
  "${TMPDIR:-/tmp}"/mss-install.*) ;;
  *) echo "temporary directory escaped the expected prefix" >&2; exit 1 ;;
esac

asset="mss-tools-${version}-${platform}-${architecture}.tar.gz"
manifest="SHA256SUMS.tools-${version}"
base_url="https://github.com/${REPOSITORY}/releases/download/${version}"

curl --fail --location --silent --show-error --retry 3 \
  --output "${temporary_dir}/${manifest}" "${base_url}/${manifest}"
curl --fail --location --silent --show-error --retry 3 \
  --output "${temporary_dir}/${asset}" "${base_url}/${asset}"

expected_hashes="$(
  awk -v target="${asset}" '
    NF == 2 {
      name=$2
      sub(/^\*/, "", name)
      if (name == target) print tolower($1)
    }
  ' "${temporary_dir}/${manifest}"
)"
expected_hash_count="$(printf '%s\n' "${expected_hashes}" | awk 'NF { count++ } END { print count + 0 }')"
if [[ "${expected_hash_count}" -ne 1 || ! "${expected_hashes}" =~ ^[0-9a-f]{64}$ ]]; then
  echo "checksum manifest must contain exactly one valid entry for ${asset}" >&2
  exit 1
fi
actual_hash="$("${checksum_command[@]}" "${temporary_dir}/${asset}" | awk '{print tolower($1)}')"
if [[ "${actual_hash}" != "${expected_hashes}" ]]; then
  echo "checksum verification failed for ${asset}" >&2
  exit 1
fi

actual_entries="$(tar -tzf "${temporary_dir}/${asset}" | sed 's#^\./##' | LC_ALL=C sort)"
expected_entries=$'BUILD-INFO\nLICENSE\nmss\nmss-mcp'
if [[ "${actual_entries}" != "${expected_entries}" ]]; then
  echo "tool archive has an unexpected file set" >&2
  exit 1
fi
mkdir -p "${temporary_dir}/unpacked"
tar -xzf "${temporary_dir}/${asset}" -C "${temporary_dir}/unpacked"
for relative in mss mss-mcp BUILD-INFO LICENSE; do
  path="${temporary_dir}/unpacked/${relative}"
  if [[ ! -f "${path}" || -L "${path}" ]]; then
    echo "tool archive member is not a regular file: ${relative}" >&2
    exit 1
  fi
done

declared_version="$(awk -F= '$1 == "version" { print $2 }' "${temporary_dir}/unpacked/BUILD-INFO")"
declared_commit="$(awk -F= '$1 == "commit" { print $2 }' "${temporary_dir}/unpacked/BUILD-INFO")"
declared_timestamp="$(awk -F= '$1 == "timestamp" { print substr($0, index($0, "=") + 1) }' "${temporary_dir}/unpacked/BUILD-INFO")"
if [[ "${declared_version}" != "${version}" || ! "${declared_commit}" =~ ^[0-9a-f]{40}$ ||
      ! "${declared_timestamp}" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}([.][0-9]+)?(Z|[+-][0-9]{2}:[0-9]{2})$ ]]; then
  echo "tool archive BUILD-INFO has invalid release provenance" >&2
  exit 1
fi
for command_name in mss mss-mcp; do
  chmod 0755 "${temporary_dir}/unpacked/${command_name}"
  output="$("${temporary_dir}/unpacked/${command_name}" --version 2>&1)"
  if [[ "${output}" != *"${version}"* || "${output}" != *"${declared_commit}"* ||
        "${output}" != *"${declared_timestamp}"* ]]; then
    echo "${command_name} version output does not match BUILD-INFO" >&2
    exit 1
  fi
done

mkdir -p -- "${install_dir}"
for destination in "${install_dir}/mss" "${install_dir}/mss-mcp"; do
  if [[ -e "${destination}" || -L "${destination}" ]]; then
    if [[ ! -f "${destination}" || -L "${destination}" ]]; then
      echo "existing tool destination is not a regular file: ${destination}" >&2
      exit 1
    fi
  fi
done
stage_mss="${install_dir}/.mss.install.$$"
stage_mcp="${install_dir}/.mss-mcp.install.$$"
backup_mss="${install_dir}/.mss.backup.$$"
backup_mcp="${install_dir}/.mss-mcp.backup.$$"
install -m 0755 "${temporary_dir}/unpacked/mss" "${stage_mss}"
install -m 0755 "${temporary_dir}/unpacked/mss-mcp" "${stage_mcp}"
had_mss=false
had_mcp=false
if [[ -f "${install_dir}/mss" && ! -L "${install_dir}/mss" ]]; then
  install -m 0755 "${install_dir}/mss" "${backup_mss}"
  had_mss=true
fi
if [[ -f "${install_dir}/mss-mcp" && ! -L "${install_dir}/mss-mcp" ]]; then
  install -m 0755 "${install_dir}/mss-mcp" "${backup_mcp}"
  had_mcp=true
fi

replaced_mss=false
replaced_mcp=false
replacement_error=""
if mv -f -- "${stage_mss}" "${install_dir}/mss"; then
  stage_mss=""
  replaced_mss=true
else
  replacement_error="replace mss"
fi
if [[ -z "${replacement_error}" ]]; then
  if mv -f -- "${stage_mcp}" "${install_dir}/mss-mcp"; then
    stage_mcp=""
    replaced_mcp=true
  else
    replacement_error="replace mss-mcp"
  fi
fi

if [[ -z "${replacement_error}" ]]; then
  for command_name in mss mss-mcp; do
    output="$("${install_dir}/${command_name}" --version 2>&1)" || {
      replacement_error="verify installed ${command_name}"
      break
    }
    if [[ "${output}" != *"${version}"* || "${output}" != *"${declared_commit}"* ||
          "${output}" != *"${declared_timestamp}"* ]]; then
      replacement_error="verify installed ${command_name}"
      break
    fi
  done
fi

if [[ -n "${replacement_error}" ]]; then
  rollback_error=""
  if [[ "${replaced_mss}" = true ]]; then
    if [[ "${had_mss}" = true ]]; then
      mv -f -- "${backup_mss}" "${install_dir}/mss" || rollback_error="${rollback_error} mss"
      backup_mss=""
    else
      rm -f -- "${install_dir}/mss" || rollback_error="${rollback_error} mss"
    fi
  fi
  if [[ "${replaced_mcp}" = true ]]; then
    if [[ "${had_mcp}" = true ]]; then
      mv -f -- "${backup_mcp}" "${install_dir}/mss-mcp" || rollback_error="${rollback_error} mss-mcp"
      backup_mcp=""
    else
      rm -f -- "${install_dir}/mss-mcp" || rollback_error="${rollback_error} mss-mcp"
    fi
  fi
  if [[ -n "${rollback_error}" ]]; then
    echo "installation failed during ${replacement_error}; rollback also failed for:${rollback_error}" >&2
  else
    echo "installation failed during ${replacement_error}; previous tool set was restored" >&2
  fi
  exit 1
fi

rm -f -- "${backup_mss}" "${backup_mcp}"
backup_mss=""
backup_mcp=""

printf 'installed mss and mss-mcp %s to %s\n' "${version}" "${install_dir}"
printf 'add this directory to PATH yourself if needed; no shell profile was changed\n'
