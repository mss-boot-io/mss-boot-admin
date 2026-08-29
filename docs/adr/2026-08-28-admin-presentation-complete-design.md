# ADR: Complete Admin page presentation design

- Status: Proposed; maintainer approval required before P2 implementation
- Date: 2026-08-28
- Owners: Admin platform, backend, frontend, security, agent infrastructure, release engineering
- Complete design contract: `.mss/features/admin-presentation-complete-design.yaml`
- P0 decision: `docs/adr/2026-08-24-governed-admin-presentation-configuration.md`
- P1 decision: `docs/adr/2026-08-24-admin-presentation-publication-workflow.md`
- Portable profile schema: `.mss/schemas/admin-page-presentation.schema.json`
- First production reference: `.mss/modules/example-supplier.yaml`

## Context

The repository has already completed two deliberately separated foundations.

P0 defined a strict, data-only `AdminPagePresentation` document, trusted page capability definitions,
deterministic `compiled default < application < role < user` resolution, exact definition-hash drift
handling, whole-layer fallback, and permission intersection applied last. Its Supplier capability is a
test-only handwritten prototype and is not part of production routing.

P1 added the authoritative operational control plane: scoped aggregates, inactive drafts, strict shape
validation, capability-aware semantic validation, strong ETags, conditional transitions, idempotent
publication, immutable revision history, rollback by republication, independent permissions, redacted
audit, a current-principal effective-layer endpoint, startup recovery mode, and a statically compiled
governance console. P1 intentionally leaves the production capability registry empty. No business page
currently consumes an effective presentation layer.

That separation was correct. It proved the security and publication boundaries before production
adoption. The remaining work is not another generic JSON settings endpoint. It is the product and
generation architecture that connects:

1. `AdminModule` as the business-page source of truth;
2. deterministic Go and TypeScript capability projections;
3. explicit backend and frontend application composition;
4. optional effective-layer resolution over compiled business adapters;
5. a visual editor that remains a client of the P1 workflow;
6. upgrade, drift, recovery, observability, and Thin Host qualification.

The design must keep the repository's removed-runtime-tools boundary intact. Presentation configuration
must never become runtime entity creation, virtual CRUD, arbitrary routing, remote components, executable
templates, browser code generation, or a substitute for backend authorization.

## Decision

Adopt a four-plane architecture and roll it out through explicit disabled, shadow, active, and recovery
states. The planes share stable identifiers and one generated definition hash but have different authority.

```text
AdminModule specification
        |
        v
normalized presentation manifest + canonical definition hash
        |
        +--------------------------+
        |                          |
        v                          v
generated Go definition      generated TypeScript definition
        |                          |
explicit backend registry     static frontend registry + compiled page adapter
        |                          |
P1 validate/publish/effective ---> browser resolver and renderer
        |                          |
writer DB + immutable history      compiled API/query/action/component behavior
        \__________________________/
                    |
          adoption mode + recovery
```

### 1. Specification and generation plane

`AdminModule` becomes the source of truth for presentation-capable generated business pages. A module may
add one optional `spec.presentation` section. Omission is fully backward compatible: the current generated
page remains static and no capability is registered.

The first schema fixes these identities:

- `pageKey`: explicit, globally unique, and stable; it is not derived from route, display name, table,
  Go package, or file path;
- `definitionVersion`: semantic contract version, changed only for an incompatible definition format or
  interpretation. Generated definitions start at version `2`: the test-only P0 version `1` excluded
  `defaultPresentation` from its hash, while the production contract hashes complete defaults;
- local field, data-source, and action references: resolved against the existing entity, API,
  permission, and UI specification;
- complete defaults for list, search, form, detail, and action surfaces.

A shape-only Supplier excerpt is shown below. The committed P2A AdminModule source must enumerate the
complete production-equivalent defaults; omitted fields in this excerpt are not implicit defaults.

