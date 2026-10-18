package steam

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/xxanqw/gtasa-open-downgrader/internal/winedrive"
)

// GameDirectoryMappingPath plans a stable child path without mutating a prefix.
// G: is preferred; occupied unrelated/broken/raw-device letters are never stolen.
func (p *ProtonContext) GameDirectoryMappingPath(game string) (string, error) {
	view := winedrive.ViewPath(filepath.Join(p.CompatDataPath, "pfx", "drive_c"))
	game = canonicalPath(game)
	parent := filepath.Dir(game)
	devices := filepath.Join(p.CompatDataPath, "pfx", "dosdevices")
	var free byte
	for letter := byte('g'); letter <= 'y'; letter++ {
		mapping := filepath.Join(devices, string(letter)+":")
		// Wine's raw-device reservation wins during startup, even if a directory
		// link was added later. Never repurpose that physical-device letter.
		if _, e := os.Lstat(mapping + ":"); e == nil {
			continue
		} else if !errors.Is(e, os.ErrNotExist) {
			return "", e
		}
		st, err := os.Lstat(mapping)
		if err == nil {
			if st.Mode()&os.ModeSymlink != 0 {
				target, e := filepath.EvalSymlinks(mapping)
				if e == nil && (canonicalPath(target) == canonicalPath(view) || canonicalPath(target) == game || canonicalPath(target) == parent) {
					return winedrive.GamePath(letter - 'a' + 'A'), nil
				}
			}
			continue
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		if free == 0 {
			free = letter
		}
	}
	if free == 0 {
		return "", fmt.Errorf("no free Wine drive letter G: through Y:; existing mappings were retained")
	}
	return winedrive.GamePath(free - 'a' + 'A'), nil
}

// EnsureGameDirectoryMapping exposes one normal GTA folder, not an entire
// Steam common/library tree. Callers guard writes against running GTA/launcher.
func (p *ProtonContext) EnsureGameDirectoryMapping(game string) (string, error) {
	result, err := p.GameDirectoryMappingPath(game)
	if err != nil {
		return "", err
	}
	driveC := filepath.Join(p.CompatDataPath, "pfx", "drive_c")
	view, err := winedrive.EnsureView(driveC, canonicalPath(game))
	if err != nil {
		return "", err
	}
	view = canonicalPath(view)
	devices := filepath.Join(p.CompatDataPath, "pfx", "dosdevices")
	if st, e := os.Stat(devices); e != nil || !st.IsDir() {
		return "", fmt.Errorf("existing Wine/Proton dosdevices directory is missing")
	}
	mapping := filepath.Join(devices, strings.ToLower(result[:2]))
	if winedrive.SameDirectory(mapping, view) {
		return result, nil
	}
	if st, e := os.Lstat(mapping); e == nil {
		if st.Mode()&os.ModeSymlink == 0 || !(winedrive.SameDirectory(mapping, game) || winedrive.SameDirectory(mapping, filepath.Dir(canonicalPath(game)))) {
			return "", fmt.Errorf("Wine drive changed; existing mapping retained")
		}
		backup := mapping + ".gtasa-before-filtered-view"
		if err = moveDriveNoReplace(mapping, backup); err != nil {
			return "", fmt.Errorf("preserve previous GTA drive mapping: %w", err)
		}
		saved, e := os.Lstat(backup)
		if e != nil || !os.SameFile(st, saved) {
			_ = moveDriveNoReplace(backup, mapping)
			return "", fmt.Errorf("Wine drive changed during migration; original retained in %s", backup)
		}
		if err = os.Symlink(view, mapping); err != nil {
			_ = moveDriveNoReplace(backup, mapping)
			return "", err
		}
	} else if errors.Is(e, os.ErrNotExist) {
		if err = os.Symlink(view, mapping); err != nil {
			return "", err
		}
	} else {
		return "", e
	}
	if !winedrive.SameDirectory(filepath.Join(mapping, winedrive.ChildName), game) {
		return "", fmt.Errorf("prepared Wine drive does not resolve to the selected game")
	}
	return result, nil
}

// prepareGameDrive runs before every managed Proton invocation, including fonts,
// installers and Registry setup. The matching launcher component refreshes the
// active mount manager, not just prefix symlinks cached by a live wineserver.
func (p *ProtonContext) prepareGameDrive(ctx context.Context) error {
	if p.DriveGuard == nil {
		return nil
	} // Generic discovery/command fixtures do not opt into game writes.
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := p.DriveGuard(); err != nil {
		return err
	}
	result, err := p.EnsureGameDirectoryMapping(p.GameDir)
	if err != nil {
		return err
	}
	if p.GameDriveExpected != "" && result != p.GameDriveExpected {
		return fmt.Errorf("Wine game drive changed; refresh Multiplayer before continuing")
	}
	helper, err := winedrive.FindHelper()
	if err != nil {
		return err
	}
	target, err := p.WindowsPath(p.GameDir)
	if err != nil {
		return err
	}
	// A utility must not go through steam.exe: the normal game-bootstrap verb
	// can detach the target, lose stdout/status, or ask Steam to launch the game.
	// runinprefix invokes Wine directly with this selected Proton environment.
	command := exec.CommandContext(ctx, p.ProtonExecutable, "runinprefix", helper, winedrive.HelperFlag, target)
	command.Env = p.utilityEnvironment(os.Environ())
	command.Dir = p.GameDir
	var output boundedDriveOutput
	var diagnostics boundedDriveDiagnostics
	command.Stdout = &output
	command.Stderr = &diagnostics
	if err = command.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if message, ok := strings.CutPrefix(strings.TrimSpace(string(output)), "GTASA-DRIVE-ERROR:"); ok {
			return fmt.Errorf("prepare Wine game drive: %s", strings.TrimSpace(message))
		}
		if detail := strings.TrimSpace(string(diagnostics)); detail != "" {
			return fmt.Errorf("activate GTA drive through selected Proton: %w\n\n%s", err, detail)
		}
		return fmt.Errorf("activate GTA drive through selected Proton: %w; the runtime returned no diagnostic output", err)
	}
	if actual := strings.TrimSpace(string(output)); actual != result {
		return fmt.Errorf("Wine selected %q instead of planned %q; close other Wine applications and refresh Multiplayer. Existing drives were retained", actual, result)
	}
	return nil
}

// Utility status must return to the caller, not be detached into a game's
// PROTON_LOG file (which can also replace an existing game-session log). This
// override belongs only to this child; game/installer logging stays untouched.
func (p *ProtonContext) utilityEnvironment(base []string) []string {
	env := p.Environment(base)
	result := make([]string, 0, len(env)+1)
	for _, item := range env {
		if !strings.HasPrefix(item, "PROTON_LOG=") {
			result = append(result, item)
		}
	}
	return append(result, "PROTON_LOG=0")
}

type boundedDriveDiagnostics []byte

func (b *boundedDriveDiagnostics) Write(p []byte) (int, error) {
	n := len(p)
	const limit = 8192
	if len(p) >= limit {
		*b = append((*b)[:0], p[len(p)-limit:]...)
	} else {
		if overflow := len(*b) + len(p) - limit; overflow > 0 {
			copy(*b, (*b)[overflow:])
			*b = (*b)[:len(*b)-overflow]
		}
		*b = append(*b, p...)
	}
	return n, nil
}

type boundedDriveOutput []byte

func (b *boundedDriveOutput) Write(p []byte) (int, error) {
	n := len(p)
	remaining := 4096 - len(*b)
	if remaining > 0 {
		*b = append(*b, p[:min(len(p), remaining)]...)
	}
	return n, nil
}
