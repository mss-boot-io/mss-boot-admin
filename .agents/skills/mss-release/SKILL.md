---
name: mss-release
description: Prepare, execute, resume, and verify an MSS foundation, framework, Admin, frontend, docs, or downstream-application release from one exact merged-main commit. Trigger for release policy or tag planning, the unique Root candidate preview, tag-driven candidate artifact or image publication, separately reviewed stable promotion, failed-release recovery, post-publication reconciliation, migration notes, and rollback preparation. Reuse exact successful evidence to shorten the release cycle; do not use this skill to merge an unverified feature PR or bypass a failed gate.
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
  frontend, Root, container, stable-promotion, and npm run URLs; record Docs
  website runs in a separate non-blocking section of the ledger;
- public release URLs, image digests, artifact checksums, and the active GitHub
  actor;
- the npm Trusted Publisher identity as package, owner, repository, workflow
  filename, GitHub environment, and allowed action; record no credential value;
- the reviewed stable-promotion version and exact Root commit, current npm
  `latest`, current GitHub Latest, and whether promotion remains disabled or has
  been authorized by the latest merged policy; record the Docs site identity
  without treating it as promotion authority;
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
- Rerun stable promotion only from the same exact Root tag after reconstructing
  the Root commit, public module, image, frontend-tarball, npm package, npm
  `latest`, and GitHub Latest identities. Docs tag, Release, credential, and
  public-site state are never stable-promotion prerequisites. An exact existing npm
  package may be reconciled; a different `gitHead`, integrity, provenance, or
  mutable alias must fail closed rather than trigger another publish.
- If a public tag, Release, package, or image already exists and repair needs source changes, preserve it and prepare a new patch version through a PR.

## Choose the release path

For the consolidated foundation repository, release the synchronized train in this order:

1. merge and freeze one exact `origin/main` commit;
2. run the unique Root candidate preview for that version and commit;
3. push the Framework tag and verify its public external-module result;
4. push the Admin tag and verify its Release and external-module result;
5. push the frontend tag and verify its GitHub Packages mirror, image, artifact,
   and Release;
6. push the Root tag, which starts only the Root Release and Root image candidate
   publication; reconcile them while keeping GitHub Latest and npm `latest` on
   the reviewed current stable version;
7. merge a separate reviewed stable-promotion policy that binds the exact Root
   version and commit without yet changing `currentStableVersion`;
8. manually dispatch `npm-release.yml` from the exact Root tag. Its first
   official npm publication uses Trusted Publishing/OIDC with
   `npm publish --tag latest --provenance`; only after npm identity and `latest`
   converge may the same workflow promote the Root GitHub Release to Latest;
9. merge one final current-stable reconciliation that atomically advances
   `currentStableVersion`, opens adopter availability from the exact complete
   Distribution ledger, and aligns checked-in human and Agent source guidance;
   it explicitly excludes the Docs tag, deployment, and public-site state;
10. independently publish or update the documentation website when ready. Its
    `docs/v*` tag identifies only that site publication and may complete before
    or after steps 7 through 9 without blocking any of them.

During steps 1 through 6, the candidate version is public evidence, not the
coordinated stable. Both mutable aliases remain exactly on the reviewed current
stable version (v1.3.2 for the v1.3.7 train). Do not create `next`, `candidate`,
or `release-*` npm dist-tags, and never run a standalone `npm dist-tag` mutation.

For a component-only or downstream release, retain the same source, evidence, immutability, and reconciliation rules while following that repository's tag namespace and workflow.

The Admin Distribution consists of Root, Framework, Admin, and frontend/package
surfaces. Docs is an independently publishable website, not a Distribution
component or stable-adoption gate. A `docs/{version}` tag identifies only one
immutable website publication; it does not recreate Root, Framework, frontend,
npm, stable aliases, or `currentStableVersion`. The initial tag uses the exact
base Root commit after that Root Release exists. A later Docs-only correction or
stable-wording revision is first merged through its own PR and frozen as an exact
source commit. A subsequent reviewed authorization PR sets
`docsRevisionPublicationReady`, binds `docsRevisionVersion` to the lowest unused
`+docs.N` identity, and binds `docsRevisionCommit` to that frozen source. Only
then may the revision tag point to the older authorized merged-main source and
reconcile the Docs Release assets, checksums, public `release.json`, and visible
site against that commit. Docs absence or failure is recorded as website
deployment pending and never changes Distribution stability or adopter availability.

When the coordinated Docs tag for the current stable version already exists but later merged documentation must replace stale public content, preserve that tag and use the lowest unused positive Docs revision allowed by policy, such as `docs/${VERSION}+docs.1`. A `+docs.N` tag updates only the Docs component, does not consume the next product patch version, and must still pass exact-tag resolution, the reviewed ancestor-source binding, merged-PR proof, protected deployment, Release, `/release.json`, and browser reconciliation. Disable the consumed one-shot Docs authorization in a follow-up PR.

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

