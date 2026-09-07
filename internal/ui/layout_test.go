package ui

import (
	"image/png"
	"os"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
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
	w.table.Refresh()
	w.refreshEnabled()
	w.version.SetText("NewSteam R2  ·  Writable")
	w.checks["SilentPatch"].SetChecked(true)
	w.modLabels["SilentPatch"].SetText("Installed · 1.1-BUILD34.1-SA")
	w.modLabels["Widescreen Fixes"].SetText("Installed · 2025-12-08")
	w.status.SetText("Scan complete: 17 manifest files checked.")
	w.progress.SetValue(1)
	w.window.Show()
	for _, size := range []fyne.Size{fyne.NewSize(840, 560), fyne.NewSize(720, 480)} {
		w.window.Resize(size)
		canvasSize := w.window.Canvas().Size()
		if canvasSize.Width > size.Width || canvasSize.Height > size.Height {
			t.Fatalf("layout forced window from %v to %v", size, canvasSize)
		}
		if _, cols := w.table.Length(); cols != 5 {
			t.Fatalf("expected all five file detail columns, got %d", cols)
		}
		if w.table.Size().Height < max(float32(150), size.Height*0.35) {
			t.Fatalf("file table gets only %v of %v", w.table.Size(), size)
		}
		for _, object := range []fyne.CanvasObject{w.version, w.modsTitle, w.downgrade, w.status} {
			pos := a.Driver().AbsolutePositionForObject(object)
			if pos.Y+object.Size().Height > canvasSize.Height+1 || pos.X+object.Size().Width > canvasSize.Width+1 {
				t.Fatalf("essential control is outside the window: %T", object)
			}
		}
	}
	w.window.Resize(fyne.NewSize(840, 560))
	if !w.modPanel.Visible() {
		t.Fatal("mod controls must always be visible")
	}
	snapshot(t, w, "GTASA_COMPACT_SNAPSHOT")
	// Long download messages and upstream release names must not increase the
	// minimum window size or push the main actions beyond the viewport.
	w.status.SetText(strings.Repeat("Long download status with a full game file path. ", 12))
	w.modLabels["SilentPatch"].SetText(strings.Repeat("upstream release name ", 20))
	w.window.Resize(fyne.NewSize(720, 480))
	if w.window.Canvas().Size().Width > 720 || w.window.Canvas().Size().Height > 480 {
		t.Fatal("long mod/status text enlarged the window")
	}
	for _, check := range w.checks {
		pos := a.Driver().AbsolutePositionForObject(check)
		if pos.X < 0 || pos.X+check.MinSize().Width > 720 || pos.Y+check.MinSize().Height > 480 {
			t.Fatalf("mod grid clips %q", check.Text)
		}
		if check.Size().Width < check.MinSize().Width {
			t.Fatalf("mod label %q does not fit its column", check.Text)
		}
	}
	if w.table.Size().Height < 150 {
		t.Fatal("mods hide the file table")
	}
	if !w.modPanel.Visible() || len(w.selected()) != 3 {
		t.Fatal("mod panel lost selected loaders or mod")
	}
	w.modLabels["SilentPatch"].SetText("Installed · 1.1-BUILD34.1-SA")
	w.status.SetText("3 mods selected, including required loaders.")
	w.window.Resize(fyne.NewSize(840, 560))
	snapshot(t, w, "GTASA_MODS_SNAPSHOT")
	if w.modsTitle.Text != "Mods" {
		t.Fatal("mod heading includes unwanted summary text")
	}
	for col, want := range fileValues(w.rows[0]) {
		cell := w.table.CreateCell()
		w.table.UpdateCell(widget.TableCellID{Row: 1, Col: col}, cell)
		if got := cell.(*container.ThemeOverride).Content.(*widget.Label).Text; got != want {
			t.Fatalf("column %d: got %q, want %q", col, got, want)
		}

	}
	if w.progress.Visible() || w.cancelButton.Visible() {
		t.Fatal("idle window shows operation controls")
	}
	pos := a.Driver().AbsolutePositionForObject(w.downgrade)
	if pos.X < w.window.Canvas().Size().Width/2 {
		t.Fatal("Downgrade is not aligned right")
	}
	w.busy = true
	w.refreshEnabled()
	w.cancelButton.Enable()
	w.progress.SetValue(0.42)
	w.status.SetText("Downgrading audio/streams/BEATS…")
	if !w.progress.Visible() || !w.cancelButton.Visible() || w.progress.Size().Height > 4 {
		t.Fatal("active progress is not a thin visible track")
	}
	for _, size := range []fyne.Size{fyne.NewSize(720, 480), fyne.NewSize(840, 560)} {
		w.window.Resize(size)
		statusPos := a.Driver().AbsolutePositionForObject(w.status)
		progressPos := a.Driver().AbsolutePositionForObject(w.progress)
		cancelPos := a.Driver().AbsolutePositionForObject(w.cancelButton)
		if statusPos.X+w.status.Size().Width >= progressPos.X || progressPos.X+w.progress.Size().Width >= cancelPos.X {
			t.Fatal("status, progress and actions overlap")
		}
		if progressPos.Y < statusPos.Y || progressPos.Y+w.progress.Size().Height > statusPos.Y+w.status.Size().Height {
			t.Fatal("progress is outside the status row")
		}
		if w.progress.Size().Width < size.Width*0.4 {
			t.Fatal("progress lane is too small")
		}
	}
	snapshot(t, w, "GTASA_PROGRESS_SNAPSHOT")

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
