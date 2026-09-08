package launcherapp

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/xxanqw/gtasa-open-downgrader/internal/entrypoint"
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
