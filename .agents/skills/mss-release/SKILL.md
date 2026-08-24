---
name: mss-release
description: Prepare, execute, resume, and verify an MSS foundation, framework, Admin, frontend, docs, or downstream-application release from one exact merged-main commit. Trigger for release readiness, version or tag planning, changelogs, phased GitHub Actions qualification, protected-environment approval, artifact or image publication, failed-release recovery, post-publication reconciliation, migration notes, and rollback preparation. Reuse exact successful evidence to shorten the release cycle; do not use this skill to merge an unverified feature PR or bypass a failed gate.
---

# Release an MSS component

Release from one clean commit already merged into `origin/main`. Bind every reusable result to the tuple `(repository, version, full commit, phase, workflow run ID)`.

## Keep one release ledger

Maintain a compact ledger in progress updates. Reconstruct it from GitHub and Git before acting after any pause; never trust conversation history alone.

Record:

- repository, component scope, version, and all tag names;
- frozen full commit and merged PR;
- feature-freeze, browser, Blueprint, pre-framework, Framework, Admin, frontend, pre-root, root-container, root-candidate, final-publication, Docs, and official-npm run IDs or URLs;
- evidence issue URL, public release URLs, image digests, artifact checksums, active GitHub actor, and approver actor;
- status as `missing`, `running`, `waiting-approval`, `successful`, `failed-before-publication`, or `public`.

Show only transitions. Do not repeatedly narrate an unchanged queued or approval-waiting run.

## Reuse evidence safely

- Reuse a successful result only when version, full commit, phase, inputs, and workflow identity match exactly.
- Run the complete feature-freeze suite once for a frozen commit. Do not repeat broad local validation after an exact successful feature-freeze run unless diagnosing a concrete failure.
- Use `pre-framework` and `pre-root` for scoped publication attestations. Execute only the commands selected for that phase; do not replay unrelated feature-freeze checks or reduce the phase to a plan-only result.
- Treat a code, contract, dependency, workflow, or release-policy change after freeze as a new commit. Invalidate every affected later phase.
- Rerun only the failed phase when no source or public state changed and the failure was transient or approval-related.
- If a public tag, Release, package, or image already exists and repair needs source changes, preserve it and prepare a new patch version through a PR.

## Choose the release path

For the consolidated foundation repository, release the synchronized train in this order:

1. feature-freeze qualification;
2. `pre-framework` authority;
3. Framework tag and public external-module probe;
4. Admin tag, Release, and public external-module probe;
5. frontend tag, GitHub Packages mirror, artifact, image, and Release;
6. independent `pre-root` authority;
7. protected root-tag promotion, root image, and exact-tag candidate artifacts;
8. exact-tag manual root publication;
9. Docs tag, protected deployment, and Release from the same commit;
10. official npm publication of the already-qualified frontend tarball as the final artifact publication;
11. independent public reconciliation.

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

Before the release-preparation PR, run the changed-impact plan and the phase-scoped checkpoint:

```bash
go run ./cmd/mss verify --changed --plan --format json
go run ./cmd/mss verify --changed
python3 tools/release/release_phase_evidence.py run \
  --target-version "${VERSION}" \
  --commit "${SHA}" \
  --phase checkpoint \
  --output ".mss/reports/${VERSION}-checkpoint-command-evidence.json"
```

Require the checkpoint to cover release-policy and workflow tests, tag namespaces, artifact portability, archive members, and native multi-architecture build contracts. This is the pre-tag guard for colon-containing paths, traversal, reserved names, case collisions, unsafe links, restrictive modes, raw directory uploads, and emulated target compilation.

Classify a missing local browser binary or package as workstation setup first. Install it locally when repository and CI contracts are already correct. Change repository setup only when the failure reproduces in the checked-in workflow or a clean supported setup.

Treat broad documentation-site visual polish as release-preparation work. Do not mix a cosmetic sweep into ordinary feature delivery. During release preparation, start the docs site locally from the candidate branch and inspect the home page plus at least one representative nested release or guide page at desktop and narrow mobile widths. Check the nested-route logo and other static assets, header wrapping, hero actions, card rhythm, dark mode when offered, content readability, table scrolling, and document-level horizontal overflow. Fix visual defects through the release-preparation PR, rebuild the docs, and capture browser evidence before freezing `SHA`; after freeze, any visual fix is a source change and requires a new PR and frozen commit.

Submit every source, workflow, dependency, generated, contract, or documentation change through a PR to `main`. After merge, freeze the new full SHA and do not carry forward topic-branch evidence as publication authority.

### 3. Reconstruct remote state before dispatching

Inspect exact-commit runs and public refs before creating work:

