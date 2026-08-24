package dev

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// Action is the development lifecycle operation represented by a report.
type Action string

const (
	ActionStart  Action = "start"
	ActionStatus Action = "status"
	ActionStop   Action = "stop"

	initialAdminPasswordEnvironment = "MSS_ADMIN_INITIAL_PASSWORD"
	managedLaunchMismatchDetail     = "did not match the managed launch identity"
)

var errProcessNotRunning = errors.New("process is not running")

// ServiceState is persisted under .mss/run for detached processes.
type ServiceState struct {
	ServiceID         string            `json:"serviceId"`
	Generation        string            `json:"generation"`
	PID               int               `json:"pid"`
	StartedAt         time.Time         `json:"startedAt"`
	ProcessStartToken string            `json:"processStartToken"`
	HealthHeader      string            `json:"healthHeader,omitempty"`
	HealthExpectation string            `json:"healthExpectation,omitempty"`
	Command           []string          `json:"command"`
	Directory         string            `json:"directory"`
	LogPath           string            `json:"logPath"`
	Detached          bool              `json:"detached"`
	Environment       map[string]string `json:"environment,omitempty"`
}

// ServiceResult describes one process lifecycle outcome.
type ServiceResult struct {
	ServiceID string    `json:"serviceId"`
	PID       int       `json:"pid,omitempty"`
	Status    string    `json:"status"`
	Healthy   *bool     `json:"healthy,omitempty"`
	Detail    string    `json:"detail,omitempty"`
	LogPath   string    `json:"logPath,omitempty"`
	StartedAt time.Time `json:"startedAt,omitempty"`
}

// Report is a stable machine-readable development lifecycle report.
type Report struct {
	Project     string          `json:"project"`
	Root        string          `json:"root"`
	GeneratedAt time.Time       `json:"generatedAt"`
	Action      Action          `json:"action"`
	Detached    bool            `json:"detached,omitempty"`
	Success     bool            `json:"success"`
	Services    []ServiceResult `json:"services"`
}

// StartOptions controls service selection and foreground behavior.
type StartOptions struct {
	Services []string
	Detach   bool
	Stdout   io.Writer
	Stderr   io.Writer
}

// StopOptions controls service selection and forced termination.
type StopOptions struct {
	Services []string
	Force    bool
}

// JSON returns stable indented JSON.
func (r Report) JSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// Text returns a compact human-readable lifecycle report.
func (r Report) Text() string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "mss dev %s\n", r.Action)
	fmt.Fprintf(&builder, "project: %s\n", r.Project)
	fmt.Fprintf(&builder, "root: %s\n", r.Root)
	if r.Action == ActionStart {
		fmt.Fprintf(&builder, "detached: %t\n", r.Detached)
	}
	fmt.Fprintf(&builder, "success: %t\n\n", r.Success)
	for _, service := range r.Services {
		fmt.Fprintf(&builder, "- %-12s %-16s", service.ServiceID, service.Status)
		if service.PID > 0 {
			fmt.Fprintf(&builder, " pid=%d", service.PID)
		}
		if service.Healthy != nil {
			fmt.Fprintf(&builder, " healthy=%t", *service.Healthy)
		}
		if service.LogPath != "" {
			fmt.Fprintf(&builder, " log=%s", service.LogPath)
		}
		if service.Detail != "" {
			fmt.Fprintf(&builder, " detail=%s", service.Detail)
		}
		builder.WriteByte('\n')
	}
	return builder.String()
}

type managedProcess struct {
	config  *Config
	service ServiceSpec
	cmd     *exec.Cmd
	log     *os.File
	state   ServiceState
	exit    <-chan error
}

type processExit struct {
	serviceID string
	err       error
}

type lifecycleGuard struct {
	release func()
	once    sync.Once
}

