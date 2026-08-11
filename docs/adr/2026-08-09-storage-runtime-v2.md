# Build storage capabilities from explicit owned runtime resources

- Status: Proposed
- Date: 2026-08-09
- Baseline: `main@ee800262c035c5f4242aca1841d077554481d2c4` (`v1.0.0`)
- Owners: Framework Platform, Admin Platform, Security
- Feature contracts:
  - `.mss/features/foundation-v1-0-1-storage-safety.yaml`
  - `.mss/features/storage-runtime-v2.yaml`
  - `.mss/features/foundation-v1-1-0-release.yaml`
- Target release: `v1.1.0`; preceding milestones are untagged development waves

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

### Internal D0 upload-admission checkpoint

The current internal working-tree checkpoint is deliberately narrower than the target
ObjectStore architecture in this ADR. It proves only the shared HTTP upload-admission
boundary and the confined Local write boundary:

- `storage:maxSize` is an integer byte count. Its default is 10 MiB
  (`10485760` bytes), and values above the hard ceiling of 100 MiB
  (`104857600` bytes) are rejected.
- `storage:allowedTypes` is a comma-separated allowlist of MIME media types and
  type wildcards such as `image/png` and `image/*`. Filename extensions are not
  policy inputs.
- The request body is bounded before multipart parsing, the selected file stream is
  checked with max-plus-one reads, and temporary multipart data is removed.
- The service generates an opaque canonical UUID key below `uploads/`. A user ID or
  original filename is never part of the physical key; the original filename is
  response metadata only.
- The Local path is confined with `os.Root`, opened create-only with `O_EXCL`, and
  removes a partial object on size, copy, cancellation, sync, or close failure.

That D0 checkpoint did **not** promote either Local or S3-compatible storage. At its
boundary, provider selection could still fall through to Local, provider settings were
not one immutable validated profile, client ownership was not singular, and returned URL
strings did not prove a delivery contract. D1 closes the profile and ownership paths
described below. S3 create-only writes and the Local/S3-compatible common provider suite
remain longer-term provider-maturity work outside the v1.1.0 release gate.

### Internal D1 object provider/owner checkpoint

The object-storage subset of `D1-provider-owner` is now implemented:

- `mss-boot/pkg/config.Storage` is a strict Local-or-S3 discriminator. `Normalize`
  validates and copies one immutable profile, resolves typed `env://` `SecretRef`
  values without retaining them in printable fields, and rejects incomplete or
  contradictory provider, credential, endpoint, TLS, region, bucket, or root state.
- One normalized profile builds at most one `StorageHandle`. Both Local and S3 work is
  leased through `Use`; `Close` rejects new leases, drains existing work, closes the
  owned HTTP transport once, is idempotent, and can be retried after a deadline.
- Admin's composition root installs the handle and pinned filesystem before registering
  Application delivery, then closes both from the same owner. Concurrent close is
  idempotent and the Config mutex is not held across a draining callback, so a callback
  can re-enter the lease boundary without deadlock. The ghost
  `Config.Storage.Init()` client and per-request S3 client
  construction are gone. The S3 configuration source uses a separate profile, handle,
  caller context, and close owner because bootstrap and application storage are different
  ownership domains. Only a missing stage object is an optional overlay; transport/read
  failure and malformed overlay data fail bootstrap closed. An HTTP bootstrap endpoint
  requires the exact explicit `s3_tls_allow_insecure_http=true` opt-in, and invalid boolean
  values are rejected.
- Storage AppConfig is now exactly the non-secret upload-policy pair
  `storage:maxSize` and `storage:allowedTypes`. Provider, endpoint, bucket, TLS,
  credential mode, and credential material are startup configuration only. Historical
  rows are not projected; attempts to write removed keys reject the whole request.
- Storage remains optional to the Admin process. Missing, invalid, unresolved, or
  unavailable profiles are not installed; the process continues, both upload routes
  return a fixed `STORAGE_UNAVAILABLE` 503, and no Local or remote write occurs.
