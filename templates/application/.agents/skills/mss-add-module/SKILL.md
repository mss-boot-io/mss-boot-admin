---
name: mss-add-module
description: Add a simple generated CRUD module to this MSS v1.3.7 Thin Host. Trigger for a new management resource using string, enum, or boolean fields. Do not use for relations, imports, workflows, or row-scoped ownership.
---

# Add a Thin Host module

Use the installed `mss` v1.3.7 binary and keep the specification as the source of truth.

## Supported profile

- fields: `string`, `enum`, `bool`;
- `spec.ownership.mode: none`;
- starter list/get/create/update/delete/export API;
- coarse backend RBAC, menu/action projection, generated Admin list/form/detail/export UI, and generated tests.

Relations, import, workflow/state-machine generation, and row-level ownership are outside this profile. If requested, stop before generation, create a separate Feature specification, and implement the behavior in handwritten business code with focused tests.

## Procedure

1. Inspect `.mss/project.yaml`, `.mss/capabilities.yaml`, and existing module specs. Check module, table, API, menu, migration-ID, and permission collisions.
2. Create the starter at its canonical path:

   ```shell
   mss spec init <name> --kind module \
     --output .mss/modules/<name>.yaml --write
   ```

3. Edit only within the supported profile, then validate and inspect the dry-run plan:

   ```shell
   mss spec validate .mss/modules/<name>.yaml --format json
   mss module generate .mss/modules/<name>.yaml --format json
   ```

4. Stop if the plan is incomplete, unsupported/deferred, escapes the repository, or collides with handwritten files.
5. Write, check, and verify:

   ```shell
   mss module generate .mss/modules/<name>.yaml --write
   mss module generate .mss/modules/<name>.yaml --check
   mss verify --module <name>
   ```

6. Put unsupported behavior in `internal/modules/<name>/` or `web/src/business/<name>/` without a generated-file header. For a larger unsupported capability, create its contract first:

   ```shell
   mss spec init <feature-name> --kind feature --module <name> \
     --output .mss/features/<feature-name>.yaml --write
   ```

Never hand-edit generated files, weaken backend authorization, or claim a projection the generation plan does not contain.
