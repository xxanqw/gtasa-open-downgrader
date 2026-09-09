// Package launcherapp contains the network-free runtime used by gta-sa.exe.
package launcherapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/xxanqw/gtasa-open-downgrader/internal/entrypoint"
	"github.com/xxanqw/gtasa-open-downgrader/internal/fsutil"
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

func RunningUnderProton() bool {
	return os.Getenv("STEAM_COMPAT_DATA_PATH") != "" || os.Getenv("STEAM_COMPAT_CLIENT_INSTALL_PATH") != "" || os.Getenv("PROTON_VERSION") != ""
}

func (runtime *Runtime) Clients() []multiplayer.Client {
	return multiplayer.Detect(multiplayer.Environment{GameDir: runtime.GameDir})
}

func (runtime *Runtime) SinglePlayer(ctx context.Context, arguments ...string) error {
	if err := runtime.restoreCompatibility(); err != nil {
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
	if err := runtime.restoreCompatibility(); err != nil {
		return err
	}
	arguments, err := connection.Arguments(runtime.GameDir)
	if err != nil {
		return err
	}
	clientPath, err := multiplayer.OpenMPClientPath(multiplayer.Environment{})
	if err != nil {
		return err
	}
	if err := multiplayer.ValidateOpenMPClient(clientPath); err != nil {
		return fmt.Errorf("%w; reinstall open.mp from the downgrader to repair its managed clients", err)
	}
	managedSAMP, err := multiplayer.OpenMPSAMPPath(multiplayer.Environment{}, connection.ClientVersion)
	if err != nil {
		return err
	}
	if err := multiplayer.ValidateInstaller(managedSAMP); err != nil {
		return fmt.Errorf("open.mp managed SA-MP client is missing or invalid; reinstall open.mp from the downgrader")
	}
	client, ok := multiplayer.Find(runtime.Clients(), multiplayer.ClientOpenMP)
	if !ok || !client.Installed {
		return fmt.Errorf("open.mp is not installed")
	}
	if err := multiplayer.ValidateExecutable(client); err != nil {
		return err
	}
	dataRoot, err := multiplayer.OpenMPDataPath(multiplayer.Environment{})
	if err != nil {
		return err
	}
	if err := runtime.installOpenMPSharedFiles(ctx, filepath.Join(dataRoot, "samp", "shared")); err != nil {
		return err
	}
	return runtime.runWithOpenMPSAMP(ctx, client, managedSAMP, arguments...)
}

func (runtime *Runtime) multiplayer(ctx context.Context, id multiplayer.ClientID, arguments ...string) error {
	if err := runtime.restoreCompatibility(); err != nil {
		return err
	}
	client, ok := multiplayer.Find(runtime.Clients(), id)
	if !ok || !client.Installed {
		return fmt.Errorf("%s is not installed", clientName(id))
	}
	if err := multiplayer.ValidateExecutable(client); err != nil {
		return err
	}
	if id == multiplayer.ClientMTA {
		return runtime.runWithoutModLoader(ctx, client)
	}
	return runtime.run(ctx, client.WorkingDir, client.Executable, arguments...)
}

type openMPSAMPState struct {
	ManagedSHA256 string `json:"managed_sha256"`
	HadOriginal   bool   `json:"had_original"`
}

func (runtime *Runtime) openMPSAMPPaths() (active, backup, state string) {
	directory := filepath.Join(runtime.GameDir, ".downgrader", "multiplayer")
	return filepath.Join(runtime.GameDir, "samp.dll"), filepath.Join(directory, "samp.dll.before-openmp"), filepath.Join(directory, "openmp-samp.json")
}

func (runtime *Runtime) runWithOpenMPSAMP(ctx context.Context, client multiplayer.Client, managed string, arguments ...string) error {
	active, backup, statePath := runtime.openMPSAMPPaths()
	digest, err := entrypoint.Hash(ctx, managed)
	if err != nil {
		return fmt.Errorf("hash open.mp managed SA-MP client: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(statePath), 0700); err != nil {
		return fmt.Errorf("prepare open.mp compatibility mode: %w", err)
	}
	if _, statErr := os.Stat(backup); statErr == nil {
		return fmt.Errorf("preserved samp.dll already exists at %s; restore or remove it before launching open.mp", backup)
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("inspect preserved samp.dll: %w", statErr)
	}
	hadOriginal := false
	if info, statErr := os.Stat(active); statErr == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("existing samp.dll is not a regular file")
		}
		hadOriginal = true
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("inspect existing samp.dll: %w", statErr)
	}
	state, err := json.Marshal(openMPSAMPState{ManagedSHA256: digest, HadOriginal: hadOriginal})
	if err != nil {
		return err
	}
	temporaryState := statePath + ".tmp"
	if err := os.WriteFile(temporaryState, state, 0600); err != nil {
		return fmt.Errorf("record open.mp compatibility mode: %w", err)
	}
	if err := os.Rename(temporaryState, statePath); err != nil {
		return fmt.Errorf("activate open.mp compatibility mode: %w", err)
	}
	if hadOriginal {
		if err := os.Rename(active, backup); err != nil {
			_ = runtime.restoreOpenMPSAMP(false)
			return fmt.Errorf("preserve existing samp.dll: %w", err)
		}
	}
	if err := fsutil.CopyFile(ctx, managed, active); err != nil {
		return errors.Join(fmt.Errorf("install selected open.mp SA-MP client: %w", err), runtime.restoreOpenMPSAMP(false))
	}
	runErr := runtime.run(ctx, client.WorkingDir, client.Executable, arguments...)
	return errors.Join(runErr, runtime.restoreOpenMPSAMP(runErr == nil))
}

