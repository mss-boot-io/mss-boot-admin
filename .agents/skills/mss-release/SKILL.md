---
name: mss-release
description: Prepare, execute, resume, and verify an MSS foundation, framework, Admin, frontend, docs, or downstream-application release from one exact merged-main commit. Trigger for release policy or tag planning, the unique Root candidate preview, automatic tag-driven artifact or image publication, failed-release recovery, post-publication reconciliation, migration notes, and rollback preparation. Reuse exact successful evidence to shorten the release cycle; do not use this skill to merge an unverified feature PR or bypass a failed gate.
---

# Release an MSS component

Release from one clean commit already merged into `origin/main`. Bind every
reusable result to `(repository, version, full commit, workflow identity)`.

## Keep one release ledger

Maintain a compact ledger in progress updates. Reconstruct it from GitHub and Git before acting after any pause; never trust conversation history alone.

Record:

- repository, component scope, version, and all tag names;
- frozen full commit and merged PR;
- browser and Blueprint evidence, Root candidate preview, Framework, Admin,
  frontend, Root, container, npm, and later Docs run URLs;
- public release URLs, image digests, artifact checksums, and the active GitHub
  actor;
- the npm Trusted Publisher identity as package, owner, repository, workflow
  filename, GitHub environment, and allowed action; record no credential value;
- status as `missing`, `running`, `successful`, `failed-before-publication`, or
  `public`.

Show only transitions. Do not repeatedly narrate an unchanged queued run.

## Reuse evidence safely

- Reuse a successful result only when repository, version, full commit, inputs,
  actor, and workflow identity match exactly.
- Run one successful Root candidate preview for the frozen commit. Multiple
  exact successful reruns are harmless; do not pass any run ID into later
  workflows.
- Treat a code, contract, dependency, workflow, or release-policy change after freeze as a new commit. Invalidate every affected later phase.
- Rerun only the failed workflow when no source or public state changed and the
  failure was transient.
- If a public tag, Release, package, or image already exists and repair needs source changes, preserve it and prepare a new patch version through a PR.

## Choose the release path

For the consolidated foundation repository, release the synchronized train in this order:

1. merge and freeze one exact `origin/main` commit;
2. run the unique Root candidate preview for that version and commit;
3. push the Framework tag and verify its public external-module result;
4. push the Admin tag and verify its Release and external-module result;
5. push the frontend tag and verify its GitHub Packages mirror, image, artifact,
   and Release;
6. push the Root tag, which automatically starts Root Release, Root image, and
   official npmjs publication;
7. reconcile those public packages, assets, images, and checksums;
8. after content is ready, push the independent Docs tag and reconcile the site.

For a component-only or downstream release, retain the same source, evidence, immutability, and reconciliation rules while following that repository's tag namespace and workflow.

The foundation components are independently releasable. A Docs-only release uses `docs/{version}` and does not recreate root, Framework, or frontend refs. Qualify the documentation build and browser evidence on the Docs candidate, merge through PR, freeze the resulting exact `origin/main` commit, create only the Docs tag, wait for the tag-triggered Docs workflow and protected `prod` deployment, then reconcile the Docs Release assets, checksums, public `release.json`, and visible site against that commit.

When the coordinated Docs tag for the current stable version already exists but later merged documentation must replace stale public content, preserve that tag and use the lowest unused positive Docs revision allowed by policy, such as `docs/${VERSION}+docs.1`. A `+docs.N` tag updates only the Docs component, does not consume the next product patch version, and must still pass the exact merged-main source guard, protected deployment, Release, `/release.json`, and browser reconciliation.

## Procedure

### 1. Establish source and policy

Read `AGENTS.md`, `.mss/project.yaml`, `.mss/lock.yaml`, `.mss/release-policy.yaml`, `.mss/release-qualification.json`, the selected Feature contracts, changelogs, migrations, and the relevant workflows. Derive the target from policy; do not type a remembered version into later commands.

Classify the release delta as feature, bug fix, security fix, migration, breaking contract, deprecation/removal, generator/Blueprint change, or release-infrastructure repair. Prepare user-visible notes, upgrade requirements, compatibility and security impact, and rollback steps before qualification.

Set shell variables once and validate them:

```bash
set -euo pipefail
REPO="$(gh repo view --json nameWithOwner --jq .nameWithOwner)"
VERSION="$(python3 -c 'from pathlib import Path; from tools.release.check_release_policy import load_policy; print(load_policy(Path(".mss/release-policy.yaml"))["nextPublicVersion"])')"
SHA="$(git rev-parse HEAD)"
ROOT_TAG="${VERSION}"
FRAMEWORK_TAG="mss-boot/${VERSION}"
ADMIN_TAG="admin/${VERSION}"
FRONTEND_TAG="web/antd-v6/${VERSION}"
DOCS_TAG="docs/${VERSION}"
```

