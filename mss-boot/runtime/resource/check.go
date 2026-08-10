package resource

import (
	"context"
	"errors"
)

// Health checks every started resource that implements HealthChecker in
// topological order. All provider errors are joined behind redacted lifecycle
// diagnostics so callers can classify multiple unhealthy resources without
// exposing provider error text.
func (g *Graph) Health(ctx context.Context) error {
	if g == nil {
		return ErrInvalidGraph
	}
	if ctx == nil {
		return ErrContextRequired
	}
	if err := ctx.Err(); err != nil {
		return lifecycleError("", OperationHealth, err)
	}
	if err := g.beginInspection(); err != nil {
		return err
	}
	defer g.finishInspection()

	var result error
	for index := range g.nodes {
		if err := ctx.Err(); err != nil {
			return errors.Join(result, lifecycleError("", OperationHealth, err))
		}
		if g.nodes[index].health == nil {
			continue
		}
		result = errors.Join(result, lifecycleError(
			g.nodes[index].name,
			OperationHealth,
			g.nodes[index].health.Health(ctx),
		))
	}
	if err := ctx.Err(); err != nil && !errors.Is(result, err) {
		result = errors.Join(result, lifecycleError("", OperationHealth, err))
	}
	return result
}

// Ready checks every started resource that implements ReadinessChecker in
// topological order. Start gates only declarations marked Required; Ready is
// also useful for observing optional-resource degradation after startup.
func (g *Graph) Ready(ctx context.Context) error {
	if g == nil {
		return ErrInvalidGraph
	}
	if ctx == nil {
		return ErrContextRequired
	}
	if err := ctx.Err(); err != nil {
		return lifecycleError("", OperationReady, err)
	}
	if err := g.beginInspection(); err != nil {
		return err
	}
	defer g.finishInspection()

	var result error
	for index := range g.nodes {
		if err := ctx.Err(); err != nil {
			return errors.Join(result, lifecycleError("", OperationReady, err))
		}
		if g.nodes[index].readiness == nil {
			continue
		}
		result = errors.Join(result, lifecycleError(
			g.nodes[index].name,
			OperationReady,
			g.nodes[index].readiness.Ready(ctx),
		))
	}
	if err := ctx.Err(); err != nil && !errors.Is(result, err) {
		result = errors.Join(result, lifecycleError("", OperationReady, err))
	}
	return result
}
