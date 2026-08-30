# Documentation agent instructions

## Scope

This file applies to the Dumi site under `docs/` and inherits the repository-wide contract from the root `AGENTS.md`.

The documentation site is no longer a separate `mss-boot-docs` checkout. Use repository-relative links and paths throughout.

## Information placement

- Public human-facing product, operations, contributor, release, and Agent-collaboration guidance: `docs/docs/`.
- `docs/docs/agent/` remains human-facing: it explains how people work with Agents and links to authoritative repository contracts. It must not become a second executable instruction tree.
- Public architecture overviews: `docs/docs/architecture/`.
- Maintainer architecture decisions: `docs/adr/`; ADRs are repository decisions rather than adopter tutorials.
- User-facing module documentation generated from a module spec: `docs/docs/modules/`.
- Foundation Agent authority: the nearest `AGENTS.md`, then `.mss/` and the applicable `.agents/skills/` workflow.
- Generated Thin Host Agent authority: the generated repository's `AGENTS.md`, `.mss/`, and local `.agents/skills/`. Do not present Foundation-maintainer Skills as downstream capabilities.
- Machine-executable facts: `.mss/`, not prose only. Public pages may summarize and link to these contracts but must not copy release or command logic into a competing source of truth.
- Historical prompts and old handoffs: archive locations; they are not automatically current requirements.

When a prose statement duplicates a machine-readable fact, link to the contract and keep the two synchronized.

## Canonical commands

From the repository root:

```shell
make docs-install
make docs-build
go run ./cmd/mss verify --changed
```

From `docs/`:

```shell
pnpm install --frozen-lockfile
pnpm build
pnpm start
```

## Authoring requirements

- State the applicable version, branch, or commit when a document evaluates current behavior.
- Use concrete repository paths, API paths, commands, and validation evidence.
- Separate implemented behavior, approved roadmap, examples, and speculation.
- Do not copy production credentials, private endpoints, personal absolute paths, or unredacted sensitive logs.
- Update both Chinese and English material when both variants are part of the same public contract.
- Keep generated module documents sourced from `modules/<module>/module.yaml`; do not hand-edit generated text.
- Keep the top navigation bounded to stable, high-frequency journeys. Add new or specialist sections under the `更多` group in `.dumirc.ts` by default; promote an item only when it replaces an existing primary journey.
- Reserve site-wide cosmetic sweeps and cross-page visual QA for release-preparation pull requests. Ordinary documentation changes should add the required content without opportunistic theme restyling.

## Cross-component changes

A docs-only change does not require starting the backend or frontend. When documentation accompanies code, follow the nearest child `AGENTS.md` for those changed directories and run the corresponding focused checks.

Pull requests that change only `docs/**` must keep unrelated required contexts as lightweight sentinels and run heavy work only in the Docs workflow. Main-branch Docs pushes likewise must not start Admin, Framework, frontend, Go vulnerability, CodeQL, or repository-mirror work. Changes to shared release tooling or workflow files intentionally fall back to full shared validation.

Architecture and roadmap documents must identify a next executable step and a measurable completion definition; avoid vague “continue improving” conclusions.
