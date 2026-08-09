# Build storage capabilities from explicit owned runtime resources

- Status: Proposed
- Date: 2026-08-09
- Baseline: `main@ee800262c035c5f4242aca1841d077554481d2c4` (`v1.0.0`)
- Owners: Framework Platform, Admin Platform, Security
- Feature contracts:
  - `.mss/features/foundation-v1-0-1-storage-safety.yaml`
  - `.mss/features/storage-runtime-v2.yaml`
- Target train: `v1.0.1` through `v1.1.0`

## Context

The current name hides several different systems. `mss-boot/pkg/config/storage/`
contains cache, Redis access, one-time verification helpers, queue adapters, and a
distributed-lock adapter. Object-storage configuration instead lives in the sibling
file `mss-boot/pkg/config/storage.go`, while upload orchestration lives in
`admin/service/storage.go`.

This is more than a package-layout problem. These capabilities have different
authoritative data, delivery guarantees, outage behavior, security boundaries, and
resource owners. Treating them as one stable adapter surface caused broad interfaces,
hidden global clients, ambiguous provider selection, and maturity claims that are not
supported by provider-specific evidence.

The target architecture is designed without preserving existing API, configuration,
or provider behavior as an input. A temporary compatibility bridge may be chosen by
release engineering, but it must stay outside the target interfaces and cannot restore
an unsafe fallback, acknowledgement, secret, or lifecycle behavior.

## Current implementation boundary

| Capability | Current production consumer | Baseline conclusion |
| --- | --- | --- |
| Cache and Redis | Application, user, theme, language, option and session caches; OAuth state; verification codes; GORM query cache | One broad `AdapterCache` exposes `redis.UniversalClient`; disposable derived data and authoritative security state share a client and failure model. |
| Queue | Admin uses it primarily for the Casbin watcher and starts it with an unowned background context | The real Admin need is replica broadcast plus reconciliation, not a general durable work queue. |
| Locker | No Admin production consumer was found | It has no evidence for a stable claim; defer it until a real fenced use case exists. |
| Object storage | Admin upload service | Provider settings are read piecemeal per request and the S3 client is rebuilt per upload; invalid settings may silently select local storage. |
| Admin startup storage | `admin/config.Config.Storage.Init()` runs during configuration assembly, but upload ignores that client | This ghost initialization can still fail the process or leak a client; the composition root must either own and inject it or remove it. |
| S3 configuration source | Bootstrap before the main configuration is loaded | This is a separate ownership domain and cannot depend on the application runtime or a hidden shared client. |
| WebSocket Redis | A second global Redis path under Admin configuration | No `SetRedisClient` call is present at the baseline, so clustered Redis pub/sub is unwired; it must become an explicit realtime scope and is not a durable queue. |

Known release-blocking paths in the baseline include:

- verification values generated through non-cryptographic randomness and consumed by
  non-atomic read/write/delete sequences;
- Kafka messages marked before decode and handler success;
- multipart size enforcement after `FormFile` has already parsed the request;
- unreadable or unknown storage configuration silently selecting local storage;
- local keys derived from user identifiers and original filenames, allowing collision
  and overwrite semantics that the API does not disclose;
- optional provider code using process termination, detached contexts, and implicit
  client ownership;
- NSQ duration values multiplied after YAML has already decoded them as `time.Duration`;
- Redis configuration mutated during initialization and one process-global client using
  first-initializer-wins semantics across unrelated consumers.

The existing query-cache algorithm is the strongest part of this area: it has safe
generation changes, variable/database separation, and an allowlist. It remains opt-in
beta because result metadata, pointer decoding, payload bounds, cross-instance data-source
identity, lifecycle ownership, and stampede behavior still need a complete contract.

## Decision

### 1. Split names by semantics

The clean target package and capability vocabulary is:

| Target boundary | Responsibility |
| --- | --- |
| `runtime/resource` | Dependency graph, lifecycle, health, readiness, close ownership, sanitized diagnostics |
| `runtime/config` | Strict discriminated runtime configuration and immutable normalized profiles |
| `runtime/redisresource` | Strict named Redis resources for standalone, Sentinel, cluster, and TLS profiles |
| `runtime/cache` | Scoped derived caches and a separately enabled `QueryCache` |
| `runtime/challenge` | Atomic security-specific `ChallengeStore` |
| `runtime/eventbus` | Best-effort replica fan-out with authoritative revision reconciliation |
| `runtime/workqueue` | Durable at-least-once work, acknowledgement, retry, and dead-letter semantics |
| `runtime/lock` | Distributed locks only for explicit fenced use cases |
| `runtime/objectstore` | Private opaque objects and provider conformance |
| `runtime/delivery` | Authenticated proxy, signed URL, and public-delivery policy |
| `pkg/config/source/s3` | Minimal configuration bootstrap independent from application object stores |

