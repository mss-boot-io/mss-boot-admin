package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/buildinfo"
	"github.com/mss-boot-io/mss-boot-admin/internal/mss/mcp"
	"github.com/mss-boot-io/mss-boot-admin/internal/mss/project"
)

func main() {
	root := flag.String("root", "", "working directory; defaults to the current directory (project tools validate .mss contracts when called)")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		_, _ = fmt.Fprintf(os.Stdout, "mss-mcp %s\n", buildinfo.String())
		return
	}

	resolvedRoot, err := resolveWorkingRoot(*root)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	server := &mcp.Server{Root: resolvedRoot, Stderr: os.Stderr}
	if err := server.Serve(ctx, os.Stdin, os.Stdout); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func resolveWorkingRoot(requested string) (string, error) {
	root := strings.TrimSpace(requested)
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("get current working directory: %w", err)
		}
		if projectRoot, discoveryErr := project.FindRoot(root); discoveryErr == nil {
			return projectRoot, nil
		}
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve working directory: %w", err)
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", fmt.Errorf("inspect working directory: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("working directory is not a directory: %s", absolute)
	}
	return absolute, nil
}
