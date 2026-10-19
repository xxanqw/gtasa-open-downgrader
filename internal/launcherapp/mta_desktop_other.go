//go:build !windows

package launcherapp

import (
	"context"
	"fmt"
)

func requireDesktopMTAClosed() error { return nil }

func executeDesktopMTA(context.Context, string, string, ...string) error {
	return fmt.Errorf("Windows desktop dispatch is unavailable on this platform")
}
