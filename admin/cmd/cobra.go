// Package cmd retains fail-closed source compatibility for the v1 Admin API.
// The executable Admin command tree is private to admin/internal/cmd.
package cmd

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"github.com/mss-boot-io/mss-boot-admin/admin/business"
)

// ErrDirectCommandAccess reports use of a legacy command-tree escape hatch.
var ErrDirectCommandAccess = errors.New(
	"direct Admin command access is unavailable; use admin/app ExecuteContext or ExecuteArgsContext",
)

// New preserves the v1.3.0 source signature without exposing the executable
// Admin command tree. The returned command is an inert compatibility sentinel
// whose execution always fails with ErrDirectCommandAccess.
//
// Deprecated: compose package app and call ExecuteContext or
// ExecuteArgsContext.
func New(_ *business.Registry) *cobra.Command {
	return &cobra.Command{
		Use:           "mss-boot-admin",
		Short:         "deprecated direct Admin command access",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(*cobra.Command, []string) error {
			return ErrDirectCommandAccess
		},
	}
}

// ExecuteContext preserves the v1.3.0 source signature and always fails
// closed before constructing or executing Admin runtime state.
//
// Deprecated: compose package app and call Application.ExecuteContext.
func ExecuteContext(context.Context, *business.Registry) error {
	return ErrDirectCommandAccess
}

// Execute preserves the historical source signature and always fails closed.
//
// Deprecated: compose package app and call Application.ExecuteContext.
func Execute() error {
	return ErrDirectCommandAccess
}
