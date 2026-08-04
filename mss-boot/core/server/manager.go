package server

import (
	"context"
	"fmt"
)

// Manager coordinates the lifecycle of one or more Runnable components.
type Manager interface {
	Add(...Runnable)
	Start(context.Context) error
}

// Runnable is a long-running component managed by Manager.
//
// Start must block until the component stops, the supplied context is
// cancelled, or an unrecoverable runtime error occurs. Implementations must
// release owned resources before returning.
type Runnable interface {
	fmt.Stringer
	Start(ctx context.Context) error
}