- Local is installed only in development mode when `application.staticPath` maps one URL
  prefix to the exact configured absolute root. The owner opens that root once as an
  `os.Root`; writes and the pinned `StaticFS` route share the same directory handle, and
  shutdown closes it. Production Local is unavailable. A returned development Local URL
  is covered by a real static-route read after router reconstruction.
- D1 may construct and own one S3 client, but Admin rejects S3 upload before `Put`.
  Private object metadata, create-only S3 semantics, authorization, `Delivery`, and a
  complete S3-compatible conformance fixture remain future optional work.

Local and S3-compatible storage therefore remain `legacy` in the machine catalog and
**Blocked** in the evidence matrix. The following Kafka lifecycle checkpoint closes the
other D1 subset without promoting any WorkQueue provider.

### v1.1.0 object-provider scope decision

On 2026-08-11 the maintainer explicitly excluded application ObjectStore promotion from the
v1.1.0 release prerequisite set. The implementation is treated as a trusted existing boundary;
this decision does not claim new validation or change provider maturity. v1.1.0 does not start a
RustFS container and does not require the complete Local/S3-compatible, object authorization,
Delivery, or application-provider matrix. If the frozen candidate changes object-storage code,
the release scope is limited to affected compilation, already existing focused owner/configuration
tests, and one basic fail-closed or development-Local smoke. If those paths are unchanged, there is
no object-specific freeze gate.

Local and S3-compatible remain Legacy/Blocked. Production Local stays unavailable, and Admin still
returns unavailable before S3 `Put`; incomplete Delivery is not enabled merely to complete v1.1.0.
The Foundation release may proceed with that honest non-promoted state. Full provider and
authorization conformance may be scheduled as an optional post-v1.1 wave.

### Internal D1 Kafka lifecycle checkpoint

The Kafka half of `D1-provider-owner` is now implemented as an additive ownership layer:

- The historical `AdapterQueue` interface and its `Register`, `Run`, and `Shutdown`
  methods remain available for source compatibility. New composition roots use
  `ManagedAdapterQueue`, which adds error-returning `RegisterContext`, blocking
  `Start`, observable `Errors`, and context-bounded `Close`.
- `Queue.InitContext` builds configuration under the caller's context and returns
  validation, authentication, TLS, and client-construction errors. The old `Queue.Init`
  wrapper is non-authoritative and logs a bounded compatibility error instead of calling
  `Exit` or `Fatal`. MSK token creation is bound to the owner context; TLS verifies
  certificates by default.
- A Kafka adapter validates and copies configuration once, owns exactly one synchronous
  producer, and rejects per-append Kafka configuration. Each accepted unique
  `{topic, group}` registration owns one consumer-group client; duplicate or post-start
  registration fails without replacing the existing consumer. Because no explicit
  manual-commit protocol exists, disabling Sarama auto-commit is rejected.
- `Start` owns both consume and consumer-error observer goroutines, blocks until owner
  cancellation or a consume-loop failure, drains each consumer group's `Errors()` into
  the observable adapter error stream, and cancels peer work before close. `Close` rejects
  new appends/registrations, closes every consumer and the producer exactly once, waits
  for in-flight operations, honors the caller deadline, and may be retried after timeout.
- The Casbin watcher registers with the caller context and returns managed registration
  errors. Admin Config is the single queue owner, installs a candidate only after database
  and watcher binding succeeds, exposes it to the server lifecycle as a `Runnable`, and
  closes it before retiring the database handle. The Runnable also consumes `Errors()`,
  cancels managed `Start` on a runtime error, drains buffered errors when `Start` completes,
  and returns the joined diagnostic to the lifecycle manager. The legacy center handle is
  only a compatibility reference and does not own or detach the queue.

