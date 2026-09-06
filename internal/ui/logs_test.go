package ui

import (
	"errors"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

func TestApplicationLogIncludesErrorsAndExportsThroughPicker(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := New(a)
	w.window = clipboardWindow{Window: w.window, clipboard: test.NewClipboard()}
	defer w.jobs.Close()
	w.window.Show()
	w.setStatus("Scanning game files")
	w.error(errors.New("xdelta3: fixture diagnostic"))
	w.activePrompt()
	text, _ := w.logs.Snapshot()
	if !strings.Contains(text, "[INFO] Scanning game files") || !strings.Contains(text, "[ERROR] xdelta3: fixture diagnostic") {
		t.Fatal("global journal missing status or error", text)
	}
	w.showLogs()
	snapshot(t, w, "GTASA_LOGS_SNAPSHOT")
	tapPrompt(t, w, "Export…")
	if !w.promptActive || w.window.Canvas().Overlays().Top() == nil {
		t.Fatal("export picker did not open")
	}
	w.activePrompt()
	if w.promptActive {
		t.Fatal("export cancellation left modal blocked")
	}
	w.showLogs()
	tapPrompt(t, w, "Copy all")
	if w.window.Clipboard().Content() != text {
		t.Fatal("copy did not include full journal")
	}
	w.activePrompt()
}

func TestLogPreviewKeepsRecentEntries(t *testing.T) {
	text := strings.Repeat("earlier entry\n", 5000) + "latest error with details\n"
	preview := logPreview(text)
	if len(preview) > 33<<10 || !strings.HasSuffix(preview, "latest error with details\n") || !strings.Contains(preview, "Export") {
		t.Fatal("preview lost newest entries or retention notice")
	}
}

type clipboardWindow struct {
	fyne.Window
	clipboard fyne.Clipboard
}

func (w clipboardWindow) Clipboard() fyne.Clipboard { return w.clipboard }
