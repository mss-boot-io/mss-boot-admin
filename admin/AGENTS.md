# Admin application contract

This directory is the deployable reference Admin application and an independent Go module.

- Module: `github.com/mss-boot-io/mss-boot-admin/admin`
- Framework dependency: sibling `mss-boot/` module
- `app/` is the complete importable application entrypoint; official and Thin
  Host binaries must compose through it instead of copying startup logic.
- `business/` is the minimal compile-time extension boundary. Business modules
  register explicitly, must not replace core auth/session/security middleware,
  and mount only below the composition root's protected API group after all
  migration readiness checks pass.
- Generated module registries return an ordered `[]business.Module`; generated
  modules must not depend on package `init()` side effects.
- Run development commands from `admin/` so runtime `config/` paths remain stable.
- Do not place Agent/Foundation tooling in this module.
- Before a coordinated Framework version is published, Admin pull requests use
  the repository `go.work` plus the repository-external consumer test with
  confined temporary replacements. After the exact Framework tag and GitHub
  Release exist, the protected Admin release must resolve and qualify that
  public dependency with `GOWORK=off` and no committed `replace`. Admin changes
  still require coverage verification, vet, tidy, and binary smoke tests.
- Public HTTP routes and database schema compatibility must be preserved unless a migration and explicit release note are included.
