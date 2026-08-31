# mss-boot Complete Admin Distribution

[简体中文](./README.zh-CN.md)

mss-boot is an agent-native management-system foundation. **v1.3.7 is the
current stable and adoptable Complete Admin Distribution.** Every component was
qualified and published from merged `main` commit
`77b53d41092741eac62fa6418c0bdbf87413c7cd`; generated Thin Hosts pin this
exact coordinated version instead of combining independent component versions.

The public Docs website is released independently and may follow the component
train. A `docs/v*` tag identifies only a website deployment: missing, failed,
or temporarily stale Docs deployment never blocks Framework, Admin, frontend,
tool, image, npm, or stable-alias publication. Checked-in human and Agent
guidance remains versioned with the source contracts.

## Stable distribution identities

| Surface | v1.3.7 identity |
| --- | --- |
| Root tools, backend image, and GitHub Release | `v1.3.7` |
| Reusable Go Framework | `mss-boot/v1.3.7` |
| Importable Admin Go module | `admin/v1.3.7` |
| Complete Admin Web | `web/antd-v6/v1.3.7` / `@mss-boot-io/admin-web@1.3.7` |
| Documentation website | independently deployed from a versioned `docs/v*` website tag |

v1.3.5 and v1.3.6 remain immutable partial release trains. Their existing
tags, Releases, and artifacts are audit evidence, not compatible substitutes
for the stable train. Do not mix stopped-train components with a source
checkout, a local `replace`, or an absent package identity.

## Install the v1.3.7 tools

The versioned installers verify release checksums, install both `mss` and
`mss-mcp`, do not use `sudo`, and do not modify a shell profile.

Linux or macOS:

```shell
curl -fL -o install-mss.sh \
  https://github.com/mss-boot-io/mss-boot-admin/releases/download/v1.3.7/install-mss.sh
bash ./install-mss.sh --version v1.3.7
export PATH="$HOME/.local/bin:$PATH"
mss --version
mss-mcp --version
```

Windows PowerShell:

```powershell
Invoke-WebRequest `
  https://github.com/mss-boot-io/mss-boot-admin/releases/download/v1.3.7/install-mss.ps1 `
  -OutFile install-mss.ps1
& .\install-mss.ps1 -Version v1.3.7
$env:Path = "$env:LOCALAPPDATA\Programs\mss\bin;$env:Path"
mss --version
mss-mcp --version
```

## Consume the stable packages

Pin the exact coordinated Go and npm versions. The Admin module contains no
committed local `replace`, and the official npmjs package installs without a
registry token. Run the Go commands from the root of an **existing external
consumer module** (a directory containing `go.mod`); for a clean public-resolution
check from an empty directory, use the [external consumer procedure](./docs/docs/getting-started/packages.md).
Run the npm command separately from an **existing frontend package root** (the
directory containing its `package.json`, normally `web/` in a Thin Host).

Go module root:

```shell
go get github.com/mss-boot-io/mss-boot-admin/mss-boot@v1.3.7
go get github.com/mss-boot-io/mss-boot-admin/admin@v1.3.7
```

Frontend package root:

```shell
corepack pnpm@10.34.5 add --save-exact @mss-boot-io/admin-web@1.3.7
```

The npm `latest` tag currently resolves to `1.3.7`, but Thin Hosts still pin
the exact version. Official npm publication uses GitHub Actions Trusted
Publishing; no stored npm token is part of the release contract.

## Create and start a Thin Host

Run the released tool from an empty destination parent, not from a Foundation
source checkout:

```shell
mss new app orders-admin --module github.com/acme/orders-admin --repository acme/orders-admin --destination ./orders-admin --write --git-init

cd orders-admin
mss doctor --strict
mss setup
mss dev --detach
```

Interactive first setup asks for the initial administrator password with
hidden input. Non-interactive automation may inject
`MSS_ADMIN_INITIAL_PASSWORD` from a secret store for the `mss setup` process
only. It must not appear in arguments, reports, generated files, dependency
installation, or the long-running service environment. After initialization,
open `http://127.0.0.1:8001` and sign in as `admin`; there is no default
password.

## Upgrade a Thin Host

Back up code, configuration, and the database; install the target v1.3.7 tools;
and verify both binaries before planning. A supported three-way upgrade requires
the generated `.mss/blueprint-manifest.json`:

```shell
mss --version
mss-mcp --version
mss upgrade admin v1.3.7 --format json
mss upgrade admin v1.3.7 --apply --yes --format json
mss doctor --strict
mss verify --all
mss upgrade admin v1.3.7 --format json
```

The first and final commands are read-only; the final plan must contain no
create, update, delete, or conflict operations. The upgrade engine changes only
Blueprint-managed files and preserves business-owned and unknown files. A
hand-assembled repository or one missing its manifest must generate a clean
v1.3.7 baseline in a new directory and migrate business-owned specifications
and files instead of fabricating a manifest.

## Architecture boundary

A generated application is a **Thin Host**. It imports the complete Admin Go
module and Admin Web package, owns only composition glue and business modules,
and does not copy Foundation core source. Backend modules register at compile
time, frontend business routes extend the packaged shell, and backend
authorization remains authoritative.

Runtime dynamic models, virtual CRUD, and browser-facing code generation remain
removed. Structured Feature and AdminModule specifications drive deterministic
development-time generation and reviewable migrations, permissions, menus,
frontend code, tests, and Agent evaluation contracts.

## Human and Agent documentation

The repository deliberately separates explanatory documentation from executable
Agent authority:

| Audience | Start here |
| --- | --- |
| Adopters, operators, and contributors | README files and `docs/docs/**` |
| Architecture maintainers | `docs/adr/**` |
| Foundation AI Agents | nearest `AGENTS.md` -> `.mss/**` -> applicable `.agents/skills/**` |
| Generated Thin Host AI Agents | the generated repository's `AGENTS.md`, `.mss/**`, and local Skills |

The public [Agent collaboration guide](./docs/docs/agent/index.md) explains the
model for humans; it is not an executable instruction source. Start with the
[quick start](./docs/docs/getting-started/index.md),
[package boundary](./docs/docs/getting-started/packages.md),
[tooling contract](./docs/docs/getting-started/tooling.md), and
[v1.3.7 release record](./docs/docs/releases/v1-3-7.md).

Foundation contributors should use
[`CONTRIBUTING.md`](./CONTRIBUTING.md) and the nearest `AGENTS.md` for
source-checkout commands and validation.

## License and security

Licensed under the [MIT License](./LICENSE). Report security issues through the
private process in [`SECURITY.md`](./SECURITY.md), not a public issue.
