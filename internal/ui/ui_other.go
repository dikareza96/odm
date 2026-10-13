//go:build !windows

package ui

import (
	"errors"

	"odm/internal/engine"
)

// Run is unavailable outside Windows; the web UI is used there instead.
func Run(*engine.Manager) error {
	return errors.New("the native UI is Windows-only")
}
