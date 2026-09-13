package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/xxanqw/gtasa-open-downgrader/internal/game"
	"image/png"
	"os"
	"testing"
)

func TestControlsRequireCurrentWritableScanAndLoaders(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := New(a)
	defer w.jobs.Close()
	if !w.downgrade.Disabled() || !w.install.Disabled() || !w.updateModsButton.Disabled() {
		t.Fatal("mutation enabled before scan")
	}
	w.path.SetText(t.TempDir())
	w.scanPath = w.path.Text
	w.patchDir = t.TempDir()
	w.refreshEnabled()
	if !w.updateModsButton.Disabled() {
		t.Fatal("update enabled without installed mods")
	}
	w.installedModCount = 1
	w.refreshEnabled()
	if w.updateModsButton.Disabled() {
		t.Fatal("update disabled for an installed mod")
	}
	w.checks["SilentPatch"].SetChecked(true)
	for _, name := range []string{"ASI Loader", "ModLoader"} {
		if !w.checks[name].Checked || !w.checks[name].Disabled() {
			t.Fatal("missing locked dependency", name)
		}
	}
	w.readonly = true
	w.refreshEnabled()
	if !w.install.Disabled() || !w.downgrade.Disabled() {
		t.Fatal("read-only mutation enabled")
	}
	w.readonly = false
	w.path.SetText(t.TempDir())
	if !w.install.Disabled() {
		t.Fatal("stale scan enabled for changed path")
	}
}
func TestWindowLayout(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := New(a)
	defer w.jobs.Close()
	w.window.Resize(fyne.NewSize(1100, 760))
	w.window.Show()
	w.rows = []game.Row{{Path: "gta-sa.exe", Status: "Original (Needs Patch)", NeedsPatch: "Yes", CurrentHash: "5bfd4dd83989a8264de4b8e771f237fd", TargetHash: "170b3a9108687b26da2d8901c6948a18"}, {Path: "audio/CONFIG/BankLkup.dat", Status: "Already Downgraded", NeedsPatch: "No"}}
	w.table.Refresh()
	w.version.SetText("Version: NewSteam R2 · Read-only: false")
	if p := os.Getenv("GTASA_UI_SNAPSHOT"); p != "" {
		f, e := os.Create(p)
		if e != nil {
			t.Fatal(e)
		}
		defer f.Close()
		if e = png.Encode(f, w.window.Canvas().Capture()); e != nil {
			t.Fatal(e)
		}
	}
}