Exact Go names may be refined during implementation, but these boundaries and dependency
directions are normative. Domain-neutral runtime packages stay in `mss-boot`; Admin owns
upload policy, HTTP responses, permissions, and business metadata.

### 2. Use strict discriminated configuration

Configuration is decoded with unknown-field rejection and exactly one provider branch.
It is normalized once into an immutable startup snapshot. `Build` does not mutate caller
configuration, open network connections, or start goroutines.

The validator rejects at least:

- unknown provider and unknown field names, including stale names such as `poolNum` and
  documented-but-unsupported providers such as Pulsar;
- multiple provider branches, empty endpoints, non-positive or overflowed duration
  relationships, Sentinel without a master name, and cluster with a nonzero database;
- an incomplete TLS client certificate/key pair or static access-key/secret-key pair;
  transport TLS and application credentials are independent axes, while credential modes
  are mutually exclusive;
- partial static credentials that would otherwise fall through to a default credential
  chain;
- local object storage without a confined root or a delivery policy that the deployment
  does not actually serve.

Secrets use typed `SecretRef` values. Admin `AppConfig` may choose a predeclared
`profileRef` and non-secret upload policy, but it cannot persist raw Redis or object-store
credentials.

An illustrative shape is:

```yaml
runtime:
  redis:
    resources:
      main:
        mode: standalone
        endpoints: [redis:6379]
        passwordRef: env://REDIS_PASSWORD
        tls:
          minVersion: "1.2"
  caches:
    config:
      resourceRef: main
      namespace: config
      failureMode: bypass
      defaultTTL: 15m
    query:
      enabled: false
      resourceRef: main
      datasourceID: primary
      tables: []
  challenges:
    email:
      provider:
        kind: redis
        resourceRef: main
      ttl: 5m
      resendCooldown: 1m
      maxAttempts: 5
  eventBus:
    provider:
      kind: redis
      resourceRef: main
  objectStores:
    uploads:
      provider:
        kind: s3
      endpoint: https://minio.example
      bucket: uploads
      credentials:
        kind: static
        accessKeyRef: env://S3_ACCESS_KEY
        secretKeyRef: env://S3_SECRET_KEY
```

### 3. Name and scope Redis resources

One `redis.resources.<name>` creates one client, one health identity, and one owner that
closes it exactly once. Consumers reference it by name and receive a narrow scoped
interface; they do not receive the raw universal client.

At minimum, the following scopes are distinct even when they share a physical client:

- `cache.config`
- `cache.query`
- `session`
- `oauth-state`
- `challenge.email`
- `eventbus.casbin`
- `realtime.websocket`

Each scope owns its key prefix, TTL and payload bounds, failure policy, and metrics labels.
A scope cannot enumerate, read, or delete another scope.

### 4. Make lifecycle ordering explicit

The lifecycle is:

`Decode -> Validate -> Build -> Start -> Health/Ready -> Run -> Close`

`Build` has no asynchronous side effect. The sole application composition root starts
resources in dependency order and waits for required health before registering and
starting network listeners. Event subscribers and reconcilers run in an errgroup derived
from the application context. A startup failure closes acquired resources in reverse
order; close is idempotent and bounded by a caller deadline.

Admin has exactly one application object-storage owner. The unused
`admin/config.Config.Storage.Init()` path is removed, or it becomes the immutable profile
that is injected into upload; it cannot coexist with a second per-request S3 constructor.

The current server manager starts sibling `Runnable` values concurrently. Therefore an
unstarted resource runtime and an HTTP listener must not be registered as peers. The top
level application must synchronously establish resource readiness first, then start a
child server manager, and finally close resources in reverse order.

Provider packages never call `os.Exit`, `log.Fatal`, or `panic` for configuration or
optional-integration failure, and never start long-lived work with
`context.Background()`.

### 5. Declare authority and outage semantics

