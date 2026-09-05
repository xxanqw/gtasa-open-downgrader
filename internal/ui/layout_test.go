package ui

import (
	"image/png"
	"os"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/xxanqw/gtasa-open-downgrader/internal/game"
)

func TestCompactWindowKeepsGameInformationVisible(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := New(a)
	defer w.jobs.Close()
	w.path.SetText("/games/Grand Theft Auto San Andreas")
	w.scanPath = w.path.Text
	w.patchDir = t.TempDir()
	for _, path := range []string{"gta-sa.exe", "anim/anim.img", "audio/CONFIG/TrakLkup.dat", "audio/streams/BEATS", "audio/streams/CH", "audio/streams/CR", "audio/streams/CUTSCENE", "audio/streams/DS", "audio/streams/HC", "audio/streams/MH", "audio/streams/MR", "audio/streams/NJ", "audio/streams/RE", "audio/streams/RG", "audio/streams/TK", "audio/streams/UV", "models/gta3.img"} {
		w.rows = append(w.rows, game.Row{Path: path, Status: "Original (Needs Patch)", NeedsPatch: "Yes", CurrentHash: "5bfd4dd83989a8264de4b8e771f237fd", TargetHash: "170b3a9108687b26da2d8901c6948a18"})
	}
	w.rows[1].Status = "Already Downgraded"
	w.rows[1].NeedsPatch = "No"
	w.rows[2].Status = "Missing"
	w.rows[2].NeedsPatch = "N/A"
	w.refreshSummary()
	w.table.Refresh()
	w.refreshEnabled()
	w.version.SetText("NewSteam R2  ·  Writable")
	w.checks["SilentPatch"].SetChecked(true)
	w.modLabels["SilentPatch"].SetText("Installed · 1.1-BUILD34.1-SA")
	w.modLabels["Widescreen Fixes"].SetText("Update available · 2026-09-05")
	w.status.SetText("Scan complete. Select a file for details.")
	w.progress.SetValue(1)
	w.window.Show()
	for _, size := range []fyne.Size{fyne.NewSize(840, 560), fyne.NewSize(720, 480)} {
		w.window.Resize(size)
		canvasSize := w.window.Canvas().Size()
		if canvasSize.Width > size.Width || canvasSize.Height > size.Height {
			t.Fatalf("layout forced window from %v to %v", size, canvasSize)
		}
		if w.table.Size().Height < size.Height*0.55 {
			t.Fatalf("file table gets only %v of %v", w.table.Size(), size)
		}
		for _, object := range []fyne.CanvasObject{w.version, w.summary, w.downgrade, w.status, w.progress, w.cancelButton} {
			pos := a.Driver().AbsolutePositionForObject(object)
			if pos.Y+object.Size().Height > canvasSize.Height+1 || pos.X+object.Size().Width > canvasSize.Width+1 {
				t.Fatalf("essential control is outside the window: %T", object)
			}
		}
	}
	w.window.Resize(fyne.NewSize(840, 560))
	snapshot(t, w, "GTASA_COMPACT_SNAPSHOT")
	// Long download messages and upstream release names must not increase the
	// minimum window size or push the main actions beyond the viewport.
	w.status.SetText(strings.Repeat("Long download status with a full game file path. ", 12))
	w.modLabels["SilentPatch"].SetText(strings.Repeat("upstream release name ", 20))
	w.tabs.SelectIndex(1)
	w.window.Resize(fyne.NewSize(720, 480))
	if w.window.Canvas().Size().Width > 720 || w.window.Canvas().Size().Height > 480 {
		t.Fatal("long mod/status text enlarged the window")
	}
	w.modLabels["SilentPatch"].SetText("Installed · 1.1-BUILD34.1-SA")
	w.status.SetText("3 mods selected, including required loaders.")
	w.window.Resize(fyne.NewSize(840, 560))
	snapshot(t, w, "GTASA_MODS_SNAPSHOT")
	w.tabs.SelectIndex(0)
	w.table.Select(widget.TableCellID{Row: 1, Col: 0})
	if w.detailsButton.Disabled() {
		t.Fatal("full file details cannot be opened")
	}
	details := fileDetails(w.rows[0])
	if !strings.Contains(details, w.rows[0].CurrentHash) || !strings.Contains(details, w.rows[0].TargetHash) {
		t.Fatal("full hashes lost from details")
	}
}
func snapshot(t *testing.T, w *Window, key string) {
	t.Helper()
	if p := os.Getenv(key); p != "" {
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
