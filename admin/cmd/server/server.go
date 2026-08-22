// Package server retains fail-closed source compatibility for the v1 Admin API.
// Executable server commands live in admin/internal/cmd/server.
package server

import (
	"github.com/spf13/cobra"

	"github.com/mss-boot-io/mss-boot-admin/admin/business"
	admincmd "github.com/mss-boot-io/mss-boot-admin/admin/cmd"
)

// ErrDirectCommandAccess aliases the common legacy command-access error.
var ErrDirectCommandAccess = admincmd.ErrDirectCommandAccess

// StartCmd preserves the v1.3.0 source symbol as an inert command sentinel.
//
// Deprecated: compose package app and call ExecuteContext.
var StartCmd = NewCommand(nil)

// NewCommand preserves the v1.3.0 source signature without exposing any Admin
// server setup, routes, resources, or lifecycle execution.
//
// Deprecated: compose package app and call ExecuteArgsContext with server
// arguments.
func NewCommand(registry *business.Registry) *cobra.Command {
	command := admincmd.New(registry)
	command.Use = "server"
	return command
}