| Capability | Authority and delivery semantics | Redis or provider outage |
| --- | --- | --- |
| Derived cache | Database is authoritative; cache only accelerates reads | Bypass to the database; a cache failure cannot change a committed response |
| Session snapshot | Session/database record is authoritative | Read authority directly; cache write failure is best effort and visible in metrics |
| OAuth state and ChallengeStore | The one-time state itself is authoritative security state | Fail closed with a typed unavailable result; production has no local fallback |
| EventBus | Low-latency fan-out; duplicate, out-of-order, or missed events are allowed | Writes may remain committed, but health degrades and revision reconciliation repairs state |
| WorkQueue | Durable at-least-once work with ack/nack, bounded retry, and dead letter | Queue capability is unavailable; it does not masquerade as EventBus |
| ObjectStore | The configured backend is authoritative for object bytes and metadata | Upload and download fail unavailable; never write to another provider |

### 6. Keep QueryCache explicit and beta

Generic GORM query caching is disabled by default and requires a stale-tolerant table
allowlist. Its contract must preserve `not found`, `RowsAffected`, preloads, scans,
pointer/value decoding, transaction boundaries, data-source identity, and table-generation
invalidation. Payload size is bounded and cache fill uses singleflight or an equivalent
stampede control. It does not cache authorization decisions, opaque system configuration,
security challenges, or mutation results.

An active database transaction bypasses the shared query cache unless a future provider
can prove snapshot-bound keys. Rollback and read-your-writes tests are mandatory; a value
read or written inside one transaction cannot populate data observed by another.

### 7. Replace verification helpers with ChallengeStore

`ChallengeStore` is not a cache convenience API. It provides a versioned
`BeginIssue`/delivery-`Commit`/`Abort` state machine plus typed `Verify` outcomes with
purpose isolation for login, registration, and password reset.

- Codes use `crypto/rand` and preserve fixed width.
- Keys bind a normalized subject and purpose through HMAC; stored values contain only a
  fixed-length verifier and bounded counters.
- The verifier uses a high-entropy versioned pepper from `SecretRef`, compares in constant
  time, and supports bounded dual-version verification during rotation. A plain hash of a
  six-digit value is not sufficient against offline enumeration.
- `BeginIssue` atomically enforces resend eligibility, charges the rolling quota, and reserves
  exactly one pending version. `Verify` never accepts pending values.
- Successful delivery commits only the matching version, rotates the active verifier, and starts
  resend cooldown. Delivery failure aborts only that pending version, keeps the quota charge,
  preserves the prior active verifier, and does not start cooldown. A stale delivery result cannot
  commit or abort a newer concurrent issue.
- Pending state has a short lease independent from active expiry. `BeginIssue` atomically reclaims
  an expired pending lease after process crash or hung delivery, never extends the prior active
  verifier, and continues to reject stale delivery completion through the version compare-and-swap.
- `Verify` atomically compares, increments attempts, locks at the maximum, and consumes
  exactly once on success. A wrong attempt does not immediately destroy the valid code.
- Provider outage fails closed. Public behavior does not reveal account existence.
- Codes and subjects never enter logs, traces, metrics, or evidence.
- Redis Cluster uses a single-key state machine or an explicit hash tag that keeps every
  atomic key in one slot; CROSSSLOT and failover behavior are required provider tests.

### 8. Separate EventBus from WorkQueue

Admin's Casbin propagation becomes a typed revision event. Memory delivery reaches every
in-process subscriber; Redis delivery reaches every currently connected replica. Neither
provider promises delivery to a stopped or disconnected process. A subscriber reloads
policy from the authoritative database, and a periodic revision reconciler repairs missed
or out-of-order events until every live replica converges. Memory and Redis implementations
may graduate through the EventBus suite without claiming durable work-queue semantics.

The policy mutation and monotonic policy-revision update commit in the same database
transaction. Event publication happens after commit and carries that revision. Tests inject
a crash between commit and publish and prove the periodic reconciler still detects and
loads the new revision.

WorkQueue is a different interface. Kafka, NSQ, and Redis implementations remain
experimental until each proves acknowledgement after handler success, bounded retry with
backoff, dead-letter handling, cancellation and rebalance, duplicate/idempotency behavior,
and observability. v1.1.0 does not promise their stable promotion.

### 9. Defer distributed-lock promotion

