---
name: mss-project-onboarding
description: Orient a coding agent in this mss-boot-admin repository before implementation. Trigger for unfamiliar-repository analysis, architecture mapping, setup, locating commands, or deciding where a management-system change belongs. Do not use as a substitute for a feature-specific skill after the target module and change type are already known.
---

# MSS project onboarding

Use this workflow to establish repository facts before proposing or changing code.

## Inputs

- The user's goal or question.
- The current Git branch and working directory.
- Any named module, page, API, issue, or failing command.

## Procedure

1. Read the repository-root `AGENTS.md` and the nearest child `AGENTS.md` for the target directory.
2. Run:

   ```shell
   go run ./cmd/mss context --format json
   go run ./cmd/mss doctor --format json
   ```

3. Read `.mss/project.yaml`, `.mss/capabilities.yaml`, and `.mss/commands.yaml` only as needed to resolve details not obvious in the normalized context output.
4. Check whether an existing capability already implements the requested behavior. Do not create a parallel login, RBAC, organization, audit, task, notification, storage, or configuration subsystem.
5. Identify the correct boundary:
   - generic reusable primitive → `mss-boot/`;
   - agent tooling or contracts → `internal/mss/`, `cmd/mss/`, `.mss/`, or `.agents/`;
   - new business capability → `modules/<name>/` plus generated frontend/docs surfaces;
   - compatibility fix for existing code → the current horizontal package unless a dedicated migration spec exists;
   - documentation-only change → `docs/`.
6. Inspect the smallest set of implementation files, tests, migrations, frontend routes, and documentation that prove the current behavior.
7. For a non-trivial change, write a short scope statement covering:
   - goal;
   - in-scope and out-of-scope behavior;
   - affected contracts and modules;
   - compatibility and security impact;
   - planned validation.
8. Select the next focused skill, such as `$mss-add-module`, `$mss-add-field`, `$mss-add-permission`, `$mss-debug-fullstack`, or `$mss-review-change`.

## Guardrails

- Never assume a former standalone repository or personal absolute path exists.
- Treat historical prompts as evidence, not current requirements.
- Do not run production migrations or request production credentials for onboarding.
- Do not change code during onboarding unless the user explicitly requested implementation and the correct boundary is already established.
- Do not claim a service or dependency is available solely because it appears in configuration.

## Output

Return a compact onboarding summary with:

- repository and branch;
- relevant capabilities and their status;
- target directories and applicable instructions;
- canonical setup and verification commands;
- known risks or missing prerequisites;
- the next executable implementation step.
