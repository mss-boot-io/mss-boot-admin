# ADR: Classify all Admin routes and adopt every table management page

- Status: Accepted; exactly fourteen Foundation built-ins are exact-allowlisted active and passed
- Date: 2026-08-30
- Evaluated branch: `codex/page-presentation-complete-design`
- Evaluated baseline commit: `1b855d0b71e2884d7b743d49edc73f4dcd853787`
- Owners: Admin platform, backend, frontend, security, agent infrastructure
- Successor FeatureSpec: `.mss/features/admin-presentation-all-management-pages.yaml`
- Page and route inventory: `.mss/admin-presentation-page-inventory.yaml`
- Inventory schema: `.mss/schemas/admin-presentation-page-inventory.schema.json`
- Predecessor design: `.mss/features/admin-presentation-complete-design.yaml`

## Context

The predecessor design established the presentation architecture and implemented two deliberate proofs:

- external generated business page `supplier.list`, with complete list, search, form, detail, and action
  presentation in an AdminModule and Thin Host;
- Foundation core page `user.list`, with a separately reviewed limited list and search surface.

Those proofs establish both supported source kinds, but the Supplier proof never made Supplier an
mss-boot-admin built-in page. This decision supersedes any ambiguous predecessor wording: Foundation owns
exactly fourteen built-in table-management page keys, while `supplier.list` remains an explicitly composed
external business-extension example in the reference Admin and in eligible Thin Hosts. Supplier does not
participate in Foundation built-in activation,
Browser acceptance, completion, or rollout. The first Foundation core-page source and binding were
intentionally specific to user management; this successor generalizes that core mechanism to the other
thirteen built-ins.

The repository also has more route shapes than one CRUD table:

- layouts and redirects;
- hidden create, edit, and security subroutes;
- one `/log` route containing login, audit, and runtime datasets;
- Workplace dashboard composition;
- personal account and security settings;
- secret-bearing application configuration;
- presentation governance and recovery;
- public authentication and exception fallbacks.

Calling this entire set configurable without first classifying it would create two failures. Product
coverage could be claimed while pages do not consume the runtime, and pressure for flexibility could
accidentally move authorization, transport, task execution, configuration content, credentials, or
recovery controls into stored data.

## Decision

Adopt a machine-closed route inventory and a bounded all-management-page target.

### 1. Classify every compiled route declaration

`.mss/admin-presentation-page-inventory.yaml` is the machine contract for product coverage. It lists the
route sources, every route declaration, every page family, inclusion or exclusion, the reason for
exclusion, page identity, source ownership, configurable facets, protected capabilities, rollout wave,
and acceptance state.

The route closure is declaration-based rather than normalized-path-based. A layout and a nested redirect
may share `/security` but remain separate compiled declarations with separate inventory identifiers.

A deterministic CI check must compare:

1. `web/antd-v6/package/core-routes.cjs`;
2. `web/antd-v6/config/routes.generated.ts`;
3. `web/antd-v6/src/generated/routes.ts`;
4. the inventory route declarations.

The two generated files are paired projections of one generated route identity: the Umi route
configuration is the compiled declaration and the packaged registration index is its distribution
contract. Their ordered path sets must agree and map to one inventory record; they are not counted as
two user-visible routes.

Any missing, extra, duplicated, or newly unclassified declaration fails. An excluded route is complete
only when it has a durable reason; it is not counted as missing implementation.

### 2. Include exactly fourteen Foundation table-management pages

The successor Foundation target contains these fourteen stable page keys, all with a limited surface:

| Page key | Route | Scope |
| --- | --- | --- |
| `user.list` | `/users` | Limited |
| `role.list` | `/role` | Limited |
| `menu.list` | `/menu` | Limited |
| `department.list` | `/departments` | Limited |
| `post.list` | `/posts` | Limited |
| `task.list` | `/task` | Limited |
| `notice.list` | `/notice` | Limited |
| `language.list` | `/language` | Limited |
| `option.list` | `/option` | Limited |
| `system-config.list` | `/system-config` | Limited, redacted metadata only |
| `online-session.list` | `/security/online-sessions` | Limited, root only |
| `log.login` | `/log` | Limited login dataset |
| `log.audit` | `/log` | Limited audit dataset |
| `log.runtime` | `/log` | Limited runtime dataset |