No current Admin production consumer justifies a stable distributed-lock capability.
The adapter remains legacy or experimental and disabled by default. A future promotion
requires a concrete use case, lease renewal, loss detection, fencing semantics, race and
partition tests, and proof that a database concurrency mechanism is insufficient.

### 10. Separate ObjectStore from Delivery

The object interface operates on an opaque `ObjectRef` and checksummed metadata through
`Put`, `Open`, `Stat`, and `Delete` semantics. `Put` is create-only: publishing an existing
`ObjectRef` returns a typed conflict/precondition result rather than overwriting bytes. The
service creates a random physical key; the original name is metadata only.

The HTTP boundary applies a hard byte limit before multipart parsing and still validates
the selected stream with a max-plus-one reader and content sniffing. Local storage uses a
fixed root, unique temporary file, no-clobber atomic publish, and explicit path and symlink
confinement. S3 clients are built once from a strict immutable profile. Static credentials
must be complete; the default chain is used only through an explicit credential mode.

Objects are private by default. Proxying, signed URLs, public aliases, caching headers,
and authorization belong to `Delivery`; they are never synthesized by concatenating a
configured endpoint with a key.

Admin owns an authoritative object metadata record containing `ObjectRef`, owner/tenant,
purpose, storage profile, size, checksum, detected content type, original-name metadata,
state, revision, and timestamps. Upload finalization and metadata state transitions have an
explicit transaction/reconciliation design. Open, signed delivery, and delete enforce
backend permission and owner/tenant negative cases; possession of an `ObjectRef` is not
authorization.

The S3 configuration source remains a minimal pre-runtime bootstrap with a separate
profile, client, health result, and close owner.

### 11. Standardize observability and conformance

Every resource reports name, provider, state, latency, retry/failure counts, and a
sanitized endpoint identity. Required resource failure blocks readiness; an optional
resource is explicitly disabled or unavailable, never silently replaced.

Conformance suites cover strict configuration negatives, repeated initialization,
cancellation, startup rollback, idempotent close, race detection, goroutine and file
descriptor leaks, provider outage, metrics, redaction, and external `GOWORK=off`
consumption. Integration fixtures are pinned and hermetic; a skipped required test fails
promotion.

Required Go-test gates use a wrapper that parses `go test -json`, proves at least one exact
test started and passed, and rejects `[no tests to run]`, package skips, and required fixture
skips. A broad `-run Storage` exit code alone is not evidence.

## Provider maturity model

The provider evidence report uses four states:

| State | Minimum meaning |
| --- | --- |
| Blocked | A known security or data-loss path exists; production defaults must not enable it. |
| Experimental | Construction and validation tests exist; use is explicitly non-production. |
| Beta | Unified conformance, race, real dependency, failure injection, lifecycle/leak, configuration-negative, metrics, and documentation evidence pass. |
| Stable | Beta evidence plus supported deployment matrix, soak/SLO, upgrade/rollback, external consumer, no required skips, and exact-release evidence pass. |

The current `.mss` schema exposes only `stable`, `beta`, `legacy`, and `planned`. Until an
evidence-status field is added, existing unsafe compatibility paths are marked `legacy`,
clean unimplemented targets are `planned`, and detailed blocked/experimental evidence
lives in this matrix. Provider state is never inferred from the framework version.