func lockLifecycle(parent context.Context, config *Config, timeout time.Duration) (*lifecycleGuard, error) {
	if err := verifyStableConfinedPath(config.Root, config.RuntimeDirectory); err != nil {
		return nil, fmt.Errorf("verify development runtime directory before locking: %w", err)
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	release, err := acquireLifecycleLock(ctx, filepath.Join(config.RuntimeDirectory, ".lifecycle.lock"))
	if err != nil {
		return nil, err
	}
	return &lifecycleGuard{release: release}, nil
}

func (guard *lifecycleGuard) Close() {
	if guard == nil {
		return
	}
	guard.once.Do(func() {
		if guard.release != nil {
			guard.release()
		}
	})
}

func withLifecycleLock(parent context.Context, config *Config, timeout time.Duration, operation func() error) error {
	guard, err := lockLifecycle(parent, config, timeout)
	if err != nil {
		return err
	}
	defer guard.Close()
	return operation()
}

// Start launches selected services in dependency order. Foreground mode blocks until cancellation or exit.
func Start(parent context.Context, config *Config, options StartOptions) (Report, error) {
	if config == nil {
		return Report{}, errors.New("development config is nil")
	}
	ctx, cancel := signal.NotifyContext(parent, terminationSignals()...)
	defer cancel()
	if options.Stdout == nil {
		options.Stdout = io.Discard
	}
	if options.Stderr == nil {
		options.Stderr = options.Stdout
	}
	services, err := config.StartServices(options.Services)
	if err != nil {
		return Report{}, err
	}
	if err := ensureDirectories(config); err != nil {
		return Report{}, err
	}
	guard, err := lockLifecycle(ctx, config, config.StartupTimeout)
	if err != nil {
		return Report{}, err
	}
	defer guard.Close()

	report := Report{
		Project:     config.Document.Metadata.Project,
		Root:        config.Root,
		GeneratedAt: time.Now().UTC(),
		Action:      ActionStart,
		Detached:    options.Detach,
		Success:     true,
		Services:    make([]ServiceResult, 0, len(services)),
	}
	started := make([]*managedProcess, 0, len(services))
	for _, service := range services {
		process, result, startErr := startService(ctx, config, service, options)
		report.Services = append(report.Services, result)
		if startErr != nil {
			report.Success = false
			rollbackErr := stopManagedLocked(context.Background(), config, started, false)
			if rollbackErr != nil {
				startErr = errors.Join(startErr, fmt.Errorf("rollback partially started development services: %w", rollbackErr))
			}
			return report, startErr
		}
		if process != nil {
			started = append(started, process)
		}
	}
	guard.Close()
	if options.Detach {
		for _, process := range started {
			_ = process.log.Close()
		}
		return report, nil
	}
	if len(started) == 0 {
		return report, nil
	}

	if config.foregroundReady != nil {
		config.foregroundReady()
	}
	exits := make(chan processExit, len(started))
	for _, process := range started {
		process := process
		go func() {
			err := <-process.exit
			_ = process.log.Close()
			cleanupErr := cleanupManagedState(config, process.state)
			if cleanupErr != nil {
				err = errors.Join(err, fmt.Errorf("clean exited service state: %w", cleanupErr))
			}
			exits <- processExit{serviceID: process.service.ID, err: err}
		}()
	}

	select {
	case <-ctx.Done():
		if err := stopManaged(context.Background(), config, started, false); err != nil {
			report.Success = false
			return report, err
		}
		return report, nil
	case exited := <-exits:
		cleanupErr := stopManaged(context.Background(), config, started, false)
		if cleanupErr != nil {
			exited.err = errors.Join(exited.err, cleanupErr)
		}
		if exited.err != nil {
			report.Success = false
			return report, fmt.Errorf("development service %s exited: %w", exited.serviceID, exited.err)
		}
		return report, fmt.Errorf("development service %s exited", exited.serviceID)
	}
}

// Status inspects process state and health without mutating running services.
func Status(parent context.Context, config *Config, selected []string) (Report, error) {
	if err := verifyDevelopmentPaths(config); err != nil {
		return Report{}, err
	}
	services, err := config.Services(selected)
	if err != nil {
		return Report{}, err
	}
	report := Report{
		Project:     config.Document.Metadata.Project,
		Root:        config.Root,
		GeneratedAt: time.Now().UTC(),
		Action:      ActionStatus,
		Success:     true,
		Services:    make([]ServiceResult, 0, len(services)),
	}
	for _, service := range services {
		state, stateErr := readState(config.StatePath(service.ID))
		if errors.Is(stateErr, os.ErrNotExist) {
			report.Services = append(report.Services, ServiceResult{ServiceID: service.ID, Status: "stopped", LogPath: relative(config.Root, config.LogPath(service.ID))})
			continue
		}
		if stateErr != nil {
			report.Success = false
			report.Services = append(report.Services, ServiceResult{ServiceID: service.ID, Status: "invalid-state", Detail: stateErr.Error()})
			continue
		}
		result := ServiceResult{
			ServiceID: service.ID,
			PID:       state.PID,
			Status:    "running",
			LogPath:   relative(config.Root, state.LogPath),
			StartedAt: state.StartedAt,
		}
		inspection, detail := inspectServiceState(config, service, state)
		switch inspection {
		case stateProcessStopped:
			result.Status = "stale"
			result.Detail = detail
			report.Success = false
			report.Services = append(report.Services, result)
			continue
		case stateProcessIdentityMismatch:
			result.Status = "identity-mismatch"
			result.Detail = detail
			report.Success = false
			report.Services = append(report.Services, result)
			continue
		case stateProcessUnverified:
			result.Status = "unverified"
			result.Detail = detail
			report.Success = false
			report.Services = append(report.Services, result)
			continue
		}
		if service.Health != nil {
			healthy, detail := checkHealth(parent, service.Health, state.HealthExpectation)
			result.Healthy = boolPointer(healthy)
			if !healthy {
				result.Status = "degraded"
				result.Detail = detail
				report.Success = false
			}
		}
		report.Services = append(report.Services, result)
	}
	return report, nil
}

// Stop terminates selected services in reverse dependency order.
func Stop(parent context.Context, config *Config, options StopOptions) (Report, error) {
	if config == nil {
		return Report{}, errors.New("development config is nil")
	}
	if err := ensureDirectories(config); err != nil {
		return Report{}, err
	}
	guard, err := lockLifecycle(parent, config, config.StopTimeout)
	if err != nil {
		return Report{}, err
	}
	defer guard.Close()

	services, err := config.Services(options.Services)
	if err != nil {
		return Report{}, err
	}
	for left, right := 0, len(services)-1; left < right; left, right = left+1, right-1 {
		services[left], services[right] = services[right], services[left]
	}
	report := Report{
		Project:     config.Document.Metadata.Project,
		Root:        config.Root,
		GeneratedAt: time.Now().UTC(),
		Action:      ActionStop,
		Success:     true,
		Services:    make([]ServiceResult, 0, len(services)),
	}
	var failures []error
	for _, service := range services {
		result, stopErr := stopService(parent, config, service, options.Force)
		report.Services = append(report.Services, result)
		if stopErr != nil {
			report.Success = false
			failures = append(failures, fmt.Errorf("stop development service %s: %w", service.ID, stopErr))
		}
	}
	if !report.Success {
		return report, errors.Join(failures...)
	}
	return report, nil
}

// Logs prints the last lines of one service log and optionally follows new output.
func Logs(ctx context.Context, config *Config, serviceID string, lines int, follow bool, writer io.Writer) error {
	if config == nil {
		return errors.New("development config is nil")
	}
	if writer == nil {
		writer = io.Discard
	}
	if _, exists := config.Service(serviceID); !exists {
		return fmt.Errorf("unknown development service %q", serviceID)
	}
	if lines < 0 {
		return errors.New("lines must be non-negative")
	}
	if err := verifyStableConfinedPath(config.Root, config.LogDirectory); err != nil {
		return fmt.Errorf("verify development log directory: %w", err)
	}
	path := config.LogPath(serviceID)
	if err := writeTail(path, lines, writer); err != nil {
		return err
	}
	if !follow {
		return nil
	}

	if err := verifyStableConfinedPath(config.Root, path); err != nil {
		return fmt.Errorf("reverify development log before following: %w", err)
	}
	file, err := openFileNoFollow(path, os.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("open log %s: %w", path, err)
	}
	defer file.Close()
	if _, err := file.Seek(0, io.SeekEnd); err != nil {
		return err
	}
	reader := bufio.NewReader(file)
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		for {
			chunk, readErr := reader.ReadString('\n')
			if chunk != "" {
				if _, err := io.WriteString(writer, chunk); err != nil {
					return err
				}
			}
			if readErr != nil {
				if errors.Is(readErr, io.EOF) {
					break
				}
				return readErr
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func startService(parent context.Context, config *Config, service ServiceSpec, options StartOptions) (*managedProcess, ServiceResult, error) {
	serviceDirectory, err := config.resolveDirectory(service)
	if err != nil {
		result := ServiceResult{ServiceID: service.ID, Status: "invalid-directory", Detail: err.Error()}
		return nil, result, fmt.Errorf("resolve development service %s directory: %w", service.ID, err)
	}
	if err := verifyStableConfinedPath(config.Root, serviceDirectory); err != nil {
		result := ServiceResult{ServiceID: service.ID, Status: "invalid-directory", Detail: err.Error()}
		return nil, result, fmt.Errorf("verify development service %s directory: %w", service.ID, err)
	}
	statePath := config.StatePath(service.ID)
	if state, err := readState(statePath); err == nil {
		result := ServiceResult{
			ServiceID: service.ID,
			PID:       state.PID,
			Status:    "already-running",
			LogPath:   relative(config.Root, state.LogPath),
			StartedAt: state.StartedAt,
		}
		inspection, detail := inspectServiceState(config, service, state)
		switch inspection {
		case stateProcessStopped:
			if err := removeStateIfMatchingLocked(config, statePath, state); err != nil {
				result.Status = "state-cleanup-failed"
				result.Detail = err.Error()
				return nil, result, err
			}
		case stateProcessIdentityMismatch:
			result.Status = "identity-mismatch"
			result.Detail = detail
			return nil, result, fmt.Errorf("development service %s state identity mismatch: %s", service.ID, detail)
		case stateProcessUnverified:
			result.Status = "unverified"
			result.Detail = detail
			return nil, result, fmt.Errorf("development service %s process identity could not be verified: %s", service.ID, detail)
		case stateProcessManaged:
			if service.Health != nil {
				healthy, healthDetail := checkHealth(parent, service.Health, state.HealthExpectation)
				result.Healthy = boolPointer(healthy)
				result.Detail = healthDetail
				if !healthy {
					result.Status = "degraded"
					return nil, result, fmt.Errorf("development service %s is already running but unhealthy: %s", service.ID, healthDetail)
				}
				if current, currentDetail := inspectServiceState(config, service, state); current != stateProcessManaged {
					result.Status = "identity-mismatch"
					result.Detail = currentDetail
					return nil, result, fmt.Errorf("development service %s changed identity during health check: %s", service.ID, currentDetail)
				}
			}
			return nil, result, nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		result := ServiceResult{ServiceID: service.ID, Status: "invalid-state", Detail: err.Error()}
		return nil, result, fmt.Errorf("read development service %s state: %w", service.ID, err)
	}

	generation, err := newStateGeneration()
	if err != nil {
		return nil, ServiceResult{ServiceID: service.ID, Status: "failed", Detail: err.Error()}, err
	}
	healthHeader := ""
	healthNonce := ""
	healthExpectation := ""
	if service.Health != nil {
		healthHeader = service.Health.LaunchHeader
		healthNonce, err = newHealthNonce()
		if err != nil {
			return nil, ServiceResult{ServiceID: service.ID, Status: "failed", Detail: err.Error()}, err
		}
		healthExpectation = digestHealthNonce(healthNonce)
		if healthy, detail := checkHealth(parent, service.Health, ""); healthy {
			result := ServiceResult{ServiceID: service.ID, Status: "health-already-active", Healthy: boolPointer(true), Detail: detail, LogPath: relative(config.Root, config.LogPath(service.ID))}
			return nil, result, fmt.Errorf("refusing to start development service %s because its health URL is already healthy without a managed process", service.ID)
		}
		if config.afterHealthPreflight != nil {
			config.afterHealthPreflight(service)
		}
	}
	if err := parent.Err(); err != nil {
		result := ServiceResult{ServiceID: service.ID, Status: "cancelled", Detail: err.Error(), LogPath: relative(config.Root, config.LogPath(service.ID))}
		return nil, result, fmt.Errorf("development service %s start cancelled before launch: %w", service.ID, err)
	}
	logPath := config.LogPath(service.ID)
	if err := verifyStableConfinedPath(config.Root, logPath); err != nil {
		return nil, ServiceResult{ServiceID: service.ID, Status: "failed", Detail: err.Error()}, fmt.Errorf("verify %s log path: %w", service.ID, err)
	}
	logFile, err := openFileNoFollow(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, ServiceResult{ServiceID: service.ID, Status: "failed"}, fmt.Errorf("open %s log: %w", service.ID, err)
	}
	command := exec.Command(service.Command[0], service.Command[1:]...)
	command.Dir = serviceDirectory
	serviceEnvironment := copyEnvironment(service.Environment)
	if healthNonce != "" {
		serviceEnvironment[healthNonceEnvironment] = healthNonce
	}
	command.Env = mergeEnvironment(os.Environ(), serviceEnvironment)
	prepareProcess(command)
	if options.Detach {
		command.Stdout = logFile
		command.Stderr = logFile
		command.Stdin = nil
	} else {
		command.Stdout = io.MultiWriter(logFile, options.Stdout)
		command.Stderr = io.MultiWriter(logFile, options.Stderr)
		command.Stdin = os.Stdin
	}
	if err := command.Start(); err != nil {
		_ = logFile.Close()
		return nil, ServiceResult{ServiceID: service.ID, Status: "failed", Detail: err.Error()}, fmt.Errorf("start development service %s: %w", service.ID, err)
	}
	processStartToken, err := readProcessStartToken(config, command.Process.Pid)
	if err != nil {
		cleanupErr := terminateUnidentifiedStartedCommand(config, command, config.StopTimeout)
		_ = logFile.Close()
		detail := err.Error()
		if cleanupErr != nil {
			detail += "; cleanup: " + cleanupErr.Error()
		}
		identityErr := fmt.Errorf("capture development service %s process identity: %w", service.ID, err)
		if cleanupErr != nil {
			identityErr = errors.Join(identityErr, cleanupErr)
		}
		return nil, ServiceResult{ServiceID: service.ID, PID: command.Process.Pid, Status: "failed", Detail: detail}, identityErr
	}
	exit := make(chan error, 1)
	go func() {
		exit <- command.Wait()
		close(exit)
	}()
	state := ServiceState{
		ServiceID:         service.ID,
		Generation:        generation,
		PID:               command.Process.Pid,
		StartedAt:         time.Now().UTC(),
		ProcessStartToken: processStartToken,
		HealthHeader:      healthHeader,
		HealthExpectation: healthExpectation,
		Command:           append([]string(nil), service.Command...),
		Directory:         command.Dir,
		LogPath:           logPath,
		Detached:          options.Detach,
		Environment:       sanitizedEnvironment(service.Environment),
	}
	process := &managedProcess{config: config, service: service, cmd: command, log: logFile, state: state, exit: exit}
	if err := writeManagedState(config, statePath, state); err != nil {
		cleanupErr := terminateIdentifiedStartedCommand(config, process, config.StopTimeout)
		_ = logFile.Close()
		if cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("cleanup development service %s after state write failure: %w", service.ID, cleanupErr))
		}
		return nil, ServiceResult{ServiceID: service.ID, PID: state.PID, Status: "failed", Detail: err.Error()}, err
	}
	result := ServiceResult{
		ServiceID: service.ID,
		PID:       state.PID,
		Status:    "running",
		LogPath:   relative(config.Root, logPath),
		StartedAt: state.StartedAt,
	}
	if service.Health != nil {
		healthy, detail := waitForHealth(parent, service.Health, process)
		result.Healthy = boolPointer(healthy)
		result.Detail = detail
		if !healthy {
			cleanupErr := stopManagedProcessLocked(context.Background(), config, process, false)
			_ = logFile.Close()
			result.Status = "failed-health"
			readinessErr := fmt.Errorf("development service %s failed readiness check: %s", service.ID, detail)
			if cleanupErr != nil {
				readinessErr = errors.Join(readinessErr, fmt.Errorf("cleanup failed readiness service: %w", cleanupErr))
			}
			return nil, result, readinessErr
		}
	}
	return process, result, nil
}

func stopService(parent context.Context, config *Config, service ServiceSpec, force bool) (ServiceResult, error) {
	statePath := config.StatePath(service.ID)
	state, err := readState(statePath)
	if errors.Is(err, os.ErrNotExist) {
		return ServiceResult{ServiceID: service.ID, Status: "already-stopped", LogPath: relative(config.Root, config.LogPath(service.ID))}, nil
	}
	if err != nil {
		return ServiceResult{ServiceID: service.ID, Status: "invalid-state", Detail: err.Error()}, err
	}
	result := ServiceResult{
		ServiceID: service.ID,
		PID:       state.PID,
		Status:    "stopped",
		LogPath:   relative(config.Root, state.LogPath),
		StartedAt: state.StartedAt,
	}
	result.Status, result.Detail, err = stopManagedStateLocked(parent, config, service, state, force)
	if err != nil {
		return result, err
	}
	return result, nil
}

func stopManaged(parent context.Context, config *Config, processes []*managedProcess, force bool) error {
	return withLifecycleLock(parent, config, config.StopTimeout, func() error {
		return stopManagedLocked(parent, config, processes, force)
	})
}

func stopManagedLocked(parent context.Context, config *Config, processes []*managedProcess, force bool) error {
	var failures []error
	for _, process := range processes {
		if process == nil || process.cmd == nil || process.cmd.Process == nil {
			continue
		}
		if err := stopManagedProcessLocked(parent, config, process, force); err != nil {
			failures = append(failures, fmt.Errorf("stop development service %s: %w", process.service.ID, err))
		}
	}
	return errors.Join(failures...)
}

func stopManagedProcessLocked(parent context.Context, config *Config, process *managedProcess, force bool) error {
	_, _, err := stopManagedStateLocked(parent, config, process.service, process.state, force)
	return err
}

func stopManagedStateLocked(
	parent context.Context,
	config *Config,
	service ServiceSpec,
	state ServiceState,
	force bool,
) (string, string, error) {
	inspection, detail := inspectServiceState(config, service, state)
	switch inspection {
	case stateProcessStopped:
		if err := removeStateIfMatchingLocked(config, config.StatePath(service.ID), state); err != nil {
			return "state-cleanup-failed", err.Error(), err
		}
		return "removed-stale-state", detail, nil
	case stateProcessIdentityMismatch:
		err := fmt.Errorf("refusing to signal development service %s because its process identity does not match: %s", service.ID, detail)
		return "identity-mismatch", detail, err
	case stateProcessUnverified:
		err := fmt.Errorf("refusing to signal development service %s because its process identity could not be verified: %s", service.ID, detail)
		return "unverified", detail, err
	}

	if err := signalManagedProcess(config, state, force); err != nil {
		return "stop-failed", err.Error(), err
	}
	inspection, detail, waitErr := waitForManagedStateExit(parent, config, service, state, config.StopTimeout)
	if waitErr != nil && !force && errors.Is(waitErr, context.DeadlineExceeded) {
		// Revalidate immediately before escalation. signalManagedProcess repeats
		// the OS start-token check, so a reused PID is never force-signalled.
		if inspection != stateProcessManaged {
			return inspectionFailure(service.ID, inspection, detail)
		}
		if err := signalManagedProcess(config, state, true); err != nil {
			return "kill-failed", err.Error(), err
		}
		inspection, detail, waitErr = waitForManagedStateExit(parent, config, service, state, config.StopTimeout)
	}
	if waitErr != nil {
		if inspection != stateProcessManaged {
			return inspectionFailure(service.ID, inspection, detail)
		}
		return "stop-timeout", waitErr.Error(), waitErr
	}
	if inspection != stateProcessStopped {
		return inspectionFailure(service.ID, inspection, detail)
	}
	if err := removeStateIfMatchingLocked(config, config.StatePath(service.ID), state); err != nil {
		return "state-cleanup-failed", err.Error(), err
	}
	return "stopped", "", nil
}

func inspectionFailure(serviceID string, inspection stateProcessInspection, detail string) (string, string, error) {
	switch inspection {
	case stateProcessIdentityMismatch:
		return "identity-mismatch", detail, fmt.Errorf("development service %s changed process identity while stopping: %s", serviceID, detail)
	case stateProcessUnverified:
		return "unverified", detail, fmt.Errorf("development service %s process identity became unverifiable while stopping: %s", serviceID, detail)
	case stateProcessStopped:
		return "stopped", "", nil
	default:
		return "stop-failed", detail, fmt.Errorf("development service %s did not stop", serviceID)
	}
}

func waitForManagedStateExit(
	parent context.Context,
	config *Config,
	service ServiceSpec,
	state ServiceState,
	timeout time.Duration,
) (stateProcessInspection, string, error) {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		inspection, detail := inspectServiceState(config, service, state)
		if inspection != stateProcessManaged {
			if inspection == stateProcessStopped {
				return inspection, detail, nil
			}
			return inspection, detail, errors.New(detail)
		}
		select {
		case <-ctx.Done():
			return inspection, detail, ctx.Err()
		case <-ticker.C:
		}
	}
}

type stateProcessInspection int

const (
	stateProcessManaged stateProcessInspection = iota
	stateProcessStopped
	stateProcessIdentityMismatch
	stateProcessUnverified
)

func inspectServiceState(config *Config, service ServiceSpec, state ServiceState) (stateProcessInspection, string) {
	token, err := readProcessStartToken(config, state.PID)
	if errors.Is(err, errProcessNotRunning) {
		return stateProcessStopped, "state file exists but process is not running"
	}
	if err != nil {
		return stateProcessUnverified, err.Error()
	}
	if detail := validateStateIdentity(config, service, state); detail != "" {
		return stateProcessIdentityMismatch, detail
	}
	if token != state.ProcessStartToken {
		return stateProcessIdentityMismatch, "PID was reused by a different process start identity"
	}
	return stateProcessManaged, ""
}

func validateStateIdentity(config *Config, service ServiceSpec, state ServiceState) string {
	if state.ServiceID != service.ID {
		return fmt.Sprintf("state service %q does not match configured service %q", state.ServiceID, service.ID)
	}
	if strings.TrimSpace(state.Generation) == "" {
		return "state does not contain a lifecycle generation"
	}
	if state.PID <= 1 {
		return "state contains an invalid process id"
	}
	if state.StartedAt.IsZero() {
		return "state does not contain the managed start timestamp"
	}
	if strings.TrimSpace(state.ProcessStartToken) == "" {
		return "state does not contain an operating-system process start identity"
	}
	if !equalStrings(state.Command, service.Command) {
		return "state command does not match the configured service command"
	}
	if !equalPath(state.Directory, config.ResolveDirectory(service)) {
		return "state directory does not match the configured service directory"
	}
	if !equalPath(state.LogPath, config.LogPath(service.ID)) {
		return "state log path does not match the configured service log path"
	}
	if service.Health != nil {
		if state.HealthHeader != service.Health.LaunchHeader {
			return "state health launch header does not match the configured service health contract"
		}
		if !validHealthExpectation(state.HealthExpectation) {
			return "state does not contain a valid managed health launch expectation"
		}
	} else if state.HealthHeader != "" || state.HealthExpectation != "" {
		return "state contains a health launch expectation for a service without health checks"
	}
	for key := range state.Environment {
		if isReservedLifecycleEnvironment(key) {
			return "state contains a forbidden lifecycle-owned environment"
		}
	}
	return ""
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func equalPath(left, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func signalManagedProcess(config *Config, state ServiceState, force bool) error {
	token, err := readProcessStartToken(config, state.PID)
	if errors.Is(err, errProcessNotRunning) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("verify process %d before signal: %w", state.PID, err)
	}
	if strings.TrimSpace(state.ProcessStartToken) == "" || token != state.ProcessStartToken {
		return fmt.Errorf("refusing to signal PID %d because its process start identity does not match", state.PID)
	}
	return sendProcessSignal(config, state.PID, force)
}

func readProcessStartToken(config *Config, pid int) (string, error) {
	if config != nil && config.processStartTokenReader != nil {
		return config.processStartTokenReader(pid)
	}
	return processStartToken(pid)
}

func sendProcessSignal(config *Config, pid int, force bool) error {
	if config != nil && config.processSignaler != nil {
		return config.processSignaler(pid, force)
	}
	return signalProcess(pid, force)
}

func newStateGeneration() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("create development lifecycle generation: %w", err)
	}
	return hex.EncodeToString(value[:]), nil
}

func newHealthNonce() (string, error) {
	var value [32]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("create development health launch nonce: %w", err)
	}
	return hex.EncodeToString(value[:]), nil
}

