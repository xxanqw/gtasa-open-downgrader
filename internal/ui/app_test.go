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
	w.installedMods["SilentPatch"] = true
	w.refreshEnabled()
	if w.updateModsButton.Disabled() {
		t.Fatal("update disabled for an installed mod")
	}
	if !w.uninstallButtons["SilentPatch"].Visible() || w.uninstallButtons["SilentPatch"].Disabled() || w.uninstallButtons["SkyGFX"].Visible() {
		t.Fatal("uninstall icons do not reflect installed mods")
	}
	w.checks["SilentPatch"].SetChecked(true)
	for _, name := range []string{"ASI Loader", "ModLoader"} {
		if !w.checks[name].Checked || !w.checks[name].Disabled() {
			t.Fatal("missing locked dependency", name)
		}
	}
	w.readonly = true
	w.refreshEnabled()
	if !w.install.Disabled() || !w.downgrade.Disabled() || !w.uninstallButtons["SilentPatch"].Disabled() {
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

func TestInstalledModActionsFitCompactWindow(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := New(a)
	defer w.jobs.Close()
	w.window.Resize(fyne.NewSize(840, 560))
	w.window.Show()
	for name := range w.uninstallButtons {
		w.installedMods[name] = true
	}
	w.refreshEnabled()
	if w.modPanel.Size().Width > w.window.Canvas().Size().Width {
		t.Fatal("mod actions overflow the compact window")
	}
	if w.uninstallButtons["ASI Loader"].Position().X < 100 {
		t.Fatal("uninstall icon overlaps the mod name")
	}
	if path := os.Getenv("GTASA_MOD_UNINSTALL_SNAPSHOT"); path != "" {
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		if err := png.Encode(file, w.window.Canvas().Capture()); err != nil {
			t.Fatal(err)
		}
	}
}
