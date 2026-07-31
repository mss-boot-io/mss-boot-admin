# mss-boot-docs

[![CI](https://github.com/mss-boot-io/mss-boot-docs/actions/workflows/ci.yml/badge.svg)](https://github.com/mss-boot-io/mss-boot-docs/actions/workflows/ci.yml)
[![CodeQL](https://github.com/mss-boot-io/mss-boot-docs/actions/workflows/codeql.yml/badge.svg)](https://github.com/mss-boot-io/mss-boot-docs/actions/workflows/codeql.yml)
[![OpenSSF Scorecard](https://github.com/mss-boot-io/mss-boot-docs/actions/workflows/scorecard.yml/badge.svg)](https://github.com/mss-boot-io/mss-boot-docs/actions/workflows/scorecard.yml)
[![License](https://img.shields.io/github/license/mss-boot-io/mss-boot-docs.svg?style=flat-square)](https://github.com/mss-boot-io/mss-boot-docs/blob/main/LICENSE)

`mss-boot-docs` 是 `mss-boot` 与 `mss-boot-admin` 的文档站点，基于 [dumi](https://d.umijs.org) 构建。

公开站点：[https://docs.mss-boot-io.top](https://docs.mss-boot-io.top)

当前文档重点围绕以下方向组织：

- `mss-boot` 框架能力
- `mss-boot-admin` 的治理、运营与扩展能力
- 单租户后台系统的启动、部署、测试与演进路线
- AI 注解协同相关文档沉淀

## 推荐阅读顺序

如果你主要关注 `mss-boot-admin`，建议按以下顺序阅读：

1. `/admin` 介绍页
2. `/admin/current-capabilities` 当前功能总览
3. `/admin/phase-4-roadmap` 四期路线图
4. `/admin/phase-5-roadmap` 五期路线图
5. `/admin/product-polish-governance-plan` 产品打磨治理计划
6. `/admin/docker` 容器化与生产部署
7. `/admin/production-standardization` 生产部署标准化
8. `/admin/pre-release-checklist` 发布前检查清单
9. `/admin/observability-guide` 性能与可观测性指南
10. `/admin/security-baseline` 安全基线指南
11. `/admin/login-troubleshooting` 登录排障
12. `/admin/integration-test-guide` 集成测试指南
13. `/admin/configuration-guide` 配置教程

如果你需要报告疑似漏洞，请先阅读 `/devops/security-policy-faq`，不要在公开
Issue 中披露可被滥用的细节。

## 本地开发

```bash
pnpm install
pnpm start
```

默认会启动本地文档站点，用于预览 `docs/` 下的内容。

## 构建

```bash
pnpm run build
```

## Testing

完整的测试文档请参考 [集成测试指南](/admin/integration-test-guide) 和各子项目的 README 文件：

### 后端测试 (mss-boot-admin)

- **单元测试**: `go test ./... -v -coverprofile=coverage.out`
- **集成测试**: `go test -tags=integration ./...`
- **覆盖率要求**: ≥80%

### 前端测试 (mss-boot-admin-antd)

- **单元测试**: `pnpm test --coverage`
- **E2E 测试**: `pnpm e2e`
- **覆盖率要求**: ≥80%

## 贡献说明

- 文档面向开源协作者，尽量使用可验证、可复现、可讨论的表述。
- 与产品实现相关的结论应尽量基于仓库代码、配置或已验证行为。
- 中文文档优先保持术语一致，避免同一能力出现多种叫法。

## License

MIT
