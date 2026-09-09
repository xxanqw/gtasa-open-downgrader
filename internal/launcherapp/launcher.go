// Package launcherapp contains the network-free runtime used by gta-sa.exe.
package launcherapp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/xxanqw/gtasa-open-downgrader/internal/entrypoint"
	"github.com/xxanqw/gtasa-open-downgrader/internal/multiplayer"
)

type Executor func(context.Context, string, string, ...string) error

type Runtime struct {
	GameDir                string
	Execute                Executor
	AfterCompatibilityExit func(context.Context) error
}

const modLoaderRestoreDelay = 12 * time.Second

func New(gameDir string) *Runtime {
	return &Runtime{GameDir: gameDir, Execute: execute}
}

func RunningUnderProton() bool {
	return os.Getenv("STEAM_COMPAT_DATA_PATH") != "" || os.Getenv("STEAM_COMPAT_CLIENT_INSTALL_PATH") != "" || os.Getenv("PROTON_VERSION") != ""
}

func (runtime *Runtime) Clients() []multiplayer.Client {
	return multiplayer.Detect(multiplayer.Environment{GameDir: runtime.GameDir})
}

func (runtime *Runtime) SinglePlayer(ctx context.Context, arguments ...string) error {
	if err := runtime.restoreModLoader(); err != nil {
		return err
	}
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
	return runtime.multiplayer(ctx, id)
}

func (runtime *Runtime) OpenMP(ctx context.Context, connection multiplayer.OpenMPConnection) error {
	arguments, err := connection.Arguments(runtime.GameDir)
	if err != nil {
		return err
	}
	clientPath, err := multiplayer.OpenMPClientPath(multiplayer.Environment{})
	if err != nil {
		return err
	}
	if err := multiplayer.ValidateOpenMPClient(clientPath); err != nil {
		return fmt.Errorf("%w; reinstall open.mp from the downgrader to add Linux command-line support", err)
	}
	return runtime.multiplayer(ctx, multiplayer.ClientOpenMP, arguments...)
}

func (runtime *Runtime) multiplayer(ctx context.Context, id multiplayer.ClientID, arguments ...string) error {
	if err := runtime.restoreModLoader(); err != nil {
		return err
	}
	client, ok := multiplayer.Find(runtime.Clients(), id)
	if !ok || !client.Installed {
		return fmt.Errorf("%s is not installed", clientName(id))
	}
	if err := multiplayer.ValidateExecutable(client); err != nil {
		return err
	}
	if id == multiplayer.ClientSAMP || id == multiplayer.ClientMTA {
		return runtime.runWithoutModLoader(ctx, client)
	}
	return runtime.run(ctx, client.WorkingDir, client.Executable, arguments...)
}

// runWithoutModLoader keeps Mod Loader out of clients that reject or conflict
// with injected ASI plugins. SA-MP R5 can crash inside Mod Loader during
// initialization, while MTA warns about every ASI in the game directory. The
// file is restored after the client exits and a short startup grace period,
// and it is also recovered before every later launch.
func (runtime *Runtime) runWithoutModLoader(ctx context.Context, client multiplayer.Client) error {
	disabled, err := runtime.disableModLoader()
	if err != nil {
		return err
	}
	runErr := runtime.run(ctx, client.WorkingDir, client.Executable)
	if disabled && runErr == nil {
		if runtime.AfterCompatibilityExit != nil {
			_ = runtime.AfterCompatibilityExit(ctx)
		} else {
			timer := time.NewTimer(modLoaderRestoreDelay)
			select {
			case <-timer.C:
			case <-ctx.Done():
				if !timer.Stop() {
					<-timer.C
				}
			}
		}
	}
	restoreErr := runtime.restoreModLoader()
	return errors.Join(runErr, restoreErr)
}

func (runtime *Runtime) modLoaderPaths() (active, disabled string) {
	return filepath.Join(runtime.GameDir, "modloader.asi"), filepath.Join(runtime.GameDir, ".downgrader", "multiplayer", "modloader.asi.disabled")
}

func (runtime *Runtime) disableModLoader() (bool, error) {
	active, disabled := runtime.modLoaderPaths()
	if _, err := os.Stat(active); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("inspect Mod Loader: %w", err)
	}
	if _, err := os.Stat(disabled); err == nil {
		return false, fmt.Errorf("cannot enable multiplayer compatibility mode because both Mod Loader copies exist")
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("inspect disabled Mod Loader: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(disabled), 0700); err != nil {
		return false, fmt.Errorf("prepare multiplayer compatibility mode: %w", err)
	}
	if err := os.Rename(active, disabled); err != nil {
		return false, fmt.Errorf("disable Mod Loader for multiplayer: %w", err)
	}
	return true, nil
}

func (runtime *Runtime) restoreModLoader() error {
	active, disabled := runtime.modLoaderPaths()
	if _, err := os.Stat(disabled); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("inspect disabled Mod Loader: %w", err)
	}
	if _, err := os.Stat(active); err == nil {
		return fmt.Errorf("cannot restore Mod Loader because both active and disabled copies exist")
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect Mod Loader: %w", err)
	}
	if err := os.Rename(disabled, active); err != nil {
		return fmt.Errorf("restore Mod Loader after multiplayer: %w", err)
	}
	_ = os.Remove(filepath.Dir(disabled))
	return nil
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