Together with the canonical `admin/modules/<name>` target established for new scaffold,
these changes close `D1-provider-owner`; development proceeds to
`D2-contract-substrate`. Kafka nevertheless remains `legacy` in the machine catalog and
**Blocked** here: manual commit, retry/backoff, dead letter, rebalance, outage,
duplicate/idempotency behavior, and a non-skipped real-broker suite are still unproved.

### Internal D3 resource lifecycle graph checkpoint

Commit `d90b4c7`, with deterministic close-generation evidence repaired by `c830b5f`
and the provider error-tree boundary repaired by `c57ffc8`, establishes the
domain-neutral `mss-boot/runtime/resource` boundary as the first
`D3-backend-runtime` checkpoint:

- `Build` validates canonical unique names, dependencies, missing references, cycles,
  required-readiness support, and duplicate pointer ownership. It copies the declaration,
  returns a deterministic topological order, and invokes no resource method or asynchronous
  work.
- `Start` is a one-way operation. It acquires in topological order, completes a required
  resource's `Ready` check before starting a dependent, marks partial acquisition before
  invoking provider code, and joins the original failure with reverse-order rollback errors
  under an independent bounded context.
- `Run` owns optional long-running workers, cancels peers after a failure or unexpected clean
  exit, and waits for every participating `Run` call. `Close` rejects new lifecycle work,
  cancels an active start or run, waits for bounded inspections, and releases acquired handles
  in reverse order. Concurrent callers share one close generation; a deadline or resource
  failure can be retried without closing an already released handle twice.
- `Health`, `Ready`, and lifecycle errors expose only validated resource names and fixed
  operations. Provider text and provider error objects are unreachable through recursive
  `Unwrap` or `errors.As`; fixed lifecycle metadata and `errors.Is` classification remain
  available without publishing the provider object.

The checked-in checkpoint evidence requires every one of the eleven top-level resource-graph
tests to run and pass twenty uncached times under the race detector. These are hermetic state
machine and owned-handle tests. They do **not** prove a real provider's health behavior, actual
goroutine or file-descriptor leak bounds, or that Admin establishes required readiness before
opening listeners. Those claims remain in the existing feature-freeze lifecycle and Admin
integration gates, including the one-hundred-cycle leak suite.

### Internal D3 named Redis resource checkpoint

Commit `86c0e8a`, composed on the redacted resource boundary repaired at `c57ffc8`, adds
`mss-boot/runtime/redisresource` without changing the legacy process-global Redis adapters:

- `Build` accepts only a normalized `ProviderRedis` profile, copies it without side effects,
  and defers client construction and all I/O to `Start`. Standalone, Sentinel, and cluster
  options enable caller-context timeouts; cluster also defensively forces database zero.
- One named `Resource` owns exactly one go-redis client and contributes one graph definition.
  Reused canonical `Scope` capabilities qualify every key with both resource and scope names,
  reject raw hash-tag syntax and cross-scope keys, and never expose the client or its `Close`.
- `Scope.Use` is a structured lease. Commands inherit the `Use` context, callback return
  cancels and drains the lease, retained or detached work receives a typed rejection, and
  resource close rejects new scopes and uses while draining active work.
- `Start`, `Ready`, and `Health` use caller-scoped `PING`. A single tracked close generation
  invokes the provider's context-free `Close` exactly once in the background; callers may time
  out while a later `Close` joins the same generation and receives the same sanitized result.
- A missing key maps to provider-neutral `ErrNotFound`. Public errors retain only fixed package
  classes and caller cancellation/deadline classification; provider objects, credentials, CA,
  certificate material, and arbitrary provider text do not enter the public error tree.
- Cluster `Delete` and `Exists` validate all keys, then execute one key at a time to avoid
  `CROSSSLOT` and scope-wide hash-tag hotspots. The operation is deliberately non-atomic and
  returns the completed partial count with a fixed error on the first failure.

