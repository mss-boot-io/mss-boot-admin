---
name: mss-upgrade-foundation
description: Plan or apply an upgrade of an MSS-generated application from one foundation or blueprint version to another while preserving business customizations. Trigger for `mss upgrade`, base-template updates, module contract changes, dependency migrations, or downstream synchronization. Do not use for routine dependency bumps inside the foundation repository itself.
---

# Upgrade an MSS application foundation

An upgrade is a versioned transformation with preconditions, dry-run output, conflict reporting, verification, and rollback guidance.

## Procedure

1. Read `.mss/lock.yaml` and identify:
   - current foundation repository/version/channel;
   - blueprint version;
   - contract versions;
   - generated module versions;
   - previously applied upgrade recipes.
2. Run an upgrade plan before writing:

   ```shell
   go run ./cmd/mss upgrade plan --format json
   ```

3. Classify each proposed change:
   - dependency-only;
   - generated-file regeneration;
   - configuration migration;
   - database migration;
   - AST codemod;
   - manual conflict;
   - removed/deprecated capability.
4. Verify recipe preconditions using file hashes, syntax/AST shape, contract versions, and required prior recipes. Never rely only on line numbers.
5. Create a rollback point through a normal Git commit or branch. Do not rewrite pushed history.
6. Apply in deterministic order:
   - contract/schema migrations;
   - dependency updates;
   - codemods;
   - generator regeneration;
   - configuration updates;
   - database migration files;
   - docs and lock manifest.
7. Preserve handwritten extension files and regions. Stop and report a conflict rather than overwriting uncertain custom code.
8. Run generated drift checks and the full upgrade validation plan.
9. Update `.mss/lock.yaml` only after all mandatory transformations are applied successfully.
10. Produce a machine-readable upgrade report containing applied recipes, changed files, conflicts, skipped optional steps, validation, and rollback instructions.

## Guardrails

- Default to dry-run.
- Never mark a recipe applied when its write or validation failed.
- Never overwrite a file whose recorded generated hash no longer matches unless the change is a known mergeable generated region.
- Never execute a destructive database migration automatically without explicit policy and backup/rollback requirements.
- Do not require access to a production environment to transform source code.
- Keep recipes forward-only and uniquely versioned; fixes use a new recipe ID.

## Output

Report source and target versions, applied and pending recipes, conflicts, business customizations preserved, validation evidence, lockfile changes, and rollback procedure.
