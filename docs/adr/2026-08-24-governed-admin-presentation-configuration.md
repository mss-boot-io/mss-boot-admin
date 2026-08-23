# ADR: Governed Admin page presentation configuration

- Status: Proposed; P0 contract and non-production prototype
- Date: 2026-08-24
- Owners: Admin platform, frontend, security, agent infrastructure
- Feature contract: `.mss/features/admin-presentation-configuration.yaml`
- Profile schema: `.mss/schemas/admin-page-presentation.schema.json`

## Context

The complete Admin backend and Ant Design 6 frontend are now distributed independently from business
projects. That solves source ownership and upgradeability, but it does not make routine page presentation
changes operational. Columns, search controls, form layout, detail layout, labels, density, and action
placement are still compiled into each React page. A harmless display change therefore requires a source
change, build, review, and release.

The repository deliberately removed runtime dynamic models, virtual CRUD, and browser-facing template or
code generation. It also requires routes and business registration to remain compile-time explicit, with
backend authorization authoritative. Page configuration must strengthen the product without crossing any
of those boundaries.

The useful target is therefore not “arbitrary pages without code.” The useful and defensible target is:

> Within capabilities already compiled and authorized by the Admin distribution, routine presentation
> choices are data; creating a new capability remains specification, generation, code, migration, review,
> and release work.

## Decision

Adopt a two-sided contract and a deterministic resolver.

### Trusted capability definitions

Maintained code or deterministic module generation emits a `PageCapabilityDefinition` for each supported
page. It is trusted application code and declares:

- a stable page key, definition version, and SHA-256 definition hash;
- registered fields and the surfaces on which each field is valid;
- value type, required state, sorting and filtering support;
- allowed component identifiers for each field;
- registered data sources and their backend permission requirements;
- registered actions, placements, destructive semantics, and backend permission requirements;
- a complete safe default presentation.

No editable document may redefine those facts. The definition hash covers the compatibility surface, not
the user's presentation choices, and lets the runtime identify profile drift after a module upgrade.

### Untrusted sparse presentation profiles

An `AdminPagePresentation` document is untrusted data and is validated twice:

1. JSON Schema validates exact version, shape, bounds, and closed objects.
2. The selected capability definition validates every page, field, component, data-source, action, sort,
   filter, surface, required-field, and condition reference.

The profile can change only localized plain text, visibility, ordering, component selection from an
allowlist, width, span, table density, page size, default sorting, layout columns, action placement, and a
bounded declarative visibility condition tree. It cannot contain permissions, routes, URLs, HTTP methods,
headers, HTML, JavaScript, expressions, SQL, imports, component implementations, plugins, or templates.

The profile metadata binds the document to one exact page key and definition hash and identifies its
application, role, or user scope. Server revision, draft, publish, and audit metadata will live in a
separate persistence envelope in P1 rather than changing the portable profile document.

### Resolution and security order

The resolver applies sparse layers in this order:

```text
compiled default < application profile < role profile < current-user profile
                                              |
                                              v
                            trusted permission intersection last
```

At most one already-selected profile participates at each layer. Resolving multiple user roles into one
deterministic role profile is a future server-side policy decision; the browser does not invent a role
merge order.

Missing values inherit from the lower layer and explicit `false` remains an explicit value. Each layer is
validated as a unit against the current definition and the current lower-layer result. A page-key or hash
mismatch, unknown identifier, duplicate reference, incompatible component, unsupported sort/filter, or
attempt to hide a required form field rejects that complete layer. Lower valid layers remain available and
the resolver returns structured diagnostics.

After all overlays, the resolver intersects the selected data source and every action with permissions
stored in the trusted capability definition. An editable profile cannot supply or remove a permission. If
the data-source permission is absent, the renderer receives a permission-denied model and no actions.
This browser check improves experience only; backend authorization remains authoritative for every request.

### Renderer boundary

The resolver outputs a framework-neutral render model: localized title, authorized data-source identifier,
ordered visible list/search/form/detail fields, allowed component identifiers, actions, and page state. A
future `ConfigurablePage` maps only registered component identifiers to imported React components. It never
imports a path supplied by configuration.

Two later adoption modes are allowed:

