//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package command

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

type managedProcessTree struct {
	command *exec.Cmd
}

func newProcessTree(command *exec.Cmd) (*managedProcessTree, error) {
	if command == nil {
		return nil, errors.New("command is required")
	}
	if command.SysProcAttr != nil {
		return nil, errors.New("command already defines process attributes")
	}
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return &managedProcessTree{command: command}, nil
}

func (tree *managedProcessTree) afterStart() error {
	if tree == nil || tree.command == nil || tree.command.Process == nil || tree.command.Process.Pid <= 0 {
		return errors.New("started command has no process identity")
	}
	return nil
}

func (tree *managedProcessTree) terminate() error {
	return tree.signal(syscall.SIGTERM)
}

func (tree *managedProcessTree) kill() error {
	err := tree.signal(syscall.SIGKILL)
	if tree != nil && tree.command != nil && tree.command.Process != nil {
		directErr := tree.command.Process.Kill()
		if directErr != nil && !errors.Is(directErr, os.ErrProcessDone) {
			err = errors.Join(err, directErr)
		}
	}
	return err
}

func (tree *managedProcessTree) close() error { return nil }

func (tree *managedProcessTree) signal(signal syscall.Signal) error {
	if tree == nil || tree.command == nil || tree.command.Process == nil || tree.command.Process.Pid <= 0 {
		return errors.New("command process tree is unavailable")
	}
	err := syscall.Kill(-tree.command.Process.Pid, signal)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}
