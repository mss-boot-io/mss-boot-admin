# ADR: Admin presentation publication workflow

- Status: Accepted for P1 implementation; no business-page adoption
- Date: 2026-08-24
- Owners: Admin platform, backend, frontend, security, agent infrastructure
- P0 decision: `docs/adr/2026-08-24-governed-admin-presentation-configuration.md`
- P1 feature contract: `.mss/features/admin-presentation-publication-workflow.yaml`
- Portable profile schema: `.mss/schemas/admin-page-presentation.schema.json`

## Context

P0 establishes the safe data boundary for Admin page presentation: a portable profile is a strict,
sparse overlay over capabilities already compiled into the application. It proves validation,
deterministic layer precedence, definition-hash drift handling, and authorization-last behavior, but it
does not persist or activate anything.

P1 must add operational publication without weakening that boundary. Treating the portable document as
an ordinary application setting is insufficient: it has no inactive draft, no strong concurrent-write
contract, no immutable publication history, no scoped authority, and no recovery mechanism. A database
row containing JSON is not a publication workflow.

## Decision

Adopt one versioned aggregate per exact `(scope, subject, pageKey)` identity, an append-only revision
stream, explicit command endpoints, and a statically compiled governance console.

### Aggregate and immutable revision model

The aggregate is server owned. It has a stable opaque ID, exact scope identity, monotonically increasing
aggregate version, optional inactive draft, optional current published revision pointer, creator,
updater, and timestamps. Application scope has an empty subject; role and user scope require an exact,
active opaque database identifier. These identifiers are compared as bounded strings and are never
revalidated as semantic names. A unique database constraint prevents two aggregates for one layer and page.

Aggregate state is derived rather than assigned by a client:

| Derived state | Draft | Published pointer | Meaning |
| --- | --- | --- | --- |
| `draft` | present | absent or present | Work exists but is not active; an older publication may remain active. |
| `published` | absent | present | The current immutable revision is active. |

Creation always creates a draft, so an empty aggregate is not persisted. P1 exposes no generic status
update and no delete endpoint.

Each publication appends an immutable revision with a profile-local sequence number, canonical document,
content SHA-256, definition hash, transition kind, actor, timestamp, hashed idempotency key, and request
fingerprint. Rollback stores the source revision number as lineage. Revision rows have no application
update or delete path.

### Transition table

| Command | Source state | Result | Additional guard |
| --- | --- | --- | --- |
| Create draft | no aggregate | `draft` | `If-None-Match: *`; unique exact identity. |
| Replace draft | `draft` or `published` | `draft` | Strong current aggregate `If-Match`; published pointer is preserved. |
| Publish draft | `draft` | `published` | Strong `If-Match`, current semantic validity, bounded `Idempotency-Key`. |
| Roll back | `published` | `published` | Strong `If-Match`, no pending draft, same-aggregate source revision, current semantic validity, bounded `Idempotency-Key`. |

Rollback never rewinds a pointer. It republishes the selected historical document as a new revision.
This preserves a complete audit chain and catches definition drift introduced after the old publication.
A pending draft blocks rollback so recovery cannot silently discard another operator's work.

### Draft and publication validation

Draft writes strictly decode the portable schema, reject unknown properties, enforce size and collection
bounds, and canonicalize JSON before hashing or persistence. A structurally safe draft may retain
capability-aware semantic issues so an author can repair it without changing the active page. The
aggregate response contains stable issue codes and paths.

Publish and rollback always rerun both structural and semantic validation against the current trusted
server registry inside the transition transaction. The registry, not the browser or stored document,
owns page, field, surface, component, data-source, action, default-presentation, and permission facts.
P1 deliberately registers no production business page; P2 will project definitions from AdminModule
specifications before adopting Supplier.

### Concurrency and idempotency

Create requires the canonical `If-None-Match: *` precondition. Every later mutation requires exactly one
strong aggregate ETag. The canonical quoted value binds aggregate ID and monotonically increasing version.
Missing preconditions return `428`, malformed values return `400`, and stale values return `412` together
with only the current opaque aggregate ID, version, and ETag so the console can reconcile without losing
its local draft. Draft content, history, page and scope metadata, and subject fingerprints remain behind
their independently authorized read paths.

Publish and rollback also require a bounded `Idempotency-Key`. Only its SHA-256 digest is stored. The
digest is unique within an aggregate and paired with a deterministic request fingerprint that includes
the authenticated actor. Retrying the same command reconstructs the original transition result from the
immutable revision, rather than projecting the aggregate's latest state, and does not append another
revision. Reusing a key from another actor or for different input returns `409`.

### Read, outage, drift, and recovery behavior