```yaml
spec:
  presentation:
    pageKey: supplier.list
    definitionVersion: "2"
    title:
      zh-CN: 供应商管理
      en-US: Suppliers
    dataSource: list
    list:
      density: large
      pageSize: 20
      defaultSort: []
      fields:
        - field: code
          component: text
          order: 10
          width: 180
        - field: name
          component: text
          order: 20
          width: 240
        - field: creditLevel
          component: tag
          order: 60
        - field: enabled
          component: boolean
          order: 70
    search:
      collapsedByDefault: false
      fields:
        - field: code
          component: input
          order: 10
        - field: creditLevel
          component: select
          order: 50
    form:
      columns: 1
      fields:
        - field: code
          component: input
          order: 10
          span: 24
        - field: enabled
          component: switch
          order: 70
          span: 24
    detail:
      columns: 1
      fields:
        - field: code
          component: text
          order: 10
          span: 24
        - field: enabled
          component: boolean
          order: 70
          span: 24
    actions:
      - action: export
        placement: toolbar
        order: 10
      - action: create
        placement: toolbar
        order: 20
      - action: read
        placement: row
        order: 30
      - action: update
        placement: row
        order: 40
      - action: delete
        placement: row
        order: 50
```

This section does not accept routes, URLs, methods, headers, SQL, permission strings, import paths,
JavaScript, React elements, or handler names. Those facts are derived from or remain in trusted compiled
contracts:

- field type, required state, validation, enum values, searchable/sortable/filterable flags, and surface
  eligibility come from the existing entity specification;
- the generated list data source comes from the compiled list operation and typed query adapter;
- action permissions come from existing generated permissions and route-policy contracts;
- component identifiers and field-type/surface compatibility come from the embedded Foundation-owned
  `.mss/admin-presentation-catalog.yaml` contract;
- modules may narrow a generated component allowlist but cannot register an implementation through data.

The presentation source accepts only unqualified local references. `dataSource: list` normalizes to
`<module>.list`, and `action: create` normalizes to `<module>.create`. A source data-source or action value
containing `.` is rejected so qualification has one meaning. Field identifiers remain page-local, component
identifiers are Foundation-global, and `pageKey` remains explicit rather than being derived by this rule.
The generated data-source capability also carries the transport-adjacent limits already enforced by the
compiled API/query adapter, including page-size choices, maximum page size, and maximum sort-field count.
Profiles may narrow behavior only within those limits and cannot provide query parameter names or encoders.

The version `2` catalog begins with the smallest closed Supplier matrix and rejects everything else:

| Component | Compatible value/surface |
| --- | --- |
| `text` | string on list/detail |
| `input` | string on search/form |
| `email-input` | email-formatted string on form |
| `tag` | enum on list/detail |
| `select` | enum on search/form |
| `boolean` | boolean on list/detail |
| `boolean-filter` | boolean on search with compiled all/true/false encoding |
| `switch` | boolean on form |
| `copyable-code` | generated identifier on detail |
| `date-time` | generated timestamp on detail |

Adding a component ID, compatibility row, or React implementation is a reviewed Foundation change. The
catalog inventory is embedded into the `mss` distribution; schema validation checks shape, semantic
validation checks this catalog, and frontend build tests prove every catalog ID has one static implementation.

The generator first produces one normalized in-memory manifest. It validates every reference and complete
default before writing a file. The definition hash is SHA-256 over canonical UTF-8 JSON for that complete
normalized compatibility surface, including title, layout, defaults, and compiled pagination/sorting limits,
but excluding the hash field, generated headers, timestamps, and file paths. Version `2` therefore retires
the P0 hash interpretation rather than silently reusing version `1`.

Canonical JSON uses fixed ASCII property-name ordering, arrays normalized by declared order and then stable
identifier (except semantically ordered sort precedence), UTF-8 JSON strings without HTML-only escaping, and
bounded integer numbers only. Go and TypeScript golden vectors cover Unicode, HTML-significant characters,
property ordering, array ordering, and empty collections. A default title, field order, layout, component,
pagination constraint, or action change must change the hash.

One manifest produces both sides:

