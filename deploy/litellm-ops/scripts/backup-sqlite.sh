#!/usr/bin/env bash
set -Eeuo pipefail

usage() {
  cat <<'EOF'
Usage: backup-sqlite.sh --context KUBE_CONTEXT --output /secure/path/backup.tgz --execute

Creates an offline archive of the litellm-ops SQLite PVC. The admin Deployment
must already be scaled to zero. The script never changes Deployment replicas.
EOF
}

kube_context=""
output_file=""
execute_backup=false

while (($#)); do
  case "$1" in
    --context) kube_context="${2:-}"; shift 2 ;;
    --output) output_file="${2:-}"; shift 2 ;;
    --execute) execute_backup=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
done

[[ -n "$kube_context" && -n "$output_file" ]] || { usage >&2; exit 2; }
[[ "$execute_backup" == true ]] || {
  echo "refusing to create a backup without --execute" >&2
  exit 2
}
[[ "$output_file" == /* && "$output_file" == *.tgz ]] || {
  echo "--output must be an absolute .tgz path" >&2
  exit 2
}
[[ -d "$(dirname -- "$output_file")" && ! -e "$output_file" ]] || {
  echo "output parent must exist and output must not already exist" >&2
  exit 1
}

command -v kubectl >/dev/null || { echo "missing command: kubectl" >&2; exit 1; }
command -v tar >/dev/null || { echo "missing command: tar" >&2; exit 1; }
replicas="$(kubectl --context "$kube_context" -n litellm-ops get deployment litellm-ops-admin -o jsonpath='{.spec.replicas}')"
[[ "$replicas" == "0" ]] || {
  echo "admin Deployment must be scaled to zero before an offline backup" >&2
  exit 1
}
if kubectl --context "$kube_context" -n litellm-ops get pods -l app=litellm-ops-admin \
  -o jsonpath='{range .items[*]}{.status.phase}{"\n"}{end}' | grep -Eq '^(Pending|Running)$'; then
  echo "admin pods are still active; wait for scale-down to complete" >&2
  exit 1
fi

helper_image="$(kubectl --context "$kube_context" -n litellm-ops get deployment litellm-ops-admin \
  -o jsonpath='{.spec.template.spec.containers[?(@.name=="admin")].image}')"
[[ "$helper_image" =~ ^[A-Za-z0-9._:/@-]+$ ]] || {
  echo "deployment contains an unsafe helper image reference" >&2
  exit 1
}

pod_name="litellm-ops-backup-$(date -u +%Y%m%d%H%M%S)-$$"
partial_file="${output_file}.partial.$$"
delete_helper() {
  kubectl --context "$kube_context" -n litellm-ops delete pod "$pod_name" \
    --ignore-not-found --wait=true --timeout=60s >/dev/null
}
cleanup() {
  rm -f -- "$partial_file"
  delete_helper >/dev/null 2>&1 || true
}
trap cleanup EXIT

kubectl --context "$kube_context" apply -f - >/dev/null <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: $pod_name
  namespace: litellm-ops
spec:
  restartPolicy: Never
  automountServiceAccountToken: false
  securityContext:
    runAsNonRoot: true
    runAsUser: 10001
    runAsGroup: 10001
    fsGroup: 10001
    seccompProfile: { type: RuntimeDefault }
  containers:
    - name: backup
      image: $helper_image
      imagePullPolicy: IfNotPresent
      command: ["/bin/sh", "-ec"]
      args:
        - |
          archive=/scratch/litellm-ops-data.tgz
          marker=/scratch/backup.ready
          rm -f -- "\$marker"
          tar -C /data -czf "\$archive" .
          test -s "\$archive"
          : >"\$marker"
          trap 'exit 0' TERM INT
          while :; do
            sleep 30 &
            wait \$!
          done
      securityContext:
        allowPrivilegeEscalation: false
        capabilities: { drop: ["ALL"] }
        readOnlyRootFilesystem: true
      resources:
        requests: { cpu: 1m, memory: 16Mi }
        limits: { cpu: 200m, memory: 128Mi }
      volumeMounts:
        - { name: db, mountPath: /data, readOnly: true }
        - { name: scratch, mountPath: /scratch }
        - { name: tmp, mountPath: /tmp }
  volumes:
    - name: db
      persistentVolumeClaim: { claimName: litellm-ops-admin-db, readOnly: true }
    - name: scratch
      emptyDir: { sizeLimit: 2Gi }
    - name: tmp
      emptyDir: { sizeLimit: 32Mi }
EOF

backup_ready=false
for _ in $(seq 1 90); do
  phase="$(kubectl --context "$kube_context" -n litellm-ops get pod "$pod_name" -o jsonpath='{.status.phase}')"
  if [[ "$phase" == "Running" ]] && kubectl --context "$kube_context" -n litellm-ops exec \
    -c backup "$pod_name" -- test -f /scratch/backup.ready -a -s /scratch/litellm-ops-data.tgz; then
    backup_ready=true
    break
  fi
  [[ "$phase" == "Failed" ]] && { echo "backup pod failed" >&2; exit 1; }
  [[ "$phase" == "Succeeded" ]] && { echo "backup pod exited before the archive was copied" >&2; exit 1; }
  sleep 2
done
[[ "$backup_ready" == true ]] || { echo "backup pod timed out" >&2; exit 1; }

kubectl --context "$kube_context" -n litellm-ops cp \
  -c backup "$pod_name:/scratch/litellm-ops-data.tgz" "$partial_file"
test -s "$partial_file"
tar -tzf "$partial_file" >/dev/null
tar -tzf "$partial_file" './mss-boot-admin.db' >/dev/null
chmod 0600 "$partial_file"
mv -- "$partial_file" "$output_file"
delete_helper
trap - EXIT
sha256sum "$output_file"
