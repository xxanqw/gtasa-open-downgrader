// Package launcherapp contains the network-free runtime used by gta-sa.exe.
package launcherapp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/xxanqw/gtasa-open-downgrader/internal/entrypoint"
	"github.com/xxanqw/gtasa-open-downgrader/internal/multiplayer"
)

type Executor func(context.Context, string, string, ...string) error

type Runtime struct {
	GameDir string
	Execute Executor
}

func New(gameDir string) *Runtime {
	return &Runtime{GameDir: gameDir, Execute: execute}
}

func (runtime *Runtime) Clients() []multiplayer.Client {
	return multiplayer.Detect(multiplayer.Environment{GameDir: runtime.GameDir})
}

func (runtime *Runtime) SinglePlayer(ctx context.Context, arguments ...string) error {
	path, err := entrypoint.GamePath(runtime.GameDir)
	if err != nil {
		return err
	}
	if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("real GTA executable %q is missing", path)
	}
	return runtime.run(ctx, runtime.GameDir, path, arguments...)
}

func (runtime *Runtime) Multiplayer(ctx context.Context, id multiplayer.ClientID) error {
	client, ok := multiplayer.Find(runtime.Clients(), id)
	if !ok || !client.Installed {
		return fmt.Errorf("%s is not installed", clientName(id))
	}
	if err := multiplayer.ValidateExecutable(client); err != nil {
		return err
	}
	return runtime.run(ctx, client.WorkingDir, client.Executable)
}

func (runtime *Runtime) run(ctx context.Context, directory, executable string, arguments ...string) error {
	if runtime.Execute == nil {
		runtime.Execute = execute
	}
	return runtime.Execute(ctx, directory, executable, arguments...)
}

func execute(ctx context.Context, directory, executable string, arguments ...string) error {
	command := exec.CommandContext(ctx, executable, arguments...)
	command.Dir = directory
	if err := command.Run(); err != nil {
		return fmt.Errorf("run %q: %w", filepath.Base(executable), err)
	}
	return nil
}

func FindDowngrader(gameDir string) string {
	if config, err := entrypoint.Load(gameDir); err == nil && config.DowngraderExecutable != "" {
		if digest, hashErr := entrypoint.Hash(context.Background(), config.DowngraderExecutable); hashErr == nil && digest == config.DowngraderSHA256 {
			return config.DowngraderExecutable
		}
	}
	for _, name := range []string{"gtasa-open-downgrader-windows.exe", "gtasa-open-downgrader.exe"} {
		path := filepath.Join(gameDir, name)
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			return path
		}
	}
	return ""
}

func OpenDowngrader(gameDir string) error {
	path := FindDowngrader(gameDir)
	if path == "" {
		return fmt.Errorf("downgrader executable was not found beside the game")
	}
	command := exec.Command(path, "--game", gameDir)
	command.Dir = gameDir
	if err := command.Start(); err != nil {
		return err
	}
	return command.Process.Release()
}

func clientName(id multiplayer.ClientID) string {
	if installer, ok := multiplayer.Installers[id]; ok {
		return installer.Name
	}
	return string(id)
}
