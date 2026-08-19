---
name: mss-new-application
description: Create a new standalone management-system repository from the MSS foundation blueprint, with project contracts, safe local setup, baseline tests, and a three-way upgrade manifest. Trigger for “create a new admin/management system/project based on mss-boot”. Do not use to add a module to the current reference application.
---

# Create a new MSS application

Generate a Thin Business Host from a versioned Admin Distribution Blueprint. The host pins the complete Admin Go and npm dependencies, contains only application composition glue and business-owned extension locations, and records managed-file hashes for later three-way upgrades. It does not copy the Foundation's backend or frontend core sources.

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
   - exact Admin Distribution, Admin Go, Framework, and Admin Web versions;
   - generated backend registry and frontend config/runtime/locale glue;
   - excluded Foundation core, local, and runtime files;
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
7. Add business capabilities through `$mss-add-module`; generated modules compile into the same backend binary and frontend `dist` as the complete Admin.
8. Keep application-specific behavior under the generated business paths. Do not copy or edit Foundation core sources in the Thin Host.

## Guardrails

- Never embed passwords, tokens, private keys, production DSNs, kubeconfigs, or environment-specific secrets.
- The command defaults to dry-run and must never silently overwrite differing or unknown destination files.
- An explicit destination may be outside the foundation checkout only for this application-creation workflow; review it before `--write`.
- Never copy `.git`, reports, PID state, logs, caches, build output, `node_modules`, or archived one-off prompts.
- Preserve the generated lock and blueprint manifest; they are the base side of future three-way upgrades.
- Preserve exact coordinated dependency versions and the generated single-runtime frontend overrides; do not replace them with ranges or local paths.
- A Thin Host must retain the generated `web/tsconfig.json`, `web/config/config.ts`, app/access/locale glue, and empty generated registries even before its first business module is added.
- Do not select a moving development foundation for a production application without explicitly documenting that risk.

## Output

Report the destination, application module/repository, Admin Distribution and component versions, blueprint version, foundation commit, generated file count, conflicts, Git initialization status, validation results, and next module-development step.
