---
title: v1.3.4 configuration cache consistency
order: 15
nav:
  order: 1
  title: admin
description: Database-authoritative cache boundaries for application, personal, and system configuration
keywords: [admin configuration cache redis revision consistency]
---

# Configuration cache consistency

Application, personal, and system configuration have different privacy and consistency needs. The
database is the source of truth for all three. Redis is optional acceleration and never supplies
authorization, ownership, or revision state.

## Resource boundaries

| Resource | Cache policy | Consistency boundary |
| --- | --- | --- |
| Public application profile | Revision-keyed snapshot, 15-minute maximum TTL | Application public-profile revision |
| Non-secret application value | Versioned read-through envelope with bounded logical expiry | Exact group and name plus database-first invalidation |
| Authenticated application secret | Not cached | Direct database read after authorization |
| Personal profile or group | Owner-digested, revision-keyed snapshot, 15-minute maximum TTL | Per-user profile revision |
| Personal theme | Owner-digested, revision-keyed canonical resource | Per-user theme and profile revisions |
| Opaque SystemConfig content | Not cached | Direct root-only database read |

The opaque `SystemConfig.Content` field may contain credentials. It deliberately bypasses the generic
GORM query cache even when an operator configures a wildcard cache allowlist.

## Read behavior

Every derived configuration entry is a versioned JSON envelope. A reader validates the schema,
resource identity, owner digest where applicable, database revision, and expiry before accepting a
hit. Legacy strings, damaged JSON, identity mismatches, and stale revisions are misses.

Generic GORM query cache keys use a fixed-length digest of the generated SQL and all bound values.
This prevents parameterized queries such as `id = ?` from sharing a result while keeping IDs and
other request values out of Redis keys and logs. A process and database-pool identity prevents the
same SQL from sharing data across deployments or databases. Each query key also includes a per-table
random epoch. Mutation invalidation atomically replaces that epoch; readers recheck it after a hit
and before publishing a miss, so an in-flight old database read cannot revive an obsolete cache entry.
If Redis evicts only the epoch key, the next reader creates a fresh non-reusable token rather than
falling back to a deterministic initial generation.

Stable personal snapshots read revisions and data from the authoritative database. If a revision
changes during the read, the service retries rather than cache a torn result. Empty profiles and
groups are valid results and may be cached.

## Write behavior

Application configuration writes commit the database first. The compatibility setter validates exact
database key identity and advances the public-profile and, for theme writes, theme revision in the same
transaction. Cache cleanup happens after commit and is best effort. A Redis outage therefore cannot turn
a committed database change into an HTTP error, and a failed database write cannot publish an uncommitted
cache value.

Option updates lock the writer row and commit the historical snapshot plus the new value/version in one
transaction. Cache misses also refill from the writer rather than a lagging replica. Cache GET, SET, and
DEL operations have a 100-millisecond deadline. If post-commit cleanup times out, the preceding Option
cache entry may remain visible until its five-minute TTL expires; the committed database row and version
history remain authoritative.

Personal group writes validate exact key identity and commit all fields in one transaction. The same
transaction advances one per-user profile revision. Personal theme patch and reset advance both the
theme and profile revisions. Readers always select cache entries using the committed revision, so an
old entry can remain until TTL without becoming visible again.

## Failure and operations policy

- Revisioned configuration-cache read, decode, write, timeout, and cleanup errors fall back to the database.
- Invalid Redis configuration or connectivity during startup is reported without terminating the application.
- Request paths never scan Redis with `KEYS` or unbounded patterns.
- Raw user IDs, query parameters, configuration payloads, and secrets are not cache-key or metric labels.
- Generic query caching is fail-closed by default and should use an explicit table allowlist. Do not use `'*'` as a production allowlist.
- Keep `queryCache: false` until the deployment has validated its explicit allowlist and Redis failure behavior.
- If Redis is unavailable exactly when a generic query-cache invalidation is published, another instance can retain its prior entry until TTL. Only explicitly stale-tolerant tables may use this cache; configuration resources use database revisions or bypass it entirely.
- Until a deployment namespace is configurable for domain caches, each application deployment must use a dedicated Redis database or endpoint.
- Cache incidents can increase database load, but they must not change authorization or committed values.

## Verification

From a generated Thin Host, validate the imported Admin Distribution and business
composition through its package-first gate:

```sh
mss verify --all
```

The v1.3.4 Admin package itself is released only after its independent fault-injection,
isolation, transaction-rollback, cache-poisoning, and Redis-failure tests pass. Downstream
applications must not copy those Foundation test packages or weaken the database-authoritative
contract.
