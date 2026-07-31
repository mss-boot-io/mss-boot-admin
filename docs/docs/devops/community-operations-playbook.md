---
title: Community Operations Playbook
order: 12
nav:
  order: 5
  title: "devops"
description: GitHub-first community operations and external feedback routing for mss-boot-io
keywords: [community, open source, GitHub, feedback, operations]
---

# Community Operations Playbook

This playbook keeps `mss-boot-io` community work useful for contributors and
cheap for a personal maintainer. External platforms may help discovery, but
GitHub remains the source of truth for decisions and follow-up work.

## Principles

- Use GitHub Issues, Discussions, pull requests, releases, and docs as the
  durable record.
- Treat external posts as controlled outreach, not as launch spam.
- Ask for specific review, reproduction, docs, and test feedback instead of
  asking for stars.
- Keep claims evidence-based. Do not promise roadmap dates, production
  guarantees, or unverified maturity scores.
- Preserve account boundaries: passwords, phone verification, real-name checks,
  and sensitive account actions require the maintainer.

## Follow-Up Cadence

| Time | Action |
| --- | --- |
| T+24h | Check GitHub Discussions, Issues, Dev.to, X, Reddit, Juejin, and CSDN for real feedback. |
| T+48h | Convert actionable external feedback into Issues, Discussions, or docs tasks. |
| Weekly | Let Weekly Digest summarize repository activity, failed workflows, and follow-up candidates. |
| Monthly | Publish a short roadmap or release readiness digest when there is enough evidence. |

Avoid repeated Reddit or social reposts in the first week after an outreach
round. Reply only when there is a real question, correction, or maintainer
action to record.

## Feedback Routing

| Feedback | Destination | Required context |
| --- | --- | --- |
| Bug or first-run failure | GitHub Issue | versions, environment, command, log, reproduction steps |
| Architecture or roadmap tradeoff | GitHub Discussion | use case, constraints, alternatives considered |
| Documentation gap | Docs issue or docs PR | page, missing step, expected reader outcome |
| Small contributor task | `good first issue` | scope, non-goals, validation, acceptance criteria |
| Security concern | Security policy path | private details, affected version, reproduction if safe |

When feedback starts outside GitHub, include the source URL in the GitHub
follow-up unless the source contains private or sensitive information.

## Reviewer Participation

The most valuable community help is concrete review:

- run quickstart steps and report the first confusing command;
- review RBAC, Casbin, API registry, config, migration, and release policies;
- reproduce open issues and reduce them to minimal steps;
- add docs, tests, examples, or smoke-test notes;
- review PRs for behavior, readability, and release impact.

`CODEOWNERS` should only name real maintainers or confirmed reviewers. Do not add
placeholder teams just to look more mature.

## External Platform Boundaries

- If a platform removes a post for account age or community rules, respect the
  rule and keep GitHub as the canonical entry point.
- If a link becomes unavailable during review or moderation, mark it as
  non-canonical and do not rely on it in docs.
- Do not use maintainer credentials, phone verification, OTP codes, or real-name
  checks through an agent.
- Do not argue with moderators. Record the outcome and move the discussion back
  to GitHub.

## Response Templates

Bug or first-run failure:

```text
Thanks for the report. Could you add the Go/Node/pnpm versions, database type,
startup command, key logs, and reproduction steps? Actionable reports are tracked
in GitHub Issues so they can be tested and documented.
```

Architecture feedback:

```text
This is useful feedback. Could you add the tradeoff or alternative design to the
GitHub Discussion? That keeps the decision visible for contributors and future
release notes.
```

Self-promotion concern:

```text
Fair point. The goal is open-source feedback on governance, docs, and
maintainability, not promotion. If the post does not fit the community, we will
respect that and keep the canonical discussion on GitHub.
```

