# AGENTS

This repository is the core Go library for `mss-boot-io`. Agents should keep
changes small, verifiable, and understandable to outside contributors.

## Repository Role

- `mss-boot`: shared Go framework and infrastructure primitives.
- `mss-boot-admin`: admin backend built on top of this framework.
- `mss-boot-admin-antd`: admin frontend.
- `mss-boot-docs`: public documentation and organization-level AI memory.

## Working Rules

- Prefer existing package patterns over new abstractions.
- Do not change public APIs, config formats, or persistence behavior without
  updating docs or `aigc` memory.
- Do not store prompts or decision notes in the repository root.
- Put repository-local AI memory under `aigc/prompts/`.
- Use lowercase kebab-case filenames; use `.zh-CN.md` for Chinese memory files.
- Never commit secrets, private endpoints, tokens, or local credentials.

## Validation

Use the narrowest command that proves the change, then broaden when shared
behavior is touched:

- Go tests: `go test ./...`
- Vulnerability scan: `go run golang.org/x/vuln/cmd/govulncheck@latest ./...`
- Workflow lint: `go run github.com/rhysd/actionlint/cmd/actionlint@latest`
- Whitespace check: `git diff --check`

## Pull Request Expectations

Every PR should explain:

- tests impact;
- docs impact;
- security impact;
- release, compatibility, migration, or rollback impact.

Workflow, deployment, API contract, config, migration, or security changes
should include docs, README, changelog, or `aigc` memory updates.

## Release And Compatibility

- Keep Dependabot updates reviewable and reversible.
- Do not allow dependency updates to silently change major interface families.
- When a dependency touches auth, policy, storage, config, or transport layers,
  document the compatibility and rollback path.