The checkpoint command requires all twenty-two top-level `runtime/redisresource` tests to pass
twenty uncached race-detected times. The matrix includes factory-injected topology/TLS
construction, namespace and close races, a real stalled `net.Pipe` deadline, and standalone
miniredis. It is **not** real Sentinel, cluster, or TLS provider conformance. Runtime v2 also has
no separate Sentinel control-plane credential references, so Sentinel control-plane ACL is
anonymous here. At this checkpoint the package did not yet provide the server-owned same-slot
atomic group needed to bridge ChallengeStore; the following `1faa9ef` checkpoint adds that bridge
without changing this provider-evidence boundary. Admin readiness-before-listen composition and real
goroutine/file-descriptor bounds remain feature-freeze gates. Consequently
`platform.storage-runtime-v2` remains Planned.

### Internal D3 Challenge runtime checkpoint

Commit `1faa9ef` adds the public additive `mss-boot/runtime/challenge` API over one named
`redisresource.Scope` and keeps the provider boundary internal:

- `NewRedis` copies and validates secrets without I/O and owns neither the shared client nor
  `Close`. Consumers receive `BeginIssue`, opaque reservation `Commit`/`Abort`, and a collapsed
  `Verify` result; raw clients, physical keys, hash tags, and arbitrary script execution remain absent.
- `runtime/internal/redisbridge` derives opaque same-slot groups and keys on the server side and
  permits only fixed repository Challenge scripts inside a structured lease. Cross-group input,
  an invalid script, detached work, and provider errors are rejected or sanitized at the adapter.
- Caller/global rate scripts recognize an existing operation before applying the cardinality limit,
  so a committed operation replayed after a lost provider response remains idempotent at the limit;
  partial operation state is rejected.
- Every syntactically valid Verify request performs one fixed read and one fixed completion script.
  Missing or damaged state also performs dummy HMAC work, while public outcomes collapse missing,
  malformed state, expiry, lock, stale state, and an incorrect code to `Rejected`.
- The D0 `pkg/config/storage/cache` exported Challenge types, constructor, and methods remain
  source-compatible and Deprecated. That compatibility bridge receives only the matching replay,
  redaction, and typed-nil repairs and is not a fallback for the public Runtime v2 API.

Five fully anchored single-package evidence commands cover all twenty-two newly introduced top-level
tests with `--count 1`, `--race`, and `--go-work off`; each required test ran and passed uncached with
no skip. This is a Framework development checkpoint only. Admin still consumes the D0 bridge, and no
real multi-node Redis Cluster was started, so listener ordering, singular Admin close ownership,
`CROSSSLOT`, failover, `NOSCRIPT`, and connection recovery remain pending. Standalone stays Beta,
Cluster stays Planned, and the aggregate Storage Runtime stays Planned rather than becoming Stable.

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
- local object storage without a confined absolute root; the Admin composition root then
  refuses installation unless development `application.staticPath` serves that exact root.

Secrets use typed `SecretRef` values. Admin object-storage `AppConfig` contains only the
non-secret upload policy; it cannot choose a provider/profile, persist credentials, or
switch the startup profile at runtime.

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
      endpoint: https://rustfs.example
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

WorkQueue is a different interface. Every Kafka, NSQ, and Redis implementation keeps its own
evidence state until it proves acknowledgement after handler success, bounded retry with
backoff, dead-letter handling, cancellation and rebalance, duplicate/idempotency behavior,
real-provider operation, owned lifecycle, and observability. The internal D0 Kafka checkpoint
proves local `MarkMessage` ordering and session-cancellation behavior; D1 adds strict
caller-context construction, one producer plus owned consumer groups, error observation,
cancellable blocking start, and bounded close. Both use hermetic fakes. They do not prove broker
offset commit, manual-commit semantics, retry/dead-letter, rebalance, outage, or idempotency and
therefore leave Kafka Blocked/legacy. v1.1.0 does not promise stable promotion for any WorkQueue
provider.

### 9. Defer distributed-lock promotion

