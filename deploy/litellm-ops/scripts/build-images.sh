#!/usr/bin/env bash
set -Eeuo pipefail

usage() {
  cat <<'EOF'
Usage: build-images.sh --registry REGISTRY_PREFIX --revision GIT_REVISION [--platform linux/amd64]

Builds the admin and web images from an exact, clean checkout. The images are
tagged with the full commit SHA. This script never pushes images.
EOF
}

registry=""
revision=""
platform="linux/amd64"

while (($#)); do
  case "$1" in
    --registry) registry="${2:-}"; shift 2 ;;
    --revision) revision="${2:-}"; shift 2 ;;
    --platform) platform="${2:-}"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
done

[[ -n "$registry" && -n "$revision" ]] || { usage >&2; exit 2; }
[[ "$registry" =~ ^[A-Za-z0-9._:/-]+$ ]] || { echo "invalid registry prefix" >&2; exit 2; }
[[ "$platform" =~ ^linux/(amd64|arm64)$ ]] || { echo "unsupported platform: $platform" >&2; exit 2; }

for command_name in docker git go corepack; do
  command -v "$command_name" >/dev/null || { echo "missing command: $command_name" >&2; exit 1; }
done

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_dir="$(cd -- "$script_dir/../../.." && pwd)"
resolved_revision="$(git -C "$repo_dir" rev-parse --verify "${revision}^{commit}")"
head_revision="$(git -C "$repo_dir" rev-parse HEAD)"

[[ "$resolved_revision" == "$head_revision" ]] || {
  echo "revision must resolve to the checked-out HEAD" >&2
  exit 1
}
[[ -z "$(git -C "$repo_dir" status --porcelain --untracked-files=normal)" ]] || {
  echo "refusing to build from a dirty checkout" >&2
  exit 1
}

stage_dir="$(mktemp -d "${TMPDIR:-/tmp}/litellm-ops-build.XXXXXX")"
cleanup() { rm -rf -- "$stage_dir"; }
trap cleanup EXIT
mkdir -p "$stage_dir/backend" "$stage_dir/web/dist"

go_arch="${platform#linux/}"
(
  cd "$repo_dir/admin"
  CGO_ENABLED=0 GOOS=linux GOARCH="$go_arch" \
    go build -trimpath -ldflags="-s -w" -o "$stage_dir/backend/mss-boot-admin" .
)

(
  cd "$repo_dir/web/antd-v6"
  corepack pnpm run build:release
)
cp -R "$repo_dir/web/antd-v6/dist/." "$stage_dir/web/dist/"
cp "$repo_dir/deploy/litellm-ops/build/web/nginx.conf" "$stage_dir/web/nginx.conf"

registry="${registry%/}"
admin_image="$registry/admin:$resolved_revision"
web_image="$registry/web:$resolved_revision"

docker build --platform "$platform" \
  --label "org.opencontainers.image.revision=$resolved_revision" \
  -f "$repo_dir/deploy/litellm-ops/build/backend/Dockerfile" \
  -t "$admin_image" "$stage_dir/backend"
docker build --platform "$platform" \
  --label "org.opencontainers.image.revision=$resolved_revision" \
  -f "$repo_dir/deploy/litellm-ops/build/web/Dockerfile" \
  -t "$web_image" "$stage_dir/web"

printf 'admin image: %s\nweb image:   %s\n' "$admin_image" "$web_image"