Management reads are authoritative writer-database reads and return database unavailability as an error.
P1 adds no Redis profile cache, so cache outage and stale-cache correctness are absent from this phase.

The effective-profile endpoint is optional presentation input, not a page-availability dependency. It
derives current user and role from the verified principal and returns only published application, role,
and user layers. Missing rows are normal. A stale hash, corrupt document, or semantic failure omits the
complete offending layer and returns diagnostics. If the presentation store is unavailable, the endpoint
returns empty layers with a bounded fallback outcome and emits an operational signal; the compiled page
therefore remains usable.

Startup-owned recovery mode always returns empty layers. It is configured outside database settings and
has no Admin mutation endpoint. Login, authentication, authorization, application configuration, release,
recovery, and the governance console are permanently rejected by the P1 capability registry, so stored
profiles cannot hide their own repair path.

### Permissions and audit

Management authority is split into four backend permissions:

| Permission | Allowed operations |
| --- | --- |
| `presentation:read` | List, detail, validate, and immutable history reads. |
| `presentation:draft-write` | Create and replace inactive drafts. |
| `presentation:publish` | Publish the current valid draft. |
| `presentation:rollback` | Republish a prior valid revision. |

No permission implies another. Root keeps the existing explicit bypass; every non-root management request
requires exact method and path policy. The current-principal effective read accepts no owner identity from
the client and exposes neither drafts nor history.

Presentation mutation bodies are excluded from generic audit-body capture. Value-free metadata records
aggregate ID, page key, scope, subject presence or one-way fingerprint, transition, bounded outcome,
aggregate version, published revision, definition hash, and content digest. It never records document
content, labels, descriptions, conditions, raw subject, raw idempotency key, cookies, or tokens.

### Governance console boundary

P1 provides a statically imported V6 route for aggregate status, strict JSON draft work, validation and
preview diagnostics, publication, immutable history, conflict reconciliation, and confirmed rollback.
It has distinct loading, empty, invalid, unavailable, permission-denied, conflict, recovery, and success
states in synchronized Chinese and English catalogs. A `412` preserves local text and shows the current
server revision; it never silently retries a write.

The console keeps recovery status visible even when the capability registry is empty, continues to expose
stored aggregates and immutable history for diagnosis, derives preview capability from the selected
aggregate's page key, and uses server pagination for both aggregate and history tables.

This is a governance console, not the P3 visual designer. It cannot load a component from configuration,
generate code, create a route or data source, or change a business schema. No production business page
consumes effective profiles in P1.

## Compatibility and rollout

The migration is forward-only. It creates aggregate and immutable revision tables, exact unique
constraints, and independent permission metadata without changing existing configuration or business
rows. Fresh, repeated, and upgrade-path tests cover the supported database contract.

P1 is shipped behind absence of production capability registrations. The governance workflow can be
qualified without altering any business page. P2 may register a generated Supplier definition behind its
own feature contract and browser evidence.

Emergency rollback first enables startup recovery mode, making every effective read resolve to compiled
defaults. The application and frontend can then return to the preceding qualified pair while P1 tables and
history remain intact. Recovery never deletes evidence.

## Rejected alternatives

- **Store the portable profile in AppConfig:** it cannot provide scoped identity, inactive drafts,
  append-only history, state guards, or independent publication permissions.
- **Let the browser submit a capability definition:** editable input cannot be the authority used to
  validate itself.
- **Publish by changing a status field:** arbitrary target-state writes bypass source-state and business
  guards and make concurrent behavior ambiguous.
- **Move the published pointer backward for rollback:** it destroys an auditable monotonic history and
  skips current-definition validation.
- **Allow rollback while a draft exists:** recovery would silently discard or strand another author's
  pending work.
- **Use last-write-wins or weak ETags:** concurrent authors can overwrite each other and retries can append
  duplicate publications.
- **Make Redis authoritative:** optional cache availability must not decide publication correctness or
  recovery behavior.
- **Make the governance page configurable:** a broken profile could remove the only browser repair path.
- **Adopt Supplier directly in P1:** production capability projection belongs to the generator and needs a
  separate P2 compatibility and rollout review.

## Consequences

P1 adds more state and explicit commands than a generic JSON settings endpoint, but every active change is
now conditional, attributable, immutable, revalidatable, and recoverable. Operators can prepare invalid
work safely, publishers cannot activate stale capabilities, retries cannot duplicate history, and runtime
pages retain compiled defaults when optional presentation data is unusable.

The next phase after P1 qualification is P2: extend AdminModule specifications and deterministic generation
to emit matching Go and TypeScript capability definitions, register Supplier behind an explicit feature
gate, and prove production overlay behavior in the built-in browser. P3 visual editing remains later work.