No current Admin production consumer justifies a stable distributed-lock capability.
The adapter remains legacy or experimental and disabled by default. A future promotion
requires a concrete use case, lease renewal, loss detection, fencing semantics, race and
partition tests, and proof that a database concurrency mechanism is insufficient.

### 10. Separate ObjectStore from Delivery

This section is the target contract, not a claim that the internal D0/D1 checkpoints
already provide ObjectStore or Delivery. D1 supplies only strict profile and ownership;
create-only S3 behavior, common provider conformance, and authenticated Delivery remain
optional post-v1.1 maturity work rather than v1.1.0 feature-freeze prerequisites.

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

| Provider or capability | Evidence at `ee800262` | Development-wave action | v1.1.0 freeze treatment |
| --- | --- | --- | --- |
| Runtime resource graph | Not present at the baseline | D3 adds deterministic Build/Start/Run/Close ownership, a redacted public error tree, and exact hermetic race evidence | Compose Admin readiness and singular close ownership; then pass the 100-cycle real leak and listener-order gates on the frozen SHA |
| Aggregate cache/lock/queue | Declared stable; provider evidence does not support it | Retire the stable aggregate and point to provider evidence | Do not restore an aggregate maturity state |
| Global Redis resource | Blocked: unsynchronized, first initializer wins, unclear close ownership | Keep the global path legacy; D3 adds an independent one-client named resource with isolated scopes and construction/lifecycle evidence | Real standalone/Sentinel/cluster/TLS conformance, Admin composition, Sentinel control-plane ACL support, and the required frozen-SHA lifecycle gates before any provider promotion |
| Redis derived cache | Beta: query generation tests exist, but API and lifecycle remain broad | Keep beta; add failure/result metadata evidence before freeze | Stable scoped key/value cache; QueryCache remains separately beta |
| Redis verification state | Blocked security path | D0 ships the Admin safety bridge; D3 adds the public Scope-based Challenge API, opaque fixed-script bridge, replay-safe rate limit, equal valid-Verify I/O, and exact new-test evidence without calling it stable | Compose the public API into Admin and separately decide promotion only after the required real Cluster/failover and frozen-SHA evidence |
| Memory EventBus | Existing memory queue is not a production broadcast contract | Do not market it as a durable queue | Stable single-process EventBus after shared suite |
| Redis EventBus | Existing Redis queue lacks explicit fan-out/reconciliation contract | Keep separate and planned; ownership/reconciliation lands in D5 | Stable fan-out plus revision reconciliation and outage behavior |
| Redis WorkQueue | Experimental | Separate it from EventBus | Remain experimental unless retry/dead-letter suite passes |
| Kafka WorkQueue | Blocked: message is marked before handler success | D0 lands hermetic Mark-order/session-cancellation safety; D1 adds non-terminating strict configuration/registration, one producer and owned consumer groups, observed errors, cancellable Start, bounded Close, and Admin Runnable ownership. Retain Blocked/legacy | Eligible for Experimental reassessment only after explicit manual-commit policy, retry/backoff, dead-letter, rebalance, outage, duplicate/idempotency, and non-skipped real-broker conformance; not a v1.1 stable commitment |
| NSQ WorkQueue | Blocked: duration, process-exit, and cancellation defects | Keep blocked/legacy; fix in a dedicated development wave or remove | Experimental or remove from default build |
| Redis lock | Experimental: no Admin consumer and no focused coverage | Downgrade aggregate claim and defer | Experimental until a fenced consumer and conformance exist |
| Local ObjectStore | Blocked at v1.0.0 by late size checks, implicit fallback, collision/overwrite risk, and unproven Delivery | D0 landed admission and confined opaque create-only writes; D1 now adds strict startup profile, one owner, fail-closed 503 behavior, and development-only exact static delivery. Retain legacy/Blocked | Not a release blocker and not promoted. Only if changed: affected compile, existing focused owner/config tests, and one basic smoke. Full authorization/metadata/Delivery conformance is optional post-v1.1 work |
| S3-compatible ObjectStore | Blocked at v1.0.0 by per-request clients, ambiguous credentials, overwrite semantics, synthesized URLs, and skipped external tests | D1 now validates an immutable profile and owns one client, but deliberately returns unavailable before Put. Retain legacy/Blocked | Not a release blocker and not promoted. Admin stays unavailable before Put; no RustFS is started. Conditional create, metadata, Delivery, and complete provider conformance are optional post-v1.1 work |
| S3 configuration source | Experimental/beta bootstrap | D1 gives bootstrap its own profile, handle, caller context, response-body cleanup, unsupported-Watch error, and close owner; it is not reused as an application client | Its in-scope bootstrap checks remain independent from application ObjectStore promotion; missing optional application-provider/RustFS evidence cannot promote or block the Foundation release |
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

