//go:build linux

package dev

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
)

// processStartToken returns the kernel starttime field, measured in clock
// ticks since boot. It remains stable across exec and process-title changes.
func processStartToken(pid int) (string, error) {
	if pid <= 1 {
		return "", errProcessNotRunning
	}
	bootIDData, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return "", fmt.Errorf("read kernel boot identity: %w", err)
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", errProcessNotRunning
		}
		return "", fmt.Errorf("read process %d start identity: %w", pid, err)
	}
	return linuxProcessStartToken(strings.TrimSpace(string(bootIDData)), data)
}

func linuxProcessStartToken(bootID string, stat []byte) (string, error) {
	compactBootID := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(bootID)), "-", "")
	if len(compactBootID) != 32 {
		return "", errors.New("kernel boot identity is malformed")
	}
	if _, err := hex.DecodeString(compactBootID); err != nil {
		return "", fmt.Errorf("kernel boot identity is malformed: %w", err)
	}
	// Field 2 (comm) is parenthesized and may itself contain spaces or closing
	// parentheses. Everything after its final ')' starts at field 3, so field
	// 22 (starttime) is offset 19 in the remaining fields.
	closing := strings.LastIndexByte(string(stat), ')')
	if closing < 0 {
		return "", errors.New("process start identity has a malformed stat record")
	}
	fields := strings.Fields(string(stat[closing+1:]))
	if len(fields) <= 19 || fields[19] == "" {
		return "", errors.New("process start identity is missing start time")
	}
	return "linux-proc:" + strings.ToLower(strings.TrimSpace(bootID)) + ":" + fields[19], nil
}