| Provider or capability | Evidence at `ee800262` | v1.0.1 action | v1.1.0 target |
| --- | --- | --- | --- |
| Aggregate cache/lock/queue | Declared stable; provider evidence does not support it | Retire the stable aggregate and point to provider evidence | Do not restore an aggregate maturity state |
| Global Redis resource | Blocked: unsynchronized, first initializer wins, unclear close ownership | Mark legacy and inventory active scopes; move ownership repair to v1.0.2+ | Stable named resource after standalone/Sentinel/cluster/TLS conformance |
| Redis derived cache | Beta: query generation tests exist, but API and lifecycle remain broad | Keep beta; add failure/result metadata evidence in v1.0.2+ | Stable scoped key/value cache; QueryCache remains separately beta |
| Redis verification state | Blocked security path | Ship atomic cryptographic safety fix; do not call it stable | Stable ChallengeStore after concurrency, limits, outage, and redaction suites |
| Memory EventBus | Existing memory queue is not a production broadcast contract | Do not market it as a durable queue | Stable single-process EventBus after shared suite |
| Redis EventBus | Existing Redis queue lacks explicit fan-out/reconciliation contract | Keep separate and planned; ownership/reconciliation starts in v1.0.2+ | Stable fan-out plus revision reconciliation and outage behavior |
| Redis WorkQueue | Experimental | Separate it from EventBus | Remain experimental unless retry/dead-letter suite passes |
| Kafka WorkQueue | Blocked: message is marked before handler success | Fix acknowledgement and keep experimental | Experimental; not a v1.1 stable commitment |
| NSQ WorkQueue | Blocked: duration, process-exit, and cancellation defects | Keep blocked/legacy in v1.0.1; fix in a v1.0.2+ dedicated slice or remove | Experimental or remove from default build |
| Redis lock | Experimental: no Admin consumer and no focused coverage | Downgrade aggregate claim and defer | Experimental until a fenced consumer and conformance exist |
| Local ObjectStore | Blocked: late size check, implicit fallback, collision/overwrite risk | Fail closed, hard-limit, randomize, no-clobber; remain beta | Stable after the common provider suite and delivery proof |
| S3-compatible ObjectStore | Experimental: per-request client, ambiguous credentials, skipped external tests | Fail closed and retain experimental status | Stable only after pinned MinIO and supported real-provider matrix |
| S3 configuration source | Experimental/beta bootstrap | Keep independent and non-stable; strict bootstrap work starts in v1.0.2+ | Beta with explicit independent ownership |
| WebSocket Redis pub/sub | Unwired at the baseline: no `SetRedisClient` call exists | Move into the resource inventory and report the single-instance boundary | Beta realtime bus after clustered conformance; never a durable-work claim |

## Consequences

Positive consequences:

- failure behavior follows the data's authority instead of a common Redis helper;
- the process composition root owns startup, readiness, cancellation, and close;
- security state, derived caches, broadcasts, and durable work cannot be confused by
  interface convenience;
- provider promotion is reviewable and reproducible;
- object bytes, object delivery, and configuration bootstrap have independent trust and
  ownership boundaries.

Costs and constraints:

- configuration and exported Go APIs may change substantially;
- conformance fixtures and failure injection are a meaningful delivery investment;
- mixed old/new ownership during rollout is dangerous and must be scoped tightly;
- a clean API removal may require `mss-boot/v2.0.0`. If so, release engineering must not
  label the breaking artifact as `v1.1.0`; a v1.1 line may add clean packages and an
  explicitly temporary bridge, while the target architecture remains unchanged.

## Rollout and recovery

`v1.0.1` closes known security and data-integrity paths without attempting the full package
replacement. `v1.0.2` establishes strict schemas, version identity, preflight, and hermetic
fixtures. `v1.0.3` proves upgrade and failure-injection scenarios. The v1.1 prereleases then
land the resource graph, named Redis, ChallengeStore, EventBus, ObjectStore/Delivery, and
provider conformance in independently reversible slices.

During a failed rollout, stop listeners and consumers, preserve authoritative database,
challenge, offset, object, and sanitized diagnostic evidence, then close the owned resource
graph in reverse order. Prefer an idempotent forward fix. Never recover by silently changing
object provider, resetting offsets without an explicit replay plan, reconstructing security
state from a cache, or restoring early acknowledgement and process termination.

## Completion definition

This ADR is implemented only when:

1. the composition root proves required resources ready before listeners start;
2. no runtime consumer creates, replaces, exposes, or closes a hidden global provider client;
3. derived cache, ChallengeStore, EventBus, WorkQueue, lock, ObjectStore, Delivery, and S3
   bootstrap have separate contracts and failure semantics;
4. the strict negative configuration matrix and lifecycle/leak suite pass;
5. ChallengeStore security, EventBus reconciliation, ObjectStore Local/MinIO, and Admin
   integration suites pass under race detection;
6. a provider-by-provider report contains no skipped required evidence;
7. an external `GOWORK=off` consumer constructs, readies, uses, and closes the released
   framework; and
8. the release tag follows a truthful SemVer decision for any public API removal.

The next executable step is the v1.0.1 internal/provisional challenge-state safety slice:
write the concurrent issue/verify contract tests first, replace weak generation and
non-atomic transitions, and run the focused race suite before changing unrelated provider
packages. This closes the known account-takeover path without freezing the public Storage
Runtime v2 `ChallengeStore` API before its v1.1 alpha.1 gate.
