package launcherapp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xxanqw/gtasa-open-downgrader/internal/entrypoint"
	"github.com/xxanqw/gtasa-open-downgrader/internal/testutil"
)

func TestSinglePlayerRecoversIdenticalModLoaderCopies(t *testing.T) {
	root := t.TempDir()
	testutil.Write(t, root, entrypoint.RealGameExecutable, "game")
	testutil.Write(t, root, "modloader.asi", "same loader")
	testutil.Write(t, root, ".downgrader/multiplayer/modloader.asi.disabled", "same loader")
	launched := 0
	runtime := &Runtime{GameDir: root, Execute: func(context.Context, string, string, ...string) error { launched++; return nil }}
	for attempt := 0; attempt < 2; attempt++ {
		if err := runtime.SinglePlayer(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if launched != 2 {
		t.Fatal("single player did not launch")
	}
	if testutil.Read(t, root, "modloader.asi") != "same loader" {
		t.Fatal("active loader changed")
	}
	if _, err := os.Lstat(filepath.Join(root, ".downgrader/multiplayer/modloader.asi.disabled")); !os.IsNotExist(err) {
		t.Fatal("redundant copy still blocks recovery")
	}
	backups, err := filepath.Glob(filepath.Join(root, ".downgrader/recovered-modloader-*/modloader.asi.disabled"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("missing preserved copy: %v %v", backups, err)
	}
	data, err := os.ReadFile(backups[0])
	if err != nil || string(data) != "same loader" {
		t.Fatal("duplicate was not preserved")
	}
}

func TestDifferentModLoaderCopiesAreNotOverwritten(t *testing.T) {
	root := t.TempDir()
	testutil.Write(t, root, "modloader.asi", "new loader")
	testutil.Write(t, root, ".downgrader/multiplayer/modloader.asi.disabled", "old loader")
	runtime := &Runtime{GameDir: root, Execute: func(context.Context, string, string, ...string) error {
		t.Fatal("launched despite conflict")
		return nil
	}}
	err := runtime.SinglePlayer(context.Background())
	if err == nil || !strings.Contains(err.Error(), "copies differ") {
		t.Fatalf("unexpected conflict: %v", err)
	}
	if testutil.Read(t, root, "modloader.asi") != "new loader" || testutil.Read(t, root, ".downgrader/multiplayer/modloader.asi.disabled") != "old loader" {
		t.Fatal("conflicting loaders were changed")
	}
}

func TestModLoaderRecoveryRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	testutil.Write(t, root, "modloader.asi", "loader")
	testutil.Write(t, root, "outside.asi", "loader")
	directory := filepath.Join(root, ".downgrader/multiplayer")
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	disabled := filepath.Join(directory, "modloader.asi.disabled")
	if err := os.Symlink(filepath.Join(root, "outside.asi"), disabled); err != nil {
		t.Skip(err)
	}
	runtime := &Runtime{GameDir: root}
	if err := runtime.restoreModLoader(); err == nil {
		t.Fatal("accepted a symlink")
	}
	if info, err := os.Lstat(disabled); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("symlink was modified")
	}
}
