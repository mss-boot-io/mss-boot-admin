// Package redisresource adapts a normalized Runtime v2 Redis profile to one
// exclusively owned go-redis client and the runtime/resource lifecycle.
//
// Build is a pure construction step: it validates and copies the profile but
// does not create a client, resolve a host, open a connection, or start a
// goroutine. Start creates exactly one client and proves connectivity with a
// caller-scoped PING. Health and Ready also use caller-scoped PINGs.
//
// Consumers never receive the owned client. A Resource reuses stable named
// Scope capabilities so multiple consumers share one client without sharing
// keys. Scope.Use lends a deliberately narrow Lease only for the duration of
// its structured callback. Lease commands inherit the Use context, are
// canceled when the callback returns, and retained/detached use is rejected.
// Opaque Key capabilities prevent cross-resource and cross-scope use.
//
// Cluster multi-key Delete and Exists are non-atomic, fail-fast sequences of
// single-key commands; their count is the completed partial result. This
// avoids CROSSSLOT and scope-wide hash-tag hotspots. Cross-key atomic groups
// require a later server-owned capability and are intentionally absent here.
// Sentinel credentials currently cover Redis data nodes only because Runtime
// v2 has no separate Sentinel control-plane credential references; control
// plane authentication is therefore anonymous in this checkpoint.
package redisresource
