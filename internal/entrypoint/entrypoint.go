// Package entrypoint manages the optional Steam entrypoint launcher without
// confusing it with the real GTA San Andreas executable.
package entrypoint

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/xxanqw/gtasa-open-downgrader/internal/fsutil"
	"github.com/xxanqw/gtasa-open-downgrader/internal/launcherversion"
)

const (
	SchemaVersion      = 1
	RealGameExecutable = "gta_sa.exe"
	SteamExecutable    = "gta-sa.exe"
	MetadataPath       = ".downgrader/launcher.json"
	backupPath         = ".downgrader/launcher-backup/gta-sa.exe"
)

type Config struct {
	SchemaVersion        int    `json:"schema_version"`
	LauncherExecutable   string `json:"launcher_executable"`
	GameExecutable       string `json:"game_executable"`
	LauncherSHA256       string `json:"launcher_sha256"`
	BackupExecutable     string `json:"backup_executable,omitempty"`
	BackupSHA256         string `json:"backup_sha256,omitempty"`
	DowngraderExecutable string `json:"downgrader_executable,omitempty"`
	DowngraderSHA256     string `json:"downgrader_sha256,omitempty"`
}

type State string

const (
	NotInstalled State = "not installed"
	Installed    State = "installed"
	Replaced     State = "replaced by Steam or another file"
	Invalid      State = "invalid launcher metadata"
)

type Status struct {
	State   State
	Config  Config
	Detail  string
	Version string
}

func metadata(root string) (string, error) { return fsutil.Join(root, MetadataPath) }

func normalize(config Config) (Config, error) {
	if config.SchemaVersion == 0 {
		// Early development builds wrote the names without a schema field.
		config.SchemaVersion = SchemaVersion
	}
	if config.SchemaVersion != SchemaVersion {
		return Config{}, fmt.Errorf("unsupported launcher metadata schema %d", config.SchemaVersion)
	}
	if config.LauncherExecutable == "" {
		config.LauncherExecutable = SteamExecutable
	}
	if config.GameExecutable == "" {
		config.GameExecutable = RealGameExecutable
	}
	if config.LauncherExecutable != SteamExecutable || config.GameExecutable != RealGameExecutable {
		return Config{}, fmt.Errorf("launcher metadata contains unsupported executable names")
	}
	if config.BackupExecutable != "" && config.BackupExecutable != backupPath {
		return Config{}, fmt.Errorf("launcher metadata contains an unsafe backup path")
	}
	if config.LauncherSHA256 != "" && !validDigest(config.LauncherSHA256) {
		return Config{}, fmt.Errorf("launcher metadata contains an invalid launcher digest")
	}
	if config.BackupSHA256 != "" && !validDigest(config.BackupSHA256) {
		return Config{}, fmt.Errorf("launcher metadata contains an invalid backup digest")
	}
	if (config.DowngraderExecutable == "") != (config.DowngraderSHA256 == "") {
		return Config{}, fmt.Errorf("launcher metadata contains incomplete downgrader identity")
	}
	if config.DowngraderExecutable != "" {
		name := strings.ToLower(portableBase(config.DowngraderExecutable))
		if !absoluteOnSupportedOS(config.DowngraderExecutable) || !strings.HasPrefix(name, "gtasa-open-downgrader") || !strings.HasSuffix(name, ".exe") || !validDigest(config.DowngraderSHA256) {
			return Config{}, fmt.Errorf("launcher metadata contains an invalid downgrader identity")
		}
	}
	return config, nil
}

func absoluteOnSupportedOS(path string) bool {
	if filepath.IsAbs(path) {
		return true
	}
	return len(path) >= 3 && ((path[0] >= 'A' && path[0] <= 'Z') || (path[0] >= 'a' && path[0] <= 'z')) && path[1] == ':' && (path[2] == '\\' || path[2] == '/')
}

func portableBase(path string) string {
	if index := strings.LastIndexAny(path, `/\\`); index >= 0 {
		return path[index+1:]
	}
	return path
}

func validDigest(value string) bool {
	b, err := hex.DecodeString(value)
	return err == nil && len(b) == sha256.Size
}

func Load(root string) (Config, error) {
	path, err := metadata(root)
	if err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var config Config
	if err = json.Unmarshal(data, &config); err != nil {
		return Config{}, fmt.Errorf("read launcher metadata: %w", err)
	}
	return normalize(config)
}

func Inspect(ctx context.Context, root string) Status {
	config, err := Load(root)
	if errors.Is(err, os.ErrNotExist) {
		return Status{State: NotInstalled}
	}
	if err != nil {
		return Status{State: Invalid, Detail: err.Error()}
	}
	if config.LauncherSHA256 == "" {
		return Status{State: Invalid, Config: config, Detail: "launcher metadata has no identity digest"}
	}
	path, err := fsutil.Join(root, SteamExecutable)
	if err != nil {
		return Status{State: Invalid, Config: config, Detail: err.Error()}
	}
	digest, err := Hash(ctx, path)
	if errors.Is(err, os.ErrNotExist) {
		return Status{State: Replaced, Config: config, Detail: "Steam entrypoint is missing"}
	}
	if err != nil {
		return Status{State: Invalid, Config: config, Detail: err.Error()}
	}
	if digest != config.LauncherSHA256 {
		return Status{State: Replaced, Config: config, Detail: "Steam entrypoint no longer matches the installed launcher"}
	}
	version, _ := launcherversion.ReadFile(path)
	return Status{State: Installed, Config: config, Version: version}
}

