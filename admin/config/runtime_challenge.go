package config

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	runtimeconfig "github.com/mss-boot-io/mss-boot-admin/mss-boot/runtime/config"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/runtime/redisresource"
	runtimeresource "github.com/mss-boot-io/mss-boot-admin/mss-boot/runtime/resource"
)

const (
	defaultChallengeResourceName = "main"
	challengeRedisScopeName      = "challenge.email"
	runtimeCloseLimit            = 5 * time.Second
)

type challengeRuntimeOwner struct {
	graph     *runtimeresource.Graph
	challenge center.RuntimeChallengeImp
}

func (o *challengeRuntimeOwner) Close(ctx context.Context) error {
	if o == nil || o.graph == nil {
		return nil
	}
	return o.graph.Close(ctx)
}

func (e *Config) buildChallengeRuntime(ctx context.Context) (*challengeRuntimeOwner, error) {
	if !e.Challenge.Enabled {
		return nil, nil
	}
	if err := e.Challenge.Validate(); err != nil {
		return nil, ErrChallengeConfigurationInvalid
	}
	snapshot, err := e.Runtime.Normalize(ctx, runtimeconfig.EnvSecretResolver{})
	if err != nil {
		if errors.Is(err, runtimeconfig.ErrInvalidConfiguration) {
			return nil, ErrChallengeConfigurationInvalid
		}
		return nil, ErrChallengeDependencyUnavailable
	}
	plan, err := snapshot.Build(ctx)
	if err != nil {
		return nil, ErrChallengeConfigurationInvalid
	}
	resourceName := strings.TrimSpace(e.Challenge.ResourceRef)
	if resourceName == "" {
		resourceName = defaultChallengeResourceName
	}
	profile, exists := plan.Resource(resourceName)
	if !exists {
		return nil, ErrChallengeConfigurationInvalid
	}
	redisResource, err := redisresource.Build(profile)
	if err != nil {
		return nil, ErrChallengeConfigurationInvalid
	}
	scope, err := redisResource.Scope(challengeRedisScopeName)
	if err != nil {
		return nil, ErrChallengeConfigurationInvalid
	}
	capability, err := e.Challenge.BuildRuntime(scope)
	if err != nil {
		return nil, err
	}
	graph, err := runtimeresource.Build(redisResource.Definition(true))
	if err != nil {
		return nil, ErrChallengeConfigurationInvalid
	}
	if err := graph.Start(ctx); err != nil {
		return nil, errors.Join(ErrChallengeDependencyUnavailable, err)
	}
	if err := capability.Ready(ctx); err != nil {
		closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), runtimeCloseLimit)
		defer cancel()
		return nil, errors.Join(ErrChallengeDependencyUnavailable, err, graph.Close(closeCtx))
	}
	return &challengeRuntimeOwner{graph: graph, challenge: capability}, nil
}

func (e *Config) prepareChallengeRuntime(ctx context.Context) (*challengeRuntimeOwner, bool, error) {
	owner, err := e.buildChallengeRuntime(ctx)
	if err == nil {
		return owner, false, nil
	}
	if e.Challenge.Required {
		return nil, false, fmt.Errorf("initialize required email challenge runtime: %w", err)
	}
	return nil, true, nil
}

func (e *Config) replaceChallengeRuntime(ctx context.Context, next *challengeRuntimeOwner) error {
	e.runtimeMu.Lock()
	previous := e.runtimeOwner
	if previous == next {
		e.runtimeMu.Unlock()
		return nil
	}
	e.runtimeOwner = nil
	e.runtimeMu.Unlock()

	// Remove the non-owning capability first; a closing or unavailable previous
	// graph must never remain observable as a stale fallback.
	center.SetRuntimeChallenge(nil)
	center.SetChallenge(nil)
	if previous != nil {
		closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), runtimeCloseLimit)
		closeErr := previous.Close(closeCtx)
		cancel()
		if closeErr != nil {
			e.runtimeMu.Lock()
			if e.runtimeOwner == nil {
				e.runtimeOwner = previous
			}
			e.runtimeMu.Unlock()
			return fmt.Errorf("close previous challenge runtime: %w", closeErr)
		}
	}

	e.runtimeMu.Lock()
	e.runtimeOwner = next
	e.runtimeMu.Unlock()
	if next != nil {
		center.SetRuntimeChallenge(next.challenge)
	}
	return nil
}

func (e *Config) closeChallengeRuntime(ctx context.Context) error {
	center.SetRuntimeChallenge(nil)
	center.SetChallenge(nil)
	e.runtimeMu.Lock()
	owner := e.runtimeOwner
	e.runtimeMu.Unlock()
	if owner == nil {
		return nil
	}
	err := owner.Close(ctx)
	if err != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
		return fmt.Errorf("close challenge runtime: %w", err)
	}
	e.runtimeMu.Lock()
	if e.runtimeOwner == owner {
		e.runtimeOwner = nil
	}
	e.runtimeMu.Unlock()
	if err != nil {
		return fmt.Errorf("close challenge runtime: %w", err)
	}
	return nil
}

func (e *Config) hasChallengeRuntimeOwner() bool {
	e.runtimeMu.Lock()
	defer e.runtimeMu.Unlock()
	return e.runtimeOwner != nil
}
