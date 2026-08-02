---
name: mss-new-application
description: Create a new standalone management-system repository from the MSS foundation blueprint, with project contracts, safe local setup, baseline tests, and a three-way upgrade manifest. Trigger for “create a new admin/management system/project based on mss-boot”. Do not use to add a module to the current reference application.
---

# Create a new MSS application

Generate a complete downstream application from a versioned blueprint. The blueprint copies only Git-tracked foundation files, applies deterministic project/module substitutions, omits local state and historical prompt archives, and records content hashes for later three-way upgrades.

## Required inputs

- lower-case kebab-case application name;
- Go module, such as `github.com/acme/customer-admin`;
- display name when the title should not be derived from the application name;
- target repository in `owner/name` form when it cannot be inferred from a GitHub module;
- destination when the safe default `.mss/output/<name>` is not appropriate.

## Procedure

1. Inspect the current foundation and blueprint contract:

   ```shell
   go run ./cmd/mss context --format json
   go run ./cmd/mss new app --help
   ```

2. Produce the default dry-run plan. Do not pass `--write` yet:

   ```shell
   go run ./cmd/mss new app <name> \
     --module <go-module> \
     --repository <owner/name> \
     --destination <path> \
     --format json
   ```

3. Review:
   - destination and whether it is empty;
   - module and repository substitutions;
   - excluded local/runtime files;
   - file count, conflicts, and total bytes;
   - generated `.mss/lock.yaml` and `.mss/blueprint-manifest.json` provenance.

4. Write only after the plan is conflict-free:

   ```shell
   go run ./cmd/mss new app <name> \
     --module <go-module> \
     --repository <owner/name> \
     --destination <path> \
     --write \
     --git-init \
     --format json
   ```

5. Enter the generated repository and verify the foundation:

   ```shell
   go run ./cmd/mss doctor --format json
   go run ./cmd/mss setup
   go run ./cmd/mss verify --all
   go run ./cmd/mss upgrade status
   ```

6. Commit the generated baseline before adding business modules. The generated Git repository is intentionally not auto-committed.
7. Add business capabilities through `$mss-add-module`; do not edit foundation templates for application-specific behavior.

## Guardrails

- Never embed passwords, tokens, private keys, production DSNs, kubeconfigs, or environment-specific secrets.
- The command defaults to dry-run and must never silently overwrite differing or unknown destination files.
- An explicit destination may be outside the foundation checkout only for this application-creation workflow; review it before `--write`.
- Never copy `.git`, reports, PID state, logs, caches, build output, `node_modules`, or archived one-off prompts.
- Preserve the generated lock and blueprint manifest; they are the base side of future three-way upgrades.
- Do not select a moving development foundation for a production application without explicitly documenting that risk.

## Output

Report the destination, application module/repository, blueprint version, foundation commit, generated file count, conflicts, Git initialization status, validation results, and next module-development step.
