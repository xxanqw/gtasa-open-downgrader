package platform

import (
	"github.com/xxanqw/gtasa-open-downgrader/internal/testutil"
	"os"
	"path/filepath"
	"testing"
)

func TestGameProcessChecksBothNamesAndCase(t *testing.T) {
	for _, name := range []string{"gta_sa.exe\n", "gta-sa.exe\n", "GTA_SA.EXE\n"} {
		root := t.TempDir()
		testutil.Write(t, root, "42/comm", name)
		if err := requireGameClosedIn(root); err == nil {
			t.Fatal("missed running game", name)
		}
	}
	root := t.TempDir()
	testutil.Write(t, root, "1/comm", "gtasa-open-down\n")
	testutil.Write(t, root, "abc/comm", "gta_sa.exe\n")
	if err := requireGameClosedIn(root); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "99"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := requireGameClosedIn(root); err != nil {
		t.Fatal("exited process should be ignored", err)
	}
	if err := requireGameClosedIn(filepath.Join(root, "missing")); err == nil {
		t.Fatal("failed enumeration accepted")
	}
}
