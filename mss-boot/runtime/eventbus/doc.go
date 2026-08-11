// Package eventbus provides typed, monotonic revision notifications.
//
// An EventBus is a best-effort signal that tells a current subscriber to
// reload authoritative state. It is deliberately not a durable work queue:
// disconnected consumers may miss events, duplicate and out-of-order events
// are ignored, and there are no acknowledgement, retry, or dead-letter
// semantics. A Reconciler periodically compares the last observed revision
// with an authoritative source so live replicas converge after a miss or a
// commit-before-publish failure.
//
// Build functions validate and copy configuration without starting goroutines
// or performing provider I/O. Redis polling belongs to Redis.Run.
package eventbus
