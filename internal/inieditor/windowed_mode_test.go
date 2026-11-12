package inieditor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/xxanqw/gtasa-open-downgrader/internal/mods"
)

func TestWindowedModeDiscoversOnlyItsINIFormattedCFG(t *testing.T) {
	root := t.TempDir()
	folder := filepath.Join(root, "modloader/WindowedMode")
	if err := os.MkdirAll(folder, 0755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{"III.VC.SA.WindowedMode.cfg": "[MAIN]\nWindowMode = 2\n", "unrelated.cfg": "unrelated"} {
		if err := os.WriteFile(filepath.Join(folder, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	files, err := Discover(root, mods.WindowedMode)
	if err != nil || len(files) != 1 || filepath.Base(files[0]) != "III.VC.SA.WindowedMode.cfg" {
		t.Fatal(files, err)
	}
	doc, err := Load(root, mods.WindowedMode, files[0])
	if err != nil {
		t.Fatal(err)
	}
	if issues := Validate(doc.Text); len(issues) != 0 {
		t.Fatal(issues)
	}
}