### 2. Run focused review gates, then one exact-main qualification

Before the release-preparation PR, run the changed-impact plan and focused verification:

```bash
go run ./cmd/mss verify --changed --plan --format json
go run ./cmd/mss verify --changed
```

Before final review and merge, run the focused tests selected by the changed-impact plan and capture Codex in-app Browser evidence for every affected visible journey. Do not run a second complete suite merely to qualify the pull-request Head; the lightweight required PR checks and focused local evidence authorize review, not publication.

After merge and source freeze in step 1, run the self-binding complete gate once on the exact clean merged-main `SHA`. The command fails closed unless tracked files are clean before and after and `HEAD` remains the frozen full commit:

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

The release ledger outside the tracked worktree records the report digest and result; the report itself binds the full `SHA` and both tracked-clean observations. In release-evidence mode, the real external Thin Host check also records a persistent system-temporary evidence directory in command output and writes its sanitized `evidence-manifest.json`; retain or hash that directory with the ledger when its detailed reports are needed. The complete verifier owns code quality, dependency policy, contracts, tests, standalone next-Foundation generation and upgrade, CLI/MCP/doctor identity parity, deterministic conflict output, empty second upgrade, evals, real external Thin Host behavior, and browser behavior. Pull requests retain only changed-impact local checks, affected browser evidence, lightweight server-side governance, vulnerability, CodeQL, the ordinary Admin test required by branch protection, and an applicable ordinary Framework smoke test; broad component matrices run locally only in the single exact-main release-evidence pass and again after merge as non-blocking audit where configured. The unique Root candidate preview must not repeat those suites. It owns only release policy plus exact binaries, archives, packages, SBOMs, checksums, portable delivery smoke, Thin Host package composition, and multi-architecture images later published byte-for-byte. Even a self-bound local report never replaces merged-main, actor, immutable-ref, public Framework resolution, OIDC, provenance, or public-reconciliation boundaries.

Classify a missing local browser binary or package as workstation setup first. Install it locally when repository and CI contracts are already correct. Change repository setup only when the failure reproduces in the checked-in workflow or a clean supported setup.

Treat broad documentation-site visual polish as Docs-publication preparation, not
as a Distribution release gate. Do not mix a cosmetic sweep into ordinary
feature delivery. Before a Docs tag, start the site locally from the Docs source
and inspect the home page plus at least one representative nested release or
guide page at desktop and narrow mobile widths. Check the nested-route logo and
other static assets, header wrapping, hero actions, card rhythm, dark mode when
offered, content readability, table scrolling, and document-level horizontal
overflow. Fix visual defects through the Docs PR, rebuild the site, and capture
browser evidence before freezing the Docs source. A later visual fix needs a new
Docs PR and, when an immutable tag already exists, the reviewed revision path;
it does not invalidate or delay a Distribution release.

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
for tag in "${FRAMEWORK_TAG}" "${ADMIN_TAG}" "${FRONTEND_TAG}" "${ROOT_TAG}"; do
  git ls-remote --tags origin "refs/tags/${tag}" "refs/tags/${tag}^{}"
  gh release view "${tag}" --json tagName,isDraft,isPrerelease,url,assets 2>/dev/null || true
done
```

Inspect `DOCS_TAG` separately only when scheduling a website publication. An
existing, missing, failed, or delayed Docs identity never stops Distribution
preview, component tags, stable promotion, or current-stable reconciliation.

Immediately before the preview and every Distribution tag push, fetch
`origin/main` again and require it to remain `SHA`. If `main` advanced, stop
before creating an immutable Distribution ref and select the new merged-main
commit. A later Docs tag follows the separately reviewed Root-ancestor or
revision-source binding in section 9, so it does not require current
`origin/main` to equal the older Root commit. If the reviewed policy lists the
version in `immutableStoppedTrains`, stop: every listed ref is permanently
rejected and must never be deleted, moved, recreated, completed, or resumed.

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
  --scope core \
  > ".mss/reports/remote-release-governance-${VERSION}.json"
```

The Distribution governance result must verify Root, Framework, Admin, frontend,
npm, the exact tag policies, no administrator bypass, and no required reviewers
on their active publishing environments. It must not read or require the Docs
`prod` environment or `CF_API_TOKEN`. Verify the Docs tag policy, protected
`prod` environment, and organization-managed `CF_API_TOKEN` separately and only
immediately before a Docs website publication. A Docs governance failure stops
only that website operation. Retired publishing environments must remain blocked
so an old workflow cannot regain publication authority.

