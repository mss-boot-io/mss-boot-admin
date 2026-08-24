# mss-boot Complete Admin Distribution

[简体中文](./README.zh-CN.md)

mss-boot is an agent-native management-system foundation. The coordinated
**v1.3.3** distribution is consumed as released tools and packages: downstream
applications do not clone or copy this repository.

## What v1.3.3 ships

| Surface | Released identity | Purpose |
| --- | --- | --- |
| Agent tools | `mss`, `mss-mcp` from the `v1.3.3` GitHub Release | Create, inspect, develop, verify, and upgrade Thin Hosts |
| Framework | `github.com/mss-boot-io/mss-boot-admin/mss-boot@v1.3.3` | Domain-neutral Go infrastructure |
| Admin | `github.com/mss-boot-io/mss-boot-admin/admin@v1.3.3` | Complete importable Admin backend |
| Admin Web | `@mss-boot-io/admin-web@1.3.3` | Complete React 19 and Ant Design 6 frontend |

Every component is qualified from one exact commit already merged into
`main`. A version is usable only after its public release and package
reconciliation are complete.

## Quick start

On Linux or macOS:

```sh
curl -fsSLO https://github.com/mss-boot-io/mss-boot-admin/releases/download/v1.3.3/install-mss.sh
bash ./install-mss.sh --version v1.3.3 --install-dir "$HOME/.local/bin"
export PATH="$HOME/.local/bin:$PATH"
mss --version
mss-mcp --version
```

On Windows PowerShell:

```powershell
Invoke-WebRequest https://github.com/mss-boot-io/mss-boot-admin/releases/download/v1.3.3/install-mss.ps1 -OutFile install-mss.ps1
& .\install-mss.ps1 -Version v1.3.3 -InstallDir "$HOME\.local\bin"
$env:Path = "$HOME\.local\bin;$env:Path"
mss --version
mss-mcp --version
```

Create a complete Thin Host from an empty directory:

```sh
mss new app orders-admin --module github.com/acme/orders-admin --destination ./orders-admin --write --git-init
cd orders-admin
mss doctor --strict
mss setup
mss dev --detach
mss dev status
mss verify --changed
```

On an interactive terminal, the first `mss setup` securely prompts for the
initial administrator password with hidden input. It must be 8-128 characters
and contain a letter and a number. Non-interactive automation injects the
one-use `MSS_ADMIN_INITIAL_PASSWORD` from its secret store for the setup process
only. After the first migration succeeds, repeated setup runs do not require it.

Open `http://127.0.0.1:8001` and sign in as `admin` with the password supplied
during that first setup. There is no default password.

The installer verifies `SHA256SUMS.tools-v1.3.3`, never requires `sudo`,
and does not edit shell profiles. See the
[package-first quick start](https://docs.mss-boot-io.top/getting-started) for
prerequisites, Windows PATH handling, upgrade commands, and troubleshooting.

## Architecture boundary

A generated application is a **Thin Host**. It pins the exact Admin Go module
and Admin Web package, contains composition glue and business-owned modules,
and never copies Foundation core source. Business backend modules register at
compile time; frontend business routes extend the packaged shell. Backend
authorization remains authoritative.

Install the target-version tools, back up the application and database, and
confirm `.mss/blueprint-manifest.json` exists before running
`mss upgrade admin v1.3.3`. Add `--apply --yes` only after reviewing a
conflict-free plan, then run `mss doctor --strict`, `mss verify --all`, and a
second plan that must be empty. A hand-assembled or manifest-less repository
must migrate business-owned files into a newly generated baseline instead of
fabricating upgrade state. No Foundation checkout is required.

## Documentation

- [Quick start](https://docs.mss-boot-io.top/getting-started)
- [Packages and import boundaries](https://docs.mss-boot-io.top/getting-started/packages)
- [Tooling](https://docs.mss-boot-io.top/getting-started/tooling)
- [mss-shop reference application](https://docs.mss-boot-io.top/getting-started/mss-shop)
- [v1.3.3 release contract](https://docs.mss-boot-io.top/releases/v1-3-3)

Foundation contributors should use
[`CONTRIBUTING.md`](./docs/CONTRIBUTING.md); source-checkout commands are
deliberately kept out of adopter onboarding.

## License and security

Licensed under the [MIT License](./LICENSE). Report security issues through the
private process in [`SECURITY.md`](./SECURITY.md), not a public issue.
