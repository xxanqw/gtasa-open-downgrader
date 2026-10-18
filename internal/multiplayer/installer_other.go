//go:build !windows

package multiplayer

import (
	"context"
	"os/exec"
)

func runInstaller(ctx context.Context, directory, executable string, arguments ...string) error {
	command := exec.CommandContext(ctx, executable, arguments...)
	command.Dir = directory
	return command.Run()
}