Every Foundation built-in exposes only:

- localized title;
- safe registered list columns and labels;
- list order, visibility, width, density, and compiled page size;
- safe registered search fields and labels;
- search order, visibility, and initial collapse.

Built-in pages do not expose forms, details, actions, mutations, privileged dialogs, route behavior,
transport, query encoding, component implementations, conditions, or sensitive fields.

### 2.1 Preserve Supplier as external business-extension evidence

`supplier.list` remains a full-surface AdminModule and Thin Host example outside the Foundation built-in
inventory and core registries. The reference Admin explicitly composes this business module and exposes its
generated `/suppliers` route; eligible Thin Hosts may compose it as well. Its separately owned extension
contract may expose:

- title;
- list, search, form, and detail fields;
- registered component choices;
- density, page size, and compiled sort choices;
- registered action visibility, order, placement, labels, and confirmation;
- bounded reviewed conditions.

This retains generator and downstream-extension evidence without adding Supplier to Foundation
`activePages`, the built-in Browser matrix, built-in acceptance state, the completion count, or any rollout
wave. External Supplier qualification may run independently, but it cannot substitute for or block the
fourteen-page Foundation decision.

### 3. Preserve sensitive subflows in compiled code

The route family may be included while a hidden route or dialog remains protected.

- User create, edit, delete, password reset, credentials, root facts, and relations stay compiled.
- Role authorization trees, root/default safeguards, and authorization actions stay compiled.
- Menu routes, permissions, HTTP methods, API binding, and mutations stay compiled.
- Post data-scope semantics stay compiled.
- Task provider, endpoint, body, metadata, Python, execution, and scheduler behavior stay compiled.
- Notice current-user scope, detail access, delivery, and mark-read mutation stay compiled.
- System configuration content, secrets, detail, create, edit, delete, import, and export stay compiled.
- Online-session identities, raw tokens, revocation, and current-session safeguards stay compiled.
- Log data sources, row scope, redaction, raw payloads, file selection, filters, truncation, and export
  remain compiled.

Presentation may reduce or reorder visible registered controls. It never grants access, broadens rows,
creates data, changes a query encoder, or replaces a mutation.

### 4. Use one reviewed source and one identity per page

Each of the fourteen built-in pages receives one source under `.mss/core-pages/` and one closed Foundation
binding. The source contains presentation defaults only; trusted permissions, transport, component
implementations, and handlers remain compiled. Supplier continues to use its external AdminModule source.
The reference Admin explicitly composes it after Foundation core capabilities, and a downstream business
host may make the same explicit choice.

One normalizer and canonical hash pipeline must produce:

- backend definition;
- frontend definition;
- normalized manifest snapshot;
- backend registry entry;
- frontend static registry entry.

Foundation backend and frontend core inventories must contain the same fourteen page keys and equal
versions and hashes. Independent handwritten Go and TypeScript definitions are prohibited. Thin Hosts
consume packaged core definitions without copying Foundation source files and may separately generate and
compose their owned Supplier extension.

### 5. Require real runtime consumption

Registry membership is not page adoption. Every included page must call the shared bounded runtime and
render the resulting model.

The optional effective read cannot block authorization, page shell, or business data. No profile,
disabled mode, shadow mode, not-allowlisted state, stale hash, bad content, database failure, network
failure, timeout, cancellation, or recovery mode must settle to complete compiled defaults.

Profiles use application, current-role, and current-user precedence. The server selects the principal.
Backend authorization and row scope are applied after presentation resolution.

### 6. Keep protected and non-management routes excluded

The following remain explicitly excluded:

- Workplace dashboard;
- account center and account settings;
- login, registration, password recovery, and OAuth callback pages;
- application configuration;
- presentation governance;
- forbidden and not-found pages;
- redirect declarations.

These exclusions are not permanent statements that no future presentation contract may exist. They are
the safety boundary of this table-page successor. Dashboard composition or personal-page presentation
requires a separately versioned product decision and contract.

Presentation governance is deliberately excluded from its own registry. It must remain usable when every
profile is invalid, adoption is disabled, or recovery mode is active.

### 7. Roll out by exact page key

All fourteen Foundation built-in pages started disabled and advanced through isolated shadow evidence
before exact activation. The accepted state now has all fourteen page keys in the exact active allowlist
with inventory acceptance passed.

