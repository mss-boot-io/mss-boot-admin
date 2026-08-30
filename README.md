# mss-boot Complete Admin Distribution

[简体中文](./README.zh-CN.md)

mss-boot is an agent-native management-system foundation. **v1.3.7 is the
selected complete package-first release candidate, but it is not stable or
adoptable.** Candidate surfaces may be at different public stages; use the
remote release ledger as authority. Until stable promotion and the final
policy/Docs reconciliation complete, do not install, create, or upgrade with
v1.3.7. v1.3.5 and v1.3.6 are immutable partial trains. No identity from either
train may be deleted, moved, recreated, reused, or completed.

## Current availability

The release policy still identifies **v1.3.2** as the current coordinated
stable distribution. Its immutable release record remains the supported
baseline for existing adopters.

The stopped identity namespaces remain explicit audit evidence:

| Train | Framework | Admin | Official npm identity | Docs identity |
| --- | --- | --- | --- | --- |
| v1.3.5 | `github.com/mss-boot-io/mss-boot-admin/mss-boot@v1.3.5` | `github.com/mss-boot-io/mss-boot-admin/admin@v1.3.5` | `@mss-boot-io/admin-web@1.3.5` | `docs/v1.3.5` |
| v1.3.6 | `github.com/mss-boot-io/mss-boot-admin/mss-boot@v1.3.6` | `github.com/mss-boot-io/mss-boot-admin/admin@v1.3.6` | `@mss-boot-io/admin-web@1.3.6` | `docs/v1.3.6` |

Some Go and GitHub identities in those rows exist while the npm or Docs
identity may be absent. A named identity is not a claim that it was published.

v1.3.6 published only part of the intended train from exact commit
`b1fe47a3a83209574e09d53526b122dd2cbc5277`:

| Surface | v1.3.6 result | Availability |
| --- | --- | --- |
| Framework | `mss-boot/v1.3.6` | Public Go component and GitHub Release |
| Admin | `admin/v1.3.6` | Public Go component and GitHub Release |
| Admin Web | `web/antd-v6/v1.3.6` | GitHub Release and GitHub Packages; official npmjs is absent |
| Root | `v1.3.6` | Public Root Release and tools; Root image is absent |
| Docs | planned `docs/v1.3.6` | Not published |

These component identities are immutable but do not form a complete Thin Host
distribution. Do not combine them with v1.3.2, a source checkout, a local
replacement, or an unpublished package.

v1.3.7 is the new candidate. It must first pass one non-publishing preview from
an exact repaired merged-main commit, including a real Root OCI artifact and
the exact credentialless npm Trusted Publisher binding for `npm-release.yml`
and `npm-auto`. Its candidate surfaces then publish in governed stages and may
not all be public at the same time. Until stable promotion and the final
policy/Docs reconciliation complete, no v1.3.7 download, install, creation, or
upgrade procedure is supported.

## Adopter status

There is no supported v1.3.5 or v1.3.6 installer, empty-directory application
creation, local setup, or distribution-upgrade procedure. v1.3.7 is now the
selected candidate for those package-first interfaces, but current onboarding continues
to withhold executable commands until stable promotion, external Thin Host
acceptance, and the final policy/Docs reconciliation have completed.

Use the [v1.3.2 stable record](./docs/docs/releases/archive/v1-3-2.md)
for the current stable boundary and the
[v1.3.5 partial-release record](./docs/docs/releases/v1-3-5.md) and
[v1.3.6 partial-release record](./docs/docs/releases/v1-3-6.md) for immutable
audit evidence. The [v1.3.7 candidate record](./docs/docs/releases/v1-3-7.md)
describes the recovery, migration, security, and rollback boundary without
claiming publication.

## Architecture boundary

The v1.3.7 candidate keeps a generated application as a **Thin Host**: it pins
one coordinated Admin Go module and Admin Web package, contains
only composition glue and business-owned modules, and never copies Foundation
core source. Business backend modules register at compile time, frontend
business routes extend the packaged shell, and backend authorization remains
authoritative. This candidate architecture contract does not make v1.3.7
adoptable before the complete stable-promotion reconciliation and does not
complete v1.3.5 or v1.3.6.

## Documentation

The repository separates human guidance from executable Agent authority:

| Audience | Start here |
| --- | --- |
| Adopters, operators, and contributors | README files and `docs/docs/**` |
| Architecture maintainers | `docs/adr/**` |
| Foundation AI Agents | nearest `AGENTS.md` -> `.mss/**` -> applicable `.agents/skills/**` |
| Generated Thin Host AI Agents | the generated repository's `AGENTS.md`, `.mss/**`, and local Skills |

The public [Agent collaboration guide](./docs/docs/agent/index.md) explains this
model for humans; it is not an executable instruction source and does not merge
Foundation-maintainer Skills into Thin Host capabilities.

- [Adopter and component status](./docs/docs/getting-started/index.md)
- [Published components and import boundaries](./docs/docs/getting-started/packages.md)
- [Tool publication status](./docs/docs/getting-started/tooling.md)
- [mss-shop reference status](./docs/docs/getting-started/mss-shop.md)
- [v1.3.7 release-candidate record](./docs/docs/releases/v1-3-7.md)
- [v1.3.6 immutable partial-release record](./docs/docs/releases/v1-3-6.md)
- [v1.3.5 immutable partial-release record](./docs/docs/releases/v1-3-5.md)

Foundation contributors should use
[`CONTRIBUTING.md`](./docs/CONTRIBUTING.md) and the nearest `AGENTS.md`;
source-checkout commands are
deliberately kept out of adopter onboarding.

## License and security

Licensed under the [MIT License](./LICENSE). Report security issues through the
private process in [`SECURITY.md`](./SECURITY.md), not a public issue.
