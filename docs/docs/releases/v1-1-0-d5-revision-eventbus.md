---
title: D5 Revision EventBus and Admin reconciliation checkpoint
order: 19
description: Typed best-effort revision delivery, commit-first Casbin publication, authoritative reconciliation, and exact development evidence
keywords: [v1.1.0 D5 eventbus revision casbin reconciliation redis memory]
---

# D5 Revision EventBus and Admin reconciliation checkpoint

This page records two untagged v1.1.0 development commits:

- `04e8e0c57db74153a87d652e07d8c9bd62b11371` adds the domain-neutral
  `mss-boot/runtime/eventbus` package.
- `160e2df196dc804f2fb1717f28404553a9538036` composes its Memory provider with
  the Admin Casbin authorization revision boundary.

This is checkpoint evidence, not a feature-freeze candidate, publication
authority, or Stable evidence. `platform.event-bus` is Beta and
`platform.storage-runtime-v2` remains Planned.

## Implemented boundary

The Framework package carries a typed payload with a non-zero monotonic
`Revision`. `BuildMemory` and `BuildRedis` allocate and validate state without
starting provider I/O or goroutines.

- Memory synchronously fans a notification out to every subscriber registered
  when the publication is accepted.
- Redis publishes and polls the latest revision through one caller-owned
  `redisresource.Scope`. A currently disconnected replica can miss a notification
  or an intermediate revision; the provider does not claim durable delivery.
- Duplicate and older revisions do not move a subscriber backward. A failed or
  panicking subscriber does not advance its observed revision, so reconciliation
  can retry it.
- `Start`/`Run`, readiness, health, reconciliation, and `Close` honor caller
  contexts. Close waits for accepted delivery within the caller deadline and the
  Redis provider never closes its shared Scope or underlying resource.
- Public errors retain fixed classifications while redacting provider and panic
  material.

The API deliberately has no ack/nack, visibility timeout, retry counter,
dead-letter queue, or job-completion contract. EventBus is a best-effort revision
signal; WorkQueue maturity remains independent.

## Admin commit, publish, and reconcile ordering

The production Admin composition uses a Memory EventBus. Multi-process repair
does not depend on Memory delivery: each replica periodically reads the global
authorization `ConfigRevision` from the authoritative database.

1. Canonical role/menu policy mutation locks the relevant role and global
   revisions.
2. Casbin rows plus the next revisions are written in the same GORM transaction.
3. The transaction commits before any EventBus publication is attempted.
4. The committed global revision is published as `AuthorizationRevisionEvent`.
5. The current-process subscriber borrows the managed Config database lease and
   calls the canonical `EnsureCurrent` boundary, which reloads authoritative
   Casbin policy when the durable revision changed.
6. A server-manager Runnable repeats authoritative reconciliation every 15
   seconds. It owns the Memory bus lifecycle and rejects a second owner.

This ordering has an intentional post-commit failure boundary. If the process
crashes or publication fails after step 3, policy and revision remain committed;
the API can surface an `AuthorizationPropagationError`, but the committed policy
is not rolled back. A later periodic reconciliation or process startup observes
the durable revision and reloads policy. Publication never becomes the authority.

| Failure or delivery case | Result |
| --- | --- |
| Policy transaction fails | Policy and revision roll back; no revision is published. |
| Commit succeeds and publish succeeds | Current subscribers reload from the database. |
| Commit succeeds, then publish fails or the process crashes | Committed policy remains; authoritative reconciliation repairs the missed signal. |
| Duplicate or out-of-order revision | The subscriber does not move backward or reload an already observed revision. |
| Subscriber panic or reload failure | Panic is isolated, health degrades, and reconciliation retries without exposing panic material. |
| Redis polling disconnect | The Redis provider reports degradation; database revision reconciliation remains the repair path. |
| EventBus runtime is not installed | The existing `EnsureCurrent` plus watcher notification path remains as a source-compatible fallback. |
| Production EventBus runtime is installed | Canonical mutation bypasses legacy WorkQueue watcher acknowledgement semantics. |

No frontend permission projection or authorization decision moved into EventBus.
Existing root-only API enforcement remains unchanged, and Casbin continues to
authorize from database-derived policy rather than notification payloads.

## Exact top-level development tests

Commit `04e8e0c` introduced seven Framework top-level tests:

- `TestMemoryFansOutToCurrentSubscribersAndRejectsOldRevisions`
- `TestMemoryPanicIsolationAndAuthoritativeReconciliation`
- `TestMemoryCloseWaitsForAcceptedDeliveryAndHonorsDeadline`
- `TestReconcilerRedactsSourceFailureAndPanic`
- `TestRedisBuildIsPureAndPollingFansOutLatestRevision`
- `TestRedisDisconnectDegradesWhileAuthorityRepairsCommitBeforePublish`
- `TestRedisRunOwnsPollingAndCloseLeavesSharedResourceOpen`