| Projection | Ownership | Purpose |
| --- | --- | --- |
| `admin/modules/<module>/presentation_generated.go` | Generated | Trusted server definition staged with the business module. |
| `web/antd-v6/src/generated/modules/<module>/presentation.generated.ts` | Generated | Trusted browser definition for resolution and editor preview. |
| `web/antd-v6/src/generated/modules/<module>/presentation.adapter.generated.tsx` | Generated | Compiled data, field, component, and action adapter wiring. |
| Application presentation registry indexes | Generated | Explicit inventory of enabled modules; no runtime discovery. |
| Per-module normalized manifest snapshot | Generated | Data-only parity evidence and future old/new upgrade diff input; never a runtime database source. |
| Golden normalized manifest in tests | Test evidence | Proves Go/TypeScript identity and canonical hash parity. |

Generated regions are never hand-edited. A module custom file may register a typed compiled formatter,
component adapter, or action implementation only through a source-controlled extension point. The stored
profile still references only the stable identifier.

### 2. Publication and control plane

The P1 aggregate and publication workflow remain authoritative and unchanged in meaning.

- Identity remains exact `(scope, subject, pageKey)`.
- Application has no subject; role and user subjects are server-verified opaque identities.
- Drafts remain inactive.
- Publish and rollback rerun current structural and semantic validation in the transition transaction.
- Published revisions remain immutable and append-only.
- Strong ETags, `If-Match`, `If-None-Match`, bounded idempotency keys, independent permissions, audit
  redaction, and startup recovery mode remain mandatory.

The capability registry changes from an intentionally empty process default to an application-owned
immutable registry built from explicit business-module composition. A module's descriptor, migrations,
readiness checks, routes, and presentation definitions are staged in one registration transaction. If any
part is invalid, none of that module registration becomes visible.

The application composition root:

1. creates the business registration transaction;
2. validates and stages generated presentation definitions;
3. commits the complete module entry;
4. freezes the business and presentation registries;
5. injects the immutable registry into presentation services and APIs;
6. performs readiness checks;
7. mounts protected business routes;
8. starts listeners.

Package `init`, database content, request input, filesystem scanning, plugins, and remote modules cannot
mutate this registry. Duplicate page keys, protected page keys, invalid defaults, and hash mismatches fail
startup before listeners.

The governance console may list cloned definitions from the registry even when runtime application is
disabled. That lets operators prepare drafts before activation without allowing the database to define its
own validator.

### 3. Runtime and render plane

The frontend application has a generated static registry keyed by `pageKey`. Each entry binds:

- the serializable trusted capability definition;
- the generated page adapter;
- statically imported component identifiers;
- compiled query and mutation functions;
- field value codecs, enum options, formatters, and validators;
- compiled action callbacks and confirmation behavior.

The adapter is executable application code and is never returned by an API or persisted. A profile cannot
replace it.

A generated page calls one shared presentation hook. The hook requests
`/presentation/effective/:pageKey`; the server derives the current user and role from the verified
principal and returns at most one published application, role, and user document. The browser resolves:

```text
compiled defaults
    < published application layer
    < published current-role layer
    < published current-user layer
    < trusted frontend permission intersection
```

The effective read is optional presentation input, not a business-page dependency. Page shell and business
data work may proceed concurrently. The presentation request is bounded. Timeout, cancellation, transport
failure, database fallback, malformed content, stale hash, unknown reference, unsupported component,
required-field violation, or recovery state settles to lower valid layers and ultimately compiled defaults.
An offending layer is never partially applied.

The renderer may change only the existing portable contract:

- localized plain text;
- registered field visibility and order;
- allowed component choice;
- width and grid span;
- list density, page size, and default sort;
- form and detail column count;
- action visibility, order, placement, and confirmation text;
- bounded typed visibility conditions over fields already present in the compiled adapter.

It cannot change:

- route identity or URL;
- request method, headers, body, endpoint, query semantics, or credentials;
- database or model shape;
- business validation;
- action handler;
- backend permission;
- component implementation or import path.

Required create/edit fields cannot be hidden. Sort and filter choices must be supported by the compiled
query contract. Conditions use only allowlisted operators compatible with the registered value type and do
not read backend-only or absent record fields.

