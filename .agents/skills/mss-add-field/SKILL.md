---
name: mss-add-field
description: Add or change a field in an existing MSS management module with a forward-compatible database migration, regenerated backend/frontend contracts, and upgrade-path tests. Trigger for requests to add columns, validation, indexes, search filters, form controls, or relation fields. Do not use to create a new module or to perform an unplanned destructive schema rewrite.
---

# Add or evolve a module field

Treat persistent schema evolution as an upgrade operation, not a search-and-replace task.

## Procedure

1. Identify the module's canonical specification:
   - preferred: `modules/<module>/module.yaml`;
   - pre-generation legacy module: current model, migration history, API contract, frontend type, and tests.
2. Inspect existing rows and compatibility assumptions:
   - nullability and default requirements;
   - uniqueness and index cardinality;
   - backfill source;
   - API clients that may omit the new value;
   - whether old application versions can read the new schema.
3. For a generated module, update the source spec field with explicit:
   - `name`, `column`, `goName`, and `displayName`;
   - type and nullability;
   - validation and default;
   - unique/index/search/filter/list/form/detail behavior;
   - UI component;
   - relation delete policy when applicable.
4. Validate and dry-run:

   ```shell
   go run ./cmd/mss spec validate modules/<module>/module.yaml --format json
   go run ./cmd/mss module generate modules/<module>/module.yaml --format json
   ```

5. Create an explicit forward migration when any of the following apply:
   - existing rows need a backfill;
   - a non-null field is introduced;
   - uniqueness is introduced;
   - data representation changes;
   - an index may be expensive;
   - rollback requires preserving old data.
6. Use an expand/backfill/contract sequence for risky changes:
   - add nullable or compatible storage;
   - deploy code that writes both or tolerates both;
   - backfill with restartable batches;
   - enforce the final constraint only after validation;
   - remove compatibility code in a later release.
7. Regenerate with `--write`, then add handwritten validation or business behavior that cannot be represented in the spec.
8. Update API examples, locale keys, list/filter/form/detail rendering, and tests as applicable.
9. Test:
   - fresh database migration;
   - upgrade from the previous schema;
   - old-row reads;
   - validation failures;
   - search and index behavior;
   - permission and ownership behavior remains unchanged.
10. Run:

   ```shell
   go run ./cmd/mss module generate modules/<module>/module.yaml --check
   go test ./modules/<module>/...
   go run ./cmd/mss verify --module <module>
   ```

## Guardrails

- Never infer a production backfill from an empty development database.
- Never add a non-null column without a valid existing-row strategy.
- Never silently rename or drop a column; preserve data through an explicit migration.
- Never add a unique index before detecting and resolving duplicates.
- Never update only the frontend or only the Go struct for a persistent field.
- Keep generated and handwritten responsibilities separate.

## Output

Summarize the old and new contract, migration order, backfill/rollback behavior, generated changes, custom logic, and the exact upgrade tests executed.
