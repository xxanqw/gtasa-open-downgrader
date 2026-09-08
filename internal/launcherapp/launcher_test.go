package launcherapp

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/xxanqw/gtasa-open-downgrader/internal/entrypoint"
	"github.com/xxanqw/gtasa-open-downgrader/internal/multiplayer"
)

func TestSinglePlayerUsesRealExecutableAndPassesArguments(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, entrypoint.RealGameExecutable), []byte("game"), 0644); err != nil {
		t.Fatal(err)
	}
	var got []string
	runtime := &Runtime{GameDir: root, Execute: func(_ context.Context, dir, exe string, args ...string) error {
		got = append([]string{dir, exe}, args...)
		return nil
	}}
	if err := runtime.SinglePlayer(context.Background(), "-windowed"); err != nil {
		t.Fatal(err)
	}
	want := []string{root, filepath.Join(root, entrypoint.RealGameExecutable), "-windowed"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("call = %#v, want %#v", got, want)
	}
}

func TestSinglePlayerNeverFallsBackToLauncher(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, entrypoint.SteamExecutable), []byte("launcher"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := New(root).SinglePlayer(context.Background()); err == nil {
		t.Fatal("launcher recursively executed itself")
	}
}

func TestSAMPTemporarilyDisablesAndRestoresModLoader(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"samp.exe", "samp.dll", "modloader.asi"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0644); err != nil {
			t.Fatal(err)
		}
	}
	runtime := &Runtime{
		GameDir: root,
		Execute: func(_ context.Context, _, _ string, _ ...string) error {
			active, disabled := runtimePaths(root)
			if _, err := os.Stat(active); !os.IsNotExist(err) {
				t.Fatalf("Mod Loader remained active during SA-MP launch: %v", err)
			}
			if _, err := os.Stat(disabled); err != nil {
				t.Fatalf("disabled Mod Loader is missing: %v", err)
			}
			return nil
		},
		AfterSAMPExit: func(context.Context) error { return nil },
	}
	if err := runtime.Multiplayer(context.Background(), multiplayer.ClientSAMP); err != nil {
		t.Fatal(err)
	}
	active, disabled := runtimePaths(root)
	if data, err := os.ReadFile(active); err != nil || string(data) != "modloader.asi" {
		t.Fatalf("Mod Loader was not restored: %q, %v", data, err)
	}
	if _, err := os.Stat(disabled); !os.IsNotExist(err) {
		t.Fatalf("disabled copy remained after launch: %v", err)
	}
}

func TestSinglePlayerRecoversInterruptedSAMPCompatibilityMode(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, entrypoint.RealGameExecutable), []byte("game"), 0644); err != nil {
		t.Fatal(err)
	}
	_, disabled := runtimePaths(root)
	if err := os.MkdirAll(filepath.Dir(disabled), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(disabled, []byte("loader"), 0644); err != nil {
		t.Fatal(err)
	}
	runtime := &Runtime{GameDir: root, Execute: func(_ context.Context, _, _ string, _ ...string) error { return nil }}
	if err := runtime.SinglePlayer(context.Background()); err != nil {
		t.Fatal(err)
	}
	active, _ := runtimePaths(root)
	if data, err := os.ReadFile(active); err != nil || string(data) != "loader" {
		t.Fatalf("stale Mod Loader was not recovered: %q, %v", data, err)
	}
}

func runtimePaths(root string) (string, string) {
	return filepath.Join(root, "modloader.asi"), filepath.Join(root, ".downgrader", "multiplayer", "modloader.asi.disabled")
}

func TestFindDowngraderUsesVerifiedMetadata(t *testing.T) {
	root, sourceDir := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(root, entrypoint.RealGameExecutable), []byte("game"), 0644); err != nil {
		t.Fatal(err)
	}
	launcher := filepath.Join(sourceDir, entrypoint.SteamExecutable)
	downgrader := filepath.Join(sourceDir, "gtasa-open-downgrader-windows.exe")
	if err := os.WriteFile(launcher, []byte("launcher"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(downgrader, []byte("downgrader"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := entrypoint.InstallWithDowngrader(context.Background(), root, launcher, downgrader); err != nil {
		t.Fatal(err)
	}
	if got := FindDowngrader(root); got != downgrader {
		t.Fatalf("downgrader = %q", got)
	}
	if err := os.WriteFile(downgrader, []byte("changed"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := FindDowngrader(root); got != "" {
		t.Fatalf("modified downgrader trusted: %q", got)
	}
}
