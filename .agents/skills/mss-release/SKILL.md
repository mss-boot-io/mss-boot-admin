---
name: mss-release
description: Prepare and verify a release of the MSS foundation, framework, frontend, docs, or a generated downstream application. Trigger for release readiness, version/tag planning, changelog, migration notes, artifacts, or rollback preparation. Do not use for merging an unverified feature PR.
---

# Release an MSS component

Release only from a reproducible commit with explicit component scope, compatibility notes, and validation evidence.

## Procedure

1. Identify the release component and tag namespace:
   - foundation/backend: `vX.Y.Z`;
   - framework: `mss-boot/vX.Y.Z`;
   - frontend: `web/antd-v6/vX.Y.Z`;
   - downstream application: its own repository policy.
2. Confirm all prerequisite stacked PRs are merged in dependency order.
3. Read `.mss/lock.yaml`, module specs, migration history, and release workflows.
4. Classify changes:
   - feature;
   - bug fix;
   - security fix;
   - migration;
   - breaking contract;
   - deprecated/removed capability;
   - generator or blueprint change.
5. For a foundation release, run:

   ```shell
   go run ./cmd/mss doctor --strict
   go run ./cmd/mss verify --all
   go run ./cmd/mss eval run --all
   ```

6. Verify generated output is current and all example specs validate.
7. For a framework release, additionally run independent `GOWORK=off` tests and confirm the nested module tag is resolvable.
8. Prepare release notes with:
   - user-visible changes;
   - required migrations and upgrade recipes;
   - new/deprecated capabilities;
   - security impact;
   - compatibility matrix;
   - rollback steps.
9. Confirm artifact, container, docs, and deployment workflows use only the consolidated repository.
10. Create the tag only after validation reports are attached or linked to the release commit.
11. After publishing, smoke-test installation from the released tag with caches disabled where practical.

## Guardrails

- Never release from a dirty working tree or an unreviewed generated diff.
- Never tag a backend version that requires an unpublished framework version.
- Never hide a breaking module path, schema, permission, configuration, or API change.
- Never publish secrets in artifacts, logs, release notes, or source maps.
- Do not archive predecessor repositories until replacement releases, redirects/notices, and rollback evidence exist.

## Output

Provide component/tag, release commit, validation and eval reports, migrations/recipes, artifacts, compatibility and security notes, smoke-test results, and rollback instructions.
