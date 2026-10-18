package entrypoint

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func launcherUpdateFixture(t *testing.T) (string, string, Status) {
	t.Helper()
	root := t.TempDir()
	old := filepath.Join(t.TempDir(), "old.exe")
	next := filepath.Join(t.TempDir(), "new.exe")
	put(t, filepath.Join(root, RealGameExecutable), "real GTA")
	put(t, filepath.Join(root, SteamExecutable), "original Steam entrypoint")
	put(t, old, "old launcher")
	put(t, next, "new launcher")
	put(t, filepath.Join(root, ".downgrader/launcher-preferences.json"), "remembered mode")
	status, err := Install(context.Background(), root, old)
	if err != nil {
		t.Fatal(err)
	}
	return root, next, status
}
func assertLauncherBytes(t *testing.T, root, expected string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, SteamExecutable))
	if err != nil || string(data) != expected {
		t.Fatalf("launcher = %q, %v", data, err)
	}
	for _, pattern := range []string{".launcher-update-*", ".launcher-previous-*"} {
		files, _ := filepath.Glob(filepath.Join(root, pattern))
		if len(files) != 0 {
			t.Fatalf("leaked update files: %v", files)
		}
	}
}

func TestLauncherUpdatePreservesOriginalBackupAndChoice(t *testing.T) {
	root, source, before := launcherUpdateFixture(t)
	after, err := Update(context.Background(), root, source, before.Config.LauncherSHA256, func() error { return nil })
	if err != nil || after.State != Installed {
		t.Fatalf("update: %+v, %v", after, err)
	}
	if after.Config.BackupSHA256 != before.Config.BackupSHA256 {
		t.Fatal("original backup changed")
	}
	assertLauncherBytes(t, root, "new launcher")
	for file, expected := range map[string]string{RealGameExecutable: "real GTA", ".downgrader/launcher-preferences.json": "remembered mode"} {
		data, err := os.ReadFile(filepath.Join(root, file))
		if err != nil || string(data) != expected {
			t.Fatalf("changed %s", file)
		}
	}
	if err := Remove(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	assertLauncherBytes(t, root, "original Steam entrypoint")
}

func TestLauncherUpdateRollsBackOnMetadataFailure(t *testing.T) {
	root, source, before := launcherUpdateFixture(t)
	_, err := update(context.Background(), root, source, before.Config.LauncherSHA256, func() error { return nil }, func(context.Context, string, Config) error { return errors.New("metadata write failed") })
	if err == nil {
		t.Fatal("ignored metadata error")
	}
	assertLauncherBytes(t, root, "old launcher")
	if got := Inspect(context.Background(), root); got.State != Installed || got.Config != before.Config {
		t.Fatalf("metadata changed: %+v", got)
	}
}

func TestLauncherUpdateRechecksScopeAndClosedGame(t *testing.T) {
	for _, scenario := range []string{"no-guard", "busy", "busy-after-staging", "changed", "cancelled"} {
		t.Run(scenario, func(t *testing.T) {
			root, source, before := launcherUpdateFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			guard := func() error {
				calls++
				if scenario == "busy" || scenario == "busy-after-staging" && calls == 2 {
					return errors.New("GTA is running")
				}
				return nil
			}
			if scenario == "no-guard" {
				guard = nil
			}
			if scenario == "changed" {
				put(t, filepath.Join(root, SteamExecutable), "Steam replaced this file")
			}
			if scenario == "cancelled" {
				cancel()
			}
			if _, err := Update(ctx, root, source, before.Config.LauncherSHA256, guard); err == nil {
				t.Fatal("unsafe update accepted")
			}
			expected := "old launcher"
			if scenario == "changed" {
				expected = "Steam replaced this file"
			}
			assertLauncherBytes(t, root, expected)
		})
	}
}
