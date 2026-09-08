// Package ui provides the native Fyne frontend. Widgets are only read or changed
// on Fyne's event goroutine; workers receive immutable operation snapshots.
package ui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/xxanqw/gtasa-open-downgrader/internal/applog"
	"github.com/xxanqw/gtasa-open-downgrader/internal/assets"
	"github.com/xxanqw/gtasa-open-downgrader/internal/download"
	"github.com/xxanqw/gtasa-open-downgrader/internal/game"
	"github.com/xxanqw/gtasa-open-downgrader/internal/jobs"
	"github.com/xxanqw/gtasa-open-downgrader/internal/mods"
	"github.com/xxanqw/gtasa-open-downgrader/internal/patch"
	"github.com/xxanqw/gtasa-open-downgrader/internal/platform"
	"github.com/xxanqw/gtasa-open-downgrader/internal/updater"
)

type Window struct {
	logs                                  *applog.Journal
	activePrompt                          func()
	prompts                               []promptRequest
	promptActive                          bool
	app                                   fyne.App
	window                                fyne.Window
	jobs                                  *jobs.Group
	client                                *download.Client
	cache, patchDir                       string
	path                                  *widget.Entry
	status, version, modStatus            *widget.Label
	progress                              *thinProgress
	table                                 *readOnlyTable
	rows                                  []game.Row
	checks                                map[string]*widget.Check
	controls                              []fyne.Disableable
	cancelButton                          *widget.Button
	downgrade, install, reinstall         *widget.Button
	cancel                                context.CancelFunc
	busy, closing, readonly, dependencies bool
	generation                            int
	warned                                map[string]bool
	scanPath                              string
	transferLabel                         string
	transferStart                         time.Time
	transferBase                          int64
	updateButton                          *widget.Button
	modLabels                             map[string]*widget.Label
	modPanel                              *fyne.Container
	modsTitle                             *widget.Label
	multiplayerButton                     *widget.Button
}

