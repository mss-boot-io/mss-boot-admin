# mss-boot Complete Admin Distribution

[简体中文](./README.zh-CN.md)

mss-boot is an agent-native management-system foundation. **v1.3.5 is an
immutable partial train, not a complete adopter distribution.** Framework,
Admin, and Admin Web were published and the Root tag is public, but the Root
Release and tools, Docs, and the public npmjs package were not published. Do
not delete, move, recreate, or reuse any v1.3.5 identity.

## Current availability

The release policy still identifies **v1.3.2** as the current coordinated
stable distribution. Its immutable release record remains the supported
baseline for existing adopters.

v1.3.5 published only part of the intended train from commit
`396f60615cdfa589353b16ef9d3531e249e65432`:

| Surface | Public v1.3.5 identity | Availability |
| --- | --- | --- |
| Framework | `github.com/mss-boot-io/mss-boot-admin/mss-boot@v1.3.5` | Public Go component |
| Admin | `github.com/mss-boot-io/mss-boot-admin/admin@v1.3.5` | Public Go component |
| Admin Web | `web/antd-v6/v1.3.5` and `@mss-boot-io/admin-web@1.3.5` release assets | GitHub Release and GitHub Packages only; npmjs is absent |
| Root | annotated `v1.3.5` tag | Tag only; no Root Release, tools, or Root image |
| Docs | planned `docs/v1.3.5` | Not published |

These component identities are immutable but do not form a complete Thin Host
distribution. Do not combine them with v1.3.2, a source checkout, a local
replacement, or an unpublished package.

## Adopter status

There is no supported v1.3.5 installer, empty-directory application creation,
local setup, or distribution-upgrade procedure. The original candidate
interfaces are intentionally absent from current onboarding so they cannot be
mistaken for downloadable commands. The next unused distribution version has
not been selected.

Use the [v1.3.2 stable record](./docs/docs/releases/archive/v1-3-2.md)
for the current stable boundary and the
[v1.3.5 partial-release record](./docs/docs/releases/v1-3-5.md)
for immutable audit evidence.

## Architecture boundary

A complete future distribution will keep a generated application as a **Thin
Host**: it pins one coordinated Admin Go module and Admin Web package, contains
only composition glue and business-owned modules, and never copies Foundation
core source. Business backend modules register at compile time, frontend
business routes extend the packaged shell, and backend authorization remains
authoritative. This architecture contract does not make the incomplete v1.3.5
train adoptable.

## Documentation

- [Adopter and component status](./docs/docs/getting-started/index.md)
- [Published components and import boundaries](./docs/docs/getting-started/packages.md)
- [Tool publication status](./docs/docs/getting-started/tooling.md)
- [mss-shop reference status](./docs/docs/getting-started/mss-shop.md)
- [v1.3.5 immutable partial-release record](./docs/docs/releases/v1-3-5.md)

Foundation contributors should use
[`CONTRIBUTING.md`](./docs/CONTRIBUTING.md); source-checkout commands are
deliberately kept out of adopter onboarding.

## License and security

Licensed under the [MIT License](./LICENSE). Report security issues through the
private process in [`SECURITY.md`](./SECURITY.md), not a public issue.
