// Package resource owns the deterministic lifecycle of named Runtime v2
// resources.
//
// A Graph is built only after configuration has been decoded and normalized.
// Build performs graph preflight and immutable in-memory copying only: it does
// not call a resource, open a connection, or start a goroutine. Start acquires
// resources in dependency order and establishes required readiness before it
// succeeds. Run owns optional long-running work until every worker returns.
// Close rejects new lifecycle work and releases acquired resources in reverse
// dependency order.
//
// This package intentionally has no compatibility fallback to the legacy
// process-global storage clients. Provider packages adapt their owned clients
// to the narrow interfaces declared here.
package resource
