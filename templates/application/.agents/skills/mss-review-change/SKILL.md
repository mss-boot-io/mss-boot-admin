---
name: mss-review-change
description: Review a Thin Host pull request or working-tree change for correctness, security, compatibility, generated drift, migrations, permissions, frontend contracts, tests, and documentation. Do not author unrelated features during review.
---

# Review a Thin Host change

1. Establish base/head refs, changed files, and the stated goal.
2. Read `AGENTS.md` and affected `.mss` contracts, then plan validation:

   ```shell
   mss context --format json
   mss verify --changed --plan --format json
   ```

3. Review backend authorization, migration safety, API compatibility, generated/spec drift, frontend loading/error/empty/permission states, concurrency, side effects, secret redaction, tests, docs, and upgrade impact.
4. For each changed generated module, run:

   ```shell
   mss spec validate .mss/modules/<module>.yaml --format json
   mss module generate .mss/modules/<module>.yaml --check
   ```

5. The v1.3.6 generated profile is limited to `string`/`enum`/`bool`, `ownership: none`, simple CRUD/export, and coarse permissions. Treat generated relation, import, workflow, or row-scope claims as unsupported until separately implemented and tested.
6. Confirm suspected defects with the smallest focused check. Rank findings by severity and give location, scenario, missing guard, minimal repair, and required validation.

Do not approve from a status badge alone, treat frontend hiding as authorization, accept destructive schema changes without upgrade evidence, or quote secrets. If no material defect is found, state remaining risks and test gaps.
