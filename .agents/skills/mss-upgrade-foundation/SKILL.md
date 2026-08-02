---
name: mss-upgrade-foundation
description: Plan or apply an upgrade of an MSS-generated application from one foundation blueprint revision to another while preserving business customizations. Trigger for `mss upgrade`, base-template updates, foundation synchronization, or downstream contract changes. Do not use for routine dependency bumps inside the foundation repository itself.
---

# Upgrade an MSS application foundation

The upgrade engine performs a three-way comparison for every foundation-managed file:

```text
recorded old foundation hash + current downstream content + new foundation content
```

Files created only by the downstream application are outside the managed manifest and are preserved. A managed file is updated automatically only when its current content still matches the recorded baseline. Concurrent downstream and foundation changes produce a conflict instead of an overwrite.

## Preconditions

- Work from a clean downstream Git branch or create a normal checkpoint commit first.
- Obtain a trusted checkout of the target foundation version.
- Run the newer foundation CLI against the downstream project when the downstream CLI predates the required upgrade behavior:

  ```shell
  cd <new-foundation-checkout>
  go run ./cmd/mss --root <downstream-project> upgrade ...
  ```

## Procedure

1. Inspect the currently installed baseline:

   ```shell
   go run ./cmd/mss upgrade status --format json
   ```

2. Produce a conflict-aware plan:

   ```shell
   go run ./cmd/mss upgrade plan \
     --foundation <new-foundation-checkout> \
     --format json
   ```

3. Review every non-unchanged action:
   - `create`: new foundation-managed file;
   - `update`: foundation changed an unmodified downstream file;
   - `delete`: foundation removed an unmodified managed file;
   - `preserve`: downstream customized a file that the foundation did not change;
   - `conflict`: both sides changed, a required file was locally deleted, or a new foundation file collides with existing downstream content.

4. Resolve every conflict in a separate committed change. Re-run the plan until `success` is true.
5. Apply only the reviewed plan:

   ```shell
   go run ./cmd/mss upgrade apply \
     --foundation <new-foundation-checkout> \
     --yes \
     --format json
   ```

6. The new blueprint manifest is written only after all conflict-free file operations complete. Confirm it records the target blueprint version and foundation commit.
7. Run:

   ```shell
   go run ./cmd/mss doctor --format json
   go run ./cmd/mss skills validate --format json
   go run ./cmd/mss verify --changed
   go run ./cmd/mss eval run --all
   ```

8. Commit the upgrade, validation reports or summaries, and any explicit conflict resolutions. Do not rewrite prior history.

## Guardrails

- Planning is always read-only; applying requires both the `apply` subcommand and `--yes`.
- Never overwrite a managed file whose current hash differs from the old foundation when the new foundation also changed it.
- Never delete a locally customized file merely because the new foundation removed it.
- Unknown downstream files and business modules are not foundation-managed and must remain untouched.
- Never mark a new baseline installed when any file operation failed.
- Database migrations and destructive runtime changes require separate explicit policies; the source upgrade engine does not execute production migrations.
- Foundation checkouts must not contain uncommitted source changes because blueprint generation reads Git-tracked files and records the checked-out commit.

## Output

Report source and target blueprint versions and commits, create/update/delete/preserve/conflict counts, all conflicts, preserved customizations, files written, post-upgrade validation, and rollback commit or branch.