Before the Root tag, independently inspect the target npm package access page
and require exactly the reviewed GitHub Actions Trusted Publisher identity. For
`@mss-boot-io/admin-web`, the identity is owner `mss-boot-io`, repository
`mss-boot-admin`, workflow filename `npm-release.yml`, environment `npm-auto`,
and allowed action `npm publish`. Match spelling and case exactly. Record
sanitized evidence in the ledger and stop if authentication prevents inspection,
the binding differs, or another publisher creates ambiguity. Changing this
access binding is an external permission mutation: obtain explicit authorization
immediately before saving it.

Before creating candidate tags, also require both mutable public aliases to
resolve to the reviewed current stable version from policy: npmjs
`@mss-boot-io/admin-web` `dist-tags.latest` and the repository's GitHub Latest
Root Release. For the v1.3.7 candidate they both remain v1.3.2. A component or
Root candidate Release, versioned npm mirror, or versioned image is not authority
to advance either alias. Stop on drift and repair the public alias through the
governed stable path; never disguise it with a second npm dist-tag.

### 7. Publish the Root candidate without moving stable aliases

After all three component Releases and their exact tag workflows succeed, `lwnmengjing` creates and pushes the annotated Root tag at the same `SHA`:

```bash
git tag -a "${ROOT_TAG}" "${SHA}" -m "${ROOT_TAG}"
git push origin "refs/tags/${ROOT_TAG}"
```

That push starts `release.yml` and `container.yml`; it does not start official
npm publication. Root and the image workflow re-verify the exact merged-main
tag, active policy, and successful exact preview before publishing the staged
bytes. The Root Release must be public with `Latest=false`, and the versioned
Root image is still candidate evidence. GitHub Latest and npm `latest` must both
remain on the current stable version.

Do not wait for or create a Docs tag as part of this Distribution step. Docs is
eligible for its own asynchronous website publication after the base Root Release
exists, but its state is irrelevant to the next stable-promotion step.

### 8. Authorize and execute stable promotion

Only after Framework, Admin, frontend, Root, both images, and the external
Distribution consumer ledger have reconciled may a separate PR set the
stable-promotion policy to ready and bind `stablePromotionVersion` plus the exact
Root `stablePromotionCommit`. Docs is explicitly outside this ledger. That
reviewed policy is read from current `origin/main`; it must not move or recreate
the older Root tag, and it must not prematurely change `currentStableVersion`.

Re-fetch policy and public state after the policy PR merges. Require the exact
Root tag and every candidate component Release to peel to `SHA`; require public
Go modules, versioned images, frontend tarball, npm `latest`, and GitHub Latest
to match the reviewed pre-promotion Distribution ledger. Do not inspect a Docs
tag, Release, credential, or site response here. Then dispatch the official npm
workflow from the exact Root tag, never from `main`:

```bash
test "$(gh api user --jq .login)" = "lwnmengjing"
gh workflow run npm-release.yml \
  --ref "${ROOT_TAG}" \
  -f version="${VERSION}"
```

This is the only official npm mutation. It removes `NPM_TOKEN` and
`NODE_AUTH_TOKEN`, publishes the already-qualified frontend tarball through the
exact Trusted Publisher using `npm publish --tag latest --provenance`, and
requires npm version, `gitHead`, integrity, provenance, and `latest` to converge.
Do not add a token, create a temporary dist-tag, or invoke `npm dist-tag` before
or after it. Only after that npm check succeeds may the workflow set the exact
Root GitHub Release as Latest.

If the dispatch is interrupted, reconstruct the ledger before rerun. Rerun only
from the same Root tag, version, commit, actor, workflow, and reviewed promotion
policy. The workflow may accept an already-public package only when its complete
identity and `latest` are exact; it must not republish mismatched bytes or advance
GitHub Latest before npm convergence. An `ENEEDAUTH` result is a Trusted
Publisher identity failure until disproved. Never restore a long-lived token as
a fallback.

After npm and GitHub Latest converge, merge a final policy PR that atomically
advances `currentStableVersion` and `currentStableCommit`, closes or disables the
consumed stable-promotion authorization, opens adopter availability, and aligns
the checked-in human and Agent guidance with those machine facts. This step
never waits for or includes a Docs tag, deployment credential, Release,
`/release.json`, or public-site response. If the public website needs the newly
merged wording, record this reconciliation PR's exact merged commit, then merge
another reviewed authorization PR that sets
`docsRevisionPublicationReady: true`, binds `docsRevisionVersion` to the lowest
unused identity such as `${VERSION}+docs.1`, and binds `docsRevisionCommit` to
that recorded source. Tag that source only after authorization, never move
`docs/${VERSION}`, and disable the consumed revision authority afterward.