Commit `160e2df` introduced eight Admin top-level tests:

- `TestCasbinRevisionReconcilesCommitPublishCrash`
- `TestAuthorizationMutationPublishesCommittedRevisionWithoutWorkQueueWatcher`
- `TestAuthorizationSubscriberPanicIsolatedAndReconciled`
- `TestAuthorizationEventBusOutagePreservesCommittedPolicy`
- `TestAuthorizationEventRuntimeRejectsSecondOwner`
- `TestAuthorizationEventRuntimeHealthRecordsAndRecoversReconcileFailure`
- `TestBuildAuthorizationEventRuntimeUsesManagedDatabaseLease`
- `TestBuildAuthorizationEventRuntimeRejectsMissingDatabaseLease`

The evidence runner parsed uncached `go test -json` output. Every required test
had run=1, pass=1, skip=0 under the race detector.

```shell
go run ./cmd/mss test evidence --directory mss-boot --package ./runtime/eventbus \
  --run '^Test.*$' --count 1 --race --go-work off \
  --require TestMemoryFansOutToCurrentSubscribersAndRejectsOldRevisions \
  --require TestMemoryPanicIsolationAndAuthoritativeReconciliation \
  --require TestMemoryCloseWaitsForAcceptedDeliveryAndHonorsDeadline \
  --require TestReconcilerRedactsSourceFailureAndPanic \
  --require TestRedisBuildIsPureAndPollingFansOutLatestRevision \
  --require TestRedisDisconnectDegradesWhileAuthorityRepairsCommitBeforePublish \
  --require TestRedisRunOwnsPollingAndCloseLeavesSharedResourceOpen

go run ./cmd/mss test evidence --directory admin --package ./service \
  --run '^TestCasbinRevision.*$' --count 1 --race \
  --require TestCasbinRevisionReconcilesCommitPublishCrash

go run ./cmd/mss test evidence --directory admin --package ./service \
  --run '^TestAuthorizationMutation.*$' --count 1 --race \
  --require TestAuthorizationMutationPublishesCommittedRevisionWithoutWorkQueueWatcher

go run ./cmd/mss test evidence --directory admin --package ./service \
  --run '^TestAuthorizationSubscriber.*$' --count 1 --race \
  --require TestAuthorizationSubscriberPanicIsolatedAndReconciled

go run ./cmd/mss test evidence --directory admin --package ./service \
  --run '^TestAuthorizationEvent.*$' --count 1 --race \
  --require TestAuthorizationEventBusOutagePreservesCommittedPolicy \
  --require TestAuthorizationEventRuntimeRejectsSecondOwner \
  --require TestAuthorizationEventRuntimeHealthRecordsAndRecoversReconcileFailure

go run ./cmd/mss test evidence --directory admin --package ./cmd/server \
  --run '^TestBuildAuthorizationEventRuntime.*$' --count 1 --race \
  --require TestBuildAuthorizationEventRuntimeUsesManagedDatabaseLease \
  --require TestBuildAuthorizationEventRuntimeRejectsMissingDatabaseLease
```

The Framework evidence uses `GOWORK=off`. Admin evidence intentionally uses the
current repository workspace because `admin/go.mod` still references the
published `mss-boot/v1.0.0`, which does not contain this new package. Independent
Admin `GOWORK=off` resolution belongs to the pre-root gate after
`mss-boot/v1.1.0` is published and the Admin dependency is updated.

## Compatibility, recovery, and remaining gates

The Framework API is additive and no WorkQueue interface is removed. Admin uses
the existing `ConfigRevision` table and may create the logical global
authorization revision row when it is absent; this checkpoint adds no schema
migration. Existing policy callers that have not installed the new runtime keep
the compatibility reconciliation path.

Before publication, rollback means removing the Admin runtime composition and
rolling the Admin and Framework source changes back together. Existing revision
rows can remain because the older path already treats the database as authority.
Never delete or decrement revision rows to recover delivery. After publication,
use a forward repair rather than moving a tag.

The following claims remain open:

- rerunning the exact suites from one selected feature-freeze SHA;
- externally resolving the published Framework tag with `GOWORK=off` and then
  updating the Admin module dependency;
- real multi-replica Redis, disconnect/reconnect, Sentinel, Cluster, TLS, and
  failover conformance;
- composing a Redis EventBus provider into Admin if that deployment mode is
  selected later;
- adding cache-specific authoritative revisions before claiming cross-process
  cache invalidation recovery;
- complete Runtime v2 lifecycle, leak, and release qualification.

There is no browser-visible route or UI change in these two commits. Browser
review of this documentation page is presentation verification only and is not
runtime EventBus evidence.
