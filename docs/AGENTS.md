# mss-boot-docs AGENTS

## Scope
- This file applies to the documentation site in `mss-boot-docs/`.
- Inherit workspace rules from the root `AGENTS.md`, especially the open-source wording constraints.

## Default interaction model
- Documentation and test handoff work also follow the repository's leader-first workflow by default.
- Use `aigc/prompts/roles/role-collaboration-map.zh-CN.md` when clarifying how leader should involve dev-test and return unified results to the user.

## Project shape
- This project is a Dumi-based static documentation site.
- Key locations:
  - `docs/`: documentation content.
  - `.dumirc.ts` and `.dumi/`: docs-site configuration.
  - `public/`: static assets.

## Authoring rules
- Follow the root `AGENTS.md` open-source collaboration rules by default.
- For evaluation or comparison writeups, include the evaluation time and baseline branch or commit when that reduces ambiguity.

## Boundary rules
- Do not assume backend or frontend startup workflows apply when editing docs only.
- If a docs task also changes sibling projects, follow the child project context for those directories.
- Keep docs-specific guidance here; backend architecture rules belong in `mss-boot-admin/AGENTS.md`.

## Working commands
- Install dependencies: `pnpm install`
- Start local docs server: `pnpm start`
- Build docs: `pnpm run build`
