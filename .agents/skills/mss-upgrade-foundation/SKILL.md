---
name: mss-upgrade-foundation
description: Plan or apply a coordinated MSS Admin Distribution upgrade in a generated Thin Host while preserving business customizations. Trigger for `mss upgrade admin`, Blueprint synchronization, or managed-host conflicts. Do not use for routine Foundation dependency bumps.
---

# Upgrade an MSS Thin Host foundation

Use the target release's installed `mss` binary. It upgrades the pinned Admin Go package, Framework dependency, Admin Web package, and managed Thin Host files as one Distribution contract.

The engine compares the recorded old foundation hash, current downstream content, and new embedded foundation content. It updates only unchanged managed files; downstream-only files remain outside the manifest, and concurrent edits become conflicts.

## Preconditions

- The application must contain `.mss/blueprint-manifest.json`; a hand-assembled or legacy application without the recorded baseline cannot enter the three-way path directly.
- Back up the repository and database as appropriate, then work from a clean branch or a normal checkpoint commit.
- Install the `mss` binary for the exact target version and verify immutable release provenance with `mss --version`.
- In the Foundation repository, this skill may be used to exercise a downstream fixture. The normal adopter path never needs a Foundation checkout; `--foundation` is contributor-only.

## Procedure

1. Inspect the recorded baseline:

   ```shell
   mss upgrade status --format json
   ```

2. Produce the read-only coordinated plan:

   ```shell
   mss upgrade admin <vX.Y.Z> --format json
   ```

3. Review every `create`, `update`, `delete`, `preserve`, and `conflict`. Resolve conflicts in tracked source and rerun until the plan is successful.
4. Apply only that reviewed plan:

   ```shell
   mss upgrade admin <vX.Y.Z> --apply --yes --format json
   ```

5. Confirm `.mss/blueprint-manifest.json` and `.mss/lock.yaml` record the target Distribution and Blueprint identities, then run:

   ```shell
   mss doctor --format json
   mss skills validate --format json
   mss verify --changed
   ```

6. Re-run the same upgrade plan and require a no-op result.
7. Commit tracked managed-file changes, manifest/lock updates, preserved customizations, and explicit conflict resolutions. `.mss/reports` is ignored runtime evidence and is not required in the commit.

## Guardrails

- Planning is read-only; Admin apply requires both `--apply` and `--yes`.
- Never upgrade only one coordinated Admin Distribution component.
- Never overwrite or delete a locally customized managed file.
- Never advance the recorded baseline after a failed operation.
- Source upgrade does not run production database migrations or destructive runtime changes.
- A missing or invalid baseline manifest is an adoption/migration task, not permission to synthesize hashes or force an upgrade.

## Output

Report source and target Distribution/component versions, Blueprint identities, create/update/delete/preserve/conflict counts, preserved business files, validation results, no-op proof, and the rollback commit or branch.