func Run(gamePath, patchPath string) {
	journal := &applog.Journal{}
	previous := log.Writer()
	log.SetOutput(io.MultiWriter(journal, previous))
	defer log.SetOutput(previous)
	journal.Record("INFO", fmt.Sprintf("GTA SA Open Downgrader %s · %s/%s · %s · offline=%t", updater.Version, runtime.GOOS, runtime.GOARCH, runtime.Version(), assets.Offline))
	// Preserve otherwise fatal runtime panics, including those on other goroutines.
	if cache, err := os.UserCacheDir(); err == nil {
		path := filepath.Join(cache, "gtasa-open-downgrader")
		if os.MkdirAll(path, 0700) == nil {
			if file, err := os.OpenFile(filepath.Join(path, "last-crash.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600); err == nil {
				if err := debug.SetCrashOutput(file, debug.CrashOptions{}); err != nil {
					journal.Record("WARN", err.Error())
				}
				file.Close()
				defer debug.SetCrashOutput(nil, debug.CrashOptions{})
			}
		}
	}
	a := app.NewWithID("pp.ua.xxanqw.gtasa-open-downgrader")
	w := newWindow(a, journal)
	if gamePath != "" {
		w.path.SetText(gamePath)
	}
	w.patchDir = patchPath
	signals, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	stopped := make(chan struct{})
	defer close(stopped)
	go func() {
		select {
		case <-signals.Done():
			fyne.Do(w.close)
		case <-stopped:
		}
	}()
	w.window.Show()
	w.start()
	a.Run()
}
func New(a fyne.App) *Window { return newWindow(a, &applog.Journal{}) }
func newWindow(a fyne.App, journal *applog.Journal) *Window {
	a.Settings().SetTheme(compactTheme{theme.DefaultTheme()})
	cache, e := os.UserCacheDir()
	if e != nil {
		cache = os.TempDir()
	}
	cache = filepath.Join(cache, "gtasa-open-downgrader")
	w := &Window{logs: journal, app: a, window: a.NewWindow("GTA SA Open Downgrader · " + updater.Version), jobs: jobs.New(), client: download.New(), cache: cache, checks: map[string]*widget.Check{}, modLabels: map[string]*widget.Label{}, warned: map[string]bool{}}
	w.window.SetIcon(fyne.NewStaticResource("icon.png", assets.Icon))
	w.window.Resize(fyne.NewSize(840, 560))
	w.window.SetCloseIntercept(w.close)
	w.path = widget.NewEntry()
	w.path.SetPlaceHolder("Select your GTA San Andreas directory")
	w.path.SetText(a.Preferences().String("game-path"))
	w.path.OnChanged = func(string) {
		w.scanPath = ""
		w.generation++
		w.rows = nil
		if w.table != nil {
			w.table.Refresh()
			w.version.SetText("Scan this folder to detect the game version.")
		}
		w.refreshEnabled()
	}
	w.path.OnSubmitted = func(string) { w.scan() }
	browse := widget.NewButtonWithIcon("", theme.FolderOpenIcon(), func() {
		w.chooseFolder("Select GTA San Andreas folder", w.path.Text, func(path string) {
			w.path.SetText(path)
			w.scan()
		})
	})
	scan := widget.NewButtonWithIcon("Scan", theme.SearchIcon(), w.scan)
	w.version = compactLabel("Select a game folder to get started.")
	w.version.TextStyle = fyne.TextStyle{Bold: true}
	w.status = compactLabel("Ready")
	w.progress = newThinProgress()
	w.modStatus = widget.NewLabel("Select mods to include with the downgrade or install separately.")
	w.modStatus.Wrapping = fyne.TextWrapWord
	w.table = newReadOnlyTable(func() (int, int) { return len(w.rows) + 1, 5 }, func() fyne.CanvasObject {
		return container.NewThemeOverride(compactLabel(""), fileTableTheme{a.Settings().Theme()})
	}, func(id widget.TableCellID, o fyne.CanvasObject) {
		label := o.(*container.ThemeOverride).Content.(*widget.Label)
		value := ""
		label.TextStyle = fyne.TextStyle{Bold: id.Row == 0}
		label.Importance = widget.MediumImportance
		if id.Row == 0 {
			value = fileColumns[id.Col]
		} else if id.Row <= len(w.rows) {
			r := w.rows[id.Row-1]
			value = fileValues(r)[id.Col]
			if id.Col == 2 {
				switch {
				case strings.HasPrefix(r.Status, "Failed") || strings.HasPrefix(r.Status, "Error"):
					label.Importance = widget.DangerImportance
					label.TextStyle.Bold = true
				case r.Status == "Missing" || r.Status == "Modified":
					label.Importance = widget.WarningImportance
					label.TextStyle.Bold = true
				case strings.HasPrefix(r.Status, "Already") || r.Status == "Success":
					label.Importance = widget.SuccessImportance
				}
			}
		}
		label.SetText(value)
	})
	w.table.StickyRowCount = 1
	var checkObjects []fyne.CanvasObject
	for _, source := range mods.Catalog {
		name := source.Name
		c := widget.NewCheck(strings.TrimSuffix(name, " (60fps fix)"), func(bool) { w.selectDependencies() })
		w.checks[name] = c
		state := compactLabel("Not installed")
		w.modLabels[name] = state
		checkObjects = append(checkObjects, container.NewThemeOverride(container.NewVBox(c, state), modGridTheme{a.Settings().Theme()}))
	}
	w.downgrade = widget.NewButtonWithIcon("Downgrade game", theme.MediaPlayIcon(), w.apply)
	w.downgrade.Importance = widget.HighImportance
	w.install = widget.NewButtonWithIcon("Install mods only", theme.DownloadIcon(), func() { w.installMods(false) })
	w.reinstall = widget.NewButtonWithIcon("Reinstall / update mods", theme.ViewRefreshIcon(), func() { w.installMods(true) })
	tools := widget.NewButtonWithIcon("Tools", theme.SettingsIcon(), w.tools)
	w.multiplayerButton = widget.NewButton("Multiplayer", w.multiplayer)
	var more *widget.Button
	more = widget.NewButtonWithIcon("", theme.MoreHorizontalIcon(), func() {
		menu := fyne.NewMenu("", fyne.NewMenuItem("Check app updates", func() { w.checkUpdates(true) }), fyne.NewMenuItem("About", w.showAbout))
		pos := a.Driver().AbsolutePositionForObject(more).Add(fyne.NewPos(0, more.Size().Height))
		widget.ShowPopUpMenuAtPosition(menu, w.window.Canvas(), pos)
	})
	w.updateButton = more
	w.cancelButton = widget.NewButton("Cancel", func() {
		if w.cancel != nil {
			w.cancel()
			w.setStatus("Cancelling; waiting for the current operation to stop…")
			w.cancelButton.Disable()
		}
	})
	w.cancelButton.Disable()
	w.controls = []fyne.Disableable{w.path, browse, scan, w.multiplayerButton, tools, more}
	w.modsTitle = widget.NewLabel("Mods")
	w.modsTitle.TextStyle.Bold = true
	table := container.NewThemeOverride(container.New(&fileTableLayout{table: w.table}, w.table), fileTableTheme{a.Settings().Theme()})
	fileActions := container.NewBorder(nil, nil, nil, w.downgrade, nil)
	buttons := container.NewThemeOverride(container.NewHBox(w.install, w.reinstall), modActionTheme{a.Settings().Theme()})
	modActions := container.NewBorder(nil, nil, nil, buttons, nil)
	modMessage := container.NewThemeOverride(w.modStatus, modGridTheme{a.Settings().Theme()})
	w.modPanel = newModPanel(container.NewVBox(container.NewGridWithColumns(5, checkObjects...), modMessage, modActions))
	sectionHeader := w.modsTitle
	files := container.NewBorder(container.NewVBox(sectionHeader, w.modPanel), fileActions, nil, nil, table)

	top := container.NewVBox(container.NewBorder(nil, nil, nil, container.NewHBox(browse, scan, w.multiplayerButton, tools, more), w.path), w.version)
	statusDetails := widget.NewButtonWithIcon("", theme.InfoIcon(), w.showLogs)
	bottom := container.New(statusLayout{}, w.status, w.progress, container.NewHBox(w.cancelButton, statusDetails))
	w.window.SetContent(container.NewBorder(top, bottom, nil, nil, files))
	w.refreshEnabled()
	return w
}
func mustURL(raw string) *url.URL { u, _ := url.Parse(raw); return u }
func (w *Window) error(e error) {
	if e != nil {
		w.logs.Record("ERROR", e.Error())
	}
	if e != nil && !w.closing {
		w.prompt(promptRequest{kind: promptError, title: "Error", text: e.Error()})
	}
}
func (w *Window) post(f func()) {
	fyne.Do(func() {
		if !w.closing {
			f()
		}
	})
}
func (w *Window) close() {
	if w.closing {
		return
	}
	w.closing = true
	w.prompts = nil
	if w.activePrompt != nil {
		w.activePrompt()
	}
	if w.cancel != nil {
		w.cancel()
	}
	w.setStatus("Stopping background work…")
	go func() { w.jobs.Close(); fyne.Do(func() { w.window.SetCloseIntercept(nil); w.app.Quit() }) }()
}
func (w *Window) refreshEnabled() {
	if w.progress != nil && w.cancelButton != nil {
		if w.busy {
			w.progress.Show()
			w.cancelButton.Show()
		} else {
			w.progress.Hide()
			w.cancelButton.Hide()
		}
	}
	for _, c := range w.controls {
		if w.busy || w.promptActive {
			c.Disable()
		} else {
			c.Enable()
		}
	}
	ready := w.scanPath != "" && w.scanPath == w.path.Text && !w.readonly && !w.busy && !w.promptActive
	for _, b := range []*widget.Button{w.downgrade, w.install, w.reinstall} {
		if ready {
			b.Enable()
		} else {
			b.Disable()
		}
	}
	if w.patchDir == "" {
		w.downgrade.Disable()
	}
	for _, c := range w.checks {
		if ready {
			c.Enable()
		} else {
			c.Disable()
		}
	}
	if ready {
		w.selectDependencies()
	}
}
func (w *Window) selectDependencies() {
	if w.dependencies {
		return
	}
	w.dependencies = true
	defer func() { w.dependencies = false; w.refreshModSelection() }()
	dependent := false
	for name, c := range w.checks {
		if name != "ASI Loader" && name != "ModLoader" && c.Checked {
			dependent = true
		}
	}
	if dependent {
		w.checks["ModLoader"].SetChecked(true)
		w.checks["ModLoader"].Disable()
	} else if !w.busy && !w.promptActive && !w.readonly && w.scanPath != "" {
		w.checks["ModLoader"].Enable()
	}
	if dependent || w.checks["ModLoader"].Checked {
		w.checks["ASI Loader"].SetChecked(true)
		w.checks["ASI Loader"].Disable()
	} else if !w.busy && !w.promptActive && !w.readonly && w.scanPath != "" {
		w.checks["ASI Loader"].Enable()
	}
}
func (w *Window) selected() []string {
	var result []string
	for _, s := range mods.Catalog {
		if w.checks[s.Name].Checked {
			result = append(result, s.Name)
		}
	}
	return result
}
func (w *Window) run(title string, work func(context.Context) error, done func(error)) {
	if w.busy || w.closing {
		return
	}
	w.busy = true
	w.generation++
	w.transferLabel = ""
	w.setStatus(title)
	w.progress.SetValue(0)
	w.refreshEnabled()
	w.cancelButton.Enable()
	ctx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel
	w.jobs.Go(func(parent context.Context) {
		stop := context.AfterFunc(parent, cancel)
		defer stop()
		err := w.logs.Guard(ctx, work)
		if errors.Is(err, context.Canceled) {
			w.logs.Record("INFO", title+": cancelled")
		} else if err != nil {
			w.logs.Record("ERROR", title+": "+err.Error())
		}
		cancel()
		w.post(func() {
			w.busy = false
			w.cancel = nil
			w.cancelButton.Disable()
			w.refreshEnabled()
			if errors.Is(err, context.Canceled) {
				w.setStatus("Cancelled. Completed work and backups have been preserved.")
			} else if err != nil {
				w.setStatus(title + " failed")
				w.error(err)
			} else {
				w.progress.SetValue(1)
				w.setStatus(title + " complete")
			}
			if done != nil {
				done(err)
			}
		})
	})
}
func (w *Window) report(label string, done, total int64) {
	w.post(func() {
		if label != w.transferLabel {
			w.logs.Record("INFO", label)
			w.transferLabel = label
			w.transferStart = time.Now()
			w.transferBase = done
		}
		seconds := time.Since(w.transferStart).Seconds()
		if seconds > 0.5 && done > w.transferBase {
			rate := float64(done-w.transferBase) / seconds
			label += fmt.Sprintf(" · %.1f MiB/s", rate/(1<<20))
			if total > done {
				label += fmt.Sprintf(" · %ds remaining", int(float64(total-done)/rate))
			}
		}
		if total > 0 {
			w.progress.SetValue(float64(done) / float64(total))
		}
		if total > 0 {
			label += fmt.Sprintf(" · %.1f / %.1f MiB", float64(done)/(1<<20), float64(total)/(1<<20))
		} else if done > 0 {
			label += fmt.Sprintf(" · %.1f MiB downloaded", float64(done)/(1<<20))
		}
		w.status.SetText(label)
	})
}
func (w *Window) start() {
	dir := w.patchDir
	detect := w.path.Text == ""
	var detected string
	var prepareError error
	w.run("Preparing patch assets", func(ctx context.Context) error {
		if detect {
			detected = platform.Detect()
		}
		if dir != "" {
			_, prepareError = patch.ValidateAssets(dir)
		} else {
			dir, prepareError = assets.PatchDirectory(ctx, w.cache)
		}
		return ctx.Err()
	}, func(e error) {
		w.patchDir = dir
		if prepareError != nil {
			w.logs.Record("WARN", "Patch assets: "+prepareError.Error())
			w.patchDir = ""
		}
		if w.path.Text == "" {
			w.path.SetText(detected)
		}
		if prepareError != nil && !errors.Is(e, context.Canceled) {
			w.confirm("Patches missing", "Download the official Patches.zip from iCloud? You can also import a local archive from Tools.", func(ok bool) {
				if ok {
					w.downloadPatches()
				} else if w.path.Text != "" {
					w.scan()
				}
			})
		} else if w.path.Text != "" {
			w.scan()
		}
		w.checkUpdates(false)
	})
}
func (w *Window) scan() {
	root := strings.TrimSpace(w.path.Text)
	if root == "" {
		w.error(fmt.Errorf("select a game directory first"))
		return
	}
	root, e := filepath.Abs(root)
	if e != nil {
		w.error(e)
		return
	}
	w.path.SetText(root)
	dir := w.patchDir
	var result game.Scan
	var tracker mods.Tracker
	w.run("Scanning game files", func(ctx context.Context) error {
		m := game.Manifest{}
		var e error
		if dir != "" {
			m, e = game.Load(filepath.Join(dir, "manifest.json"))
			if e != nil {
				return e
			}
		}
		result, e = game.Inspect(ctx, root, m, func(i, n int) { w.post(func() { w.progress.SetValue(float64(i) / float64(n)) }) })
		if e != nil {
			return e
		}
		tracker, e = mods.LoadTracker(root)
		return e
	}, func(e error) {
		if e != nil {
			return
		}
		w.logs.Record("INFO", fmt.Sprintf("Game scan: %s · %s · read-only=%t", root, result.Version, result.ReadOnly))
		w.scanPath = root
		w.readonly = result.ReadOnly
		w.rows = result.Rows
		w.table.Refresh()
		access := "Writable"
		if result.ReadOnly {
			access = "Read-only — changes disabled"
		}
		w.version.SetText(result.Version + "  ·  " + access)
		w.app.Preferences().SetString("game-path", root)
		w.dependencies = true
		for name, c := range w.checks {
			c.SetChecked(tracker.Has(name))
			label := "Not installed"
			if tracker.Has(name) {
				label = installedModLabel(tracker.Versions[name].Version)
			}
			w.modLabels[name].SetText(label)
		}
		w.dependencies = false
		w.refreshModSelection()
		w.refreshEnabled()
		if tracker.Legacy() && !w.warned[root] {
			w.warned[root] = true
			w.info("Older mod tracking format", "The next mod installation will reinstall every recorded mod to migrate version tracking. Back up customized configuration files first.")
		}
		w.setStatus(fmt.Sprintf("Scan complete: %d manifest files checked.", len(result.Rows)))
		w.checkMods(root, tracker)
	})
}
func (w *Window) checkMods(root string, t mods.Tracker) {
	if len(t.Installed) == 0 {
		w.modStatus.SetText("No downgrader-installed mods detected.")
		return
	}
	generation := w.generation
	w.modStatus.SetText("Checking installed mod versions…")
	w.background(func(ctx context.Context) {
		resolver := mods.NewResolver(w.client)
		labels := map[string]string{}
		updates, unknown, failed := 0, 0, 0
		for _, name := range t.Installed {
			if ctx.Err() != nil {
				return
			}
			if !mods.Supported(name) {
				continue
			}
			installed := t.Versions[name]
			latest, e := resolver.Resolve(ctx, name)
			labels[name] = modUpdateLabel(installed, latest, e)
			if e != nil {
				w.logs.Record("WARN", "Mod update check ("+name+"): "+e.Error())
				failed++
				continue
			}
			switch {
			case installed.Fingerprint == "":
				unknown++
			case installed.Fingerprint != latest.Fingerprint:
				updates++
			}
		}
		w.post(func() {
			if w.generation != generation || w.path.Text != root {
				return
			}
			for name, label := range labels {
				w.modLabels[name].SetText(label)
			}
			w.modStatus.SetText(modCheckSummary(updates, unknown, failed))
		})
	})
}
func (w *Window) confirmMods(root string, force bool, proceed func()) {
	t, e := mods.LoadTracker(root)
	if e != nil {
		w.error(e)
		return
	}
	if force || t.Legacy() {
		w.confirm("Reinstall mod files", "Reinstallation can overwrite customized mod configuration files. Legacy trackers reinstall every recorded mod. Continue after backing up any settings you want to keep.", func(ok bool) {
			if ok {
				proceed()
			}
		})
	} else {
		proceed()
	}
}
func (w *Window) installMods(force bool) {
	root := w.scanPath
	selected := w.selected()
	if root == "" || len(selected) == 0 {
		w.error(fmt.Errorf("scan a game and select at least one mod"))
		return
	}
	w.confirmMods(root, force, func() {
		w.run("Installing mods", func(ctx context.Context) error {
			return w.modInstaller().InstallSelected(ctx, root, selected, force, w.modEvent)
		}, func(e error) {
			if e == nil {
				w.launchAdvice(selected)
			}
			w.scan()
		})
	})
}
func (w *Window) modInstaller() mods.Installer {
	return mods.Installer{Client: w.client, AppVersion: updater.Version}
}
func (w *Window) modEvent(name, status string, done, total int64) {
	w.report(name+" · "+status, done, total)
}
func (w *Window) launchAdvice(selected []string) {
	if runtime.GOOS != "linux" || len(selected) == 0 {
		return
	}
	w.launchOptions()
}
func (w *Window) apply() {
	root, dir := w.scanPath, w.patchDir
	selected := w.selected()
	if root == "" || dir == "" {
		return
	}
	warning := "Back up originals and downgrade this game? Modified files will be attempted; each result must match its target hash."
	w.confirm("Confirm downgrade", warning, func(ok bool) {
		if !ok {
			return
		}
		proceed := func() {
			w.run("Downgrading game", func(ctx context.Context) error {
				m, e := game.Load(filepath.Join(dir, "manifest.json"))
				if e != nil {
					return e
				}
				bin, cleanup, e := assets.Xdelta(ctx)
				if e != nil {
					return e
				}
				defer cleanup()
				_, e = (patch.Engine{Xdelta: bin}).Apply(ctx, root, dir, m, func(i int, status, message string) {
					w.post(func() {
						if i < len(w.rows) {
							w.rows[i].Status = status
							if message != "" {
								w.rows[i].Status += " · " + message
							}
							w.table.Refresh()
						}
						w.progress.SetValue(float64(i+1) / float64(len(m.Files)))
						w.setStatus(m.Files[i].Path + " · " + status)
						if message != "" {
							w.logs.Record("INFO", m.Files[i].Path+": "+message)
						}
					})
				})
				if e != nil {
					return e
				}
				if len(selected) > 0 {
					return w.modInstaller().InstallSelected(ctx, root, selected, false, w.modEvent)
				}
				return nil
			}, func(e error) {
				if e == nil {
					w.launchAdvice(selected)
				}
				w.scan()
			})
		}
		if len(selected) > 0 {
			w.confirmMods(root, false, proceed)
		} else {
			proceed()
		}
	})
}
func (w *Window) downloadPatches() {
	target := filepath.Join(w.cache, "Patches")
	w.run("Downloading patch assets", func(ctx context.Context) error {
		return patch.DownloadAssets(ctx, w.client, download.PatchURL, target, func(d, t int64) { w.report("Downloading Patches.zip", d, t) })
	}, func(e error) {
		if e == nil {
			w.patchDir = target
			if w.path.Text != "" {
				w.scan()
			}
		}
	})
}
func (w *Window) checkUpdates(manual bool) {
	w.background(func(ctx context.Context) {
		r, e := updater.Check(ctx, w.client)
		if e != nil {
			w.logs.Record("WARN", "App update check: "+e.Error())
		} else {
			w.logs.Record("INFO", "App update check: latest release "+r.Tag)
		}
		w.post(func() {
			if e != nil {
				if manual {
					w.error(e)
				}
				return
			}
			if !updater.Newer(r.Tag, updater.Version) {
				if manual {
					w.info("App updates", "You are running the current version.")
				}
				return
			}
			w.updateButton.SetText("Update available")
			if w.busy {
				return
			}
			a, ok := updater.Select(r, runtime.GOOS, assets.Offline, os.Getenv("APPIMAGE") != "")
			if !ok {
				w.info("Update available", r.Tag+" is available. Download a matching build from the releases page.")
				return
			}
			w.confirm("Update available", "Download "+r.Tag+" and restart the application?", func(ok bool) {
				if !ok {
					return
				}
				var staged string
				target := updater.Executable()
				w.run("Downloading app update", func(ctx context.Context) error {
					var e error
					staged, e = updater.Prepare(ctx, w.client, a, target, func(d, t int64) { w.report("Downloading update", d, t) })
					return e
				}, func(e error) {
					if e != nil {
						return
					}
					if e = updater.StartHelper(context.Background(), staged, target); e != nil {
						w.error(e)
						return
					}
					w.close()
				})
			})
		})
	})
}

func (w *Window) setStatus(text string) { w.logs.Record("INFO", text); w.status.SetText(text) }

// Background checks have no operation completion callback, but still need the
// same panic reporting boundary as a foreground scan or installation.
func (w *Window) background(work func(context.Context)) {
	w.jobs.Go(func(ctx context.Context) {
		if err := w.logs.Guard(ctx, func(ctx context.Context) error { work(ctx); return nil }); err != nil {
			w.post(func() { w.error(err) })
		}
	})
}
