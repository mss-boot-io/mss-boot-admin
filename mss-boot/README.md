# mss-boot Framework

[简体中文](./README.Zh-cn.md)

`github.com/mss-boot-io/mss-boot-admin/mss-boot@v1.3.3` is the reusable,
domain-neutral Go framework in the Complete Admin Distribution.

```sh
go get github.com/mss-boot-io/mss-boot-admin/mss-boot@v1.3.3
```

It provides lifecycle, configuration, logging, cache, queues, locking, storage,
transport, migration, response, condition, retry, idempotency, and
reconciliation primitives. It does **not** contain Admin entities, menus,
business workflows, React code, or Agent orchestration.

The supported dependency direction is:

```text
Thin Host business code -> Admin -> mss-boot
```

Most applications should import the complete Admin module and let Go resolve
the matching Framework transitively. Import `mss-boot` directly only for
domain-neutral infrastructure extensions, and keep the exact v1.3.3 version.

Public APIs, configuration keys, interfaces, and persistence behavior are
compatibility surfaces. See the
[package contract](../docs/docs/getting-started/packages.md) and
[`CHANGELOG.md`](./CHANGELOG.md).

Repository-source test commands are contributor-only and remain in
[`AGENTS.md`](./AGENTS.md).