Condition context is surface-specific. List columns and toolbar actions do not accept conditions in the
first production contract because they have no single record context. Search fields read the current filter
draft; form fields and form actions read the current form draft; detail fields and detail actions read the
loaded record; row actions read that row. Only fields explicitly exposed by the compiled adapter for that
context are readable. Missing is distinct from `null`; `exists` is true for a present `null`, comparisons on
a missing value are false, equality and membership are strictly typed with no coercion, and ordered
comparisons are limited to registered number/date/date-time values. A condition controls rendering only and
never removes a query or submission value, changes a handler, or grants permission.

Permissions are applied last. A selected data source is usable only when the verified principal has its
trusted required permissions. Every action is intersected with its generated permission requirement.
Frontend checks improve the experience; direct backend authorization remains the security boundary.

With no profile or with presentation disabled, the resolved render model must be semantically equivalent to
the current generated page. Pagination, search, create/edit forms, detail, export, errors, empty states,
conflicts, responsive behavior, and API calls must remain unchanged.

### 4. Operations, rollout, and recovery plane

Capability registration and runtime application are separate. Startup-owned configuration adds:

```yaml
presentation:
  recoveryMode: false
  adoptionMode: disabled # disabled | shadow | active
  activePages: []        # exact pageKey allowlist; empty means no active page
```

Database profiles cannot alter these controls.

| State | Registry and governance | Effective resolution | Page rendering |
| --- | --- | --- | --- |
| `disabled` | Definitions visible; drafts and publication available. | Returns an adoption-disabled outcome. | Compiled defaults only. |
| `shadow` | Same as disabled. | Loads, validates, and resolves current published layers for diagnostics and parity. | Compiled defaults only. |
| `active` but page not allowlisted | Definitions visible. | Returns not-allowlisted outcome. | Compiled defaults only. |
| `active` and page allowlisted | Definitions visible. | Returns valid selected layers or bounded fallback diagnostics. | Applies resolved presentation. |
| `recoveryMode: true` | Management, history, and diagnostics remain available. | Empty layers and recovery outcome. | Compiled defaults only, regardless of mode or allowlist. |

Shadow mode must not perform a hidden business mutation, alternative query, or action. It may compute the
resolved render model and compare structural facts such as field/action counts and validation outcomes for
bounded metrics.

Observability is value-free. Logs, metrics, audit, and upgrade reports may contain bounded compiled page
keys, opaque aggregate/profile identifiers, scope kind, definition hash, mode, layer, and stable outcome.
They must not contain profile text values, raw subjects, business records, condition values, secrets,
cookies, tokens, or idempotency keys.

The writer database remains authoritative. An in-process or browser in-memory cache may avoid duplicate
reads, but it is bounded derived state, uses no persistent browser storage, and cannot decide publish or
repair correctness. A distributed cache is not part of the first production adoption.

## Visual editor decision

The existing governance console remains the only publication UI. P3 adds a visual editing mode inside that
console rather than a second configuration product.

The editor has three coordinated areas:

1. capability and scope selection;
2. list/search/form/detail/action workspace with drag ordering and synthetic preview;
3. property inspector for inheritance, localized text, visibility, component, width/span, density,
   page size, sorting, placement, confirmation, and typed conditions.

Visual mode and raw JSON mode share one typed draft abstract syntax tree. Omission means inheritance and
must remain omission. Explicit `false` remains explicit. Switching modes, saving, reloading, conflict
reconciliation, or previewing must not drop conditions or normalize away meaning. Canonical JSON generated
from the AST is the document sent to existing P1 endpoints.

The editor derives every option from the selected capability. It cannot offer an unsupported component,
field, operator, action placement, or property. Validation issues link both to the visual control and raw
JSON path. Structurally safe but semantically invalid work may remain an inactive draft; publish remains
blocked.

Generated adapters expose two type-separated surfaces: a pure view adapter containing static component,
formatter, codec, validation, and synthetic-preview bindings, and an effectful business adapter containing
query, mutation, and action callbacks. Preview may import only the view adapter. A build test rejects any
query, mutation, action callback, or network client in the governance preview dependency graph.

