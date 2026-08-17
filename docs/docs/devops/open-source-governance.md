---
title: Open Source Governance
order: 10
nav:
  order: 5
  title: "devops"
description: mss-boot-io open source governance, quality gates, release policy, and AI memory rules
keywords: [open source, governance, quality gates, release, AI memory]
---

# Open Source Governance

`mss-boot-io` uses a governance-first open source workflow. The goal is to keep
the project readable by contributors, maintainers, and AI agents.

## Quality Gates

- Pull requests must include tests, documentation impact, security impact, and
  release impact.
- Go repositories run CI, CodeQL, govulncheck, Dependabot, and OpenSSF
  Scorecard.
- Frontend and docs repositories run CI, CodeQL, Dependabot, and OpenSSF
  Scorecard.
- API, config, deployment, or workflow changes should update documentation or AI
  memory.

## Release Policy

- Alpha/dev is for fast integration.
- Beta is public preview and must pass browser and delivery smoke testing before
  V6 artifact publication.
- Production uses tagged backend releases and verified frontend deployments.

## Security

Do not report vulnerabilities as public issues. Follow the security policy of
the affected repository and use private GitHub Security Advisories when enabled.

## AI Memory

Organization-level workflow, release policy, and cross-repository decisions must
be recorded in `mss-boot-docs/aigc/prompts/`.

## GitHub-First Community Flow

External platforms can introduce the project, but they are not the decision
system. Bugs, deployment failures, architecture tradeoffs, documentation gaps,
and security concerns must be routed back to GitHub Issues, Discussions, docs,
or the security policy path.

Use Discussions for review and roadmap tradeoffs. Use Issues for actionable
work with clear reproduction or acceptance criteria. Use docs for stable
answers, FAQ entries, and release/process policy.

## Repository About And Topics

Repository descriptions and topics should match the current governance-first
direction:

- RBAC, configuration, API registry, operations, release policy, and AI-assisted
  maintenance are current public signals.
- Runtime virtual models, virtual CRUD, and browser-facing code generation have
  been removed and must not be presented as current capabilities. The separate
  development-time `cmd/mss` generator remains a supported, deterministic
  repository workflow.
- Homepage links should point to the docs site unless a repository has a more
  specific stable entry point.

## Reviewer And CODEOWNERS Boundary

Invite reviewers through Discussions and issues first. Add `CODEOWNERS` only for
real maintainers or confirmed community reviewers. Placeholder owners reduce
trust and create false expectations during PR review.
