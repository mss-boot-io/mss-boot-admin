package dev

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
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
)

// ServiceState is persisted under .mss/run for detached processes.
type ServiceState struct {
	ServiceID   string            `json:"serviceId"`
	PID         int               `json:"pid"`
	StartedAt   time.Time         `json:"startedAt"`
	Command     []string          `json:"command"`
	Directory   string            `json:"directory"`
	LogPath     string            `json:"logPath"`
	Detached    bool              `json:"detached"`
	Environment map[string]string `json:"environment,omitempty"`
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
	service ServiceSpec
	cmd     *exec.Cmd
	log     *os.File
	state   ServiceState
}

type processExit struct {
	serviceID string
	err       error
}

// Start launches selected services in dependency order. Foreground mode blocks until cancellation or exit.
func Start(parent context.Context, config *Config, options StartOptions) (Report, error) {
	if config == nil {
		return Report{}, errors.New("development config is nil")
	}
	if options.Stdout == nil {
		options.Stdout = io.Discard
	}
	if options.Stderr == nil {
		options.Stderr = options.Stdout
	}
	services, err := config.Services(options.Services)
	if err != nil {
		return Report{}, err
	}
	if err := ensureDirectories(config); err != nil {
		return Report{}, err
	}

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
		process, result, startErr := startService(parent, config, service, options)
		report.Services = append(report.Services, result)
		if startErr != nil {
			report.Success = false
			stopManaged(config, started, true)
			return report, startErr
		}
		if process != nil {
			started = append(started, process)
		}
	}
	if options.Detach {
		for _, process := range started {
			_ = process.log.Close()
			_ = process.cmd.Process.Release()
		}
		return report, nil
	}
	if len(started) == 0 {
		return report, nil
	}

	ctx, cancel := signal.NotifyContext(parent, os.Interrupt)
	defer cancel()
	exits := make(chan processExit, len(started))
	for _, process := range started {
		process := process
		go func() {
			err := process.cmd.Wait()
			_ = process.log.Close()
			_ = os.Remove(config.StatePath(process.service.ID))
			exits <- processExit{serviceID: process.service.ID, err: err}
		}()
	}

	select {
	case <-ctx.Done():
		stopManaged(config, started, false)
		return report, nil
	case exited := <-exits:
		stopManaged(config, started, false)
		if exited.err != nil {
			report.Success = false
			return report, fmt.Errorf("development service %s exited: %w", exited.serviceID, exited.err)
		}
		return report, fmt.Errorf("development service %s exited", exited.serviceID)
	}
}

