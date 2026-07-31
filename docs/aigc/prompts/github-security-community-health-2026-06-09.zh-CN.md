# GitHub Security And Community Health 2026-06-09

## 背景

本轮目标是继续把 `mss-boot-io` 四个核心开源仓库推进到更稳定的
GitHub-first 治理状态，并把可由 agent 直接完成的社区健康事项落盘。

适用仓库：

- `mss-boot-io/mss-boot`
- `mss-boot-io/mss-boot-admin`
- `mss-boot-io/mss-boot-admin-antd`
- `mss-boot-io/mss-boot-docs`

## 已执行的安全设置

2026-06-09 通过 GitHub API 复核并完成：

- 四个核心仓库的 Dependabot security updates 已启用。
- 四个核心仓库的 secret scanning 已启用。
- 四个核心仓库的 secret scanning push protection 已启用。
- 四个核心仓库的 private vulnerability reporting 专门端点返回
  `{"enabled":true}`。

复核命令：

```bash
for r in mss-boot mss-boot-admin mss-boot-admin-antd mss-boot-docs; do
  gh api repos/mss-boot-io/$r/private-vulnerability-reporting --jq .
  gh api repos/mss-boot-io/$r --jq '{security_and_analysis:.security_and_analysis}'
done
```

注意：`repos/{owner}/{repo}` 响应中的
`private_vulnerability_reporting_enabled` 字段仍显示 `null`，但专门端点已
返回 enabled。后续以专门端点为准，并在 GitHub Web UI 中抽查。

## Content Reporting 状态

GitHub Web UI 已将 `mss-boot` 与 `mss-boot-docs` 的 reported content 设置
切到 `All users` 并保存；`mss-boot-admin` 与 `mss-boot-admin-antd` 之前
已经是 enabled。

GitHub Community Profile API 在 2026-06-09 仍返回：

- `mss-boot`: `content_reports_enabled=false`, `health_percentage=87`
- `mss-boot-docs`: `content_reports_enabled=false`, `health_percentage=87`
- `mss-boot-admin`: `content_reports_enabled=true`, `health_percentage=100`
- `mss-boot-admin-antd`: `content_reports_enabled=true`, `health_percentage=100`

当前判断为 Community Profile API 缓存或回读延迟。后续复核时应同时查看
Web UI 设置页与 API。

## 文档落地

本轮补齐两个公开文档缺口：

- `docs/devops/security-policy-faq.md`：说明疑似漏洞不要发 public issue，
  提供四个仓库 SECURITY policy 链接、私密报告字段和脱敏要求。
- `docs/admin/login-troubleshooting.md`：覆盖 401、token 过期、菜单为空、
  API 域名/代理错误和浏览器缓存问题。

对应 GitHub Issues：

- `mss-boot-docs#5` docs: add SECURITY policy FAQ
- `mss-boot-docs#6` docs: add troubleshooting page for login failures

## 当前阻塞

- `mss-boot#382`、`mss-boot#383`、`mss-boot-admin-antd#97`、
  `mss-boot-admin-antd#98` 已通过 CI，仍等待 code owner review。
- `mss-boot`、`mss-boot-admin`、`mss-boot-admin-antd` 的 branch protection
  API 回读仍显示 force push 相关设置需要 GitHub Web UI 或 ruleset 再确认。
- Cloudflare 前端发布仍必须遵守低频发布策略：本地验证完成后再发布 beta。

## 后续建议

- 定期用 GitHub API 复核 private vulnerability reporting 与 security settings。
- Community Profile API 刷新后，如果 `mss-boot` 和 `mss-boot-docs` 仍是 87，
  用 GitHub Web UI 复核 reported content 设置。
- 将公开安全说明、登录排障、社区运营 playbook 作为外部推广时的固定链接。
