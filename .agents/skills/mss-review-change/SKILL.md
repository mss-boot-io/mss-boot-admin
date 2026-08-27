---
name: mss-review-change
description: Review an MSS Foundation or Thin Host pull request or working-tree change for correctness, security, compatibility, generated drift, migrations, permissions, frontend contracts, tests, and docs. Do not author unrelated features during review.
---

# Review an MSS change

Lead with evidence-backed defects and missing validation, not style preferences.

## Procedure

1. Establish base/head refs, changed files, stated goal, and whether the project layout is `foundation` or `thin-host`:

   ```shell
   mss context --format json
   mss verify --changed --plan --format json
   ```

   A Foundation contributor may substitute `GOWORK=off go run ./cmd/mss` only while reviewing unpublished CLI behavior; a Thin Host uses the installed v1.3.6 binary.

2. Read applicable `AGENTS.md` files and affected `.mss` contracts.
3. Review behavior and error handling, backend authorization, migration safety, API compatibility, generated/spec drift, frontend loading/error/empty/permission states, concurrency and side effects, secret redaction, tests, docs, and upgrade impact.
4. For generated modules, run:

   ```shell
   mss spec validate .mss/modules/<module>.yaml --format json
   mss module generate .mss/modules/<module>.yaml --check
   ```

   Check the request against the public v1.3.6 profile: `string`/`enum`/`bool`, `ownership: none`, simple CRUD/export, and coarse permissions. Treat claims of generated relation, import, workflow, or row-scope behavior as unsupported until separately implemented and tested.

5. Run the smallest focused checks needed to confirm each suspected defect. Do not report speculation as fact.
6. Rank findings as blocker, high, medium, or low. For each one include a concrete location, failing scenario, why current guards miss it, minimal repair, and required validation.
7. If no material defect is found, state remaining test gaps and risks instead of inventing findings.

## Guardrails

- Do not approve based only on a green status badge.
- Frontend hiding is not authorization.
- Do not accept destructive schema changes without upgrade evidence.
- Do not request a broad rewrite when a focused fix is sufficient.
- Do not quote secrets or sensitive payloads.
- Separate regressions introduced by the change from pre-existing debt.

## Output

Findings come first, ordered by severity, followed by validation performed, assumptions, and a concise merge-readiness conclusion.
