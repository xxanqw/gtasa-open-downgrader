package ui

import (
	"context"
	"fmt"
	"net/url"

	"fyne.io/fyne/v2/widget"
	"github.com/xxanqw/gtasa-open-downgrader/internal/download"
	appwidget "github.com/xxanqw/gtasa-open-downgrader/internal/uiwidget"
)

func (w *Window) browserDownloadOptions(ctx context.Context, page *url.URL, filename string, folders chan<- string) {
	if ctx.Err() != nil || w.promptActive {
		return
	}
	w.prompt(promptRequest{ctx: ctx, title: "Browser download options", present: func(done func(string, bool)) func() {
		modal := appwidget.ShowModal(w.window, appwidget.ModalOptions{
			Title: "Browser download options",
			Body:  promptText(w.browserDetails + "\n\nIf you change folders, start a new download there. Existing files are ignored; use Tools → Import mod archive for an existing archive."),
			Actions: []appwidget.ModalAction{
				{Text: "Keep waiting"},
				{Text: "Open page again", KeepOpen: true, Run: func(m *appwidget.Modal) {
					if ctx.Err() == nil {
						if err := w.app.OpenURL(page); err != nil {
							m.SetSubtitle(err.Error())
						}
					}
				}},
				{ID: "folder", Text: "Change watched folder…", Importance: widget.HighImportance},
			},
			OnClosed: func(result string) { done(result, result != "") },
		})
		return func() { modal.Close("") }
	}, done: func(result string, ok bool) {
		if !ok || result != "folder" || ctx.Err() != nil {
			return
		}
		w.prompt(promptRequest{ctx: ctx, kind: promptFolder, title: "Watch for " + filename, initial: w.app.Preferences().String("mod-download-directory"), done: func(folder string, ok bool) {
			if ok && ctx.Err() == nil {
				select {
				case folders <- folder:
				default:
				}
			}
		}})
	}})
}

// Join each watcher before changing folders: two watchers must never commit to
// the same destination concurrently. Existing-file and checksum rules stay in
// download.DownloadWatch; changing folders does not relax verification.
func waitForBrowserArchive(ctx context.Context, watcher *download.DownloadWatch, directory, filename, destination, checksum string, folders <-chan string, notify func(string, string)) error {
	announce := true
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if announce {
			notify(directory, "Waiting for "+filename+" — finish the browser download or open Download options.")
		}
		waitCtx, cancel := context.WithCancel(ctx)
		finished := make(chan error, 1)
		go func() {
			finished <- watcher.Wait(waitCtx, destination, checksum, func(message string) { notify(directory, message) })
		}()
		select {
		case err := <-finished:
			cancel()
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		case <-ctx.Done():
			cancel()
			<-finished
			return ctx.Err()
		case folder := <-folders:
			cancel()
			err := <-finished
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if err == nil {
				return nil
			}
			next, err := download.WatchDownloads(folder, filename)
			if err != nil {
				notify(directory, fmt.Sprintf("Could not watch %s: %v. Still watching %s.", folder, err, directory))
				announce = false
				continue
			}
			watcher, directory = next, folder
			announce = true
		}
	}
}
