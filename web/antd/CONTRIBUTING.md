# Contributing to mss-boot-admin-antd

## Issues

Use the issue templates and include browser, environment, commit, reproduction
steps, expected behavior, actual behavior, and screenshots when useful. Do not
report vulnerabilities in public issues.

## Pull requests

- Use a Conventional Commits title, for example `fix(login): handle expired token`.
- Keep UI, API contract, and docs changes scoped.
- Run `pnpm install --frozen-lockfile`, `pnpm tsc`, `pnpm test`, and the relevant
  build before requesting review.
- Do not publish to Cloudflare frequently. Finish local development and smoke
  testing first; beta/prod deployments are public-facing release actions.
- Update README, docs, release notes, or `mss-boot-docs/aigc/` when behavior,
  release policy, or AI workflow changes.

AI-assisted changes are welcome when the generated output is reviewed and
verified by the contributor.