Fetch and fail closed before publication:

```bash
git fetch origin main --tags
test -z "$(git status --porcelain=v1 --untracked-files=all)"
test "${SHA}" = "$(git rev-parse origin/main)"
python3 tools/release/verify_release_source.py \
  --repository "${REPO}" \
  --commit "${SHA}" \
  --policy .mss/release-policy.yaml
```

Confirm all prerequisite PRs are merged in dependency order. Confirm root, Framework, Admin, and frontend version references agree. Check every proposed remote tag before creating any one of them. Stop on any existing unexpected ref; never move or reuse it.

### 2. Run the cheap failure gates first

Before the release-preparation PR, run the changed-impact plan and focused verification:

```bash
go run ./cmd/mss verify --changed --plan --format json
go run ./cmd/mss verify --changed
```

Before final review and merge, run the complete local qualification and capture the required Codex in-app Browser evidence from the same commit. Treat this PR-head result as review evidence until the commit is merged:

```bash
go run ./cmd/mss verify --all
```

After merge and source freeze in step 1, run the self-binding complete gate again on the exact clean merged-main `SHA`. The command fails closed unless tracked files are clean before and after and `HEAD` remains the frozen full commit:

```bash
go run ./cmd/mss verify --all \
  --release-evidence \
  --expect-commit "${SHA}"
jq -e \
  --arg commit "${SHA}" \
  '.success == true and .evidenceMode == true and .commit == $commit and
   .trackedCleanBefore == true and .trackedCleanAfter == true' \
  .mss/reports/verify.json >/dev/null
sha256sum .mss/reports/verify.json
```

The release ledger outside the tracked worktree records the report digest and result; the report itself binds the full `SHA` and both tracked-clean observations. In release-evidence mode, the real external Thin Host check also records a persistent system-temporary evidence directory in command output and writes its sanitized `evidence-manifest.json`; retain or hash that directory with the ledger when its detailed reports are needed. The complete verifier owns code quality, dependency policy, contracts, tests, standalone next-Foundation generation and upgrade, CLI/MCP/doctor identity parity, deterministic conflict output, empty second upgrade, evals, real external Thin Host behavior, and browser behavior. Pull requests retain only lightweight server-side governance, vulnerability, CodeQL, the ordinary Admin test required by branch protection, and an applicable ordinary Framework smoke test; broad component matrices run locally and again after merge as non-blocking audit where configured. The unique Root candidate preview must not repeat those suites. It owns only release policy plus exact binaries, archives, packages, SBOMs, checksums, portable delivery smoke, Thin Host package composition, and multi-architecture images later published byte-for-byte. Even a self-bound local report never replaces merged-main, actor, immutable-ref, public Framework resolution, OIDC, provenance, or public-reconciliation boundaries.

Classify a missing local browser binary or package as workstation setup first. Install it locally when repository and CI contracts are already correct. Change repository setup only when the failure reproduces in the checked-in workflow or a clean supported setup.

Treat broad documentation-site visual polish as release-preparation work. Do not mix a cosmetic sweep into ordinary feature delivery. During release preparation, start the docs site locally from the candidate branch and inspect the home page plus at least one representative nested release or guide page at desktop and narrow mobile widths. Check the nested-route logo and other static assets, header wrapping, hero actions, card rhythm, dark mode when offered, content readability, table scrolling, and document-level horizontal overflow. Fix visual defects through the release-preparation PR, rebuild the docs, and capture browser evidence before freezing `SHA`; after freeze, any visual fix is a source change and requires a new PR and frozen commit.

Treat reusable-workflow call mode as explicit data. A called workflow inherits the
caller's `github` context, so it must not infer `workflow_call` mode from
`github.event_name`. Require a typed input for release-preview behavior, pass it
explicitly from the caller, fail closed when a release version is supplied
without that mode, and execute the metadata script in tests with the caller's
real event name. A successful Root preview is valid only when both the Root
package artifact and the exact multi-platform Root OCI artifact exist,
are non-empty, unexpired, and verified.

Submit every source, workflow, dependency, generated, contract, or documentation change through a PR to `main`. After merge, freeze the new full SHA and do not carry forward topic-branch evidence as publication authority.

### 3. Reconstruct remote state before preview or tags

Inspect exact-commit runs and public refs before creating work:

```bash
gh run list --commit "${SHA}" --limit 100 \
  --json databaseId,workflowName,event,status,conclusion,headSha,url,createdAt
for tag in "${FRAMEWORK_TAG}" "${ADMIN_TAG}" "${FRONTEND_TAG}" "${ROOT_TAG}" "${DOCS_TAG}"; do
  git ls-remote --tags origin "refs/tags/${tag}" "refs/tags/${tag}^{}"
  gh release view "${tag}" --json tagName,isDraft,isPrerelease,url,assets 2>/dev/null || true
done
```