func digestHealthNonce(nonce string) string {
	digest := sha256.Sum256([]byte(nonce))
	return "sha256:" + hex.EncodeToString(digest[:])
}

func validHealthExpectation(expectation string) bool {
	if !strings.HasPrefix(expectation, "sha256:") || len(expectation) != len("sha256:")+sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(strings.TrimPrefix(expectation, "sha256:"))
	return err == nil
}

func writeManagedState(config *Config, path string, state ServiceState) error {
	if config != nil && config.stateWriter != nil {
		return config.stateWriter(path, state)
	}
	return writeState(path, state)
}

func terminateUnidentifiedStartedCommand(config *Config, command *exec.Cmd, timeout time.Duration) error {
	if command == nil || command.Process == nil {
		return errors.New("started command process is unavailable")
	}
	exit := make(chan error, 1)
	go func() {
		exit <- command.Wait()
		close(exit)
	}()
	signalErr := sendProcessSignal(config, command.Process.Pid, true)
	if signalErr != nil {
		// The process is still represented by the handle returned from Start.
		// Kill that direct process as a last resort, but preserve the tree-signal
		// error because descendants may not have been terminated.
		killErr := directKillStartedProcess(command.Process)
		if !waitExitWithin(exit, timeout) {
			return errors.Join(signalErr, killErr, fmt.Errorf("timed out waiting for PID %d after direct forced cleanup", command.Process.Pid))
		}
		return errors.Join(signalErr, killErr)
	}
	if waitExitWithin(exit, timeout) {
		return nil
	}
	timeoutErr := fmt.Errorf("timed out waiting for PID %d after process-tree forced cleanup", command.Process.Pid)
	killErr := directKillStartedProcess(command.Process)
	if !waitExitWithin(exit, timeout) {
		return errors.Join(timeoutErr, killErr, fmt.Errorf("timed out waiting for PID %d after direct forced cleanup", command.Process.Pid))
	}
	return errors.Join(timeoutErr, killErr)
}