func (runtime *Runtime) restoreCompatibility() error {
	return errors.Join(runtime.restoreOpenMPSAMP(false), runtime.restoreModLoader())
}

// restoreOpenMPSAMP restores the game-local client before a new launch. After
// open.mp successfully injects the managed DLL, Wine keeps that file locked
// until GTA exits. In that one case, leave the verified state and backup in
// place; the next launcher start performs the strict restoration.
func (runtime *Runtime) restoreOpenMPSAMP(allowDeferred bool) error {
	active, backup, statePath := runtime.openMPSAMPPaths()
	data, err := os.ReadFile(statePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read open.mp compatibility state: %w", err)
	}
	var state openMPSAMPState
	if err := json.Unmarshal(data, &state); err != nil || state.ManagedSHA256 == "" {
		return fmt.Errorf("open.mp compatibility state is invalid")
	}
	backupExists := false
	if info, statErr := os.Stat(backup); statErr == nil {
		backupExists = info.Mode().IsRegular()
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("inspect preserved samp.dll: %w", statErr)
	}
	if info, statErr := os.Stat(active); statErr == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("active samp.dll is not a regular file")
		}
		digest, hashErr := entrypoint.Hash(context.Background(), active)
		if hashErr != nil {
			return hashErr
		}
		if digest == state.ManagedSHA256 {
			if err := os.Remove(active); err != nil {
				if allowDeferred {
					return nil
				}
				return fmt.Errorf("remove temporary open.mp samp.dll: %w", err)
			}
		} else if backupExists {
			return fmt.Errorf("samp.dll changed while open.mp compatibility mode was active; preserved original left at %s", backup)
		} else if state.HadOriginal {
			return os.Remove(statePath)
		} else {
			return fmt.Errorf("samp.dll changed while open.mp compatibility mode was active")
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("inspect temporary open.mp samp.dll: %w", statErr)
	}
	if state.HadOriginal {
		if !backupExists {
			return fmt.Errorf("preserved samp.dll is missing")
		}
		if err := os.Rename(backup, active); err != nil {
			return fmt.Errorf("restore preserved samp.dll: %w", err)
		}
	}
	if err := os.Remove(statePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("clear open.mp compatibility state: %w", err)
	}
	return nil
}

func (runtime *Runtime) installOpenMPSharedFiles(ctx context.Context, source string) error {
	for _, relative := range multiplayer.OpenMPSharedFiles {
		from := filepath.Join(source, filepath.FromSlash(relative))
		if info, err := os.Stat(from); err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("open.mp managed SA-MP resource is missing: %s", relative)
		}
		if err := fsutil.CopyFile(ctx, from, filepath.Join(runtime.GameDir, filepath.FromSlash(relative))); err != nil {
			return err
		}
	}
	return nil
}

// runWithoutModLoader keeps Mod Loader out of MTA, which rejects game
// installations containing ASI plugins. The file is restored after MTA exits
// and is also recovered before every later launch.
func (runtime *Runtime) runWithoutModLoader(ctx context.Context, client multiplayer.Client) error {
	_, err := runtime.disableModLoader()
	if err != nil {
		return err
	}
	runErr := runtime.run(ctx, client.WorkingDir, client.Executable)
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