Development waves close known security and data-integrity paths before they assemble the full
package replacement. D1 has established strict object startup profiles, fail-closed upload behavior,
owned object resources, and the additive managed Kafka lifecycle without changing Kafka's
Legacy/Blocked maturity. D2 establishes strict schemas, version
identity, migration preflight, and hermetic fixtures. D3 now has the resource graph and named
Redis construction/lifecycle checkpoints plus the public Challenge runtime and internal atomic bridge;
D3-D5 still land Admin composition, EventBus, upgrade paths, and selected provider evidence in
independently reversible commits. ObjectStore/Delivery maturity moves to an optional post-v1.1 wave.
None of these waves creates a public tag.

After the architecture and selected provider scope reach `FF-v1.1.0`, one frozen commit enters
concentrated conformance: complete database, browser, selected in-scope provider, failure-injection, lifecycle,
upgrade, external-consumer, recovery, verify, and eval matrices. Publication authority is a later
phase and does not determine whether the target architecture is feature-complete. Missing optional
ObjectStore/RustFS evidence leaves those providers Legacy/Blocked and does not block publication.

During a failed rollout, stop listeners and consumers, preserve authoritative database,
challenge, offset, object, and sanitized diagnostic evidence, then close the owned resource
graph in reverse order. Prefer an idempotent forward fix. Never recover by silently changing
object provider, resetting offsets without an explicit replay plan, reconstructing security
state from a cache, or restoring early acknowledgement and process termination.

## Completion definition

The following list describes the ADR's full long-term target, not the narrower v1.1.0 release
definition. By the scope decision above, the ObjectStore/Delivery and RustFS portions may remain
incomplete after v1.1.0 without blocking the Foundation release:

1. the composition root proves required resources ready before listeners start;
2. no runtime consumer creates, replaces, exposes, or closes a hidden global provider client;
3. derived cache, ChallengeStore, EventBus, WorkQueue, lock, ObjectStore, Delivery, and S3
   bootstrap have separate contracts and failure semantics;
4. the strict negative configuration matrix and lifecycle/leak suite pass;
5. ChallengeStore security and EventBus reconciliation pass for the selected release scope; the
   ObjectStore Local/S3-compatible suite is completed only in an optional later maturity wave;
6. a provider-by-provider report contains no skipped required evidence;
7. an external `GOWORK=off` consumer constructs, readies, uses, and closes the released
   framework; and
8. the release tag follows a truthful SemVer decision for any public API removal.

The next executable D3 step is to compose the named Redis definition and public Challenge API into
Admin so required readiness completes before listeners, one reverse owner closes the resource,
delivery uses Begin/Commit/Abort, and consumers use the collapsed Verify result. Real
Sentinel/cluster/TLS/failover fixtures and goroutine/file-descriptor evidence remain
freeze gates, not checkpoint claims. In parallel, the Generator/Blueprint track can implement
the supplier backend against the canonical `admin/modules/<name>` contract. Kafka, Local, and
S3-compatible storage remain Legacy/Blocked; S3 Put, conditional create-only writes,
authenticated Delivery, and any RustFS conformance remain optional post-v1.1 work rather than
feature-freeze gates.
