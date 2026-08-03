# mss-boot-io AI Memory

This directory stores durable organization memory for AI agents and maintainers.
It is not a scratchpad. Important cross-repository decisions should be recorded
here so they survive local multi-git workspace changes and context resets.

## Entry Points

- [Organization memory](./prompts/organization-memory.zh-CN.md)
- [Release environment strategy](./prompts/release-environment-strategy.zh-CN.md)
- [Multi-agent delivery workflow](./prompts/multi-agent-delivery-workflow.zh-CN.md)
- [Open source AI-era growth plan](./prompts/open-source-ai-era-growth-plan.zh-CN.md)
- [Good first issue factory](./prompts/open-source-good-first-issue-factory.zh-CN.md)
- [Operations backlog](./prompts/open-source-operations-backlog.zh-CN.md)
- [CI stability incident](./prompts/ci-stability-incident-2026-06-05.zh-CN.md)
- [Open source ecosystem playbook memory](./prompts/open-source-ecosystem-playbook-2026-06-05.zh-CN.md)
- [mss-boot v0.7.2 and admin dependency follow-up](./prompts/mss-boot-v0.7.2-admin-dependency-update-2026-06-05.zh-CN.md)
- [Community governance sweep](./prompts/community-governance-sweep-2026-06-06.zh-CN.md)
- [Core repositories open source hardening](./prompts/core-repos-open-source-hardening-2026-06-07.zh-CN.md)
- [Community PR validation and merge sweep](./prompts/community-pr-validation-2026-06-07.zh-CN.md)
- [mss-boot-admin external issue upgrade](./prompts/admin-external-issue-upgrade-2026-06-07.zh-CN.md)
- [mss-boot-admin external issue dev validation](./prompts/admin-external-issue-dev-validation-2026-06-07.zh-CN.md)
- [External community publishing execution](./prompts/external-community-publishing-execution-2026-06-08.zh-CN.md)
- [Community publishing follow-up](./prompts/community-publishing-follow-up-2026-06-09.zh-CN.md)
- [GitHub governance baseline](./prompts/github-governance-baseline-2026-06-09.zh-CN.md)
- [Community triage round](./prompts/community-triage-round-2026-06-09.zh-CN.md)
- [GitHub security and community health](./prompts/github-security-community-health-2026-06-09.zh-CN.md)
- [Dependency security governance round](./prompts/dependency-security-governance-round-2026-06-09.zh-CN.md)
- [Docs dependency security convergence](./prompts/docs-dependency-security-convergence-2026-06-09.zh-CN.md)
- [Community issue and PR governance round](./prompts/community-issue-pr-governance-round-2026-06-09.zh-CN.md)
- [PR merge completion round](./prompts/pr-merge-completion-2026-06-09.zh-CN.md)
- [Domestic community publishing drafts](./prompts/domestic-community-publishing-drafts-2026-06-09.zh-CN.md)
- [Community ops heartbeat 2026-06-09 09:30Z](./prompts/community-ops-heartbeat-2026-06-09-0930z.zh-CN.md)
- [Community publishing and good-first split](./prompts/community-publishing-and-good-first-split-2026-06-09.zh-CN.md)
- [Release leader policy and v0.7.x status](./prompts/release-leader-policy-and-v0.7x-2026-06-10.zh-CN.md)

## Rules

- Record decisions that affect more than one repository.
- Keep environment and release policy changes in this docs repository first.
- Do not store secrets, tokens, private endpoints, or account credentials.
- Prefer concrete commands, file paths, validation evidence, and rollback notes.
- When a repository-level change is important, record a local `aigc` memory in
  that repository and summarize the organization-level decision here.

## Maintainer Workflow

1. Before a cross-repository change, read the relevant memory file.
2. During implementation, update repository-local `aigc` files when behavior or
   workflow changes.
3. After implementation, update this organization memory when the decision
   affects release flow, CI, governance, community operations, or AI workflows.
4. Link memory updates from PR bodies so PR Guard and Docs Drift can explain the
   change.
