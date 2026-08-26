# Contributing to mss-boot Admin documentation

## Scope

The documentation site lives in `docs/` of the
[`mss-boot-io/mss-boot-admin`](https://github.com/mss-boot-io/mss-boot-admin)
repository. It is an independently publishable component, not a separate source
repository. Product prose, implementation, tests, and `.mss/` machine contracts
must describe the same behavior.

Use these locations consistently:

- long-lived product and operator guidance: `docs/docs/`;
- architecture decisions: `docs/adr/`;
- executable project, release, capability, and module facts: `.mss/`.

Prompt dumps and one-off handoffs are not documentation inputs. Do not restore
them to the active tree or rewrite an archived release record to describe a
newer train.

## Pull requests

- Target `main` through a pull request; do not publish Docs from a topic branch.
- Use a Conventional Commits title, for example
  `docs(release): document v1.3.5 upgrade`.
- State the applicable version, branch, or commit for current-behavior claims.
- Use repository-relative source paths and links. Public GitHub links must point
  to `mss-boot-io/mss-boot-admin` unless the target is genuinely external.
- Keep Chinese and English descriptions synchronized when they form one public
  contract.
- Never include credentials, private endpoints, personal absolute paths, or
  unredacted logs.

## Validation

From the repository root:

```bash
corepack pnpm@9.15.9 --dir docs install --frozen-lockfile
python3 tools/docs/check_current_docs.py
corepack pnpm@9.15.9 --dir docs build
go run ./cmd/mss verify --changed
```

For release-preparation changes, also check the rendered homepage, Admin entry,
release entry, navigation, dark theme, and narrow layout in the Codex in-app
browser. Record only checks that actually ran.

## Review checklist

- Commands, paths, package names, versions, and release states match code and
  `.mss/` contracts.
- A future target is not described as already published.
- Existing public tags and historical failure records remain immutable.
- Internal links build and the page remains readable on desktop and narrow
  viewports.
- The change identifies its next executable step when work is not complete.
