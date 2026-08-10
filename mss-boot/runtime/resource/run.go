package resource

import (
	"context"
	"errors"
	"sync"
)

type runResult struct {
	name       string
	err        error
	contextErr error
}

// Run owns every optional Runner concurrently. The first unexpected error
// cancels peers, then Run waits for all of them before returning a joined,
// redacted diagnostic. Caller cancellation remains classifiable as the caller
// context error. Cancellation initiated by Close is a normal shutdown path.
func (g *Graph) Run(ctx context.Context) (result error) {
	if g == nil {
		return ErrInvalidGraph
	}
	if ctx == nil {
		return ErrContextRequired
	}
	if err := ctx.Err(); err != nil {
		return lifecycleError("", OperationRun, err)
	}

	runCtx, cancel, runners, err := g.beginRun(ctx)
	if err != nil {
		return err
	}
	defer func() {
		cancel()
		g.finishRun()
	}()
	if len(runners) == 0 {
		return nil
	}

	results := make(chan runResult, len(runners))
	var wait sync.WaitGroup
	wait.Add(len(runners))
	for _, current := range runners {
		current := current
		go func() {
			defer wait.Done()
			runErr := current.runner.Run(runCtx)
			results <- runResult{name: current.name, err: runErr, contextErr: runCtx.Err()}
		}()
	}

	unexpected := false
	for range len(runners) {
		current := <-results
		if current.err == nil {
			if current.contextErr != nil {
				continue
			}
			result = errors.Join(result, lifecycleError(current.name, OperationRun, ErrRunStopped))
			if !unexpected {
				unexpected = true
				cancel()
			}
			continue
		}

		if current.contextErr != nil && errors.Is(current.err, current.contextErr) {
			current.err = nonContextRunCause(current.err, current.contextErr)
			if current.err == nil {
				continue
			}
		}

		result = errors.Join(result, lifecycleError(current.name, OperationRun, current.err))
		if !unexpected {
			unexpected = true
			cancel()
		}
	}
	// Every worker has delivered its terminal result. Wait once more for the
	// goroutine epilogues so Run has no graph-owned work after it returns.
	wait.Wait()

	if callerErr := ctx.Err(); callerErr != nil {
		result = errors.Join(result, lifecycleError("", OperationRun, callerErr))
	}
	return result
}

type namedRunner struct {
	name   string
	runner Runner
}

func (g *Graph) beginRun(ctx context.Context) (context.Context, context.CancelFunc, []namedRunner, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.startSucceeded {
		return nil, nil, nil, ErrNotStarted
	}
	if g.closeRequested || g.closed {
		return nil, nil, nil, ErrClosing
	}
	if g.runAttempted {
		return nil, nil, nil, ErrRunRejected
	}

	g.runAttempted = true
	g.runDone = make(chan struct{})
	runCtx, cancel := context.WithCancel(ctx)
	g.runCancel = cancel
	runners := make([]namedRunner, 0, len(g.nodes))
	for index := range g.nodes {
		if g.nodes[index].runner != nil {
			runners = append(runners, namedRunner{name: g.nodes[index].name, runner: g.nodes[index].runner})
		}
	}
	return runCtx, cancel, runners, nil
}

func (g *Graph) finishRun() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.runCancel = nil
	close(g.runDone)
}
