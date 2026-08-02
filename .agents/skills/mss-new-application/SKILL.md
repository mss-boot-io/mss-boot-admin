---
name: mss-new-application
description: Create a new standalone management-system repository from the MSS foundation blueprint, with project contracts, selected capabilities, safe local setup, baseline tests, and an upgrade lock. Trigger for “create a new admin/management system/project based on mss-boot”. Do not use to add a module to the current reference application.
---

# Create a new MSS application

Generate a downstream application from a versioned blueprint instead of cloning and permanently forking the whole foundation without upgrade metadata.

## Inputs

- application name, Go module, display name, and repository destination;
- selected database and deployment profile;
- required built-in capabilities;
- frontend branding and default locale;
- initial organization/admin seed policy;
- foundation and blueprint version or channel.

## Procedure

1. Confirm the destination is empty or explicitly approved and remains outside the current repository unless creating a test fixture.
2. Inspect available blueprint and capability contracts:

   ```shell
   go run ./cmd/mss context --format json
   go run ./cmd/mss new app --help
   ```

3. Produce a dry-run plan:

   ```shell
   go run ./cmd/mss new app <name> \
     --module <go-module> \
     --destination <path> \
     --dry-run \
     --format json
   ```

4. Review planned files, substitutions, selected capabilities, generated secrets policy, and the initial `.mss/lock.yaml`.
5. Generate only after the plan is valid.
6. Run in the new application:

   ```shell
   go run ./cmd/mss doctor --strict
   go run ./cmd/mss setup
   go run ./cmd/mss verify --all
   ```

7. Confirm that:
   - no foundation Git history or hidden credentials were copied unintentionally;
   - the downstream repository has its own module, project metadata, and README;
   - generated files record blueprint and generator versions;
   - the FoundationLock points to a released or explicitly selected development foundation;
   - upgrade planning can discover the installed version.
8. Initialize Git and commit the generated baseline before adding business modules.
9. Add business capabilities through `$mss-add-module`, not by editing blueprint templates in the downstream repository.

## Guardrails

- Never embed real passwords, tokens, private keys, production DSNs, or kubeconfigs in a new application.
- Never overwrite a non-empty destination silently.
- Never copy `.git`, local reports, node_modules, build output, or personal editor state.
- Do not remove the foundation lock or generator provenance.
- Do not make downstream applications depend on unreleased moving branch content unless the user explicitly selects a development channel.

## Output

Report destination, foundation/blueprint versions, selected capabilities, generated files, setup and validation results, initial Git commit, and the next module-development step.