Immediately before the preview and every tag push, fetch `origin/main` again and require it to remain `SHA`. If `main` advanced, stop before creating an immutable ref and select the new merged-main commit. If the reviewed policy lists the version in `immutableStoppedTrains`, stop: every listed ref is permanently rejected and must never be deleted, moved, recreated, completed, or resumed.

### 4. Run the unique Root artifact-staging preview

The only current artifact-staging preview is `release.yml` on `workflow_dispatch`. It accepts only `version`, never publishes, and builds and verifies only the exact archives, packages, SBOMs, checksums, portable delivery paths, Thin Host composition, and multi-platform images that formal tag workflows later reuse. It does not repeat dependency audits, Go suites, lint, unit tests, Playwright, next-Foundation compatibility, CLI/MCP/doctor parity, conflict/no-op rehearsals, or evals. The external ledger binds the complete local gate to the frozen `SHA`; lightweight PR checks authorize the preceding merge but are not reinterpreted as exact merged-main evidence.

```bash
test "$(gh api user --jq .login)" = "lwnmengjing"
gh workflow run release.yml --ref main -f version="${VERSION}"
```

Locate the resulting run by the exact title `Root Release candidate VERSION`, then require `event=workflow_dispatch`, `headSha=SHA`, actor and triggering actor `lwnmengjing`, and a successful conclusion. Multiple successful reruns for the same version and SHA are harmless; at least one exact success is required. Do not record or pass a run ID into publication workflows.

### 5. Publish Framework, Admin, and frontend tags

Only `lwnmengjing` creates release tags. Push one annotated component tag at a time from the exact clean `SHA`, and verify the immutable Release and workflow before continuing:

```bash
git tag -a "${FRAMEWORK_TAG}" "${SHA}" -m "${FRAMEWORK_TAG}"
git push origin "refs/tags/${FRAMEWORK_TAG}"
git tag -a "${ADMIN_TAG}" "${SHA}" -m "${ADMIN_TAG}"
git push origin "refs/tags/${ADMIN_TAG}"
git tag -a "${FRONTEND_TAG}" "${SHA}" -m "${FRONTEND_TAG}"
git push origin "refs/tags/${FRONTEND_TAG}"
```

The workflows fail before checkout, secrets, or writes unless both actor identities are `lwnmengjing`; each also verifies the exact merged-main source and active release policy. The first irreversible Framework publication performs one cheap exact-SHA/version artifact-preview lookup so an operator mistake cannot create a partial train before staging. Admin does not repeat that lookup; after the Framework Release exists, it keeps the unique remote dependency boundary by resolving the exact public Framework through `proxy.golang.org` with `GOWORK=off` and testing the compile-time composition before publishing. The consolidated controlled-creation ruleset covers Root, component, and Docs tag namespaces with `lwnmengjing` as its only bypass. The immutable ruleset has no bypass. SullivanPrime remains an independent PR reviewer and is not a release-environment reviewer.

### 6. Verify remote governance without adding an approval pause

Before the formal Root tag, an authenticated repository administrator must run:

```bash
bash tools/release/verify_remote_release_governance.sh \
  --repository "${REPO}" \
  --release-actor-login lwnmengjing \
  > ".mss/reports/remote-release-governance-${VERSION}.json"
```

The verifier requires exactly one consolidated controlled-creation ruleset, one exact no-bypass stopped-tag creation ruleset for every train in `immutableStoppedTrains`, and the no-bypass immutable tag ruleset. It also requires exact tag policies, no administrator bypass, and no required reviewers on the active publishing environments. The Docs credential must be the organization-managed `CF_API_TOKEN` shared with this repository, with no repository or environment override; the environment-bound Docs workflow checks effective availability without printing its value before deployment. The retired publishing environments must remain blocked so an old workflow cannot regain publication authority.

Before the Root tag, independently inspect the target npm package access page
and require exactly the reviewed GitHub Actions Trusted Publisher identity. For
`@mss-boot-io/admin-web`, the identity is owner `mss-boot-io`, repository
`mss-boot-admin`, workflow filename `npm-release.yml`, environment `npm-auto`,
and allowed action `npm publish`. Match spelling and case exactly. Record
sanitized evidence in the ledger and stop if authentication prevents inspection,
the binding differs, or another publisher creates ambiguity. Changing this
access binding is an external permission mutation: obtain explicit authorization
immediately before saving it.

### 7. Push the Root tag; publish Docs later

After all three component Releases and their exact tag workflows succeed, `lwnmengjing` creates and pushes the annotated Root tag at the same `SHA`:

```bash
git tag -a "${ROOT_TAG}" "${SHA}" -m "${ROOT_TAG}"
git push origin "refs/tags/${ROOT_TAG}"
```

