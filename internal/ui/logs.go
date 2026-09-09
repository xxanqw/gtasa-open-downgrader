package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/xxanqw/gtasa-open-downgrader/internal/applog"
	appwidget "github.com/xxanqw/gtasa-open-downgrader/internal/uiwidget"
)

func (w *Window) showLogs() {
	w.prompt(promptRequest{present: func(done func(string, bool)) func() {
		text, revision := w.logs.Snapshot()
		entries := promptText(logPreview(text))
		entries.TextStyle.Monospace = true
		scroll := container.NewVScroll(entries)
		follow := widget.NewCheck("Follow new entries", nil)
		follow.SetChecked(true)
		note := compactLabel("Session log · timestamps and full error details · export includes all retained entries")
		body := container.NewBorder(note, follow, nil, nil, scroll)
		var closePanel func()
		exporting := false
		export := appwidget.NewButtonWithIcon("Export…", theme.DocumentSaveIcon(), func() { exporting = true; closePanel() })
		copy := appwidget.NewButtonWithIcon("Copy all", theme.ContentCopyIcon(), func() {
			snapshot, _ := w.logs.Snapshot()
			w.window.Clipboard().SetContent(snapshot)
		})
		stopped := make(chan struct{})
		closed := false // accessed only on Fyne's event goroutine
		closePanel = w.panel("Application log", body, fyne.NewSize(740, 440), "", "Close", container.NewHBox(copy, export), func(bool) {
			closed = true
			close(stopped)
			if exporting {
				done("export", true)
			} else {
				done("", false)
			}
		})
		scroll.ScrollToBottom()
		// Batch log updates, keeping worker writes independent of rendering speed.
		w.jobs.Go(func(ctx context.Context) {
			ticker := time.NewTicker(500 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-stopped:
					return
				case <-ticker.C:
					text, next := w.logs.Snapshot()
					if next == revision {
						continue
					}
					revision = next
					w.post(func() {
						if closed {
							return
						}
						entries.SetText(logPreview(text))
						if follow.Checked {
							scroll.ScrollToBottom()
						}
					})
				}
			}
		})
		return closePanel
	}, done: func(action string, ok bool) {
		if ok && action == "export" {
			w.exportLogs()
		}
	}})
}

// Bound text layout work; exports and Copy all retain the complete journal.
func logPreview(text string) string {
	const limit = 32 << 10
	if len(text) > limit {
		text = text[len(text)-limit:]
		if i := strings.IndexByte(text, '\n'); i >= 0 {
			text = text[i+1:]
		}
		return "Earlier entries omitted from this view; use Export or Copy all.\n\n" + text
	}
	if text == "" {
		return "No log entries yet."
	}
	return text
}

func (w *Window) exportLogs() {
	text, _ := w.logs.Snapshot()
	directory := w.cache
	if home, err := os.UserHomeDir(); err == nil {
		directory = home
	}
	initial := filepath.Join(directory, "gtasa-downgrader-"+time.Now().Format("20060102-150405")+".log")
	w.chooseDestination("Export application log", initial, []string{".log"}, func(path string) {
		file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
		if err == nil {
			err = applog.Export(file, text)
		}
		if err != nil {
			w.error(fmt.Errorf("export application log: %w", err))
			return
		}
		w.setStatus("Application log exported.")
		w.logs.Record("INFO", "Log exported to "+path)
	})
}
