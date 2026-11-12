package ui

import (
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	appwidget "github.com/xxanqw/gtasa-open-downgrader/internal/uiwidget"
)

type refreshProbe struct {
	widget.Label
	refreshes int
}

func (p *refreshProbe) Refresh() { p.refreshes++; p.Label.Refresh() }

func TestPromptsDoNotRedrawHiddenPages(t *testing.T) {
	w := newTestWindow(t)
	probe := &refreshProbe{}
	probe.Text = "Hidden page"
	probe.ExtendBaseWidget(probe)
	hidden := container.NewStack(probe)
	hidden.Hide()
	w.hubContent.Add(hidden)
	probe.refreshes = 0
	for i := 0; i < 3; i++ {
		start := time.Now()
		w.info("Performance check", "Opening a small message should not rebuild the workspace.")
		opened := time.Since(start)
		appwidget.DismissModals(w.window)
		t.Logf("modal open=%v close=%v", opened, time.Since(start)-opened)
	}
	if probe.refreshes != 0 {
		t.Fatalf("opening/closing prompts redrew hidden content %d times", probe.refreshes)
	}
}

func TestHiddenModRowsUpdateWhenShown(t *testing.T) {
	w := newTestWindow(t)
	verifiedTestGame(t, w)
	w.installedMods["SilentPatch"] = true
	w.refreshModBatch()
	w.showHub(hubMods)
	row := w.modTable.rows["SilentPatch"]
	test.WidgetRenderer(row)
	if row.state.Text != "Installed" {
		t.Fatalf("hidden model update was lost: %q", row.state.Text)
	}
	w.info("Test prompt", "Check modal control state.")
	if !row.check.Disabled() {
		t.Fatal("mod row remained enabled during prompt")
	}
	appwidget.DismissModals(w.window)
	if row.check.Disabled() {
		t.Fatal("mod row remained disabled after prompt")
	}
}

func TestWorkspaceRelayoutSettlesFooterVisibility(t *testing.T) {
	w := newTestWindow(t)
	w.window.Resize(fyne.NewSize(640, 480))
	w.busy = true
	w.operationTitle = "Downloading mods"
	w.browserDetails = "Waiting for a browser download"
	w.setStatus("Waiting")
	w.refreshEnabled()
	if w.footer.Size().Height <= 0 || !w.progressTrack.Visible() {
		t.Fatal("operation footer was not laid out")
	}
	w.busy = false
	w.browserDetails = ""
	w.refreshEnabled()
	if w.progressTrack.Visible() {
		t.Fatal("progress track did not collapse")
	}
}

func TestCaptureUnifiedProgressAcrossViews(t *testing.T) {
	w := seedCompactMods(t)
	w.window.Resize(fyne.NewSize(640, 480))
	for _, operation := range []struct{ page, title, message, name string }{
		{hubDowngrade, "Downgrading game", "models/gta_int.img · Backup & patching the original game archive", "progress-downgrade"},
		{hubMods, "Installing mods", "Widescreen Fixes · Downloading GTASA.WidescreenFix.zip from the official release · 42%", "progress-mods"},
		{hubRecovery, "Restoring backups", "models/gta_int.img · Restoring the original game archive from backups/", "progress-recovery"},
	} {
		w.busy = true
		w.operationTitle = operation.title
		w.setStatus(operation.message)
		w.setProgress(.42)
		w.refreshEnabled()
		w.cancelButton.Enable()
		w.showHub(operation.page)
		w.window.Canvas().Capture()
		if !strings.Contains(w.status.Text, operation.message) {
			t.Fatal("current action was shortened")
		}
		if w.status.Size().Width <= 300 || w.version.Visible() {
			t.Fatal("active action did not reclaim the footer width")
		}
		if w.hubReadiness.Visible() {
			t.Fatal("operation feedback duplicated above workspace")
		}
		if w.progressTrack.Size().Height != 3 {
			t.Fatalf("track grew to %v", w.progressTrack.Size())
		}
		captureUX(t, w, operation.name)
	}
	w.setIndeterminate(true)
	w.setStatus("Waiting for a completed ShareMods archive")
	captureUX(t, w, "progress-unknown")
}