```bash
gh run list --commit "${SHA}" --limit 100 \
  --json databaseId,workflowName,event,status,conclusion,headSha,url,createdAt
for tag in "${FRAMEWORK_TAG}" "${ADMIN_TAG}" "${FRONTEND_TAG}" "${ROOT_TAG}" "${DOCS_TAG}"; do
  git ls-remote --tags origin "refs/tags/${tag}" "refs/tags/${tag}^{}"
  gh release view "${tag}" --json tagName,isDraft,isPrerelease,url,assets 2>/dev/null || true
done
```

Verify a candidate run with `gh run view` and the repository verifier before recording its ID. Do not select a run merely because it is the newest one.

Immediately before every readiness dispatch and tag push, fetch `origin/main` again and require it to remain `SHA`. If `main` advanced, stop before creating a failed run or immutable ref and select the new merged-main commit.

### 4. Run feature-freeze once

Dispatch `release-readiness.yml` from the branch whose current head is exactly `SHA`. Supply the exact browser and Blueprint evidence commits and references. Set `phase=feature-freeze` and `publication_authority=false`.

```bash
gh workflow run release-readiness.yml --ref main \
  -f version="${VERSION}" \
  -f frozen_commit="${SHA}" \
  -f feature_freeze_confirmed=true \
  -f phase=feature-freeze \
  -f publication_authority=false \
  -f in_app_browser_evidence_commit="${SHA}" \
  -f in_app_browser_evidence_ref="${BROWSER_EVIDENCE_REF}" \
  -f blueprint_rehearsal_evidence_commit="${SHA}" \
  -f blueprint_rehearsal_evidence_ref="${BLUEPRINT_EVIDENCE_REF}"
```

Capture the run created after dispatch, require `headSha == SHA`, inspect its inputs/evidence artifact, and wait with `gh run watch RUN_ID --exit-status`. Preserve the successful run ID. A later phase may cite it but may not reinterpret it as publication authority.

### 5. Authorize and publish Framework, Admin, and frontend

Dispatch a separate readiness run with `phase=pre-framework` and `publication_authority=true`. Verify it before use:

```bash
gh workflow run release-readiness.yml --ref main \
  -f version="${VERSION}" \
  -f frozen_commit="${SHA}" \
  -f feature_freeze_confirmed=true \
  -f phase=pre-framework \
  -f publication_authority=true
```

```bash
bash tools/release/verify_readiness_run.sh \
  --repository "${REPO}" \
  --run-id "${PRE_FRAMEWORK_RUN_ID}" \
  --target-version "${VERSION}" \
  --commit "${SHA}" \
  --phase pre-framework \
  --policy .mss/release-policy.yaml
```

Set the selected run ID at every scope that consumes it, then read it back:

```bash
gh variable set RELEASE_READINESS_RUN_ID --body "${PRE_FRAMEWORK_RUN_ID}"
gh variable set RELEASE_READINESS_RUN_ID --env release --body "${PRE_FRAMEWORK_RUN_ID}"
gh variable set RELEASE_READINESS_RUN_ID --env release-v6 --body "${PRE_FRAMEWORK_RUN_ID}"
gh variable get RELEASE_READINESS_RUN_ID
gh variable get RELEASE_READINESS_RUN_ID --env release
gh variable get RELEASE_READINESS_RUN_ID --env release-v6
```

Create and push one annotated tag at a time. Wait for and verify its release before creating the next tag:

```bash
git tag -a "${FRAMEWORK_TAG}" "${SHA}" -m "mss-boot ${VERSION}"
test "$(git rev-list -n 1 "${FRAMEWORK_TAG}")" = "${SHA}"
git push origin "refs/tags/${FRAMEWORK_TAG}"
```

After Framework publication, resolve `github.com/mss-boot-io/mss-boot-admin/mss-boot@VERSION` from a clean external module with `GOWORK=off`. Require the origin hash and checksum, then create the Admin tag from the same SHA:

```bash
git tag -a "${ADMIN_TAG}" "${SHA}" -m "admin ${VERSION}"
test "$(git rev-list -n 1 "${ADMIN_TAG}")" = "${SHA}"
git push origin "refs/tags/${ADMIN_TAG}"
```

Wait for the exact `admin-release.yml` push run and its protected `release` environment. Require the public, immutable Admin Release, then resolve `github.com/mss-boot-io/mss-boot-admin/admin@VERSION` from a second clean external module with `GOWORK=off`; its required Framework version, origin hash, and checksum must all match the coordinated version and `SHA`.

Only after both Go modules are public and externally resolvable may the frontend tag be created. Require the GitHub Packages mirror of `@mss-boot-io/admin-web` to publish the exact unprefixed version with `gitHead == SHA` and immutable registry integrity. Also require frontend Release assets, checksums, portable archive, `linux/amd64` plus `linux/arm64` image manifest, and OCI version/revision labels before proceeding. Do not publish to npmjs in this phase.