func terminateIdentifiedStartedCommand(config *Config, process *managedProcess, timeout time.Duration) error {
	if process == nil || process.cmd == nil || process.cmd.Process == nil || process.exit == nil {
		return errors.New("managed process is unavailable")
	}
	signalErr := signalManagedProcess(config, process.state, true)
	if signalErr != nil {
		killErr := directKillStartedProcess(process.cmd.Process)
		if !waitExitWithin(process.exit, timeout) {
			return errors.Join(signalErr, killErr, fmt.Errorf("timed out waiting for PID %d after direct forced cleanup", process.state.PID))
		}
		return errors.Join(signalErr, killErr)
	}
	if waitExitWithin(process.exit, timeout) {
		return nil
	}
	timeoutErr := fmt.Errorf("timed out waiting for PID %d after process-tree forced cleanup", process.state.PID)
	killErr := directKillStartedProcess(process.cmd.Process)
	if !waitExitWithin(process.exit, timeout) {
		return errors.Join(timeoutErr, killErr, fmt.Errorf("timed out waiting for PID %d after direct forced cleanup", process.state.PID))
	}
	return errors.Join(timeoutErr, killErr)
}

func waitExitWithin(exit <-chan error, timeout time.Duration) bool {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-exit:
		return true
	case <-timer.C:
		return false
	}
}