// MetadataPresent reports whether the Steam entrypoint name is reserved for a
// managed launcher. A malformed record also reserves the name: patching a
// possible launcher is more dangerous than asking the user to repair metadata.
func MetadataPresent(root string) bool {
	path, err := metadata(root)
	if err != nil {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func GamePath(root string) (string, error) { return fsutil.Join(root, RealGameExecutable) }

func Install(ctx context.Context, root, launcherSource string) (Status, error) {
	return InstallWithDowngrader(ctx, root, launcherSource, "")
}

func InstallWithDowngrader(ctx context.Context, root, launcherSource, downgraderSource string) (Status, error) {
	game, err := GamePath(root)
	if err != nil {
		return Status{}, err
	}
	if err = regular(game, "real GTA executable"); err != nil {
		return Status{}, err
	}
	if err = regular(launcherSource, "launcher binary"); err != nil {
		return Status{}, err
	}
	target, err := fsutil.Join(root, SteamExecutable)
	if err != nil {
		return Status{}, err
	}
	sourceAbs, _ := filepath.Abs(launcherSource)
	targetAbs, _ := filepath.Abs(target)
	if filepath.Clean(sourceAbs) == filepath.Clean(targetAbs) {
		return Status{}, fmt.Errorf("launcher source is already the Steam entrypoint")
	}
	launcherDigest, err := Hash(ctx, launcherSource)
	if err != nil {
		return Status{}, err
	}
	gameDigest, err := Hash(ctx, game)
	if err != nil {
		return Status{}, err
	}
	if gameDigest == launcherDigest {
		return Status{}, fmt.Errorf("gta_sa.exe is the launcher, not the real game executable")
	}

	old, loadErr := Load(root)
	if loadErr != nil && !errors.Is(loadErr, os.ErrNotExist) {
		return Status{}, loadErr
	}
	config := Config{SchemaVersion: SchemaVersion, LauncherExecutable: SteamExecutable, GameExecutable: RealGameExecutable, LauncherSHA256: launcherDigest}
	if loadErr == nil {
		config.BackupExecutable, config.BackupSHA256 = old.BackupExecutable, old.BackupSHA256
		config.DowngraderExecutable, config.DowngraderSHA256 = old.DowngraderExecutable, old.DowngraderSHA256
	}
	if downgraderSource != "" {
		downgraderSource, err = filepath.Abs(downgraderSource)
		if err != nil {
			return Status{}, err
		}
		if err = regular(downgraderSource, "downgrader executable"); err != nil {
			return Status{}, err
		}
		config.DowngraderExecutable = downgraderSource
		config.DowngraderSHA256, err = Hash(ctx, downgraderSource)
		if err != nil {
			return Status{}, err
		}
	}

	if _, statErr := os.Stat(target); statErr == nil && config.BackupExecutable == "" {
		preserve := true
		if loadErr == nil {
			current, hashErr := Hash(ctx, target)
			preserve = hashErr != nil || current != old.LauncherSHA256
		}
		if preserve {
			backup, joinErr := fsutil.Join(root, backupPath)
			if joinErr != nil {
				return Status{}, joinErr
			}
			if info, statErr := os.Stat(backup); statErr == nil && info.Mode().IsRegular() {
				return Status{}, fmt.Errorf("untracked Steam entrypoint backup already exists at %q", backup)
			} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
				return Status{}, statErr
			}
			if err = fsutil.CopyFile(ctx, target, backup); err != nil {
				return Status{}, fmt.Errorf("preserve existing Steam entrypoint: %w", err)
			}
			config.BackupExecutable = backupPath
			config.BackupSHA256, err = Hash(ctx, backup)
			if err != nil {
				return Status{}, err
			}
		}
	} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return Status{}, statErr
	}
	if err = writeConfig(ctx, root, config); err != nil {
		return Status{}, err
	}
	if err = fsutil.CopyFile(ctx, launcherSource, target); err != nil {
		return Status{}, fmt.Errorf("install Steam launcher: %w", err)
	}
	status := Inspect(ctx, root)
	if status.State != Installed {
		return status, fmt.Errorf("launcher verification failed: %s", status.Detail)
	}
	return status, nil
}

func Remove(ctx context.Context, root string) error {
	config, err := Load(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	target, err := fsutil.Join(root, SteamExecutable)
	if err != nil {
		return err
	}
	if _, statErr := os.Stat(target); statErr == nil {
		digest, hashErr := Hash(ctx, target)
		if hashErr != nil {
			return hashErr
		}
		if digest != config.LauncherSHA256 {
			return fmt.Errorf("Steam entrypoint is not the managed launcher; refusing to overwrite it")
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return statErr
	}
	if config.BackupExecutable != "" {
		backup, joinErr := fsutil.Join(root, config.BackupExecutable)
		if joinErr != nil {
			return joinErr
		}
		digest, hashErr := Hash(ctx, backup)
		if hashErr != nil {
			return fmt.Errorf("restore Steam entrypoint backup: %w", hashErr)
		}
		if digest != config.BackupSHA256 {
			return fmt.Errorf("Steam entrypoint backup digest does not match metadata")
		}
		if err = fsutil.CopyFile(ctx, backup, target); err != nil {
			return err
		}
	} else if err = os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	path, err := metadata(root)
	if err != nil {
		return err
	}
	if err = os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if config.BackupExecutable != "" {
		if backup, joinErr := fsutil.Join(root, config.BackupExecutable); joinErr == nil {
			_ = os.Remove(backup)
			_ = os.Remove(filepath.Dir(backup))
		}
	}
	return nil
}

func writeConfig(ctx context.Context, root string, config Config) error {
	path, err := metadata(root)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.Write(ctx, path, 0600, func(w io.Writer) error { _, err := w.Write(data); return err })
}

func Hash(ctx context.Context, path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err = fsutil.Copy(ctx, hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func regular(path, description string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%s %q: %w", description, path, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s %q is not a regular file", description, path)
	}
	return nil
}
