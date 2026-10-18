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

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/ncruces/zenity"
	"github.com/xxanqw/gtasa-open-downgrader/internal/applog"
	"github.com/xxanqw/gtasa-open-downgrader/internal/assets"
	"github.com/xxanqw/gtasa-open-downgrader/internal/download"
	"github.com/xxanqw/gtasa-open-downgrader/internal/game"
	"github.com/xxanqw/gtasa-open-downgrader/internal/jobs"
	"github.com/xxanqw/gtasa-open-downgrader/internal/mods"
	"github.com/xxanqw/gtasa-open-downgrader/internal/patch"
	"github.com/xxanqw/gtasa-open-downgrader/internal/platform"
	appwidget "github.com/xxanqw/gtasa-open-downgrader/internal/uiwidget"
	"github.com/xxanqw/gtasa-open-downgrader/internal/updater"
)

type Window struct {
	logs                                            *applog.Journal
	pickers                                         pickerBackend
	activePrompt                                    func()
	prompts                                         []promptRequest
	promptActive                                    bool
	app                                             fyne.App
	window                                          fyne.Window
	jobs                                            *jobs.Group
	client                                          *download.Client
	cache, patchDir                                 string
	path                                            *widget.Entry
	status, version, modStatus                      *widget.Label
	progress                                        *thinProgress
	table                                           *readOnlyTable
	rows                                            []game.Row
	checks                                          map[string]*modCheck
	controls, postDowngradeActions                  []fyne.Disableable
	cancelButton                                    *appwidget.Button
	downgrade, install, updateModsButton, reinstall *appwidget.Button
	cancel                                          context.CancelFunc
	busy, closing, readonly, dependencies           bool
	generation                                      int
	installedModCount                               int
	warned                                          map[string]bool
	scanPath                                        string
	transferLabel                                   string
	updateButton                                    *appwidget.Button
	modLabels                                       map[string]*widget.Label
	modDetails                                      map[string]string
	installedMods                                   map[string]bool
	modPanel                                        *fyne.Container
	patchButton                                     *appwidget.Button
	hubNav                                          map[string]*appwidget.Button
	hubPages                                        map[string]fyne.CanvasObject
	hubContent, multiplayerBody                     *fyne.Container
	footer, footerActions                           *fyne.Container
	footerLogButton                                 *appwidget.Button
	hubReadiness, hubBackups                        *widget.Label
	lastDowngradeOutcome                            string
	hubSelected                                     string
	navigation, gameHeader                          *fyne.Container
	checksums                                       bool
	checksumCheck                                   *modCheck
	modTracker                                      mods.Tracker
	modEmpty                                        *widget.Label
	modSelection                                    *widget.Label
	modTable                                        *modWebTable
	modTableNames                                   []string
	modExplicit                                     map[string]bool
	modRequiredBy                                   map[string][]string
	iniEditors                                      map[string]*iniEditor
	iniDiscardConfirmed                             bool
	modFilter                                       *controlSelect
	modSortColumn                                   int
	modSortDescending                               bool
	bulkActions                                     *fyne.Container
	bulkRemove, bulkClear                           *appwidget.Button
	recoveryActions                                 []fyne.Disableable
	legacyRussianRemove                             *appwidget.Button
	legacyRussianSection                            fyne.CanvasObject
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
	appwidget.ConfigureDesktopCursors()
	a := app.NewWithID("me.xxanqw.gtasa-open-downgrader")
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
func newWindow(a fyne.App, journal *applog.Journal) *Window {
	a.Settings().SetTheme(appwidget.CompactTheme{Theme: theme.DefaultTheme()})
	cache, e := os.UserCacheDir()
	if e != nil {
		cache = os.TempDir()
	}
	cache = filepath.Join(cache, "gtasa-open-downgrader")
	w := &Window{logs: journal, app: a, window: a.NewWindow("GTA SA Open Downgrader · " + updater.Version), jobs: jobs.New(), client: download.New(), cache: cache, checks: map[string]*modCheck{}, modLabels: map[string]*widget.Label{}, modDetails: map[string]string{}, installedMods: map[string]bool{}, warned: map[string]bool{}}
	if _, ok := w.window.(driver.NativeWindow); ok {
		w.pickers = pickerBackend{available: zenity.IsAvailable, show: systemPicker}
	}
	w.window.SetIcon(fyne.NewStaticResource("icon.png", assets.Icon))
	w.window.Resize(fyne.NewSize(940, 640))
	w.window.SetPadded(false)
	w.window.SetCloseIntercept(w.close)
	w.path = widget.NewEntry()
	w.path.TextStyle.Monospace = true
	w.path.SetPlaceHolder("Select your GTA San Andreas directory")
	w.path.SetText(a.Preferences().String("game-path"))
	w.path.OnChanged = func(path string) {
		if w.scanPath != "" && filepath.Clean(path) != w.scanPath {
			w.lastDowngradeOutcome = ""
		}
		w.scanPath = ""
		w.installedModCount = 0
		w.installedMods = map[string]bool{}
		w.modTracker = mods.Tracker{}
		w.generation++
		w.rows = nil
		w.modExplicit = map[string]bool{}
		w.modRequiredBy = map[string][]string{}
		w.dependencies = true
		for name, check := range w.checks {
			check.SetChecked(false)
			w.setModLabel(name, "Not installed")
		}
		w.dependencies = false
		if w.multiplayerBody != nil {
			w.multiplayerBody.Objects = []fyne.CanvasObject{hubText("Scan a game to view multiplayer options.")}
			w.multiplayerBody.Refresh()
		}
		if w.table != nil {
			w.table.Refresh()
			if strings.TrimSpace(path) == "" {
				w.version.SetText("No game selected")
			} else {
				w.version.SetText("Not scanned")
			}
			w.version.Importance = widget.MediumImportance
		}
		w.refreshEnabled()
	}
	w.path.OnSubmitted = func(string) { w.scan() }
	browse := appwidget.NewButtonWithIcon("Browse", theme.FolderOpenIcon(), func() {
		w.chooseFolder("Select GTA San Andreas folder", w.path.Text, func(path string) {
			w.path.SetText(path)
			w.scan()
		})
	})
	scan := appwidget.NewButtonWithIcon("Scan game", theme.SearchIcon(), w.scan)
	w.version = compactLabel("No game selected")
	if strings.TrimSpace(w.path.Text) != "" {
		w.version.SetText("Not scanned")
	}
	w.version.Alignment = fyne.TextAlignTrailing
	w.status = compactLabel("Ready")
	w.progress = newThinProgress()
	w.modStatus = widget.NewLabel("Scan a game to check installed mods.")
	w.modStatus.Truncation = fyne.TextTruncateEllipsis
	w.modStatus.Alignment = fyne.TextAlignTrailing
	w.modStatus.Importance = widget.LowImportance
	w.table = newReadOnlyTable(func() (int, int) {
		if w.checksums {
			return len(w.rows) + 1, len(fileColumns)
		}
		return len(w.rows) + 1, 2
	}, func() fyne.CanvasObject {
		return container.NewThemeOverride(compactLabel(""), fileTableTheme{a.Settings().Theme()})
	}, func(id widget.TableCellID, o fyne.CanvasObject) {
		label := o.(*container.ThemeOverride).Content.(*widget.Label)
		value := ""
		label.TextStyle = fyne.TextStyle{Bold: id.Row == 0, Monospace: id.Row > 0 && (id.Col == 0 || w.checksums && id.Col >= 3)}
		label.Importance = widget.MediumImportance
		statusColumn := 1
		if w.checksums {
			statusColumn = 2
		}
		if id.Row == 0 {
			if w.checksums {
				value = fileColumns[id.Col]
			} else {
				value = []string{"File", "Status"}[id.Col]
			}
		} else if id.Row <= len(w.rows) {
			r := w.rows[id.Row-1]
			if w.checksums {
				value = fileValues(r)[id.Col]
			} else {
				value = []string{r.Path, r.Status}[id.Col]
			}
			if id.Col == statusColumn {
				switch {
				case strings.HasPrefix(r.Status, "Failed") || strings.HasPrefix(r.Status, "Error"):
					label.Importance = widget.DangerImportance
					label.TextStyle.Bold = true
				case r.Status == "Missing" || r.Status == "Modified" || r.Status == "Source edition unknown":
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
	w.modSelection = widget.NewLabel("0 selected")
	w.modSelection.Truncation = fyne.TextTruncateEllipsis
	w.modEmpty = widget.NewLabel("No matching mods.")
	w.modEmpty.Hide()
	for _, source := range mods.Catalog {
		name := source.Name
		c := newModCheck("", func(value bool) {
			if w.modTable != nil && !w.dependencies && len(w.modRequiredBy[name]) == 0 {
				w.modExplicit[name] = value
			}
			w.selectDependencies()
		})
		w.checks[name] = c
		state := widget.NewLabel("Not installed")
		state.Importance = widget.LowImportance
		w.modLabels[name] = state
	}
	w.downgrade = appwidget.NewButtonWithIcon("Review downgrade", theme.MediaPlayIcon(), w.apply)
	w.downgrade.Importance = widget.HighImportance
	w.install = appwidget.NewButtonWithIcon("Install selected", theme.DownloadIcon(), func() { w.installMods(false) })
	w.install.Importance = widget.HighImportance
	w.updateModsButton = appwidget.NewButtonWithIcon("Update mods", theme.ViewRefreshIcon(), w.updateMods)
	w.reinstall = appwidget.NewButton("Reinstall selected", func() { w.installMods(true) })
	updates := appwidget.NewButtonWithIcon("Check app updates", theme.ViewRefreshIcon(), func() { w.checkUpdates(true) })
	w.updateButton = updates
	w.cancelButton = appwidget.NewButton("Cancel", func() {
		if w.cancel != nil {
			w.cancel()
			w.setStatus("Cancelling; waiting for the current operation to stop…")
			w.cancelButton.Disable()
		}
	})
	w.cancelButton.Disable()
	w.controls = []fyne.Disableable{w.path, browse, scan, updates}
	w.modPanel = container.NewStack(w.newModTable())

	w.gameHeader = container.New(layout.NewCustomPaddedLayout(10, 0, 14, 14),
		container.NewBorder(nil, nil, nil, container.NewHBox(browse, scan), styleInput(w.path)))
	w.footerLogButton = appwidget.NewButtonWithIcon("Log", theme.InfoIcon(), w.showLogs)
	w.footerActions = container.NewHBox(w.cancelButton, w.footerLogButton, w.downgrade)
	w.footer = container.NewVBox(container.NewBorder(nil, nil, nil, w.footerActions, container.NewGridWithColumns(2, w.status, w.version)), container.NewThemeOverride(w.progress, progressBarTheme{a.Settings().Theme()}))
	pages := w.buildHub()
	workspace := container.NewBorder(nil, container.New(layout.NewCustomPaddedLayout(5, 5, 12, 12), w.footer), nil, nil, pages)
	w.window.SetContent(container.NewBorder(w.navigation, nil, nil, nil, workspace))
	w.refreshEnabled()
	w.window.CenterOnScreen()
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
	if !w.iniDiscardConfirmed {
		for _, editor := range w.iniEditors {
			if editor.dirty() {
				w.confirm("Close with unsaved INI changes?", "One or more settings editors have unsaved changes. Close the application and discard them?", func(ok bool) {
					if ok {
						w.iniDiscardConfirmed = true
						w.close()
					}
				})
				return
			}
		}
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
		w.refreshFooterActions()
	}
	for _, c := range w.controls {
		if w.busy || w.promptActive {
			c.Disable()
		} else {
			c.Enable()
		}
	}
	ready := w.scanPath != "" && w.scanPath == w.path.Text && !w.readonly && !w.busy && !w.promptActive
	complete := w.downgradeComplete()
	if ready && w.patchDir != "" {
		w.downgrade.Enable()
	} else {
		w.downgrade.Disable()
	}
	for _, b := range []*appwidget.Button{w.install, w.updateModsButton, w.reinstall} {
		if ready && complete {
			b.Enable()
		} else {
			b.Disable()
		}
	}
	for _, action := range w.postDowngradeActions {
		if ready && complete {
			action.Enable()
		} else {
			action.Disable()
		}
	}
	if w.installedModCount == 0 {
		w.updateModsButton.Disable()
	}
	if w.modPanel != nil {
		w.modPanel.Refresh()
	}
	for _, c := range w.checks {
		if ready && complete {
			c.Enable()
		} else {
			c.Disable()
		}
	}
	if ready && complete {
		w.selectDependencies()
	}
	w.refreshModBatch()
	if w.legacyRussianSection != nil {
		if w.installedMods[mods.SanLtdRussian] {
			w.legacyRussianSection.Show()
		} else {
			w.legacyRussianSection.Hide()
		}
		if ready && complete && w.installedMods[mods.SanLtdRussian] {
			w.legacyRussianRemove.Enable()
		} else {
			w.legacyRussianRemove.Disable()
		}
	}
	w.refreshHubNavigation()
	w.refreshHub()
}
func (w *Window) selectDependencies() {
	if !w.dependencies {
		w.refreshModBatch()
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
	w.runOperation(title, work, done, true)
}

// Guides present their own inline result instead of queueing a second error sheet.
func (w *Window) runHandled(title string, work func(context.Context) error, done func(error)) {
	w.runOperation(title, work, done, false)
}
func (w *Window) runOperation(title string, work func(context.Context) error, done func(error), showError bool) {
	if w.busy || w.closing {
		return
	}
	w.busy = true
	w.generation++
	w.transferLabel = ""
	w.setStatus(title)
	w.setProgress(0)
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
				if showError {
					w.error(err)
				}
			} else {
				w.setProgress(1)
				w.setStatus(title + " complete")
			}
			if done != nil {
				done(err)
			}
		})
	})
}
func (w *Window) setProgress(value float64) {
	w.progress.SetValue(value)
}
func (w *Window) report(label string, done, total int64) {
	w.post(func() {
		if label != w.transferLabel {
			w.logs.Record("INFO", label)
			w.transferLabel = label
		}
		if total > 0 {
			w.setProgress(float64(done) / float64(total))
			label += fmt.Sprintf(" · %.0f%%", 100*float64(done)/float64(total))
		}
		w.status.SetText(compactStatusText(label))
		if w.busy && w.hubReadiness != nil {
			w.hubReadiness.SetText(w.status.Text)
		}
	})
}
func (w *Window) start() {
	dir := w.patchDir
	detect := strings.TrimSpace(w.path.Text) == "" || !game.HasEXE(w.path.Text)
	var detected string
	var prepareError error
	w.run("Preparing patch assets", func(ctx context.Context) error {
		if detect {
			detected = platform.Detect()
		}
		if dir != "" {
			_, prepareError = patch.ValidateReleaseAssets(dir)
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
		if detect {
			if detected != "" {
				w.path.SetText(detected)
			} else if w.path.Text != "" && !game.HasEXE(w.path.Text) {
				w.path.SetText("") // A remembered install may have moved to another library.
			}
		}
		if !errors.Is(e, context.Canceled) && w.path.Text != "" {
			w.scan()
		} else {
			w.refreshEnabled()
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
		result, e = game.Inspect(ctx, root, m, func(i, n int) { w.post(func() { w.setProgress(float64(i) / float64(n)) }) })
		if e != nil {
			return e
		}
		tracker, e = mods.LoadTracker(root)
		return e
	}, func(e error) {
		if e != nil {
			// A failed rescan cannot keep authorizing game-changing actions
			// using results from before a possibly modified installation.
			w.scanPath = ""
			w.rows = nil
			w.table.Refresh()
			w.version.SetText("Scan failed | rescan")
			w.lastDowngradeOutcome = "Scan again to verify the downgrade before using Mods or Multiplayer."
			w.refreshEnabled()
			return
		}
		w.logs.Record("INFO", fmt.Sprintf("Game scan: %s · %s · read-only=%t", root, result.Version, result.ReadOnly))
		w.scanPath = root
		w.installedModCount = 0
		w.installedMods = map[string]bool{}
		for _, name := range tracker.Installed {
			if mods.Supported(name) {
				w.installedModCount++
				w.installedMods[name] = true
			}
		}
		if tracker.Has(mods.SanLtdRussian) {
			w.installedMods[mods.SanLtdRussian] = true
		}
		w.readonly = result.ReadOnly
		w.rows = result.Rows
		if verifiedDowngrade(result.Rows) && w.lastDowngradeOutcome != "" {
			w.lastDowngradeOutcome = "Downgrade verified. Original game files remain in backups/."
		} else if w.lastDowngradeOutcome == downgradeVerifyPending {
			w.lastDowngradeOutcome = "Downgrade is not fully verified. Review the hash results or use Recovery."
		} else if w.lastDowngradeOutcome == "Scan again to verify the downgrade before using Mods or Multiplayer." || strings.HasPrefix(w.lastDowngradeOutcome, "Downgrade verified.") {
			w.lastDowngradeOutcome = ""
		}
		w.table.Refresh()
		access := "Writable"
		w.version.Importance = widget.MediumImportance
		if result.ReadOnly {
			access = "Read-only"
			w.version.Importance = widget.WarningImportance
		}
		w.version.SetText(result.Version + " | " + access)
		w.app.Preferences().SetString("game-path", root)
		w.dependencies = true
		w.modTracker = tracker
		w.modExplicit = map[string]bool{}
		w.modRequiredBy = map[string][]string{}
		for name, c := range w.checks {
			c.SetChecked(false)
			label := "Not installed"
			if tracker.Has(name) {
				label = installedModLabel(tracker.Versions[name].Version)
			}
			w.setModLabel(name, label)
		}
		w.dependencies = false
		w.refreshEnabled()
		if tracker.Legacy() && !w.warned[root] {
			w.warned[root] = true
			w.info("Older mod tracking format", "The next mod installation will reinstall every recorded mod to migrate version tracking. Back up customized configuration files first.")
		}
		w.setStatus(fmt.Sprintf("Scan complete: %d manifest files checked.", len(result.Rows)))
		if w.hubSelected == hubMultiplayer {
			w.multiplayer()
		}
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
			case installed.Fingerprint == "" || latest.Fingerprint == "":
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
				w.setModLabel(name, label)
			}
			w.modStatus.SetText(modCheckSummary(updates, unknown, failed))
		})
	})
}
func (w *Window) confirmMods(root string, force bool, selected []string, proceed func()) {
	t, e := mods.LoadTracker(root)
	if e != nil {
		w.error(e)
		return
	}
	if !force && !t.Legacy() {
		plan := mods.Selection(selected, t)
		w.confirm("Install selected mods", strings.Join(plan, "\n")+"\n\nRequired missing dependencies are included automatically. Each mod is installed separately with its own safeguards/backups; cancellation or failure can leave completed installations in place.", func(ok bool) {
			if ok {
				proceed()
			}
		})
		return
	}
	w.confirm("Reinstall mod files", strings.Join(mods.Selection(selected, t), "\n")+"\n\nReinstallation includes required packages and can overwrite customized mod configuration files. Legacy trackers reinstall every recorded mod. Continue after backing up any settings you want to keep.", func(ok bool) {
		if ok {
			proceed()
		}
	})
}
func (w *Window) installMods(force bool) {
	if !w.downgradeComplete() {
		w.info("Downgrade required", "Finish downgrading and scan the game before installing mods.")
		return
	}
	if w.readonly || w.busy || w.promptActive {
		return
	}
	root := w.scanPath
	selected := w.modSelectedInstalled(force)
	if root == "" || len(selected) == 0 {
		w.error(fmt.Errorf("scan a game and select at least one mod"))
		return
	}
	w.confirmMods(root, force, selected, func() {
		installer := w.modInstaller()
		w.run("Installing mods", func(ctx context.Context) error {
			return installer.InstallSelected(ctx, root, selected, force, w.modEvent)
		}, func(e error) {
			if e == nil {
				w.launchAdvice(selected)
			}
			w.scan()
		})
	})
}

func (w *Window) updateMods() {
	if !w.downgradeComplete() {
		w.info("Downgrade required", "Finish downgrading and scan the game before updating mods.")
		return
	}
	root := w.scanPath
	if root == "" || w.installedModCount == 0 {
		return
	}
	var result mods.UpdateResult
	installer := w.modInstaller()
	scoped := len(w.selected()) > 0
	selected := w.modSelectedInstalled(true)
	w.run("Updating installed mods", func(ctx context.Context) error {
		var err error
		if scoped {
			result, err = installer.UpdateSelected(ctx, root, selected, w.modEvent)
		} else {
			result, err = installer.UpdateInstalled(ctx, root, w.modEvent)
		}
		return err
	}, func(err error) {
		w.logs.Record("INFO", fmt.Sprintf("Mod updates: %d updated, %d current, %d untracked", result.Updated, result.Current, result.Untracked))
		w.scan()
		if err == nil && result.Updated == 0 {
			message := "All tracked installed mods are already current."
			if result.Untracked > 0 {
				message = fmt.Sprintf("No tracked updates found. %d mod(s) have no recorded version; use Reinstall selected if you want to refresh them.", result.Untracked)
			}
			w.info("No mod updates", message)
		}
	})
}

func (w *Window) uninstallMod(name string) {
	if !w.downgradeComplete() {
		w.info("Downgrade required", "Finish downgrading and scan the game before removing mods.")
		return
	}
	root := w.scanPath
	if root == "" || !w.installedMods[name] {
		return
	}
	message := "Remove " + name + " from this game? Its known files will be moved into backups/mods before the installation record is removed. Files you added inside its mod folder will be backed up too."
	if name == "ASI Loader" || name == "ModLoader" {
		message = "Remove " + name + " from this game? Its loader binaries will be backed up. Shared mod folders and configuration files will stay in place. Remove dependent mods first."
	}
	w.confirm("Uninstall "+name, message, func(ok bool) {
		if !ok {
			return
		}
		var result mods.UninstallResult
		w.run("Uninstalling "+name, func(ctx context.Context) error {
			var err error
			result, err = mods.Uninstall(ctx, root, name, updater.Version)
			return err
		}, func(err error) {
			if err == nil {
				if result.BackupDir != "" {
					w.logs.Record("INFO", name+" uninstall backup: "+result.BackupDir)
				} else {
					w.logs.Record("INFO", name+" uninstall: known files already absent")
				}
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
	if root == "" || dir == "" {
		return
	}
	w.prompt(promptRequest{kind: promptConfirm, title: "Review downgrade", text: downgradeReview(w.rows), accept: "Downgrade game", dismiss: "Cancel", done: func(_ string, ok bool) {
		if !ok {
			return
		}
		proceed := func() {
			w.lastDowngradeOutcome = ""
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
						w.setProgress(float64(i+1) / float64(len(m.Files)))
						w.setStatus(m.Files[i].Path + " · " + status)
						if message != "" {
							w.logs.Record("INFO", m.Files[i].Path+": "+message)
						}
					})
				})
				return e
			}, func(e error) {
				if e == nil {
					w.lastDowngradeOutcome = downgradeVerifyPending
				} else {
					w.lastDowngradeOutcome = "Operation stopped; completed changes and backups remain. Review files or Recovery."
					w.showHub(hubDowngrade)
				}
				w.scan()
			})
		}
		proceed()
	}})
}
func (w *Window) downloadPatches() {
	target := filepath.Join(w.cache, "Patches")
	w.run("Downloading patch assets", func(ctx context.Context) error {
		return patch.DownloadAssets(ctx, w.client, download.PatchURL, target, func(d, t int64) { w.report("Downloading Patches_v2.1.zip", d, t) })
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

func (w *Window) setStatus(text string) {
	w.logs.Record("INFO", text)
	w.status.SetText(compactStatusText(text))
	if w.busy && w.hubReadiness != nil {
		w.hubReadiness.SetText(w.status.Text)
	}
}

// Background checks have no operation completion callback, but still need the
// same panic reporting boundary as a foreground scan or installation.
func (w *Window) background(work func(context.Context)) {
	w.jobs.Go(func(ctx context.Context) {
		if err := w.logs.Guard(ctx, func(ctx context.Context) error { work(ctx); return nil }); err != nil {
			w.post(func() { w.error(err) })
		}
	})
}