### 9. Publish and reconcile the Docs website asynchronously

After the base Root Release exists, the initial website tag may be created at
the exact Root commit whenever the documentation team is ready:

```bash
git ls-remote --tags origin "refs/tags/${DOCS_TAG}" "refs/tags/${DOCS_TAG}^{}"
git tag -a "${DOCS_TAG}" "${SHA}" -m "${DOCS_TAG}"
git push origin "refs/tags/${DOCS_TAG}"
```

Before creating it, require the Docs-specific tag governance, Root Release,
merged-main source, protected `prod` environment, and effective organization
credential. The Docs workflow retains exact Root ancestry, immutable tag and
Release identity, checksums, monotonic deployment, `/release.json`, and browser
reconciliation. A `docs/v*` tag means only “this website revision was published.”
It does not certify or change Distribution, package, alias, current-stable, or
adopter state.

```bash
bash tools/release/verify_remote_release_governance.sh \
  --repository "${REPO}" \
  --release-actor-login lwnmengjing \
  --scope docs \
  > ".mss/reports/remote-docs-governance-${DOCS_TAG//\//-}.json"
```

If the base tag already exists, never recreate or move it. Publish later source
or stable wording only through the reviewed lowest-unused `+docs.N` authorization
described above. If any Docs check fails, leave the Distribution untouched,
record `website deployment pending`, and continue the Docs repair independently.

### 10. Reconcile public truth independently

After publication, verify all of the following without relying only on workflow badges:

- all component tags peel to `SHA` and all Releases are public, non-draft, and have the expected prerelease state;
- Framework and Admin resolve through the public Go proxy with exact origin hashes and stable sums, and Admin requires the matching Framework version;
- `@mss-boot-io/admin-web` resolves from npmjs with the exact version, `gitHead`, provenance, and immutable integrity recorded by the release; GitHub Packages has the same version, integrity, and dist-tag, and both downloads are byte-identical;
- npmjs `latest` resolves to the promoted version first; the Root GitHub Release
  becomes Latest only afterward, and no `next`, `candidate`, or `release-*`
  dist-tag was introduced;
- root and frontend images expose `linux/amd64` and `linux/arm64`, expected digests, and exact version/revision labels;
- frontend checksum, portable archive members, file modes, and embedded `release.json` match version and commit;
- root contains six platform ZIPs plus `SHA256SUMS`; checksums, portable members, and every embedded build identity match version and commit;
- root also contains six `mss-tools-${VERSION}-*` archives, `SHA256SUMS.tools-${VERSION}`, `install-mss.sh`, and `install-mss.ps1`; each tool archive has only `BUILD-INFO`, `LICENSE`, `mss`, and `mss-mcp` (with Windows suffixes), and no raw `admin` or internal `mss-pr` asset;
- install the public tool bundle into an empty temporary directory, verify version/commit/timestamp, and create and validate a Thin Host without cloning the Foundation; separately verify both public command packages still compile with `go install`, while keeping release-provenance creation and upgrade confined to the checksummed bundle;
- when Docs is published, its independent website ledger exposes the expected
  application, version, and authorized commit at `/release.json`; any later Docs
  revision exposes its own identity and merged-main commit without moving an
  earlier tag. Missing or pending Docs is reported but never makes the complete
  Distribution ledger fail;
- fresh-install and upgrade migrations, API registry synchronization, menu API binding, authorization negative cases, and rollback evidence are attached where required;
- the local checkout still matches fetched `origin/main`, the tracked worktree is clean, unrelated local services are untouched, and the original GitHub actor is active.

Write one sanitized reconciliation comment to the evidence issue with run URLs, release URLs, digests, checksums, exact commit, and any explicitly unverified item.

## Stop and repair rules

- Stop before the first tag when a checkpoint, exact-SHA evidence, policy, portability, workflow-governance, or source check fails. Repair through a PR, merge, and freeze the new commit.
- Stop before the next component when publication fails. Inspect whether any public mutation occurred before deciding between an exact-stage rerun and a new patch version.
- If a Root tag or Release is already public and the Root image candidate
  requires a source repair, freeze the partial train and prepare the next unused
  patch through a PR. A Docs source or deployment repair stays on the independent
  Docs PR and revision-tag path and never freezes the Distribution. If stable
  npm promotion fails without publishing and no source or policy changes are required, repair
  only the external Trusted Publisher identity under explicit authorization and
  rerun from the exact Root tag after reconstructing the ledger. Never add a
  token or move an immutable ref.
- Never move npm `latest` or GitHub Latest during candidate publication. Never
  use an auxiliary dist-tag as a holding area or run `npm dist-tag` manually.
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