1. Wave 0 validates route closure and fresh or upgraded governance.
2. Wave 1 qualifies `user.list` as the first Foundation built-in.
3. Wave 2 adds department, post, language, and option.
4. Wave 3 adds limited role, menu, task, notice, system configuration, and online sessions.
5. Wave 4 adds login, audit, and runtime log identities.
6. Wave 5 advances each accepted page through isolated shadow to exact allowlisted active.

Supplier is absent from every Foundation wave. Its compiled route remains available in the reference Admin;
any presentation activation or qualification for that business extension, whether in the reference Admin or
a downstream host, follows a separate non-blocking lifecycle.

Wildcard, empty-means-all, database-controlled, and newly-generated automatic activation are prohibited.
Recovery mode overrides every Foundation built-in page. Removing one page key from the allowlist restores its compiled
defaults without deleting profiles or history.

## Acceptance record

Files or green unit tests alone do not establish completion. The accepted record satisfies these gates:

- strict inventory schema and semantic validation;
- compiled route closure;
- one source and paired generation for every one of the fourteen Foundation built-in pages;
- equal backend and frontend inventory and hashes;
- runtime-consumer tests for every one of the fourteen Foundation built-in pages;
- direct positive and negative authorization and row-scope tests;
- whole-layer failure and recovery tests;
- fresh and upgrade-path governance-console acceptance;
- external Thin Host qualification for packaged Foundation consumption; any Supplier extension evidence is
  recorded separately and does not define built-in completion;
- built-in Browser evidence on one final unchanged remote Head.

The final Browser record covered all fourteen page keys across twelve real routes using visible titles,
table rows, or an exact empty state. `/users`, `/task`, `/language`, `/security/online-sessions`, and `/log`
retained published presentation after reload. User and log pages remained usable at 390x844. The limited
editor exposed only general, list, and search workspaces; hid data source, component, default sort, search
placeholder, and help controls; and rejected raw `spec.dataSource` as `unsupported-limited-surface`.
Supplier published presentation did not apply and its reference-Admin route retained compiled defaults.
Browser console warnings and errors were both zero.

The screenshots and browser record remain outside the repository in the system-temporary evidence bundle
`mss-presentation-acceptance-20260830`; no screenshot, console log, or request log is committed here.

If the accepted Head, page identities, definition hashes, route set, limited surface, or exact active
allowlist changes, the affected Browser evidence must be rerun rather than inferred from this record.

## Consequences

### Positive

- Product coverage becomes measurable and cannot silently omit a legacy route.
- All fourteen Foundation table-management pages receive meaningful runtime display adjustment without creating a low-code
  runtime or weakening backend authority.
- Sensitive page families can participate through narrow list presentation while their privileged
  subflows remain compiled.
- New generated routes cannot ship without a classification and default-disabled adoption decision.
- One-page rollout and rollback constrain the blast radius.

### Cost

- Foundation must generalize the user-specific core-page source and binding mechanism.
- Fourteen Foundation built-in page identities require source, generation, registry, runtime, permission,
  and browser evidence.
- The `/log` route needs three independent definition identities even though the current UI uses one
  compiled Tabs component.
- Route closure and page acceptance add maintained machine contracts and CI work.

## Rejected alternatives

### Treat external Supplier and user management as all-page completion

Rejected because the registries and runtime consumers would still omit most shipped management pages.

### Infer definitions from React components at runtime

Rejected because it creates unstable identities, executable discovery, dual authority, and an
unreviewable security boundary.

### Give every built-in page the external Supplier full-surface contract

Rejected because role authorization, menu API binding, task execution, configuration content, session
revocation, and log internals are not routine display choices.

### Activate all registered pages at once

Rejected because it increases failure blast radius, obscures page-specific regressions, and weakens
recovery.

### Configure the governance console itself

Rejected because the recovery and publication control plane must remain available independently of the
optional feature it controls.

## Next executable step

Submit this accepted contract and its machine-state synchronization through the normal pull-request path.
Keep the historical disabled, shadow, fallback, and recovery tests as regression gates. Any later Head,
definition, route, surface, or allowlist change must rerun its affected exact-Head checks before preserving
`acceptanceState=passed`. Supplier remains outside Foundation rollout as optional business-extension
evidence in the reference Admin or a downstream host.
