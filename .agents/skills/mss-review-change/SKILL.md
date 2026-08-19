---
name: mss-review-change
description: Review an MSS pull request or working-tree change for correctness, security, compatibility, generated drift, migrations, permissions, frontend contracts, tests, and documentation. Trigger for PR review, pre-merge review, architectural assessment, or “check whether this implementation is complete”. Do not use to author the original feature unless the review discovers a concrete fix the user asked you to implement.
---

# Review an MSS change

Prioritize defects and missing evidence over stylistic preferences.

## Procedure

1. Establish the base and head refs, changed files, and stated goal.
2. Read applicable `AGENTS.md` files and the affected `.mss` contracts.
3. Generate a change-impact plan:

   ```shell
   go run ./cmd/mss verify --changed --plan --format json
   ```

4. Review by risk area:
   - behavior and error handling;
   - authentication and backend authorization;
   - row-level data scope;
   - migration safety and rollback;
   - API compatibility and HTTP semantics;
   - generated/spec drift and hand-edited generated files;
   - frontend loading/error/empty/permission states;
   - concurrency, retries, transactions, and side effects;
   - secret handling and audit redaction;
   - tests, docs, and upgrade impact.
5. For generated modules, run:

   ```shell
   go run ./cmd/mss spec validate .mss/modules/<module>.yaml
   go run ./cmd/mss module generate .mss/modules/<module>.yaml --check
   ```

6. Run the smallest checks needed to confirm each suspected defect. Do not report speculative findings as confirmed facts.
7. Rank findings:
   - blocker: data loss, privilege bypass, unusable build/release, incompatible migration;
   - high: common-path correctness or security failure;
   - medium: edge-case correctness, operability, missing important test;
   - low: maintainability or documentation issue with limited runtime impact.
8. For each finding include:
   - file and concrete location;
   - failing scenario;
   - why existing tests or guards do not prevent it;
   - minimal repair direction;
   - validation needed.
9. If no material defect is found, state remaining test gaps and risks instead of inventing findings.

## Guardrails

- Do not approve based only on a green status badge.
- Do not treat frontend hiding as authorization.
- Do not accept destructive schema changes without migration evidence.
- Do not request broad rewrites when a focused fix is sufficient.
- Do not quote secrets or sensitive log payloads in review comments.
- Separate defects introduced by the change from pre-existing debt.

## Output

Lead with findings ordered by severity. Follow with validation performed, assumptions, and a concise merge-readiness conclusion.