// Status inspects process state and health without mutating running services.
func Status(parent context.Context, config *Config, selected []string) (Report, error) {
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
		if !processAlive(state.PID) {
			result.Status = "stale"
			result.Detail = "state file exists but process is not running"
			report.Success = false
			report.Services = append(report.Services, result)
			continue
		}
		if service.Health != nil {
			healthy, detail := checkHealth(parent, service.Health)
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
	for _, service := range services {
		result, stopErr := stopService(parent, config, service, options.Force)
		report.Services = append(report.Services, result)
		if stopErr != nil {
			report.Success = false
		}
	}
	if !report.Success {
		return report, errors.New("one or more development services could not be stopped")
	}
	return report, nil
}

// Logs prints the last lines of one service log and optionally follows new output.
func Logs(ctx context.Context, config *Config, serviceID string, lines int, follow bool, writer io.Writer) error {
	if writer == nil {
		writer = io.Discard
	}
	if _, exists := config.Service(serviceID); !exists {
		return fmt.Errorf("unknown development service %q", serviceID)
	}
	if lines < 0 {
		return errors.New("lines must be non-negative")
	}
	path := config.LogPath(serviceID)
	if err := writeTail(path, lines, writer); err != nil {
		return err
	}
	if !follow {
		return nil
	}

	file, err := os.Open(path)
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
	statePath := config.StatePath(service.ID)
	if state, err := readState(statePath); err == nil && processAlive(state.PID) {
		result := ServiceResult{
			ServiceID: service.ID,
			PID:       state.PID,
			Status:    "already-running",
			LogPath:   relative(config.Root, state.LogPath),
			StartedAt: state.StartedAt,
		}
		if service.Health != nil {
			healthy, detail := checkHealth(parent, service.Health)
			result.Healthy = boolPointer(healthy)
			result.Detail = detail
		}
		return nil, result, nil
	} else if err == nil || !errors.Is(err, os.ErrNotExist) {
		_ = os.Remove(statePath)
	}

	logPath := config.LogPath(service.ID)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, ServiceResult{ServiceID: service.ID, Status: "failed"}, fmt.Errorf("open %s log: %w", service.ID, err)
	}
	command := exec.Command(service.Command[0], service.Command[1:]...)
	command.Dir = config.ResolveDirectory(service)
	command.Env = mergeEnvironment(os.Environ(), service.Environment)
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
	state := ServiceState{
		ServiceID:   service.ID,
		PID:         command.Process.Pid,
		StartedAt:   time.Now().UTC(),
		Command:     append([]string(nil), service.Command...),
		Directory:   command.Dir,
		LogPath:     logPath,
		Detached:    options.Detach,
		Environment: cloneMap(service.Environment),
	}
	if err := writeState(statePath, state); err != nil {
		_ = signalProcess(state.PID, true)
		_ = logFile.Close()
		return nil, ServiceResult{ServiceID: service.ID, PID: state.PID, Status: "failed"}, err
	}
	result := ServiceResult{
		ServiceID: service.ID,
		PID:       state.PID,
		Status:    "running",
		LogPath:   relative(config.Root, logPath),
		StartedAt: state.StartedAt,
	}
	if service.Health != nil {
		healthy, detail := waitForHealth(parent, service.Health)
		result.Healthy = boolPointer(healthy)
		result.Detail = detail
		if !healthy {
			_ = signalProcess(state.PID, true)
			_ = os.Remove(statePath)
			_ = logFile.Close()
			result.Status = "failed-health"
			return nil, result, fmt.Errorf("development service %s failed readiness check: %s", service.ID, detail)
		}
	}
	return &managedProcess{service: service, cmd: command, log: logFile, state: state}, result, nil
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
	if !processAlive(state.PID) {
		_ = os.Remove(statePath)
		result.Status = "removed-stale-state"
		return result, nil
	}
	if err := signalProcess(state.PID, force); err != nil {
		result.Status = "stop-failed"
		result.Detail = err.Error()
		return result, err
	}
	deadline := time.Now().Add(config.StopTimeout)
	for processAlive(state.PID) && time.Now().Before(deadline) {
		select {
		case <-parent.Done():
			return result, parent.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	if processAlive(state.PID) {
		if err := signalProcess(state.PID, true); err != nil {
			result.Status = "kill-failed"
			result.Detail = err.Error()
			return result, err
		}
	}
	if err := os.Remove(statePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		result.Status = "state-cleanup-failed"
		result.Detail = err.Error()
		return result, err
	}
	return result, nil
}

func stopManaged(config *Config, processes []*managedProcess, force bool) {
	var wait sync.WaitGroup
	for _, process := range processes {
		if process == nil || process.cmd == nil || process.cmd.Process == nil {
			continue
		}
		wait.Add(1)
		go func(process *managedProcess) {
			defer wait.Done()
			_ = signalProcess(process.cmd.Process.Pid, force)
			_ = os.Remove(config.StatePath(process.service.ID))
		}(process)
	}
	wait.Wait()
}

func waitForHealth(parent context.Context, health *HealthSpec) (bool, string) {
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
		healthy, currentDetail := checkHealth(ctx, health)
		if healthy {
			return true, currentDetail
		}
		detail = currentDetail
		select {
		case <-ctx.Done():
			if detail == "" {
				detail = ctx.Err().Error()
			}
			return false, detail
		case <-time.After(interval):
		}
	}
}

func checkHealth(parent context.Context, health *HealthSpec) (bool, string) {
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
			return true, response.Status
		}
	}
	return false, response.Status
}

func ensureDirectories(config *Config) error {
	for _, directory := range []string{config.RuntimeDirectory, config.LogDirectory} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			return fmt.Errorf("create development directory %s: %w", directory, err)
		}
	}
	return nil
}

func readState(path string) (ServiceState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ServiceState{}, err
	}
	state := ServiceState{}
	if err := json.Unmarshal(data, &state); err != nil {
		return ServiceState{}, fmt.Errorf("parse development state %s: %w", path, err)
	}
	if state.PID <= 0 || state.ServiceID == "" {
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
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return fmt.Errorf("write development state: %w", err)
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("commit development state: %w", err)
	}
	return nil
}

func writeTail(path string, lines int, writer io.Writer) error {
	file, err := os.Open(path)
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
			values[entry[:index]] = entry[index+1:]
		}
	}
	for key, value := range overrides {
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

func cloneMap(source map[string]string) map[string]string {
	if len(source) == 0 {
		return nil
	}
	result := make(map[string]string, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
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
