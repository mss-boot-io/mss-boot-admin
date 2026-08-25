package dev

import (
	"errors"
	"fmt"
	"strings"
)

// processCreationSnapshot deliberately excludes every mutable property from
// process identity. Command is retained only as a regression-test input: an
// exec or process-title change must not affect the token.
type processCreationSnapshot struct {
	Platform    string
	Seconds     int64
	Nanoseconds int64
	Command     string
}

func processStartTokenFromSnapshot(snapshot processCreationSnapshot) (string, error) {
	platform := strings.TrimSpace(snapshot.Platform)
	if platform == "" {
		return "", errors.New("process creation platform is required")
	}
	if snapshot.Seconds <= 0 {
		return "", errors.New("process creation time is required")
	}
	if snapshot.Nanoseconds < 0 || snapshot.Nanoseconds >= 1_000_000_000 {
		return "", errors.New("process creation nanoseconds are invalid")
	}
	return fmt.Sprintf("%s-created:%d.%09d", platform, snapshot.Seconds, snapshot.Nanoseconds), nil
}