```bash
git tag -a "${FRONTEND_TAG}" "${SHA}" -m "web/antd-v6 ${VERSION}"
test "$(git rev-list -n 1 "${FRONTEND_TAG}")" = "${SHA}"
git push origin "refs/tags/${FRONTEND_TAG}"
```

### 6. Approve protected environments deliberately

Treat `waiting` and `pending_deployment` as approval states, not failures. Query the exact run's pending deployments and verify the expected environment name before approval.

When repository policy forbids self-approval, record the initiating actor, switch `gh` to a configured reviewer with write access, approve only the exact run and environment IDs, then restore the original actor even if approval fails. Never hardcode personal account names or tokens in the skill, repository, logs, or evidence.

Use the pending-deployments API with a JSON body shaped as:

```json
{
  "environment_ids": [123456],
  "state": "approved",
  "comment": "Approved for the exact VERSION / SHA release phase"
}
```

Set `EXPECTED_ENV` to the exact environment (`release`, `release-v6`, or `root-promotion`). Use a repository-confined ignored report file so the request is inspectable, and restore the original account on both success and failure:

```bash
EXPECTED_ENV=release
PENDING_FILE=".mss/reports/pending-deployments-${RUN_ID}.json"
APPROVAL_FILE=".mss/reports/pending-approval-${RUN_ID}.json"
gh api "/repos/${REPO}/actions/runs/${RUN_ID}/pending_deployments" > "${PENDING_FILE}"
jq -e --arg expected "${EXPECTED_ENV}" \
  'length > 0 and all(.[]; .environment.name == $expected)' \
  "${PENDING_FILE}" >/dev/null
jq --arg comment "Approved for ${VERSION} / ${SHA}" \
  '{environment_ids: [.[].environment.id] | unique, state: "approved", comment: $comment}' \
  "${PENDING_FILE}" > "${APPROVAL_FILE}"

ORIGINAL_GH_USER="$(gh api user --jq .login)"
TRIGGER_ACTOR="$(gh api "/repos/${REPO}/actions/runs/${RUN_ID}" --jq .actor.login)"
test "${APPROVER_GH_USER}" != "${TRIGGER_ACTOR}"
gh auth switch --user "${APPROVER_GH_USER}"
approval_status=0
gh api --method POST \
  "/repos/${REPO}/actions/runs/${RUN_ID}/pending_deployments" \
  --input "${APPROVAL_FILE}" || approval_status=$?
gh auth switch --user "${ORIGINAL_GH_USER}"
test "$(gh api user --jq .login)" = "${ORIGINAL_GH_USER}"
test "${approval_status}" -eq 0
```

Verify the active account after restoration with `gh api user --jq .login`. Do not approve a run until its workflow, tag or ref, full SHA, version, and expected environment all match the ledger.

### 7. Authorize and publish root, Docs, and official npm

After Framework and Admin external resolution and frontend publication succeed, dispatch and verify an independent `pre-root` readiness run. Update only the repository and `release` environment variables to its run ID; keep `release-v6` bound to the successful pre-framework run.

```bash
gh workflow run release-readiness.yml --ref main \
  -f version="${VERSION}" \
  -f frozen_commit="${SHA}" \
  -f feature_freeze_confirmed=true \
  -f phase=pre-root \
  -f publication_authority=true
```

```bash
bash tools/release/verify_readiness_run.sh \
  --repository "${REPO}" \
  --run-id "${PRE_ROOT_RUN_ID}" \
  --target-version "${VERSION}" \
  --commit "${SHA}" \
  --phase pre-root \
  --policy .mss/release-policy.yaml
gh variable set RELEASE_READINESS_RUN_ID --body "${PRE_ROOT_RUN_ID}"
gh variable set RELEASE_READINESS_RUN_ID --env release --body "${PRE_ROOT_RUN_ID}"
gh variable get RELEASE_READINESS_RUN_ID
gh variable get RELEASE_READINESS_RUN_ID --env release
```

Never create the root tag from a human Git credential. Dispatch the protected promotion workflow from `main`; before checkout it requires the workflow SHA, requested frozen commit, and remote `main` tip to be identical, then rechecks completed `pre-root` authority, all three public component Releases, and their exact successful tag workflows before its GitHub Actions identity creates or resumes the immutable annotated root tag. Use a dedicated `root-promotion` environment restricted to `main`, with `prevent_self_review=true`, `can_admins_bypass=false`, and the second account as required reviewer. The root-only creation ruleset must match only `refs/tags/v*` and allow only GitHub Actions Integration ID `15368`; keep human bypass confined to the separate component and Docs creation ruleset, and keep the immutable ruleset free of every bypass.

