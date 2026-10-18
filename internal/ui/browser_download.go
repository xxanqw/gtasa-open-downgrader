package ui

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2/widget"
	"github.com/xxanqw/gtasa-open-downgrader/internal/download"
	appwidget "github.com/xxanqw/gtasa-open-downgrader/internal/uiwidget"
)

// Called by an installation worker; all prompts and browser actions stay on the
// UI thread. Cancellation also dismisses a queued/active download prompt.
func (w *Window) downloadPrompt(ctx context.Context, request promptRequest) (string, error) {
	type answer struct {
		value string
		ok    bool
	}
	replies := make(chan answer, 1)
	request.ctx = ctx
	request.done = func(value string, ok bool) { replies <- answer{value, ok} }
	w.post(func() {
		if ctx.Err() == nil {
			w.prompt(request)
		}
	})
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case result := <-replies:
		if !result.ok {
			return "", context.Canceled
		}
		return result.value, nil
	}
}

func (w *Window) browserModDownload(ctx context.Context, raw, destination, checksum string, progress download.Progress) error {
	page, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if page.Scheme != "https" || (!strings.EqualFold(page.Host, "sharemods.com") && !strings.EqualFold(page.Host, "www.sharemods.com")) || page.User != nil {
		return fmt.Errorf("browser-assisted downloads require an official HTTPS ShareMods page")
	}
	filename := strings.TrimSuffix(path.Base(page.Path), ".html")
	cache := ""
	if checksum != "" {
		digest, err := hex.DecodeString(checksum)
		if err != nil || len(digest) != 32 {
			return fmt.Errorf("invalid reviewed archive checksum")
		}
		cache = filepath.Join(w.cache, "mod-archives", checksum+strings.ToLower(filepath.Ext(filename)))
		if err := download.CopyBrowserArchive(ctx, cache, destination, checksum); err == nil {
			if progress != nil {
				info, err := os.Stat(destination)
				if err != nil {
					return err
				}
				progress(info.Size(), info.Size())
			}
			return nil
		} else if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	directory := w.app.Preferences().String("mod-download-directory")
	if directory == "" {
		directory, err = download.DownloadsDirectory()
		if err != nil {
			return err
		}
	}
	for {
		request := promptRequest{title: "Download mod in your browser"}
		request.present = func(done func(string, bool)) func() {
			text := "ShareMods requires a browser confirmation. Open the official page, complete its check, then click Download.\n\nExpected file: " + filename + "\nWatching folder: " + directory + "\n\nOnly a new completed matching ZIP/7z will be copied and validated. Existing/unrelated files are ignored, and your download is kept. Use the folder button if your browser saves elsewhere. Cancel stops waiting; it does not cancel the browser download."
			modal := appwidget.ShowModal(w.window, appwidget.ModalOptions{
				Title: request.title, Body: promptText(text), Actions: []appwidget.ModalAction{
					{Text: "Cancel"}, {ID: "folder", Text: "Choose download folder…"}, {ID: "open", Text: "Open download page", Importance: widget.HighImportance},
				}, OnClosed: func(result string) { done(result, result != "") },
			})
			return func() { modal.Close("") }
		}
		result, err := w.downloadPrompt(ctx, request)
		if err != nil {
			return err
		}
		if result == "folder" {
			directory, err = w.downloadPrompt(ctx, promptRequest{kind: promptFolder, title: "Select your browser's download folder", initial: directory})
			if err != nil {
				return err
			}
			continue
		}
		break
	}
	watcher, err := download.WatchDownloads(directory, filename)
	if err != nil {
		return fmt.Errorf("select an existing browser download folder: %w", err)
	}
	opened := make(chan error, 1)
	w.post(func() {
		if ctx.Err() != nil {
			opened <- ctx.Err()
			return
		}
		w.app.Preferences().SetString("mod-download-directory", directory)
		opened <- w.app.OpenURL(page)
	})
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-opened:
		if err != nil {
			return err
		}
	}
	w.report("Waiting for "+filename+" in "+directory+" — complete the browser download, or Cancel", 0, 0)
	if err := watcher.Wait(ctx, destination, checksum, func(message string) { w.report(message, 0, 0) }); err != nil {
		return err
	}
	if cache != "" {
		if err := os.MkdirAll(filepath.Dir(cache), 0700); err == nil {
			if err := download.CopyBrowserArchive(ctx, destination, cache, checksum); err != nil {
				w.logs.Record("WARN", "Could not cache verified mod archive: "+err.Error())
			}
		}
	}
	if progress != nil {
		info, err := os.Stat(destination)
		if err != nil {
			return err
		}
		progress(info.Size(), info.Size())
	}
	return nil
}
