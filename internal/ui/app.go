// Package ui provides the native Fyne frontend. Widgets are only read or changed
// on Fyne's event goroutine; workers receive immutable operation snapshots.
package ui

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
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
	app                                   fyne.App
	window                                fyne.Window
	jobs                                  *jobs.Group
	client                                *download.Client
	cache, patchDir                       string
	path                                  *widget.Entry
	status, version, modStatus            *widget.Label
	progress                              *widget.ProgressBar
	table                                 *widget.Table
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
}

func Run(gamePath, patchPath string) {
	a := app.NewWithID("pp.ua.xxanqw.gtasa-open-downgrader")
	w := New(a)
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
func New(a fyne.App) *Window {
	cache, e := os.UserCacheDir()
	if e != nil {
		cache = os.TempDir()
	}
	cache = filepath.Join(cache, "gtasa-open-downgrader")
	w := &Window{app: a, window: a.NewWindow("GTA SA Open Downgrader · " + updater.Version), jobs: jobs.New(), client: download.New(), cache: cache, checks: map[string]*widget.Check{}, warned: map[string]bool{}}
	w.window.SetIcon(fyne.NewStaticResource("icon.png", assets.Icon))
	w.window.Resize(fyne.NewSize(1100, 760))
	w.window.SetCloseIntercept(w.close)
	w.path = widget.NewEntry()
	w.path.SetPlaceHolder("Select your GTA San Andreas directory")
	w.path.SetText(a.Preferences().String("game-path"))
	w.path.OnChanged = func(string) { w.scanPath = ""; w.generation++; w.refreshEnabled() }
	w.path.OnSubmitted = func(string) { w.scan() }
	browse := widget.NewButtonWithIcon("Browse", theme.FolderOpenIcon(), func() {
		d := dialog.NewFolderOpen(func(uri fyne.ListableURI, e error) {
			if e != nil {
				w.error(e)
				return
			}
			if uri != nil {
				w.path.SetText(uri.Path())
				w.scan()
			}
		}, w.window)
		d.Show()
	})
	scan := widget.NewButtonWithIcon("Scan", theme.SearchIcon(), w.scan)
	w.version = widget.NewLabel("Version: waiting for scan")
	w.status = widget.NewLabel("Ready")
	w.status.Wrapping = fyne.TextWrapWord
	w.progress = widget.NewProgressBar()
	w.modStatus = widget.NewLabel("Select fixes to install after downgrading, or install them separately.")
	w.modStatus.Wrapping = fyne.TextWrapWord
	w.table = widget.NewTable(func() (int, int) { return len(w.rows) + 1, 5 }, func() fyne.CanvasObject { return widget.NewLabel("") }, func(id widget.TableCellID, o fyne.CanvasObject) {
		value := ""
		if id.Row == 0 {
			value = []string{"File", "Needs patch", "Status", "Current MD5", "Target MD5"}[id.Col]
		} else if id.Row <= len(w.rows) {
			r := w.rows[id.Row-1]
			value = []string{r.Path, r.NeedsPatch, r.Status, r.CurrentHash, r.TargetHash}[id.Col]
		}
		o.(*widget.Label).SetText(value)
	})
	for col, width := range []float32{230, 115, 235, 310, 310} {
		w.table.SetColumnWidth(col, width)
	}
	var checkObjects []fyne.CanvasObject
	for _, source := range mods.Catalog {
		name := source.Name
		c := widget.NewCheck(name, func(bool) { w.selectDependencies() })
		w.checks[name] = c
		checkObjects = append(checkObjects, c)
	}
	w.downgrade = widget.NewButtonWithIcon("Downgrade & install selected mods", theme.DownloadIcon(), w.apply)
	w.downgrade.Importance = widget.HighImportance
	w.install = widget.NewButton("Install selected mods", func() { w.installMods(false) })
	w.reinstall = widget.NewButton("Force reinstall / update selected", func() { w.installMods(true) })
	tools := widget.NewButtonWithIcon("Tools", theme.SettingsIcon(), w.tools)
	about := widget.NewButton("About", func() {
		dialog.ShowCustom("About GTA SA Open Downgrader", "Close", container.NewVBox(widget.NewLabel("Go / Fyne rewrite · "+updater.Version), widget.NewLabel("Inspired by the RockstarNexus Downgrader. MIT licensed."), widget.NewHyperlink("Source and releases", mustURL("https://github.com/xxanqw/gtasa-open-downgrader")), widget.NewLabel("Bundled xdelta3: Apache License 2.0"), container.NewVScroll(widget.NewLabel(assets.XdeltaLicense))), w.window)
	})
	updates := widget.NewButton("Check app updates", func() { w.checkUpdates(true) })
	w.updateButton = updates
	w.cancelButton = widget.NewButton("Cancel", func() {
		if w.cancel != nil {
			w.cancel()
			w.status.SetText("Cancelling; waiting for the current operation to stop…")
			w.cancelButton.Disable()
		}
	})
	w.cancelButton.Disable()
	w.controls = []fyne.Disableable{w.path, browse, scan, tools, updates}
	top := container.NewVBox(container.NewBorder(nil, nil, widget.NewLabel("Game folder"), container.NewHBox(browse, scan), w.path), container.NewBorder(nil, nil, w.version, container.NewHBox(tools, updates, about)))
	bottom := container.NewVBox(widget.NewSeparator(), widget.NewLabelWithStyle("Community fixes & enhancements", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), container.NewGridWithColumns(3, checkObjects...), w.modStatus, container.NewHBox(w.downgrade, w.install, w.reinstall), w.progress, container.NewBorder(nil, nil, nil, w.cancelButton, w.status))
	w.window.SetContent(container.NewBorder(top, bottom, nil, nil, w.table))
	w.refreshEnabled()
	return w
}
func mustURL(raw string) *url.URL { u, _ := url.Parse(raw); return u }
func (w *Window) error(e error) {
	if e != nil && !w.closing {
		dialog.ShowError(e, w.window)
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
	if w.cancel != nil {
		w.cancel()
	}
	w.status.SetText("Stopping background work…")
	go func() { w.jobs.Close(); fyne.Do(func() { w.window.SetCloseIntercept(nil); w.app.Quit() }) }()
}
func (w *Window) refreshEnabled() {
	for _, c := range w.controls {
		if w.busy {
			c.Disable()
		} else {
			c.Enable()
		}
	}
	ready := w.scanPath != "" && w.scanPath == w.path.Text && !w.readonly && !w.busy
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
	defer func() { w.dependencies = false }()
	dependent := false
	for name, c := range w.checks {
		if name != "ASI Loader" && name != "ModLoader" && c.Checked {
			dependent = true
		}
	}
	if dependent {
		w.checks["ModLoader"].SetChecked(true)
		w.checks["ModLoader"].Disable()
	} else if !w.busy && !w.readonly && w.scanPath != "" {
		w.checks["ModLoader"].Enable()
	}
	if dependent || w.checks["ModLoader"].Checked {
		w.checks["ASI Loader"].SetChecked(true)
		w.checks["ASI Loader"].Disable()
	} else if !w.busy && !w.readonly && w.scanPath != "" {
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
	w.status.SetText(title)
	w.progress.SetValue(0)
	w.refreshEnabled()
	w.cancelButton.Enable()
	ctx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel
	w.jobs.Go(func(parent context.Context) {
		stop := context.AfterFunc(parent, cancel)
		defer stop()
		err := work(ctx)
		cancel()
		w.post(func() {
			w.busy = false
			w.cancel = nil
			w.cancelButton.Disable()
			w.refreshEnabled()
			if errors.Is(err, context.Canceled) {
				w.status.SetText("Cancelled. Completed work and backups have been preserved.")
			} else if err != nil {
				w.status.SetText(title + " failed")
				w.error(err)
			} else {
				w.progress.SetValue(1)
				w.status.SetText(title + " complete")
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
			w.patchDir = ""
		}
		if w.path.Text == "" {
			w.path.SetText(detected)
		}
		if prepareError != nil && !errors.Is(e, context.Canceled) {
			dialog.ShowConfirm("Patches missing", "Download the official Patches.zip from iCloud? You can also import a local archive from Tools.", func(ok bool) {
				if ok {
					w.downloadPatches()
				} else if w.path.Text != "" {
					w.scan()
				}
			}, w.window)
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
		w.scanPath = root
		w.readonly = result.ReadOnly
		w.rows = result.Rows
		w.table.Refresh()
		w.version.SetText(fmt.Sprintf("Version: %s · Read-only: %t", result.Version, result.ReadOnly))
		w.app.Preferences().SetString("game-path", root)
		w.dependencies = true
		for name, c := range w.checks {
			c.SetChecked(tracker.Has(name))
			label := name
			if tracker.Has(name) {
				version := tracker.Versions[name].Version
				if version == "" {
					version = "version unknown"
				}
				label += " (" + version + ")"
			}
			c.SetText(label)
		}
		w.dependencies = false
		w.refreshEnabled()
		if tracker.Legacy() && !w.warned[root] {
			w.warned[root] = true
			dialog.ShowInformation("Older mod tracking format", "The next mod installation will reinstall every recorded mod to migrate version tracking. Back up customized configuration files first.", w.window)
		}
		w.status.SetText(fmt.Sprintf("Scan complete: %d manifest files checked.", len(result.Rows)))
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
	w.jobs.Go(func(ctx context.Context) {
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
			latest, e := resolver.Resolve(ctx, name)
			if e != nil {
				failed++
				labels[name] = name + " (update check failed)"
				continue
			}
			installed := t.Versions[name]
			switch {
			case installed.Fingerprint == "":
				unknown++
				labels[name] = name + " (version unknown)"
			case installed.Fingerprint != latest.Fingerprint:
				updates++
				labels[name] = name + " (update available: " + latest.Version + ")"
			default:
				labels[name] = name + " (" + installed.Version + ")"
			}
		}
		w.post(func() {
			if w.generation != generation || w.path.Text != root {
				return
			}
			for name, label := range labels {
				w.checks[name].SetText(label)
			}
			w.modStatus.SetText(fmt.Sprintf("Mod checks: %d updates · %d unknown versions · %d unavailable. Reinstall selected mods to update.", updates, unknown, failed))
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
		dialog.ShowConfirm("Reinstall mod files", "Reinstallation can overwrite customized mod configuration files. Legacy trackers reinstall every recorded mod. Continue after backing up any settings you want to keep.", func(ok bool) {
			if ok {
				proceed()
			}
		}, w.window)
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
	dialog.ShowConfirm("Confirm downgrade", warning, func(ok bool) {
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
						w.status.SetText(m.Files[i].Path + " · " + status)
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
	}, w.window)
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
	w.jobs.Go(func(ctx context.Context) {
		r, e := updater.Check(ctx, w.client)
		w.post(func() {
			if e != nil {
				if manual {
					w.error(e)
				}
				return
			}
			if !updater.Newer(r.Tag, updater.Version) {
				if manual {
					dialog.ShowInformation("App updates", "You are running the current version.", w.window)
				}
				return
			}
			w.updateButton.SetText("Update to " + r.Tag)
			if w.busy {
				return
			}
			a, ok := updater.Select(r, runtime.GOOS, assets.Offline, os.Getenv("APPIMAGE") != "")
			if !ok {
				dialog.ShowInformation("Update available", r.Tag+" is available. Download a matching build from the releases page.", w.window)
				return
			}
			dialog.ShowConfirm("Update available", "Download "+r.Tag+" and restart the application?", func(ok bool) {
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
			}, w.window)
		})
	})
}
