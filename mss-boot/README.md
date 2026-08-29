# mss-boot Framework

[简体中文](./README.Zh-cn.md)

## v1.3.7 candidate component status

v1.3.7 is the selected complete package-first candidate, but
`github.com/mss-boot-io/mss-boot-admin/mss-boot@v1.3.7` is not public yet.
Candidate preview may qualify it from the exact repository workspace; formal
Admin publication must later resolve this exact public Framework with
`GOWORK=off`.

v1.3.5 and v1.3.6 remain immutable-partial trains. The public v1.3.6 Framework
component is bound to commit `b1fe47a3a83209574e09d53526b122dd2cbc5277`,
but its coordinated train lacks the Root image, official npmjs package, Docs,
and complete Thin Host qualification.

The release policy still identifies **v1.3.2** as the current stable
distribution. Public partial-train Go components are immutable audit evidence;
they must not be mixed with another patch, local
replacements, source checkouts, or unpublished packages to manufacture a
complete Admin Distribution.

## Framework boundary

mss-boot provides lifecycle, configuration, logging, cache, queues, locking,
storage, transport, migration, response, condition, retry, idempotency, and
reconciliation primitives. It does **not** contain Admin entities, menus,
business workflows, React code, or Agent orchestration.

The supported dependency direction for the v1.3.7 candidate is:

```text
Thin Host business code -> Admin -> mss-boot
```

Most applications should depend on the complete Admin module and let Go
resolve the matching Framework transitively. Direct Framework imports are
reserved for domain-neutral infrastructure extensions. This is a candidate
complete-distribution contract under qualification, not a public v1.3.6 or
v1.3.5 Thin Host installation path.

Public APIs, configuration keys, interfaces, and persistence behavior remain
compatibility surfaces. This is not a public v1.3.7 installation path. See the
[package status](../docs/docs/getting-started/packages.md) and
[`CHANGELOG.md`](./CHANGELOG.md).

Repository-source test commands are source-only contributor contracts in
[`AGENTS.md`](./AGENTS.md).
