#!/usr/bin/env bash
set -Eeuo pipefail

usage() {
  cat <<'EOF'
Usage: deploy-release.sh --context KUBE_CONTEXT --admin-image IMAGE --web-image IMAGE --revision REVISION [--apply]

Without --apply this performs a server dry-run and diff only. With --apply it
applies the rendered manifests and waits for both Deployments to become ready.
EOF
}

kube_context=""
admin_image=""
web_image=""
revision=""
apply_release=false

while (($#)); do
  case "$1" in
    --context) kube_context="${2:-}"; shift 2 ;;
    --admin-image) admin_image="${2:-}"; shift 2 ;;
    --web-image) web_image="${2:-}"; shift 2 ;;
    --revision) revision="${2:-}"; shift 2 ;;
    --apply) apply_release=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
done

[[ -n "$kube_context" && -n "$admin_image" && -n "$web_image" && -n "$revision" ]] || {
  usage >&2
  exit 2
}
command -v kubectl >/dev/null || { echo "missing command: kubectl" >&2; exit 1; }
kubectl config get-contexts "$kube_context" -o name | grep -Fxq "$kube_context" || {
  echo "unknown Kubernetes context: $kube_context" >&2
  exit 1
}

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
render_dir="$(mktemp -d "${TMPDIR:-/tmp}/litellm-ops-release.XXXXXX")"
cleanup() { rm -rf -- "$render_dir"; }
trap cleanup EXIT

"$script_dir/render-release.sh" \
  --admin-image "$admin_image" \
  --web-image "$web_image" \
  --revision "$revision" \
  --output "$render_dir"

kubectl --context "$kube_context" apply --dry-run=server -f "$render_dir" >/dev/null
set +e
kubectl --context "$kube_context" diff -f "$render_dir"
diff_status=$?
set -e
((diff_status <= 1)) || { echo "kubectl diff failed" >&2; exit "$diff_status"; }

if [[ "$apply_release" != true ]]; then
  echo "plan complete; rerun with --apply after review"
  exit 0
fi

kubectl --context "$kube_context" apply -f "$render_dir"
kubectl --context "$kube_context" -n litellm-ops rollout status deployment/litellm-ops-admin --timeout=180s
kubectl --context "$kube_context" -n litellm-ops rollout status deployment/litellm-ops-web --timeout=180s
printf 'release %s is ready\n' "$revision"