GitHub's built-in Actions Integration bypass is repository-wide, not scoped to one workflow. This release therefore also trusts workflow definitions already merged at the exact remote `main` tip; the environment and second-account approval protect the supported promotion path, but they do not turn Integration ID `15368` into a workflow-exclusive identity. If workflow-exclusive tag authority becomes mandatory, provision a dedicated GitHub App and expose only its short-lived installation token through `root-promotion`; do not claim that property or substitute a long-lived PAT before that boundary exists.

The ordinary Actions token can inspect public ruleset structure but cannot reliably see `bypass_actors`. Immediately before dispatch, the active repository administrator must run the checked-in governance verifier with the exact second reviewer and attach its sanitized JSON output to the evidence issue. This admin-side check is mandatory; do not inject a broad administration PAT into the workflow to replace it. The promotion workflow independently rejects missing, overlapping, excluded, or structurally invalid rulesets and its tag push remains fail-closed.

```bash
bash tools/release/verify_remote_release_governance.sh \
  --repository "${REPO}" \
  --reviewer-login "${APPROVER_GH_USER}" \
  > ".mss/reports/remote-release-governance-${VERSION}.json"
```

Approve the promotion job with the second account only after matching the exact workflow, main SHA, version, environment, and attached remote-governance result.

```bash
gh workflow run root-tag-promotion.yml --ref main \
  -f version="${VERSION}" \
  -f frozen_commit="${SHA}" \
  -f readiness_run_id="${PRE_ROOT_RUN_ID}"
```

The promotion workflow explicitly dispatches `container.yml` with `publish=true` and `release.yml` with `publish=false` from the exact root tag because a tag created with `GITHUB_TOKEN` does not recursively trigger workflows. A retry first finds an exact tag/SHA run by its stable run title and skips every waiting, active, or successful stage. It dispatches only a stage with no exact run; an existing failed run stops promotion so the operator can inspect or rerun that same run without creating a duplicate. The container job never overwrites an immutable version tag; on a retry it reuses an existing image only when its manifest digest, `linux/amd64` and `linux/arm64` manifests, release version, and frozen revision all match exactly, then reruns the post-publication checks. Wait for the promotion, root image publication, and candidate assembly to succeed. Their shared concurrency group may serialize the last two; do not treat a queued sibling as failed or missing.

Require the root image publication and tag-triggered candidate assembly to succeed before the final dispatch. Then dispatch `release.yml` from the exact tag:

```bash
gh workflow run release.yml --ref "${ROOT_TAG}" \
  -f version="${VERSION}" \
  -f feature_freeze_confirmed=true \
  -f publish=true \
  -f manual_evidence_complete=true \
  -f evidence_url="${EVIDENCE_URL}" \
  -f readiness_run_id="${PRE_ROOT_RUN_ID}"
```

Require `EVIDENCE_URL` to be exactly `https://github.com/OWNER/REPOSITORY/issues/NUMBER`. Do not pass a comment URL, fragment, pull request, empty issue, or cross-repository URL. Wait for the final run to conclude; its full test and six-platform build are expected work, not a reason to start another run.

For a coordinated release, next create `DOCS_TAG` from the same `SHA`, wait for the protected Docs deployment and immutable Docs Release, and verify the public `release.json`. Do not create any tag or artifact after the official npm step.

Finally dispatch `npm-release.yml` from the exact root tag with the exact version and full SHA. This workflow must download the existing frontend Release tarball and evidence rather than rebuild, prove every coordinated tag and Release points to `SHA`, verify the GitHub Packages mirror integrity, and publish the same bytes to npmjs. The npmjs publish is the final artifact publication; only read-only reconciliation, consumer verification, and the one-time Trusted Publisher credential handoff may follow it.

For the first npmjs version, use one short-lived granular bootstrap credential scoped to `@mss-boot-io/admin-web` in the protected `release-v6` environment. Immediately after the first successful publication, bind npm Trusted Publishing to `mss-boot-io/mss-boot-admin`, `npm-release.yml`, and `release-v6`, then revoke and remove the bootstrap token. Future releases use GitHub OIDC and never store a long-lived npm write token.

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
- Never delete, move, overwrite, or reuse a public tag, Release, package, image, or checksum.
- Never release from a topic branch, detached commit, local-only fix, dirty worktree, or commit absent from current `origin/main`.
- Never weaken a test, skip an environment, forge manual evidence, or use an old run ID to shorten the cycle.
- Never expose credentials, cookies, database DSNs, tokens, or sensitive bodies in artifacts or evidence.

## Output

Report the component train, version, exact commit, merged PR, phase ledger, approvals and actor restoration, commands actually run, migrations, security and compatibility impact, public tags/Releases, artifact checksums, image digests and platforms, smoke-test results, rollback path, skipped checks with reasons, and the next executable step.
