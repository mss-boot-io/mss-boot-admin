# MSS Admin — Ant Design 6

This is the independent React 19 and Ant Design 6.6.0 Admin application. It has
its own dependency graph, build output, image, tag namespace, deployment, and
rollback history. The legacy application remains in `web/antd`.

This checkpoint is a buildable foundation, not a production release candidate.
Backend cookie/CSRF and WebSocket-ticket support, retained business modules, the
v6 generator target, and required browser evidence remain release blockers and
are fail-closed by the repository qualification contract.

The upstream engineering reference is Ant Design Pro v6.0.2 commit
`2b453c67b535b76f5f95d6542397a4b987b61de2`; runtime and build packages are
resolved and pinned independently for this repository.

```shell
corepack pnpm@10.34.5 install --frozen-lockfile
corepack pnpm@10.34.5 lint
corepack pnpm@10.34.5 test:ci
corepack pnpm@10.34.5 build:release
```

See `.mss/features/admin-antd-v6-application.yaml` and
`docs/adr/2026-08-15-independent-ant-design-v6-application.md` for the complete
contract and rollout boundary.
