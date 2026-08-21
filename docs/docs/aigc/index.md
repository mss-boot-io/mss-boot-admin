---
title: Historical AI handoffs
order: 1
nav:
  order: 6
  title: aigc
description: Historical prompts and handoffs retained inside the mss-boot-admin monorepo
keywords: [AI memory aigc agents governance history]
---

# Historical AI handoffs

<code>docs/aigc/</code> preserves prompts, organization notes, release incidents,
community operations, and agent handoffs imported into the
[`mss-boot-admin` monorepo](https://github.com/mss-boot-io/mss-boot-admin/tree/main/docs/aigc).
It is no longer a separate <code>mss-boot-docs</code> repository.

:::warning
These files are historical evidence. They may refer to retired repositories,
old branches, old versions, or workflows that no longer exist. Current code,
<code>.mss/</code> contracts, tests, architecture pages, and release pages take precedence.
:::

## Useful historical entry points

- [Organization memory](https://github.com/mss-boot-io/mss-boot-admin/blob/main/docs/aigc/prompts/organization-memory.zh-CN.md)
- [Release environment strategy](https://github.com/mss-boot-io/mss-boot-admin/blob/main/docs/aigc/prompts/release-environment-strategy.zh-CN.md)
- [Multi-agent delivery workflow](https://github.com/mss-boot-io/mss-boot-admin/blob/main/docs/aigc/prompts/multi-agent-delivery-workflow.zh-CN.md)
- [Complete Admin Distribution implementation prompt](https://github.com/mss-boot-io/mss-boot-admin/blob/main/docs/aigc/prompts/complete-admin-distribution-implementation-2026-08-19.zh-CN.md)
- [Open source operations backlog](https://github.com/mss-boot-io/mss-boot-admin/blob/main/docs/aigc/prompts/open-source-operations-backlog.zh-CN.md)
- [Role collaboration map](https://github.com/mss-boot-io/mss-boot-admin/blob/main/docs/aigc/prompts/roles/role-collaboration-map.zh-CN.md)

## Where new information belongs

- Durable product guidance: <code>docs/docs/</code>.
- Architecture decisions: <code>docs/adr/</code>.
- Executable project and release facts: <code>.mss/</code>.
- A one-time implementation prompt or handoff that must be retained for audit:
  <code>docs/aigc/prompts/</code>.

Never store credentials, production endpoints, private logs, or adopter telemetry
in any of these locations.
