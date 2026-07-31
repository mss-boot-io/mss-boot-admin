---
title: SECURITY Policy FAQ
order: 13
nav:
  order: 5
  title: 'devops'
description: How to report suspected vulnerabilities in mss-boot-io without exposing sensitive details in public issues
keywords: [security, vulnerability, advisory, disclosure, faq]
---

# SECURITY Policy FAQ

This page explains how contributors should handle suspected security issues in
`mss-boot-io` projects. It is written for public open source collaboration:
enough detail to help maintainers reproduce a problem, without exposing exploit
steps, credentials, or private deployment information in public issues.

## Should I open a public issue for a vulnerability?

No. Do not post suspected vulnerabilities, exploit details, private logs,
tokens, passwords, database URLs, or production endpoints in public issues,
pull requests, Discussions, or comments.

Use the security policy of the affected repository instead:

| Repository            | Security policy                                                                   |
| --------------------- | --------------------------------------------------------------------------------- |
| `mss-boot`            | [SECURITY.md](https://github.com/mss-boot-io/mss-boot/security/policy)            |
| `mss-boot-admin`      | [SECURITY.md](https://github.com/mss-boot-io/mss-boot-admin/security/policy)      |
| `mss-boot-admin-antd` | [SECURITY.md](https://github.com/mss-boot-io/mss-boot-admin-antd/security/policy) |
| `mss-boot-docs`       | [SECURITY.md](https://github.com/mss-boot-io/mss-boot-docs/security/policy)       |

When GitHub private vulnerability reporting is available on the affected
repository, prefer that path so maintainers can triage the report privately.

## What should a useful report include?

Please include only information that is safe to share privately with maintainers:

- Affected repository and component.
- Affected version, release tag, commit SHA, or deployment date.
- Minimal reproduction steps.
- Expected behavior and observed behavior.
- Security impact and who can trigger it.
- Relevant logs, screenshots, or request examples with secrets removed.
- Whether the issue is reproducible in a local or beta environment.
- Suggested mitigation if you already have one.

Avoid sending full production logs, real user data, real tokens, private keys,
database connection strings, or Cloudflare/Kubernetes credentials.

## What happens after a private report?

Maintainers should first confirm the scope and severity, then decide whether the
fix belongs in one repository or needs a coordinated change across backend,
frontend, docs, or deployment configuration.

Expected flow:

1. A maintainer acknowledges the private report.
2. The team reproduces the issue with sanitized data.
3. A fix is prepared in a private advisory or a public PR without sensitive
   exploit details.
4. The fix passes CI and relevant smoke tests.
5. The advisory or release note is published after the safe fix is available.

## What belongs in public issues instead?

Public issues are still the right place for non-sensitive work:

- Documentation gaps.
- Feature requests.
- Reproducible non-security bugs.
- Local setup failures without secrets.
- Permission or login questions that do not expose real credentials.

If you are not sure whether a problem is security-sensitive, use the private
security path first. Maintainers can route it back to a public issue when it is
safe.

## Related Docs

- [Open Source Governance](/devops/open-source-governance)
- [Community Operations Playbook](/devops/community-operations-playbook)
- [Security Baseline Guide](/admin/security-baseline)