func directKillStartedProcess(process *os.Process) error {
	if process == nil {
		return errors.New("started process handle is unavailable")
	}
	if err := process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("directly kill started PID %d: %w", process.Pid, err)
	}
	return nil
}

func removeStateIfMatchingLocked(config *Config, path string, expected ServiceState) error {
	if err := verifyStableConfinedPath(config.Root, path); err != nil {
		return fmt.Errorf("verify development state before cleanup: %w", err)
	}
	current, err := readState(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !sameManagedStateIdentity(current, expected) {
		return errors.New("development state changed identity before cleanup")
	}
	if config != nil && config.beforeStateRemove != nil {
		config.beforeStateRemove(expected)
	}
	if err := verifyStableConfinedPath(config.Root, path); err != nil {
		return fmt.Errorf("reverify development state before cleanup: %w", err)
	}
	latest, err := readState(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !sameManagedStateIdentity(latest, expected) {
		return errors.New("development state changed identity immediately before cleanup")
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func sameManagedStateIdentity(left, right ServiceState) bool {
	return left.ServiceID == right.ServiceID &&
		left.Generation == right.Generation &&
		left.PID == right.PID &&
		left.StartedAt.Equal(right.StartedAt) &&
		left.ProcessStartToken == right.ProcessStartToken &&
		left.HealthHeader == right.HealthHeader &&
		left.HealthExpectation == right.HealthExpectation &&
		equalStrings(left.Command, right.Command) &&
		equalPath(left.Directory, right.Directory) &&
		equalPath(left.LogPath, right.LogPath) &&
		left.Detached == right.Detached &&
		equalEnvironment(left.Environment, right.Environment)
}

func equalEnvironment(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}

func cleanupManagedState(config *Config, state ServiceState) error {
	return withLifecycleLock(context.Background(), config, config.StopTimeout, func() error {
		return removeStateIfMatchingLocked(config, config.StatePath(state.ServiceID), state)
	})
}

func managedProcessRunning(process *managedProcess) (bool, string) {
	if process == nil {
		return false, "managed process is unavailable"
	}
	select {
	case exitErr, ok := <-process.exit:
		if !ok {
			return false, "process exited during readiness"
		}
		return false, processExitDetail(exitErr)
	default:
	}
	token, err := readProcessStartToken(process.config, process.state.PID)
	if errors.Is(err, errProcessNotRunning) {
		return false, "process exited during readiness"
	}
	if err != nil {
		return false, "verify process during readiness: " + err.Error()
	}
	if token != process.state.ProcessStartToken {
		return false, "PID changed process identity during readiness"
	}
	return true, ""
}

func processExitDetail(err error) string {
	if err == nil {
		return "process exited during readiness"
	}
	return "process exited during readiness: " + err.Error()
}

func waitForHealth(parent context.Context, health *HealthSpec, process *managedProcess) (bool, string) {
	interval, _ := time.ParseDuration(health.Interval)
	timeout, _ := time.ParseDuration(health.Timeout)
	if interval <= 0 {
		interval = 500 * time.Millisecond
	}
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	var detail string
	for {
		if running, processDetail := managedProcessRunning(process); !running {
			return false, processDetail
		}
		healthy, currentDetail := checkHealth(ctx, health, process.state.HealthExpectation)
		if running, processDetail := managedProcessRunning(process); !running {
			return false, processDetail
		}
		if healthy {
			// Require the nonce-bound response and launched process identity to
			// remain stable for another readiness sampling interval.
			stability := interval
			if stability < 250*time.Millisecond {
				stability = 250 * time.Millisecond
			}
			if stability > time.Second {
				stability = time.Second
			}
			if stability >= timeout {
				stability = timeout / 2
			}
			if stability <= 0 {
				stability = time.Millisecond
			}
			timer := time.NewTimer(stability)
			select {
			case exitErr := <-process.exit:
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				return false, processExitDetail(exitErr)
			case <-ctx.Done():
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				return false, ctx.Err().Error()
			case <-timer.C:
			}
			if running, processDetail := managedProcessRunning(process); !running {
				return false, processDetail
			}
			stableHealthy, stableDetail := checkHealth(ctx, health, process.state.HealthExpectation)
			if running, processDetail := managedProcessRunning(process); !running {
				return false, processDetail
			}
			if stableHealthy {
				return true, stableDetail
			}
			detail = stableDetail
			continue
		}
		detail = preferredHealthDetail(detail, currentDetail)
		select {
		case exitErr := <-process.exit:
			return false, processExitDetail(exitErr)
		case <-ctx.Done():
			if detail == "" {
				detail = ctx.Err().Error()
			}
			return false, detail
		case <-time.After(interval):
		}
	}
}

func checkHealth(parent context.Context, health *HealthSpec, expectedLaunchDigest string) (bool, string) {
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, health.URL, nil)
	if err != nil {
		return false, err.Error()
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return false, err.Error()
	}
	defer response.Body.Close()
	for _, status := range health.SuccessStatus {
		if response.StatusCode == status {
			if expectedLaunchDigest != "" {
				actualDigest := digestHealthNonce(response.Header.Get(health.LaunchHeader))
				if subtle.ConstantTimeCompare([]byte(actualDigest), []byte(expectedLaunchDigest)) != 1 {
					return false, fmt.Sprintf("%s; response header %s %s", response.Status, health.LaunchHeader, managedLaunchMismatchDetail)
				}
			}
			return true, response.Status
		}
	}
	return false, response.Status
}

func preferredHealthDetail(previous, current string) string {
	if strings.Contains(previous, managedLaunchMismatchDetail) && !strings.Contains(current, managedLaunchMismatchDetail) {
		return previous
	}
	if current != "" {
		return current
	}
	return previous
}

func ensureDirectories(config *Config) error {
	for _, directory := range []string{config.RuntimeDirectory, config.LogDirectory} {
		if err := verifyStableConfinedPath(config.Root, directory); err != nil {
			return fmt.Errorf("verify development directory %s before creation: %w", directory, err)
		}
		if err := os.MkdirAll(directory, 0o755); err != nil {
			return fmt.Errorf("create development directory %s: %w", directory, err)
		}
		if err := verifyStableConfinedPath(config.Root, directory); err != nil {
			return fmt.Errorf("verify development directory %s after creation: %w", directory, err)
		}
	}
	return nil
}

func verifyDevelopmentPaths(config *Config) error {
	if config == nil {
		return errors.New("development config is nil")
	}
	for _, directory := range []string{config.RuntimeDirectory, config.LogDirectory} {
		if err := verifyStableConfinedPath(config.Root, directory); err != nil {
			return fmt.Errorf("verify development directory %s: %w", directory, err)
		}
	}
	return nil
}

func readState(path string) (ServiceState, error) {
	file, err := openFileNoFollow(path, os.O_RDONLY, 0)
	if err != nil {
		return ServiceState{}, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, (1<<20)+1))
	if err != nil {
		return ServiceState{}, err
	}
	if len(data) > 1<<20 {
		return ServiceState{}, fmt.Errorf("development state %s exceeds 1 MiB", path)
	}
	state := ServiceState{}
	if err := json.Unmarshal(data, &state); err != nil {
		return ServiceState{}, fmt.Errorf("parse development state %s: %w", path, err)
	}
	if state.PID <= 1 || state.ServiceID == "" {
		return ServiceState{}, fmt.Errorf("invalid development state %s", path)
	}
	return state, nil
}

func writeState(path string, state ServiceState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".state-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary development state: %w", err)
	}
	temporaryPath := temporary.Name()
	committed := false
	defer func() {
		_ = temporary.Close()
		if !committed {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return fmt.Errorf("secure temporary development state: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		return fmt.Errorf("write development state: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync development state: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close development state: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("commit development state: %w", err)
	}
	committed = true
	return nil
}

func writeTail(path string, lines int, writer io.Writer) error {
	file, err := openFileNoFollow(path, os.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("open log %s: %w", path, err)
	}
	defer file.Close()
	if lines == 0 {
		_, err = io.Copy(writer, file)
		return err
	}
	scanner := bufio.NewScanner(file)
	buffer := make([]string, 0, lines)
	for scanner.Scan() {
		if len(buffer) < lines {
			buffer = append(buffer, scanner.Text())
			continue
		}
		copy(buffer, buffer[1:])
		buffer[len(buffer)-1] = scanner.Text()
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	for _, line := range buffer {
		if _, err := fmt.Fprintln(writer, line); err != nil {
			return err
		}
	}
	return nil
}

func mergeEnvironment(base []string, overrides map[string]string) []string {
	values := make(map[string]string, len(base)+len(overrides))
	for _, entry := range base {
		if index := strings.IndexByte(entry, '='); index > 0 {
			key := entry[:index]
			if isReservedLifecycleEnvironment(key) {
				continue
			}
			values[key] = entry[index+1:]
		}
	}
	for key, value := range overrides {
		if isReservedLifecycleEnvironment(key) && !strings.EqualFold(key, healthNonceEnvironment) {
			continue
		}
		values[key] = value
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, key+"="+values[key])
	}
	return result
}

func sanitizedEnvironment(source map[string]string) map[string]string {
	if len(source) == 0 {
		return nil
	}
	result := make(map[string]string, len(source))
	for key, value := range source {
		if isReservedLifecycleEnvironment(key) {
			continue
		}
		result[key] = value
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func copyEnvironment(source map[string]string) map[string]string {
	result := make(map[string]string, len(source)+1)
	for key, value := range source {
		result[key] = value
	}
	return result
}

func isReservedLifecycleEnvironment(key string) bool {
	return strings.EqualFold(key, initialAdminPasswordEnvironment) || strings.EqualFold(key, healthNonceEnvironment)
}

func canonicalReservedLifecycleEnvironment(key string) string {
	if strings.EqualFold(key, initialAdminPasswordEnvironment) {
		return initialAdminPasswordEnvironment
	}
	if strings.EqualFold(key, healthNonceEnvironment) {
		return healthNonceEnvironment
	}
	return key
}

func relative(root, path string) string {
	relativePath, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(relativePath)
}

func boolPointer(value bool) *bool {
	return &value
}
