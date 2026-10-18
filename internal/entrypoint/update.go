package entrypoint

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/xxanqw/gtasa-open-downgrader/internal/fsutil"
)

// Update replaces only a recognized, closed launcher. The original Steam backup,
// downgrader identity, remembered mode, and real GTA executable are left intact.
func Update(ctx context.Context, root, source, expectedDigest string, closed func() error) (Status, error) {
	return update(ctx, root, source, expectedDigest, closed, writeConfig)
}

func update(ctx context.Context, root, source, expectedDigest string, closed func() error, save func(context.Context, string, Config) error) (Status, error) {
	if closed == nil {
		return Status{}, fmt.Errorf("launcher updates require a closed-game guard")
	}
	if err := closed(); err != nil {
		return Status{}, err
	}
	before := Inspect(ctx, root)
	if before.State != Installed || expectedDigest == "" || before.Config.LauncherSHA256 != expectedDigest {
		return Status{}, fmt.Errorf("installed launcher changed; rescan before updating")
	}
	if before.Config.BackupExecutable != "" {
		backup, err := fsutil.Join(root, before.Config.BackupExecutable)
		if err != nil {
			return Status{}, err
		}
		digest, err := Hash(ctx, backup)
		if err != nil || digest != before.Config.BackupSHA256 {
			return Status{}, fmt.Errorf("original Steam entrypoint backup is missing or changed")
		}
	}
	target, err := fsutil.Join(root, SteamExecutable)
	if err != nil {
		return Status{}, err
	}
	sourceAbs, err := filepath.Abs(source)
	if err != nil {
		return Status{}, err
	}
	if sourceAbs == target {
		return Status{}, fmt.Errorf("launcher update source is already installed")
	}
	stage, err := os.CreateTemp(root, ".launcher-update-*.exe")
	if err != nil {
		return Status{}, err
	}
	stage.Close()
	defer os.Remove(stage.Name())
	if err := fsutil.CopyFile(ctx, source, stage.Name()); err != nil {
		return Status{}, err
	}
	if err := os.Chmod(stage.Name(), 0755); err != nil {
		return Status{}, err
	}
	digest, err := Hash(ctx, stage.Name())
	if err != nil {
		return Status{}, err
	}
	originalDigest, err := Hash(ctx, source)
	if err != nil || digest != originalDigest {
		return Status{}, fmt.Errorf("launcher update source changed while staging")
	}
	previous, err := os.CreateTemp(root, ".launcher-previous-*.exe")
	if err != nil {
		return Status{}, err
	}
	previous.Close()
	preservePrevious := false
	defer func() {
		if !preservePrevious {
			os.Remove(previous.Name())
		}
	}()
	if err := fsutil.CopyFile(ctx, target, previous.Name()); err != nil {
		return Status{}, err
	}
	previousDigest, err := Hash(ctx, previous.Name())
	if err != nil || previousDigest != before.Config.LauncherSHA256 {
		return Status{}, fmt.Errorf("installed launcher changed while preserving the previous executable")
	}
	if err := closed(); err != nil {
		return Status{}, err
	}
	current := Inspect(ctx, root)
	if current.State != Installed || current.Config != before.Config {
		return Status{}, fmt.Errorf("installed launcher changed while preparing the update")
	}
	if err := ctx.Err(); err != nil {
		return Status{}, err
	}
	if err := os.Rename(stage.Name(), target); err != nil {
		return Status{}, fmt.Errorf("replace launcher (close GTA and the launcher first): %w", err)
	}
	config := before.Config
	config.LauncherSHA256 = digest
	// Once replacement starts, finish metadata or rollback even if cancelled.
	commitCtx := context.WithoutCancel(ctx)
	if err := save(commitCtx, root, config); err != nil {
		if rollback := os.Rename(previous.Name(), target); rollback != nil {
			preservePrevious = true
			return Status{}, fmt.Errorf("save launcher metadata: %v; rollback failed: %v; previous launcher retained at %s", err, rollback, previous.Name())
		}
		return Status{}, fmt.Errorf("save launcher metadata; previous launcher restored: %w", err)
	}
	result := Inspect(commitCtx, root)
	if result.State != Installed {
		preservePrevious = true
		return result, fmt.Errorf("launcher changed after update; previous launcher retained at %s", previous.Name())
	}
	return result, nil
}
