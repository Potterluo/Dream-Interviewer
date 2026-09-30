//go:build !windows

package main

// Non-Windows stub: there is no console-less GUI subsystem to work around
// there, so stderr is enough. Kept as a separate file so cmd/desktop still
// builds on Linux/macOS (see the build notes in main.go).

import (
	"fmt"
	"os"
)

func showError(title, message string) {
	fmt.Fprintf(os.Stderr, "\n%s\n%s\n", title, message)
}
