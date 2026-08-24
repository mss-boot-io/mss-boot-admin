---
name: mss-upgrade-foundation
description: Plan or apply a coordinated Admin Distribution upgrade in this generated Thin Host while preserving business customizations. Trigger for `mss upgrade admin`, managed-file updates, or conflicts.
---

# Upgrade this Thin Host

Use the installed `mss` binary for the exact target release. A coordinated upgrade changes the Admin Go package, Framework, Admin Web package, and managed host files together.

## Preconditions

- `.mss/blueprint-manifest.json` must exist and validate; a hand-assembled application without that baseline needs a separate adoption path.
- Back up the repository and database as appropriate, then work from a clean branch or checkpoint commit.
- Verify target release provenance with `mss --version`.

## Procedure

1. Inspect the baseline: `mss upgrade status --format json`.
2. Produce a read-only plan: `mss upgrade admin <vX.Y.Z> --format json`.
3. Review every create/update/delete/preserve/conflict action. Resolve conflicts in tracked source and rerun until successful.
4. Apply the reviewed plan only:

   ```shell
   mss upgrade admin <vX.Y.Z> --apply --yes --format json
   ```

5. Confirm `.mss/blueprint-manifest.json` and `.mss/lock.yaml` record the target identities, then run:

   ```shell
   mss doctor --format json
   mss skills validate --format json
   mss verify --changed
   ```

6. Re-run the same plan and require a no-op result.
7. Commit tracked managed files, manifest/lock updates, preserved business changes, and conflict resolutions. `.mss/reports` is ignored runtime evidence and is not required in the commit.

Never upgrade only one Distribution component, overwrite a customization, synthesize a missing baseline, or treat source upgrade as authorization to run production database migrations.
