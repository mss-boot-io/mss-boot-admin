//go:build windows

package command

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

type managedProcessTree struct {
	command *exec.Cmd
	job     windows.Handle
}

func newProcessTree(command *exec.Cmd) (*managedProcessTree, error) {
	if command == nil {
		return nil, errors.New("command is required")
	}
	if command.SysProcAttr != nil {
		return nil, errors.New("command already defines process attributes")
	}
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("create Job Object: %w", err)
	}
	information := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	information.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&information)),
		uint32(unsafe.Sizeof(information)),
	); err != nil {
		_ = windows.CloseHandle(job)
		return nil, fmt.Errorf("configure Job Object: %w", err)
	}
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP}
	return &managedProcessTree{command: command, job: job}, nil
}

func (tree *managedProcessTree) afterStart() error {
	if tree == nil || tree.command == nil || tree.command.Process == nil || tree.command.Process.Pid <= 0 {
		return errors.New("started command has no process identity")
	}
	process, err := windows.OpenProcess(
		windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE|windows.PROCESS_QUERY_LIMITED_INFORMATION,
		false,
		uint32(tree.command.Process.Pid),
	)
	if err != nil {
		return fmt.Errorf("open command process: %w", err)
	}
	defer windows.CloseHandle(process)
	if err := windows.AssignProcessToJobObject(tree.job, process); err != nil {
		return fmt.Errorf("assign command to Job Object: %w", err)
	}
	return nil
}

func (tree *managedProcessTree) terminate() error {
	if tree == nil || tree.command == nil || tree.command.Process == nil || tree.command.Process.Pid <= 0 {
		return errors.New("command process tree is unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	output, taskkillErr := exec.CommandContext(
		ctx,
		"taskkill",
		"/PID",
		strconv.Itoa(tree.command.Process.Pid),
		"/T",
		"/F",
	).CombinedOutput()
	if taskkillErr != nil && ctx.Err() == nil {
		taskkillErr = fmt.Errorf("taskkill process tree: %w: %s", taskkillErr, strings.TrimSpace(string(output)))
	}
	jobErr := windows.TerminateJobObject(tree.job, 1)
	if taskkillErr != nil && jobErr != nil {
		return errors.Join(taskkillErr, fmt.Errorf("terminate Job Object: %w", jobErr))
	}
	return nil
}

func (tree *managedProcessTree) kill() error {
	var failures []error
	if tree != nil && tree.job != 0 {
		if err := windows.TerminateJobObject(tree.job, 1); err != nil {
			failures = append(failures, fmt.Errorf("terminate Job Object: %w", err))
		}
	}
	if tree != nil && tree.command != nil && tree.command.Process != nil {
		if err := tree.command.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

func (tree *managedProcessTree) close() error {
	if tree == nil || tree.job == 0 {
		return nil
	}
	err := windows.CloseHandle(tree.job)
	tree.job = 0
	return err
}
