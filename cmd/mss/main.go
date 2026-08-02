package main

import (
	"fmt"
	"os"

	"github.com/mss-boot-io/mss-boot-admin/internal/mss/app"
)

func main() {
	if err := app.ExecuteAgent(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
