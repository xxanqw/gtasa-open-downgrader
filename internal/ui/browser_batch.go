package ui

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/xxanqw/gtasa-open-downgrader/internal/download"
	"github.com/xxanqw/gtasa-open-downgrader/internal/mods"
	appwidget "github.com/xxanqw/gtasa-open-downgrader/internal/uiwidget"
)

type browserQueueEntry struct {
	request                      mods.BrowserArchive
	page                         *url.URL
	filename, destination, cache string
	watcher                      *download.DownloadWatch
	imports                      chan browserQueueSource
	finished                     chan struct{}
	buttons                      []*appwidget.Button
	state                        *widget.Label // UI goroutine only
	ready                        bool          // UI goroutine only
}

type browserQueueSource struct {
	file      string
	watcher   *download.DownloadWatch
	directory string
}

func browserArchiveKey(raw, checksum string) string { return raw + "\n" + strings.ToLower(checksum) }

func browserQueueEntries(requests []mods.BrowserArchive, work, cache string) ([]*browserQueueEntry, error) {
	var entries []*browserQueueEntry
	seen := map[string]*browserQueueEntry{}
	for _, request := range requests {
		page, err := url.Parse(request.URL)
		if err != nil || page.Scheme != "https" || page.User != nil || (!strings.EqualFold(page.Host, "sharemods.com") && !strings.EqualFold(page.Host, "www.sharemods.com")) {
			return nil, fmt.Errorf("batch downloads require official HTTPS ShareMods pages")
		}
		if request.Checksum != "" {
			digest, err := hex.DecodeString(request.Checksum)
			if err != nil || len(digest) != 32 {
				return nil, fmt.Errorf("invalid reviewed archive checksum")
			}
		}
		key := browserArchiveKey(request.URL, request.Checksum)
		if entry := seen[key]; entry != nil {
			entry.request.Name += ", " + request.Name
			continue
		}
		filename := strings.TrimSuffix(path.Base(page.Path), ".html")
		ext := strings.ToLower(filepath.Ext(filename))
		if ext != ".zip" && ext != ".7z" {
			return nil, fmt.Errorf("browser download must be a ZIP or 7z archive")
		}
		entry := &browserQueueEntry{request: request, page: page, filename: filename, destination: filepath.Join(work, strconv.Itoa(len(entries))+filepath.Ext(filename)), imports: make(chan browserQueueSource, 1), finished: make(chan struct{})}
		if request.Checksum != "" {
			entry.cache = filepath.Join(cache, "mod-archives", strings.ToLower(request.Checksum)+strings.ToLower(filepath.Ext(filename)))
		}
		seen[key] = entry
		entries = append(entries, entry)
	}
	return entries, nil
}

