package launcherapp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/xxanqw/gtasa-open-downgrader/internal/multiplayer"
	"github.com/xxanqw/gtasa-open-downgrader/internal/testutil"
)

func TestMTAUncertainExitKeepsCompatibilityFilesForRecovery(t *testing.T) {
	root := t.TempDir()
	testutil.Write(t, root, "modloader.asi", "original loader")
	runtime := &Runtime{GameDir: root, Execute: func(context.Context, string, string, ...string) error { return errMTAExitUnknown }, snapshotGameProcesses: func() (gameProcessSet, error) { return gameProcessSet{}, nil }}
	err := runtime.runWithoutModLoader(context.Background(), multiplayer.Client{Executable: "unused.exe"})
	if !errors.Is(err, errMTAExitUnknown) {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "modloader.asi")); !os.IsNotExist(err) {
		t.Fatal("restored Mod Loader before confirming client exit")
	}
	if testutil.Read(t, root, ".downgrader/multiplayer/modloader.asi.disabled") != "original loader" {
		t.Fatal("lost paused loader")
	}
	if err := runtime.restoreCompatibility(); err != nil {
		t.Fatal(err)
	}
	if testutil.Read(t, root, "modloader.asi") != "original loader" {
		t.Fatal("did not recover after confirmed close")
	}
}
