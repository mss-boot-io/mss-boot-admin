# Admin application contract

This directory is the deployable reference Admin application and an independent Go module.

- Module: `github.com/mss-boot-io/mss-boot-admin/admin`
- Framework dependency: sibling `mss-boot/` module
- Run development commands from `admin/` so runtime `config/` paths remain stable.
- Do not place Agent/Foundation tooling in this module.
- Admin changes require independent tests with `GOWORK=off`, a workspace compatibility test against the current local framework, coverage verification, vet, tidy, and binary smoke tests.
- Public HTTP routes and database schema compatibility must be preserved unless a migration and explicit release note are included.
