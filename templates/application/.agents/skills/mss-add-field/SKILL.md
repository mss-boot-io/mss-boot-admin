---
name: mss-add-field
description: Add or safely adjust a string, enum, or boolean field in an existing generated Thin Host module. Trigger for a simple field, validation, index, filter, or form change. Do not use for relations or destructive schema evolution.
---

# Add or evolve a generated field

The v1.3.7 public profile covers non-relational `string`, `enum`, and `bool` fields while `spec.ownership.mode` remains `none`.

1. Inspect `.mss/modules/<module>.yaml`, generated output, migration history, API clients, and existing-row constraints.
2. Decide whether the change is fresh-install-only or needs a separate forward migration. Never rewrite a released migration as an upgrade strategy.
3. Update the specification with explicit validation/default, unique/index/search/filter/list/form/detail behavior.
4. Validate and review before writing:

   ```shell
   mss spec validate .mss/modules/<module>.yaml --format json
   mss module generate .mss/modules/<module>.yaml --format json
   ```

5. If deployed rows need a backfill, representation change, nullable transition, destructive rename/drop, relation, import, workflow, or row scope, stop and define a separate Feature plus handwritten forward migration and tests. Generation alone is not upgrade proof.
6. Write, check, and run one focused verification pass:

   ```shell
   mss module generate .mss/modules/<module>.yaml --write
   mss module generate .mss/modules/<module>.yaml --check
   mss verify --module <module>
   ```

Test fresh migration, previous-version upgrade when applicable, old-row reads, validation failures, API compatibility, and UI rendering. Never hand-edit generated regions.
