package mods

import (
	"context"
	"github.com/xxanqw/gtasa-open-downgrader/internal/testutil"
	"os"
	"path/filepath"
	"testing"
)

func TestRetiredRussianRefusesNewInstallation(t *testing.T) {
	root := t.TempDir()
	archive := testutil.Zip(t, map[string]string{"00 - Copy to game folder/text/american.gxt": "translation"})
	for _, name := range []string{SanLtdRussian, legacySALtdRussian} {
		if Supported(name) {
			t.Fatal("retired mod supported", name)
		}
		if err := Extract(context.Background(), root, name, archive, "zip"); err == nil {
			t.Fatal("retired archive accepted", name)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "modloader", "SALtdRussian")); !os.IsNotExist(err) {
		t.Fatal("retired import wrote game files")
	}
	if len(uninstallPaths(SanLtdRussian)) == 0 {
		t.Fatal("legacy uninstall lost")
	}
}
