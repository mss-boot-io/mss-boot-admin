# mss-boot Framework

[简体中文](./README.Zh-cn.md)

## v1.3.7 stable component status

v1.3.7 is the current stable, adoptable Complete Admin Distribution. The public
Framework module
`github.com/mss-boot-io/mss-boot-admin/mss-boot@v1.3.7` was qualified with
`GOWORK=off` and the matching Admin, Admin Web, Root tools, and images from exact
merged-main commit `77b53d41092741eac62fa6418c0bdbf87413c7cd`.

v1.3.5 and v1.3.6 remain immutable partial trains. The public v1.3.6 Framework
component is bound to commit `b1fe47a3a83209574e09d53526b122dd2cbc5277`,
but its coordinated train never completed. Public partial-train Go components
are immutable audit evidence; they must not be mixed with another patch, local
replacements, source checkouts, or unpublished packages to manufacture a
complete Admin Distribution. The Docs website publishes independently and does
not gate this Framework module's stable status.

## Framework boundary

mss-boot provides lifecycle, configuration, logging, cache, queues, locking,
storage, transport, migration, response, condition, retry, idempotency, and
reconciliation primitives. It does **not** contain Admin entities, menus,
business workflows, React code, or Agent orchestration.

The supported dependency direction for v1.3.7 is:

```text
Thin Host business code -> Admin -> mss-boot
```

Most applications should depend on the complete Admin module and let Go
resolve the matching Framework transitively. Direct Framework imports are
reserved for domain-neutral infrastructure extensions. A Thin Host must pin the
complete v1.3.7 distribution rather than assemble a path from v1.3.5 or v1.3.6
components.

Public APIs, configuration keys, interfaces, and persistence behavior are
compatibility surfaces. See the
[package status](../docs/docs/getting-started/packages.md) and
[`CHANGELOG.md`](./CHANGELOG.md).

Repository-source test commands are source-only contributor contracts in
[`AGENTS.md`](./AGENTS.md).
