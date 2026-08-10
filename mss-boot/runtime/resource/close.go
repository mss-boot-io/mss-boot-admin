package resource

import (
	"context"
	"errors"
)

// closeGeneration binds one provider cleanup attempt to the result observed
// by every caller that joined it. The result is written before done is closed
// and is immutable afterward.
type closeGeneration struct {
	done   chan struct{}
	result error
}

// Close irreversibly rejects new Start, Run, Health, and Ready work, cancels
// active lifecycle work, and releases every acquired resource in reverse
// topological order. Concurrent callers share the active close attempt. If an
// attempt fails or reaches its deadline, a later Close retries only resources
// that have not yet closed successfully.
func (g *Graph) Close(ctx context.Context) error {
	if g == nil {
		return ErrInvalidGraph
	}
	if ctx == nil {
		return ErrContextRequired
	}

	g.mu.Lock()
	if g.closed {
		g.mu.Unlock()
		return nil
	}
	g.closeRequested = true
	startCancel := g.startCancel
	runCancel := g.runCancel
	startDone := g.startDone
	runDone := g.runDone
	g.mu.Unlock()

	if startCancel != nil {
		startCancel()
	}
	if runCancel != nil {
		runCancel()
	}
	if err := ctx.Err(); err != nil {
		return lifecycleError("", OperationClose, err)
	}
	if err := waitForLifecycle(ctx, startDone); err != nil {
		return err
	}
	if err := waitForLifecycle(ctx, runDone); err != nil {
		return err
	}
	if err := g.waitForInspections(ctx); err != nil {
		return err
	}

	g.mu.Lock()
	if g.allClosedLocked() {
		g.closed = true
		g.mu.Unlock()
		return nil
	}
	if g.activeClose != nil {
		generation := g.activeClose
		g.mu.Unlock()
		return waitForCloseGeneration(ctx, generation)
	}

	generation := &closeGeneration{done: make(chan struct{})}
	g.activeClose = generation
	indexes := g.openIndexesLocked()
	g.mu.Unlock()

	result := g.closeIndexes(ctx, indexes, OperationClose)

	g.mu.Lock()
	if g.allClosedLocked() {
		g.closed = true
	}
	generation.result = result
	g.activeClose = nil
	close(generation.done)
	g.mu.Unlock()
	return result
}

func waitForCloseGeneration(ctx context.Context, generation *closeGeneration) error {
	select {
	case <-generation.done:
		return generation.result
	case <-ctx.Done():
		return lifecycleError("", OperationClose, ctx.Err())
	}
}

func (g *Graph) openIndexesLocked() []int {
	indexes := make([]int, 0, len(g.nodes))
	for index := range g.nodes {
		if g.nodes[index].acquired && !g.nodes[index].closed {
			indexes = append(indexes, index)
		}
	}
	return indexes
}

func (g *Graph) allClosedLocked() bool {
	for index := range g.nodes {
		if g.nodes[index].acquired && !g.nodes[index].closed {
			return false
		}
	}
	return true
}

// closeIndexes receives topological indexes and always traverses them in
// reverse. Successful nodes are never closed again. Errors do not prevent
// best-effort cleanup of earlier dependencies while the context remains live.
func (g *Graph) closeIndexes(ctx context.Context, indexes []int, operation Operation) error {
	var result error
	for position := len(indexes) - 1; position >= 0; position-- {
		if err := ctx.Err(); err != nil {
			result = errors.Join(result, lifecycleError("", operation, err))
			break
		}
		index := indexes[position]

		g.mu.Lock()
		alreadyClosed := g.nodes[index].closed
		name := g.nodes[index].name
		resource := g.nodes[index].resource
		g.mu.Unlock()
		if alreadyClosed {
			continue
		}

		err := resource.Close(ctx)
		if err == nil {
			g.mu.Lock()
			g.nodes[index].closed = true
			g.mu.Unlock()
			if contextErr := ctx.Err(); contextErr != nil {
				result = errors.Join(result, lifecycleError("", operation, contextErr))
				break
			}
			continue
		}
		result = errors.Join(result, lifecycleError(name, operation, err))
		if contextErr := ctx.Err(); contextErr != nil {
			result = joinLifecycleContext(result, operation, contextErr)
			break
		}
	}
	return result
}