Preview uses generated synthetic values and the pure view adapter by default. It does not fetch a production
business record merely to make the editor realistic. It does not execute actions. Existing P1 permissions
remain independent:

- read to inspect;
- draft-write to save;
- publish to activate;
- rollback to republish history.

A user-directory or role-directory picker may reuse an existing authorized directory API. Presentation
permissions do not silently grant broader directory access; an operator without that access may use only
identities already available through an authorized workflow.

## Definition drift and upgrade decision

Exact definition hashes remain conservative and fail closed. Any normalized semantic definition change
produces a new hash. An older draft or publication is stale even when a human believes the change is
additive. This avoids silent reinterpretation and makes upgrade review explicit.

Runtime behavior for stale data is fixed:

1. omit the complete stale layer;
2. retain lower valid layers;
3. retain compiled defaults;
4. emit a bounded diagnostic;
5. leave stored draft and immutable history untouched.

The governance console distinguishes:

- current;
- stale definition;
- unregistered page;
- corrupt structure;
- semantically invalid;
- disabled;
- shadow;
- active;
- recovery bypass.

A rebase helper may prepare an inactive draft against the current definition. Version `2` reports retained,
added, removed, changed, and incompatible identifiers. It never guesses a rename from similar names; a
future explicit source-controlled rename-hint contract is required before `renamed` can be reported. The
helper cannot publish, cannot move a history pointer, and cannot silently delete an unresolved reference.
Updating only the hash without current validation is forbidden.

`mss upgrade admin` planning gains source-level presentation impact information from the old and new
generated snapshots:

- old and new page keys and hashes;
- added, removed, and changed fields, actions, components, surfaces, and defaults;
- whether generated backend/frontend inventories still match;
- which page keys may have stale persisted profiles.

The upgrade tool does not connect to a production database and does not claim how many runtime profiles are
affected. Runtime status remains the governance console's responsibility.

Thin Host upgrades preserve P1 tables and stored documents. Generated artifacts change through the existing
managed snapshot and three-way upgrade engine. Business-owned custom files remain untouched. Backend and
frontend projections move as one coordinated Admin Distribution version; mixed identities are not
supported.

## Supplier production reference

Supplier is the first and only initial active reference.

The current handwritten `supplier.prototype.ts` proves P0 resolver behavior but is not a production-equivalent
default and is not production source. P2A replaces it as the definition source with a version `2` declaration
in `.mss/modules/example-supplier.yaml` and generator-owned Go and TypeScript outputs. The generated default
matches the current page: large list density, page size 20 from `[20, 50, 100]`, no initial sort, expanded
search with keyword/country/credit-level/three-state-enabled controls, single-column form and detail, derived
read-only ID/timestamps, and export before create in the toolbar. The keyword field remains the compiled
`code -> q` adapter binding; profiles never receive the transport key. The stable page key remains
`supplier.list`; current APIs, permissions, route, query behavior, CRUD, export, validation, locales,
responsive rules, and page states remain compiled.

Qualification order is mandatory:

1. **Disabled parity:** generated capability is registered and visible, but Supplier renders current
   compiled defaults.
2. **Shadow parity:** published application/role/user documents resolve, diagnostics are recorded, and
   Supplier still renders defaults.
3. **Active allowlist:** only `supplier.list` applies profiles.
4. **Failure matrix:** no profile, invalid draft, stale publication, unknown references, store outage,
   request timeout, permission denial, and recovery mode all produce expected defaults or forbidden state.
5. **Behavior matrix:** list, search, pagination, sort, create/edit form, detail, actions, export,
   conditions, localization, refresh, and responsive layout.
6. **Security matrix:** direct unauthorized API calls remain denied regardless of visible controls.
7. **Thin Host matrix:** external generation, build, run, upgrade, idempotency, and custom-file
   preservation with matching Go and TypeScript hashes.

Supplier remains the only allowlisted page until exact-Head tests, built-in browser evidence, external
Thin Host qualification, and maintainer approval pass. Later generated pages are adopted independently.

## Implementation checkpoints

