# LiteLLM Ops deployment

This directory contains the live `litellm-ops` Kubernetes manifests, immutable
release helpers, and container recipes. It does not contain credentials or a
generated ConfigMap. The only live workload directory is `k8s/`;
`k8s-litellm-ns/` is retained as an explicit abandoned-layout warning and must
never be applied.

## Security and availability invariants

- Pods do not receive Kubernetes service-account tokens.
- Containers run as fixed non-root users, use `RuntimeDefault` seccomp, drop all
  Linux capabilities, cannot gain privileges, and have read-only root filesystems.
- The backend can write only its PVC at `/data` and bounded ephemeral volumes at
  `/app/logs` and `/tmp`. Nginx can write only its bounded cache/run/tmp volumes.
- The intentionally small CPU and memory requests are preserved.
- The checked-in image references and revision are non-runnable placeholders.
  A release must be rendered with explicit, non-`latest` image references.
- This repository does not add a NetworkPolicy. Cross-namespace LiteLLM and
  TimescaleDB allow rules remain destination-owned in
  `/root/workspace/cliproxy/k8s/litellm/network-policies.yaml`; changing that
  boundary requires a separate connectivity review.

## Prerequisites

Use an explicit Kubernetes context for every command. Confirm that the
`litellm-ops-admin-config` ConfigMap and the referenced Secrets exist without
printing their values:

```sh
kubectl --context CONTEXT -n litellm-ops get configmap litellm-ops-admin-config >/dev/null
kubectl --context CONTEXT -n litellm-ops get secret litellm-ops-admin-init litellmops-env litellm-masterkey >/dev/null
```

The production ConfigMap must point SQLite at a file below `/data`, keep file
logs below `/app/logs` (or use stdout), bind HTTP to `0.0.0.0:8080`, use the exact
HTTPS browser origin, and enable secure browser cookies. Review these settings
in the source that generates the ConfigMap; do not export the live ConfigMap to
the repository or an operator transcript.

## Build

Build from a reviewed, clean checkout. The helper builds the release frontend,
compiles a static Linux backend, uses a temporary build context, and tags both
images with the full Git commit. It never pushes:

```sh
deploy/litellm-ops/scripts/build-images.sh \
  --registry REGISTRY/litellm-ops \
  --revision "$(git rev-parse HEAD)"
```

Scan the images, push them through the approved registry path, and record the
resulting `repo/image@sha256:...` references. Digest references are preferred;
the render helper rejects `latest`, while registry policy must make any other
release tag immutable. Build staging,
database files, WAL/SHM files, logs, binaries, rendered manifests, and local
backups are ignored by Git.

## Plan and deploy

First render locally and inspect the exact output:

```sh
out="$(mktemp -d)"
deploy/litellm-ops/scripts/render-release.sh \
  --admin-image REGISTRY/litellm-ops/admin@sha256:ADMIN_DIGEST \
  --web-image REGISTRY/litellm-ops/web@sha256:WEB_DIGEST \
  --revision GIT_SHA \
  --output "$out"
```

Then run a server dry-run and diff. The deploy helper is plan-only unless
the operator supplies `--apply`:

```sh
deploy/litellm-ops/scripts/deploy-release.sh \
  --context CONTEXT \
  --admin-image REGISTRY/litellm-ops/admin@sha256:ADMIN_DIGEST \
  --web-image REGISTRY/litellm-ops/web@sha256:WEB_DIGEST \
  --revision GIT_SHA
```

After reviewing the diff and taking the backup described below, rerun the same
command with `--apply`. The helper waits for both Deployments. Also verify the
public health endpoint, login, one read-only LiteLLM operation, and audit log.
Do not paste response bodies, cookies, tokens, or Secret values into logs.

## Offline backup and restore drill

The SQLite backup contains authentication and audit data and is sensitive.
Store it encrypted with mode `0600`, outside the repository, with an owner,
retention date, checksum, and release revision.

1. Announce a maintenance window and stop writes:
   `kubectl --context CONTEXT -n litellm-ops scale deployment/litellm-ops-admin --replicas=0`.
2. Wait until no admin pod is Pending or Running.
3. Run `backup-sqlite.sh --context CONTEXT --output /secure/path/NAME.tgz --execute`.
   It mounts the PVC read-only in a short-lived hardened pod and refuses to run
   while the Deployment is active. It never changes the replica count.
4. Restart the backend with one replica and verify readiness and login.
5. On a schedule, restore a copy into a new isolated PVC and start the matching
   image without public Ingress. A checksum alone is not a restore test.

Never overwrite the live PVC during a restore drill. A real restore is an
incident action: preserve the failed volume, keep the backend stopped, restore
into a new PVC, validate offline, then switch the claim reference in a reviewed
change. Restoring an older archive discards all later orders and audit records.

## Rollback

Record the previous admin image, web image, revision, and backup checksum before
each release. For a code-only rollback, run `deploy-release.sh` with those exact
previous immutable image references, review the plan, and then add `--apply`.
Do not use an unrecorded mutable tag.

The migration init container may make a database rollback unsafe. If the old
binary is not forward-compatible with the migrated schema, keep the service in
maintenance and follow the reviewed restore procedure instead. Never restore a
pre-release database while allowing new orders or top-ups, because that can
replay fulfilled transactions.