That one push naturally starts `release.yml`, `container.yml`, and `npm-release.yml` in parallel. Root and the image workflow re-verify the exact merged-main tag, active policy, and successful exact preview before publishing the staged bytes. The npm workflow does not repeat the preview lookup: it requires the already-completed exact frontend Release, verifies its GitHub Packages version-specific mirror and byte identity, and publishes those same bytes through npm Trusted Publishing. Root, image, and npm never wait on one another, and none waits for Docs. Root and official npm publication are serialized across versions and never move a `latest` pointer backward.

Never dispatch a second publish run or approve a release environment after the tag. A retry is an ordinary workflow rerun by `lwnmengjing`; immutable identity checks refuse conflicting public state.

The automatic npm path is OIDC-only. If the npm package does not exist at all, it fails closed and requires a separately reviewed one-time bootstrap; do not expose a bootstrap token to the automatic tag workflow. Existing packages never use a long-lived npm write token. An `ENEEDAUTH` result from `npm publish --provenance` is a Trusted Publisher identity failure until disproved: compare the package, owner, repository, workflow filename, and environment first. Never add or restore `NPM_TOKEN` or `NODE_AUTH_TOKEN` as a fallback, and keep the workflow's explicit token unsetting intact.

After the documentation content is merged, create `DOCS_TAG` from that later exact `origin/main` commit. Docs remains an independent later tag workflow: it requires the already-published base Root Release, then publishes the immutable Docs Release and deployment from its own merged-main source. Docs is not a prerequisite for Root or npm, and its commit need not equal the older Root commit.

### 8. Reconcile public truth independently

After publication, verify all of the following without relying only on workflow badges:

- all component tags peel to `SHA` and all Releases are public, non-draft, and have the expected prerelease state;
- Framework and Admin resolve through the public Go proxy with exact origin hashes and stable sums, and Admin requires the matching Framework version;
- `@mss-boot-io/admin-web` resolves from npmjs with the exact version, `gitHead`, provenance, and immutable integrity recorded by the release; GitHub Packages has the same version, integrity, and dist-tag, and both downloads are byte-identical;
- root and frontend images expose `linux/amd64` and `linux/arm64`, expected digests, and exact version/revision labels;
- frontend checksum, portable archive members, file modes, and embedded `release.json` match version and commit;
- root contains six platform ZIPs plus `SHA256SUMS`; checksums, portable members, and every embedded build identity match version and commit;
- root also contains six `mss-tools-${VERSION}-*` archives, `SHA256SUMS.tools-${VERSION}`, `install-mss.sh`, and `install-mss.ps1`; each tool archive has only `BUILD-INFO`, `LICENSE`, `mss`, and `mss-mcp` (with Windows suffixes), and no raw `admin` or internal `mss-pr` asset;
- install the public tool bundle into an empty temporary directory, verify version/commit/timestamp, and create and validate a Thin Host without cloning the Foundation; separately verify both public command packages still compile with `go install`, while keeping release-provenance creation and upgrade confined to the checksummed bundle;
- Docs-only publication exposes the expected application, version, and full commit at `/release.json`; its portable archive, build identity, checksum manifest, GitHub Release, protected deployment, and visible routes match the Docs tag commit;
- fresh-install and upgrade migrations, API registry synchronization, menu API binding, authorization negative cases, and rollback evidence are attached where required;
- the local checkout still matches fetched `origin/main`, the tracked worktree is clean, unrelated local services are untouched, and the original GitHub actor is active.

Write one sanitized reconciliation comment to the evidence issue with run URLs, release URLs, digests, checksums, exact commit, and any explicitly unverified item.

## Stop and repair rules

- Stop before the first tag when a checkpoint, exact-SHA evidence, policy, portability, workflow-governance, or source check fails. Repair through a PR, merge, and freeze the new commit.
- Stop before the next component when publication fails. Inspect whether any public mutation occurred before deciding between an exact-stage rerun and a new patch version.
- If a Root tag or Release is already public and either the Root image candidate
  or npm publication requires a source or access-contract repair, freeze the
  partial train and prepare the next unused patch through a PR. Do not complete
  the old version by adding a token or moving an immutable ref.
- Never delete, move, overwrite, or reuse a public tag, Release, package, image, or checksum.
- Never release from a topic branch, detached commit, local-only fix, dirty worktree, or commit absent from current `origin/main`.
- Never weaken a test, bypass an environment policy, forge evidence, or invent a
  preview result to shorten the cycle.
- Never expose credentials, cookies, database DSNs, tokens, or sensitive bodies in artifacts or evidence.

## Output

Report the component train, version, exact commit, merged PR, preview and tag
workflow ledger, active actor, commands actually run, migrations, security and
compatibility impact, public tags/Releases, artifact checksums, image digests
and platforms, smoke-test results, rollback path, skipped checks with reasons,
and the next executable step.
