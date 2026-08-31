# Replaceable Docs deployment pointer

- Status: Accepted
- Date: 2026-08-31
- Scope: Public documentation website publication

## Context

The Admin Distribution publishes immutable Root, Framework, Admin, frontend,
npm, image, checksum, and Release identities. The Docs site is different: its
tag exists only to trigger and identify a website deployment, and a Docs failure
must not consume another product patch version or block adopter availability.

The former Docs policy made every `docs/vX.Y.Z` publication immutable and used
`+docs.N` tags for later wording or site corrections. That created permanent
release identities for website-only changes and added one-shot policy fields,
revision ordering, predecessor-asset checks, and extra governance unrelated to
Distribution artifacts.

## Decision

`docs/vX.Y.Z` is a replaceable deployment pointer for the current stable
website. The authorized release operator may:

1. qualify a new Docs source committed through a pull request and merged into
   `main`;
2. verify that the matching Root Release exists and its commit is an ancestor of
   the selected Docs commit;
3. delete the existing Docs GitHub Release;
4. delete the same remote Docs tag;
5. prove both public identities are absent;
6. recreate the same annotated `docs/vX.Y.Z` tag at the qualified commit.

Direct tag update or force-push remains prohibited. Controlled creation and
deletion belong only to the release operator. A no-bypass rule rejects in-place
update. Exact stopped-train rules reject creation, deletion, and update of their
Docs refs with no bypass.

The workflow still verifies the merged-PR source, Root ancestry, protected
production environment, organization-managed credential, portable archive,
checksums, public `/release.json`, GitHub Release assets, and browser-visible
site. A deployment may advance only from the commit currently exposed by the
production site to a descendant commit.

All non-Docs release identities remain immutable.

## Consequences

- Website corrections reuse the stable Docs tag and never consume a product
  patch version or `+docs.N` identity.
- A Docs Release URL and tag no longer prove historical bytes. The current
  `/release.json` full commit, merged pull request, workflow run, deployment run,
  and Git history are the audit record.
- Deleting the Docs tag or Release can temporarily leave the website without a
  matching GitHub identity. That state is reported as `website deployment
  pending` and never changes Distribution stability.
- Existing historical `+docs.N` tags and Releases remain historical evidence;
  the current workflow does not create new ones.

## Validation

The decision is complete when repository tests prove:

- the policy accepts only the current stable base Docs version;
- a same-version descendant commit is deployable;
- the Docs tag must contain the matching Root commit and come from merged main;
- remote governance excludes Docs from core immutability, controls deletion,
  and blocks in-place update;
- the workflow refuses a stale same-name Release until the operator deletes it;
- checksum, deployment, `/release.json`, Release, and browser reconciliation
  remain mandatory.
