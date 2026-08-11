// Package cache provides database-authoritative, reconstructable caches over
// a named redisresource scope.
//
// A Derived cache owns no Redis client and never closes its Scope. Its Policy
// explicitly fixes the authority, namespace, TTL, payload bound, failure mode,
// and reconstruction contract. Provider failures bypass to the authority;
// authentication state and other fail-closed state do not belong here.
//
// QueryCache is an opt-in GORM adapter. It preserves not-found and
// RowsAffected metadata and bypasses shared state whenever the supplied GORM
// handle is an active transaction.
package cache