// Each archive has its own watcher from before any page is opened. Workers are
// joined before the session directory is removed, including on cancellation.
func (w *Window) prepareBrowserDownloads(parent context.Context, requests []mods.BrowserArchive) (mods.BrowserDownloadFunc, func(), error) {
	work, err := os.MkdirTemp("", "gtasa-browser-batch-*")
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() { _ = os.RemoveAll(work) }
	entries, err := browserQueueEntries(requests, work, w.cache)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	ctx, cancel := context.WithTimeout(parent, time.Hour)
	var workers sync.WaitGroup
	defer func() { cancel(); workers.Wait() }()
	fail := func(err error) (mods.BrowserDownloadFunc, func(), error) {
		cancel()
		workers.Wait()
		cleanup()
		return nil, nil, err
	}
	directory := w.app.Preferences().String("mod-download-directory")

	// Cache lookup requires no browser interaction and retains checksum checks.
	var pending []*browserQueueEntry
	for _, entry := range entries {
		if entry.cache != "" && download.CopyBrowserArchive(ctx, entry.cache, entry.destination, entry.request.Checksum) == nil {
			continue
		}
		pending = append(pending, entry)
	}
	if len(pending) > 0 && directory == "" {
		directory, err = download.DownloadsDirectory()
		if err != nil {
			return fail(err)
		}
	}
	for len(pending) > 0 {
		for _, entry := range pending {
			entry.watcher, err = download.WatchDownloads(directory, entry.filename)
			if err != nil {
				break
			}
		}
		if err == nil {
			break
		}
		directory, err = w.downloadPrompt(ctx, promptRequest{kind: promptFolder, title: "Choose your browser's download folder", initial: directory})
		if err != nil {
			return fail(err)
		}
	}
	if len(pending) > 0 {
		initialized := make(chan struct{})
		uiDirectory := directory
		w.post(func() {
			for _, entry := range pending {
				entry.state = widget.NewLabel("Needs browser download")
				entry.state.Wrapping = fyne.TextWrapWord
			}
			close(initialized)
		})
		select {
		case <-initialized:
		case <-ctx.Done():
			return fail(ctx.Err())
		}
		// Buffered completion channel lets workers finish while a file chooser is open.
		type result struct {
			entry   *browserQueueEntry
			message string
			err     error
			done    bool
		}
		results := make(chan result, len(pending)*2)
		failures := make(chan error, 1)
		publish := func(r result) {
			select {
			case results <- r:
			case <-ctx.Done():
			}
		}
		for _, entry := range pending {
			entry := entry
			workers.Add(1)
			go func() {
				defer workers.Done()
				defer close(entry.finished)
				err := waitQueueArchive(ctx, entry, func(message string) { publish(result{entry: entry, message: message}) })
				if err == nil && entry.cache != "" {
					if cacheErr := os.MkdirAll(filepath.Dir(entry.cache), 0700); cacheErr == nil {
						if cacheErr = download.CopyBrowserArchive(ctx, entry.destination, entry.cache, entry.request.Checksum); cacheErr != nil {
							w.logs.Record("WARN", "Could not cache verified mod archive: "+cacheErr.Error())
						}
					}
				}
				publish(result{entry: entry, err: err, done: true})
			}()
		}
		// All UI state is owned by the UI goroutine, including the active modal.
		var active *appwidget.Modal
		completed := 0
		allDone := make(chan struct{})
		workers.Add(1)
		go func() {
			defer workers.Done()
			for completedCount := 0; completedCount < len(pending); {
				select {
				case <-ctx.Done():
					return
				case r := <-results:
					if r.done && r.err != nil {
						failures <- fmt.Errorf("%s: %w", r.entry.filename, r.err)
						cancel()
						return
					}
					if r.done {
						completedCount++
					}
					w.post(func() {
						if ctx.Err() != nil {
							return
						}
						if r.done {
							r.entry.ready = true
							for _, button := range r.entry.buttons {
								button.Disable()
							}
							completed++
							state := "Ready to install"
							if r.entry.request.Checksum != "" {
								state = "Verified · ready to install"
							}
							r.entry.state.SetText(state)
						} else {
							r.entry.state.SetText(r.message)
						}
						w.browserDetails = fmt.Sprintf("Browser downloads: %d of %d ready\nWatching folder: %s", completed, len(pending), uiDirectory)
						w.refreshProgressFooter()
						if active != nil {
							active.SetSubtitle(w.browserDetails)
						}
						if completed == len(pending) {
							close(allDone)
							if active != nil {
								active.Close("ready")
							}
						}
					})
				}
			}
		}()
		defer w.post(func() { w.browserDetails = ""; w.refreshProgressFooter() })
		for {
			response, err := w.downloadPrompt(ctx, promptRequest{title: "Download selected mod archives", present: func(done func(string, bool)) func() {
				select {
				case <-allDone:
					done("ready", true)
					return func() {}
				default:
				}
				body := w.browserQueueBody(pending, func(index int) {
					if active != nil {
						active.Close("import:" + strconv.Itoa(index))
					}
				})
				active = appwidget.ShowModal(w.window, appwidget.ModalOptions{Title: "Download selected mod archives", Subtitle: fmt.Sprintf("%d of %d ready · Watching %s", completed, len(pending), uiDirectory), Body: body, Actions: []appwidget.ModalAction{
					{Text: "Cancel"},
					{ID: "folder", Text: "Change download folder…"},
					{Text: "Open remaining pages", KeepOpen: true, Importance: widget.HighImportance, Run: func(m *appwidget.Modal) {
						for _, entry := range pending {
							if !entry.ready {
								if err := w.app.OpenURL(entry.page); err != nil {
									entry.state.SetText(err.Error())
								} else {
									entry.state.SetText("Waiting for browser download")
								}
							}
						}
					}},
				}, OnClosed: func(result string) { active = nil; done(result, result != "") }})
				return func() {
					if active != nil {
						active.Close("")
					}
				}
			}})
			if err != nil {
				cancel()
				workers.Wait()
				select {
				case failure := <-failures:
					err = failure
				default:
				}
				return fail(err)
			}
			if response == "ready" {
				break
			}
			if response == "folder" {
				folder, chooseErr := w.downloadPrompt(ctx, promptRequest{kind: promptFolder, title: "Choose download folder (start new downloads after changing)", initial: directory})
				if chooseErr != nil {
					if ctx.Err() != nil {
						cancel()
						workers.Wait()
						return fail(ctx.Err())
					}
					continue
				}
				// Snapshot every new folder before reopening the queue. Downloads
				// started immediately afterwards cannot become baseline files.
				watchers := make([]*download.DownloadWatch, len(pending))
				var watchErr error
				for index, entry := range pending {
					watchers[index], watchErr = download.WatchDownloads(folder, entry.filename)
					if watchErr != nil {
						break
					}
				}
				if watchErr != nil {
					w.post(func() { w.info("Could not watch folder", watchErr.Error()) })
					continue
				}
				for index, entry := range pending {
					select {
					case entry.imports <- browserQueueSource{watcher: watchers[index], directory: folder}:
					case <-entry.finished:
					case <-ctx.Done():
					}
				}
				directory = folder
				w.post(func() { uiDirectory = folder; w.app.Preferences().SetString("mod-download-directory", folder) })
			} else if strings.HasPrefix(response, "import:") {
				index, _ := strconv.Atoi(strings.TrimPrefix(response, "import:"))
				entry := pending[index]
				file, chooseErr := w.downloadPrompt(ctx, promptRequest{kind: promptFile, title: "Select " + entry.filename, initial: directory, extensions: []string{".zip", ".7z"}})
				if chooseErr == nil {
					select {
					case entry.imports <- browserQueueSource{file: file}:
					case <-entry.finished:
					case <-ctx.Done():
					}
				}
			}
		}
		w.post(func() { w.app.Preferences().SetString("mod-download-directory", directory) })
	}
	// Include checksum-less archives in this session only; they are not persisted.
	downloader := func(ctx context.Context, raw, destination, checksum string, progress download.Progress) error {
		for _, entry := range entries {
			if browserArchiveKey(raw, checksum) == browserArchiveKey(entry.request.URL, entry.request.Checksum) {
				if err := download.CopyBrowserArchive(ctx, entry.destination, destination, checksum); err != nil {
					return err
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
		}
		return fmt.Errorf("archive was not included in the prepared download queue")
	}
	return downloader, cleanup, nil
}

func waitQueueArchive(ctx context.Context, entry *browserQueueEntry, notify func(string)) error {
	for {
		waitCtx, cancel := context.WithCancel(ctx)
		finished := make(chan error, 1)
		go func() { finished <- entry.watcher.Wait(waitCtx, entry.destination, entry.request.Checksum, notify) }()
		select {
		case err := <-finished:
			cancel()
			return err
		case <-ctx.Done():
			cancel()
			<-finished
			return ctx.Err()
		case source := <-entry.imports:
			cancel()
			err := <-finished
			if err == nil {
				return nil
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if source.watcher != nil {
				entry.watcher = source.watcher
				notify("Needs browser download · watching " + source.directory)
			} else {
				notify("Checking selected archive…")
				if err := download.CopyBrowserArchive(ctx, source.file, entry.destination, entry.request.Checksum); err == nil {
					return nil
				} else {
					notify("Archive rejected: " + err.Error() + ". Choose another file or download again.")
				}
			}
		}
	}
}

func (w *Window) browserQueueBody(pending []*browserQueueEntry, importArchive func(int)) *container.Scroll {
	body := container.NewVBox(promptText("Open the pages below and finish each ShareMods download. You can download in any order; installation starts automatically when every archive is ready. Shared archives are listed once. Cancel stops waiting, not browser downloads."))
	for index, entry := range pending {
		index, entry := index, entry
		open := appwidget.NewButton("Open page", func() {
			if entry.ready {
				return
			}
			if err := w.app.OpenURL(entry.page); err != nil {
				entry.state.SetText(err.Error())
			} else {
				entry.state.SetText("Waiting for browser download")
			}
		})
		if entry.ready {
			open.Disable()
		}
		use := appwidget.NewButton("Use existing archive…", func() {
			if !entry.ready {
				importArchive(index)
			}
		})
		if entry.ready {
			use.Disable()
		}
		entry.buttons = []*appwidget.Button{open, use}
		body.Add(container.NewVBox(promptText(entry.filename+"\nFor: "+entry.request.Name), entry.state, container.NewHBox(open, use), widget.NewSeparator()))
	}
	return container.NewVScroll(body)
}
