# mss-boot Framework

[简体中文](./README.Zh-cn.md)

## v1.3.5 component status

v1.3.5 is an immutable-partial train. The Framework component
`github.com/mss-boot-io/mss-boot-admin/mss-boot@v1.3.5` is publicly
available and remains bound to its original release commit, but the Root tools,
official npmjs package, Docs, backend image, and complete Thin Host
qualification were not published.

The release policy still identifies **v1.3.2** as the current stable
distribution. The v1.3.5 Go component is audit evidence and a reusable
framework identity; it must not be mixed with another patch, local
replacements, source checkouts, or unpublished packages to manufacture a
complete Admin Distribution.

## Framework boundary

mss-boot provides lifecycle, configuration, logging, cache, queues, locking,
storage, transport, migration, response, condition, retry, idempotency, and
reconciliation primitives. It does **not** contain Admin entities, menus,
business workflows, React code, or Agent orchestration.

The supported dependency direction for a future complete distribution is:

```text
Thin Host business code -> Admin -> mss-boot
```

Most applications should depend on the complete Admin module and let Go
resolve the matching Framework transitively. Direct Framework imports are
reserved for domain-neutral infrastructure extensions. This is a future
complete-distribution contract, not a v1.3.5 Thin Host installation path.

Public APIs, configuration keys, interfaces, and persistence behavior remain
compatibility surfaces. See the
[package status](../docs/docs/getting-started/packages.md) and
[`CHANGELOG.md`](./CHANGELOG.md).

Repository-source test commands are source-only contributor contracts in
[`AGENTS.md`](./AGENTS.md).
