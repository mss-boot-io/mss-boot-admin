package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/buildinfo"
	"github.com/mss-boot-io/mss-boot-admin/internal/mss/mcp"
	"github.com/mss-boot-io/mss-boot-admin/internal/mss/project"
)

func main() {
	root := flag.String("root", "", "repository root; defaults to discovery from the current directory")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		_, _ = fmt.Fprintf(os.Stdout, "mss-mcp %s\n", buildinfo.String())
		return
	}

	resolvedRoot := *root
	if resolvedRoot == "" {
		var err error
		resolvedRoot, err = project.FindRoot("")
		if err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	server := &mcp.Server{Root: resolvedRoot, Stderr: os.Stderr}
	if err := server.Serve(ctx, os.Stdin, os.Stdout); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
