package launcherversion_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/xxanqw/gtasa-open-downgrader/internal/launcherversion"
	"github.com/xxanqw/gtasa-open-downgrader/internal/testutil"
)

func TestReadTrimmedLauncherVersionsWithoutExecuting(t *testing.T) {
	for _, version := range []string{"1", "2", "10"} {
		binary := testutil.Launcher(t, version)
		actual, err := launcherversion.ReadFile(binary)
		if err != nil || actual != version {
			t.Fatalf("version = %q, %v; want %s", actual, err, version)
		}
	}
	path := filepath.Join(t.TempDir(), "fake.exe")
	if err := os.WriteFile(path, []byte("MZ GTASA_STEAM_LAUNCHER_VERSION[99]"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := launcherversion.ReadFile(path); err == nil {
		t.Fatal("accepted marker without Go launcher identity")
	}
}
