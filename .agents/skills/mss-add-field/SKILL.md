---
name: mss-add-field
description: Add or safely adjust a string, enum, or boolean field in an existing generated MSS module and regenerate its v1.3.7 contracts. Trigger for a simple field, validation, index, filter, or form change. Do not use for relations or destructive schema evolution.
---

# Add or evolve a generated module field

Treat a persistent field change as both a specification change and a database compatibility decision.

## Command and scope context

- A Thin Host uses the installed `mss` v1.3.7 binary and `.mss/modules/<module>.yaml`.
- A Foundation contributor may substitute `GOWORK=off go run ./cmd/mss` while testing unpublished CLI changes. Use `mss context --format json` to locate generated output instead of hardcoding one repository layout.
- The public v1.3.7 frontend generator contract for this workflow is limited to non-relational `string`, `enum`, and `bool` fields with `ownership.mode: none`.

If the change needs a relation, nullable/representation transition, import surface, workflow, row scope, destructive rename/drop, or automatic live-data backfill, stop and create a separate Feature specification plus handwritten migration/business code. The v1.3.7 module generator does not make that upgrade safe by itself.

## Procedure

1. Read the canonical module specification and inspect current generated output, migration history, API clients, and existing-row constraints.
2. Decide whether the change is safe for fresh installs only or also needs a forward migration for deployed databases. Never rewrite a released migration as an upgrade strategy.
3. Update the field in `.mss/modules/<module>.yaml`. Keep the change within `string`, `enum`, or `bool`; define validation/default, unique/index/search/filter/list/form/detail behavior explicitly.
4. Validate and inspect the dry-run plan:

   ```shell
   mss spec validate .mss/modules/<module>.yaml --format json
   mss module generate .mss/modules/<module>.yaml --format json
   ```

5. If deployed rows are affected, add a separate forward-only handwritten migration and upgrade-path test before generation is considered complete. Use an expand/backfill/contract sequence when one deployment cannot safely enforce the final constraint.
6. Write and check generated output:

   ```shell
   mss module generate .mss/modules/<module>.yaml --write
   mss module generate .mss/modules/<module>.yaml --check
   ```

7. Test fresh migration, previous-version upgrade when applicable, old-row reads, validation failures, API compatibility, and UI rendering.
8. Run one focused verification pass:

   ```shell
   mss verify --module <module>
   ```

## Guardrails

- Never infer production compatibility from an empty development database.
- Never add a required or unique field without an existing-row and duplicate-data strategy.
- Never silently rename or drop a column.
- Never update only the frontend or only the Go model for a persistent field.
- Never hand-edit generated regions; keep custom migrations and behavior business-owned.

## Output

Summarize the old and new contract, supported-profile decision, migration/backfill behavior, generated changes, focused tests, and any capability moved to a separate Feature specification.
