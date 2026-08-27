---
name: mss-update-release-docs
description: Reconcile MSS source documentation, machine-readable guidance, and public Docs-site state to one exact stable, current, candidate, or historical release baseline. Trigger for post-release documentation, stable-version changes, README and changelog version sweeps, release-note reconciliation, install or image example updates, stale “latest” claims, and a deployed Docs site that still shows old release facts. Do not use this skill by itself to create or move tags or publish artifacts; route authorized publication through the MSS release workflow.
---

# Update MSS release documentation

Make every current-facing claim traceable to compiling code, a machine-readable contract, an exact merged commit, or a public artifact. Preserve historical truth and published checksums while doing so.

## Establish the release ledger

Read the applicable `AGENTS.md`, `.mss/project.yaml`, `.mss/lock.yaml`, `.mss/release-policy.yaml`, `.mss/release-qualification.json`, selected Feature contracts, release workflows, package manifests, and existing changelogs before editing.

Record a compact ledger:

- repository and component;
- public version and whether it is stable, candidate, partial, previous, or planned;
- exact merged-main commit;
- root, Framework, Admin, frontend, Docs, image, and package tag names as applicable;
- public Release, registry, image digest, and evidence references;
- public Docs URL plus its `/release.json` version and full commit;
- previous stable and rollback baseline;
- capability maturity that has independent evidence.

Verify unstable facts remotely. A version string in one document is not publication evidence. When sources conflict, use the repository source-of-truth order and state the conflict before resolving it.

## Inventory the documentation surface

Use `git ls-files` and `rg` to find:

- the target version and recent version strings;
- `latest`, `stable`, `current`, `recommended`, `candidate`, and rollback language;
- install commands, Go module versions, npm versions, image tags, and release URLs;
- duplicated English and Chinese guidance;
- machine-readable descriptions under `.mss/` that agents use as documentation.

Classify every match before changing it:

| Class | Treatment |
| --- | --- |
| Current guidance | Reconcile to the verified current baseline. |
| Historical release record | Preserve the historical version and add context only when it prevents confusion. |
| Archived prompt or handoff | Leave unchanged unless the current contract explicitly references it. |
| Generated output | Change its source contract or generator, not the generated region. |
| Published module or package tree | Preserve bytes unless a new version will be released through the normal PR path. |

Do not perform a blanket version replacement. Keep a small fact-to-file matrix when the sweep spans several surfaces.

## Write the release story consistently

- Call a release stable only when the exact public train has been reconciled. Describe partial releases and release candidates as immutable history.
- Describe the previous stable as previous or rollback baseline; do not leave it labeled current.
- Keep component tag namespaces exact. Do not imply that root, Framework, Admin, frontend, Docs, npm, and images share one ref when they do not.
- Keep install, upgrade, rollback, compatibility, migration, and security guidance aligned with the verified artifact.
- Keep English and Chinese current-facing guidance semantically equivalent.
- Separate release stability from capability maturity. Do not promote a beta capability merely because the product version became stable.
- Record credentialless publishing or Trusted Publisher facts without exposing tokens, account secrets, private endpoints, or sensitive logs.
- Prefer exact commands and durable public links over conversational summaries.

## Protect immutable boundaries

A tagged Go module checksum covers every file in that module tree, including documentation and changelogs. Never edit a previously published nested module tree and continue to claim the old tag or checksum is reproducible. Preserve the file or prepare a new patch version through a separate PR and release.

Do not change `go.mod`, `go.sum`, `go.work`, `go.work.sum`, pnpm lockfiles, or package metadata merely to make documentation validation pass. Do not move or recreate an existing release tag to pick up documentation changes. A source-documentation PR merged into `main` does not silently update an immutable Docs deployment; that requires the repository's explicit Docs release path.

## Close the source-to-site gap

When the repository has a public Docs domain, source reconciliation is not the same as publication:

1. Before editing, record the visible home page and a nested release route in the available browser, and read the public `/release.json` version and full commit.
2. After the source PR merges, compare that identity with the exact merged `main` commit. If the site still exposes an older commit or stale visible claims, report the state as `source-updated / deployment-pending`, never complete.
3. If the user authorized a public-site update, use `mss-release` for the mutation. Publish only from the exact merged-main commit after its Docs build and browser evidence pass.
4. Preserve an existing coordinated Docs tag. For a correction to the current stable documentation, select the lowest unused positive revision accepted by policy, such as `docs/${VERSION}+docs.1`; this is a Docs component revision, not a new product version.
5. Wait for the tag-triggered workflow, protected production environment, portable archive checksums, public `/release.json`, and immutable GitHub Release. A successful main-branch Docs build without the tag deployment is preliminary evidence only.
6. Reopen the production home page and a nested release route, refresh both, verify visible stable-version language, inspect console errors and failed network requests, and save final screenshots.

If publication was not authorized or the tag or environment policy blocks the exact merged commit, stop before creating a tag and provide the exact proposed Docs revision, merged commit, workflow, and policy blocker. Do not request or invent a manual environment approval for the automatic tag path.

## Validate proportionally

Start with the cheapest relevant checks and use commands declared by `.mss/commands.yaml`:

```shell
git diff --check
go run ./cmd/mss spec validate .mss/features/<changed-feature>.yaml
make docs-install docs-build
go run ./cmd/mss verify --changed
```

Run only the commands that match the changed surface, but always:

1. rebuild the documentation site when site content or navigation changed;
2. validate every changed Feature, AdminModule, or other machine-readable contract;
3. rescan current-facing files for stale stable/latest claims;
4. confirm historical and archived exclusions are intentional;
5. inspect the final diff for module metadata, lockfile, generated-output, and immutable-tree drift;
6. recheck remote release facts if validation lasted long enough for them to change.

For changes that affect landing pages, navigation, release indexes, or visible version selectors, run the local Docs site and inspect the changed routes in the available browser. Refresh a nested route, check visible content, browser-console errors, and failed network requests, and preserve screenshots when the layout or release presentation materially changed. A successful static build alone does not prove those browser-facing contracts.

If a workflow changed while repairing documentation CI, add a regression test that fails on the old behavior and rerun the workflow-governance suite. Do not classify a transient network or workstation-tool failure as a documentation defect until the checked-in workflow reproduces it.

## Deliver through the governed path

Submit source, contract, workflow, and documentation changes through a PR to `main`. Bind review evidence to the exact PR Head, wait for required checks, and invalidate approval evidence when the Head changes. Never publish from the topic branch.

Report:

- the verified stable/current ledger;
- current-facing files changed and historical files intentionally preserved;
- immutable boundaries and metadata kept unchanged;
- commands actually run and their results;
- skipped checks with concrete reasons;
- merged commit and any separate Docs release still required.
- source state and public-site state separately, including the deployed Docs tag, `/release.json`, workflow run, Release, browser routes, and screenshot paths.
