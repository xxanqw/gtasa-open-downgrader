package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
		entries := newLogEntry(text)
		find := widget.NewEntry()
		find.SetPlaceHolder("Find in log…")
		follow := newModCheck("Follow new entries", nil)
		follow.SetChecked(true)
		note := compactLabel("Local time · original log preserved in exports")
		levels := newControlSelect([]string{"All events", "Activity", "Warnings & errors"}, func(level string) {
			entries.SetLevel(level)
			if follow.Checked {
				entries.ScrollToBottom()
			}
		})
		levels.SetSelected("All events")
		matchStatus := compactLabel("")
		next := func(reverse bool) {
			if find.Text != "" {
				follow.SetChecked(false)
				if findLog(entries, find.Text, reverse) {
					matchStatus.SetText("")
				} else {
					matchStatus.SetText("No match")
				}
			}
		}
		follow.OnChanged = func(on bool) {
			if on {
				entries.ScrollToBottom()
			}
		}
		find.OnSubmitted = func(string) { next(false) }
		previous := appwidget.NewButton("Previous", func() { next(true) })
		nextButton := appwidget.NewButton("Next", func() { next(false) })
		search := container.NewBorder(nil, nil, nil, container.NewHBox(matchStatus, previous, nextButton), styleInput(find))
		body := container.NewBorder(container.NewVBox(container.NewBorder(nil, nil, nil, levels, note), search), follow, nil, nil, entries.view)
		var closePanel func()
		action := ""
		export := appwidget.NewButtonWithIcon("Export…", theme.DocumentSaveIcon(), func() { action = "export"; closePanel() })
		copy := appwidget.NewButtonWithIcon("Copy all", theme.ContentCopyIcon(), func() {
			snapshot, _ := w.logs.Snapshot()
			w.window.Clipboard().SetContent(snapshot)
		})
		stopped := make(chan struct{})
		closed := false // accessed only on Fyne's event goroutine
		issue := appwidget.NewButton("I'm creating an issue on GitHub", func() { action = "issue"; closePanel() })
		closePanel = w.panel("Application log", body, fyne.NewSize(880, 560), "", "Close", container.NewHBox(issue, copy, export), func(bool) {
			closed = true
			close(stopped)
			if action != "" {
				done(action, true)
			} else {
				done("", false)
			}
		})
		entries.ScrollToBottom()
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
					text, next := w.logs.ChangesSince(revision)
					if next == revision {
						continue
					}
					revision = next
					w.post(func() {
						if closed {
							return
						}
						entries.UpdateText(text)
						if follow.Checked {
							entries.ScrollToBottom()
						}
					})
				}
			}
		})
		return closePanel
	}, done: func(action string, ok bool) {
		if !ok {
			return
		}
		switch action {
		case "export":
			w.exportLogs()
		case "issue":
			text, _ := w.logs.Snapshot()
			w.proposeIssueLogs(text)
		}
	}})
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
