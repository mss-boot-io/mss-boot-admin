# Admin Distribution extension-contract hardening

Date: 2026-08-22
Status: Proposed

## Context

The Complete Admin Distribution exposes two narrow downstream extension points:
compile-time Go business modules and frontend business routes. Three public
surfaces allowed callers to bypass the intended fail-closed boundary:

1. a failed `Module.Register` call could leave migrations in the shared business
   runner before the module itself was rejected;
2. `Application.Command` returned a fully executable Cobra tree outside the
   single-use and process-global lifecycle guards;
3. the Admin Web `routes` option could replace core routes, collision checks, and
   the final 403/404 fallbacks instead of extending them.

## Decision

### Atomic Go module registration

Each `Registry.Add` call owns a transaction-local migration runner and staged
runtime entry. `Registry.Register` writes only to that transaction. The registry
combines and validates staged migrations against committed Business and Core
migrations before publishing the module identity, descriptor, routes, and
migration set together. Any returned, ignored, partial, or post-registration
error discards the complete transaction.

Core and Business migration phases remain separate and continue to execute in
Core-then-Business order. The combined runner remains a collision-checked
compatibility view.

### Guarded Admin execution only

The importable Admin no longer exposes a public executable command tree. Public
hosts use `ExecuteContext` or `ExecuteArgsContext`; both retain the one-runtime
and one-execution guards. Cobra remains an internal implementation detail of the
Admin composition root.

### Immutable Admin Web route foundation

`defineBusinessAdmin` always creates the complete Admin route set internally and
accepts only `businessRoutes` for extension. A runtime-supplied `routes` property
is rejected, including inherited JavaScript properties, and the TypeScript
public options no longer declare it. Business routes still pass through core
route-collision checks and remain before the fail-closed fallbacks.

The official frontend uses the same `businessRoutes` contract as generated Thin
Hosts; the redundant local full-route composition file is removed.

## Compatibility

This intentionally removes the unsafe Go `Application.Command` API and the
unsafe Admin Web `routes` option. Consumers using the documented Thin Host
contract are unaffected because generated hosts already use `ExecuteContext`
and `businessRoutes`. Callers using either escape hatch must migrate to those
canonical entrypoints.

## Verification

Regression coverage must prove:

- partial migration registration and post-registration module errors leave no
  descriptor, identity, or migration and allow exact identity/ID reuse;
- the public `Application` method set has no `Command` method while guarded help,
  single-use, and concurrent-runtime behavior still work;
- Admin Web rejects full-route overrides at runtime and in its type contract,
  while business routes preserve core routes and final fallbacks;
- external Go and Thin Host consumers continue to test, vet, build, and execute
  from clean dependency graphs.