1. Overlay an existing compiled page while retaining its route and data adapter.
2. Create another view of an existing registered resource beneath one compiled route such as
   `/views/:viewKey`.

Neither mode creates a new resource, backend endpoint, permission, route implementation, or component.

## P0 scope

P0 delivers only:

- the FeatureSpec and this decision;
- the strict `mss.io/v1alpha1` JSON Schema;
- TypeScript capability, profile, resolver, validator, and render-model contracts;
- a non-production Supplier capability and compact application profile;
- contract tests for precedence, drift, reference safety, required-field safety, immutability, and
  authorization-last behavior;
- a planned capability-catalog entry.

The Supplier prototype is not registered in the production route tree, does not replace generated output,
does not call an API, and does not persist data. P0 therefore requires no migration or runtime rollback.

## Security invariants

- Configuration is data-only and objects are closed by schema.
- Only exact compiled identifiers are accepted.
- Definition drift rejects the whole layer; it is never partially “best effort.”
- Permission requirements exist only in trusted definitions and are applied last.
- Visibility is never treated as authorization.
- Required form inputs cannot be hidden by a profile.
- Conditions read registered record fields and use a bounded operator tree; they do not execute code.
- Localized text is plain text. Renderers must not use raw HTML injection.
- Authentication, authorization, application configuration, release, and recovery pages remain excluded
  until separately qualified.
- A later production editor must include an unconfigurable recovery path.

## Compatibility and generation

Stable identifiers are compatibility surfaces. Generator changes that rename or remove an identifier must
change the definition hash and report affected profiles. A later upgrade workflow may offer a reviewed
profile migration, but it must never silently reinterpret a stale identifier.

Generated business pages ultimately need generated capability definitions sourced from the same
`AdminModule` specification as backend, API, permissions, routes, frontend, tests, and documentation.
P0 keeps Supplier in a handwritten prototype fixture to prove the contract without hand-editing generated
files. Production adoption requires updating the generator source and proving deterministic, two-run
idempotent output.

## Persistence and publication roadmap

P1 may add an authoritative backend resource only after a dedicated design covers:

- profile ownership and scoped read/write permissions;
- draft validation and preview;
- strong ETag conditional writes;
- atomic publish with immutable revision history;
- rollback by publishing a prior valid document as a new revision;
- redacted audit events and metrics;
- definition-drift diagnostics and a compiled-default fallback;
- database authority, bounded cache behavior, and outage semantics;
- an unconfigurable recovery mode.

P2 can project capability definitions from generated business modules and adopt Supplier behind an explicit
feature gate. P3 can add a visual editor and additional views of registered resources. P4 can evaluate
selected core pages after recovery and security evidence. No phase is authorized by this P0 ADR alone.

## Rejected alternatives

- **Store arbitrary React or JSON component trees:** couples persisted data to implementation details and
  enables an unsafe component-loading surface.
- **Allow URLs, methods, headers, GraphQL, or SQL in a profile:** turns presentation configuration into a
  data-access and credential-exfiltration mechanism.
- **Allow JavaScript or expression evaluation:** recreates executable browser templates and makes review,
  CSP, reproducibility, and authorization reasoning unreliable.
- **Make permissions configurable:** lets a presentation operator mutate authority and conflicts with
  backend-authoritative RBAC.
- **Use menu component strings as dynamic imports:** bypasses the compiled route registry and package export
  contract.
- **Revive runtime dynamic models or virtual CRUD:** violates the explicit removed developer-tools boundary.
- **Fork one bespoke settings format per page:** prevents deterministic validation, publishing, upgrades,
  and a generic renderer.

## Consequences

Routine display choices can become configurable while the application keeps a small, auditable attack
surface. New capabilities still require code and release work, which is an intentional product boundary.
The platform must maintain stable identifiers, registry validation, compatibility hashes, and renderer
components as public contracts.

P0 completion is measurable: the feature validates, the schema safety tests pass, the focused frontend
tests produce the expected Supplier render model, lint and documentation build pass, no production route
imports the prototype, and the tracked worktree remains clean after verification. The next executable step
after P0 review is a separate P1 FeatureSpec for draft, publish, history, rollback, and recovery persistence;
it is not to wire the prototype directly into production.