This ADR authorizes no implementation by itself. After explicit maintainer approval, work proceeds as
coherent pushed checkpoints:

| Checkpoint | Scope | Exit evidence |
| --- | --- | --- |
| D0 | This complete design only. | Remote commit verified; FeatureSpec valid; docs build; maintainer approval. |
| P2A | AdminModule schema, semantic validation, normalized manifest, hash vectors, Go/TS generation. | Schema/generator/golden/parity/two-run tests. |
| P2B | Backend module staging, immutable registry injection, adoption configuration, effective diagnostics. | Focused backend, migration/config, authorization, outage, recovery tests. |
| P2C | Static frontend registry, shared hook/resolver/renderer, generated Supplier adapter, disabled and shadow modes. | Lint, typecheck, unit/integration, compiled-default parity. |
| P2D | Supplier active allowlist and full production qualification. | Built-in browser, direct API permissions, drift/outage/recovery, Thin Host. |
| P3 | Lossless visual editor integrated with P1 governance. | AST round-trip, editor workflow, conflict, publish/history/rollback browser evidence. |
| P4 | Additional generated pages one by one. | Separate page contract and browser evidence for each adoption. |

Each checkpoint is inspected, committed, pushed, and remotely verified before broader validation. A push
is not validation. Failed gates are repaired with follow-up commits; pushed history is not rewritten.

## Security invariants

- Stored presentation is closed, bounded, data-only JSON.
- The exact compiled definition is the validator.
- No profile carries routes, transport, permissions, imports, implementations, handlers, or executable
  expressions.
- Module composition is explicit and frozen before serving requests.
- Page identity and definition hash are stable compatibility surfaces.
- Invalid layers fail as units.
- Required forms remain usable.
- Conditions are typed and bounded.
- Permissions are trusted facts and applied last.
- Backend APIs enforce authorization independently.
- Protected core and recovery surfaces are excluded.
- Recovery is startup-owned and outside database control.
- Audit and observability are value-free.
- No dynamic import, remote component, micro-frontend, virtual CRUD, runtime model, or browser code
  generation is reintroduced.

## Rejected alternatives

- **Keep a handwritten backend and frontend capability per page:** creates an inevitable dual-source drift
  and bypasses deterministic generation.
- **Derive page keys from routes or filenames:** makes refactoring silently break persisted identities.
- **Send component paths or React trees in profiles:** creates a code-loading surface and couples stored
  data to implementation details.
- **Let the server return a complete executable page schema:** confuses presentation data with query,
  action, and authorization authority.
- **Resolve permissions before overlays or store them in profiles:** allows editable data to weaken or
  replace trusted requirements.
- **Render live production records in the editor by default:** expands data exposure and makes draft work
  depend on business availability.
- **Automatically update definition hashes during deployment:** silently reinterprets stored intent and
  can activate invalid profiles without review.
- **Activate every generated page immediately:** removes the ability to prove parity and contain
  regressions.
- **Use presentation availability as a page readiness gate:** turns an optional display feature into an
  outage multiplier.
- **Make the governance or recovery page configurable:** allows configuration to remove its own repair
  path.
- **Reintroduce runtime dynamic models or virtual CRUD:** violates the permanent removed-runtime-tools
  boundary.

## Consequences

The design adds generator and runtime integration work, but it keeps one authoritative business
specification, one compatibility hash, explicit compiled behavior, recoverable publication, and bounded
operational rollout. Operators gain useful presentation control without gaining schema, transport,
component, or permission authority. Generated modules and Thin Hosts remain upgradeable because stable
identities and generated ownership are explicit.

Exact hashes deliberately prefer safe fallback over permissive compatibility. Some harmless module changes
will require a reviewed rebase draft. That is an accepted cost until a future version can prove a more
granular compatibility model without weakening current safety.

## Approval gate

The branch containing this ADR is a D0 design checkpoint only. It must contain no implementation code,
migration, generated capability, runtime registry change, active page, PR, merge, tag, or release. P2A
starts only after the maintainer reviews the remotely pushed design commit and explicitly approves
development.
