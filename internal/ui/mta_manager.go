package ui

import (
	"context"
	"fmt"
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/xxanqw/gtasa-open-downgrader/internal/multiplayer"
	"github.com/xxanqw/gtasa-open-downgrader/internal/platform"
	appwidget "github.com/xxanqw/gtasa-open-downgrader/internal/uiwidget"
)

func (w *Window) showMTAManager(environment multiplayer.Environment, runner multiplayer.Runner) {
	if w.closing || w.busy || w.scanPath != environment.GameDir {
		return
	}
	canChange := w.modTableReady() && w.scanPath == environment.GameDir
	var after func()
	w.prompt(promptRequest{done: func(string, bool) {
		if w.closing {
			return
		}
		if after != nil {
			after()
		} else {
			w.multiplayer()
		}
	}, present: func(done func(string, bool)) func() {
		installed := multiplayer.DetectMTAInstallations(environment)
		active := multiplayer.SelectedMTAClient(environment)
		_, choiceErr := multiplayer.LoadMTAChoice(environment.GameDir)
		body := container.NewVBox()
		summary := "No active client — install or choose a version."
		importance := widget.MediumImportance
		if active.Installed {
			summary = "Active: MTA " + active.Version
			if active.Version == "custom" {
				summary = "Active: configured MTA"
			}
			if active.Version == "1.7" {
				summary += " · Development / untested"
				importance = widget.WarningImportance
			}
		} else if choiceErr != nil {
			summary = "Version selection needs attention — choose an installed client."
			importance = widget.WarningImportance
		} else if active.Version != "" {
			summary = "Selected MTA " + active.Version + " is unavailable or unverified — choose another version."
			importance = widget.WarningImportance
		}
		status := appwidget.ModalText(summary)
		status.Importance = importance
		body.Add(status)
		body.Add(widget.NewSeparator())
		var modal *appwidget.Modal
		action := func(f func()) { after = f; modal.Close("") }
		addRow := func(version, channel string, release *multiplayer.MTARelease) {
			_, present := multiplayer.FindMTAInstallation(installed, version)
			name := "MTA " + version
			if version == "custom" {
				name = "Configured MTA"
			}
			state := "Not installed"
			isActive := active.Installed && active.Version == version
			if present {
				state = "Installed"
			}
			if isActive {
				state = "Active"
			}
			title := widget.NewLabelWithStyle(name+" · "+state, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
			title.Wrapping = fyne.TextWrapWord
			caption := appwidget.ModalText(channel)
			caption.Importance = widget.LowImportance
			if release != nil && release.Experimental {
				caption.Importance = widget.WarningImportance
			}
			buttons := container.NewHBox()
			if !present && release != nil {
				r := *release
				install := appwidget.NewButton("Install", func() { action(func() { w.confirmMTAInstall(environment, runner, r, false) }) })
				install.AccessibleLabel = "Install " + name
				if runner == nil || !canChange {
					install.Disable()
				}
				buttons.Add(install)
			}
			if present {
				use := appwidget.NewButton("Use "+version, func() { action(func() { w.chooseMTAVersion(environment, version) }) })
				use.AccessibleLabel = "Use " + name + " in the Steam launcher"
				if isActive || !canChange {
					use.Disable()
				}
				buttons.Add(use)
				if release != nil {
					r := *release
					reinstall := appwidget.NewButton("Reinstall…", func() { action(func() { w.confirmMTAInstall(environment, runner, r, true) }) })
					reinstall.AccessibleLabel = "Reinstall " + name
					if runner == nil || !canChange {
						reinstall.Disable()
					}
					buttons.Add(reinstall)
				}
				uninstall := appwidget.NewButton("Uninstall…", func() { action(func() { w.removeMTAVersion(environment, runner, version) }) })
				uninstall.AccessibleLabel = "Uninstall " + name
				if runner == nil || !canChange {
					uninstall.Disable()
				}
				buttons.Add(uninstall)
			}
			body.Add(container.NewBorder(nil, nil, nil, container.NewCenter(buttons), container.NewVBox(title, caption)))
			body.Add(widget.NewSeparator())
		}
		for _, entry := range multiplayer.MTAReleases {
			release := entry
			addRow(release.Version, release.Channel, &release)
		}
		for _, version := range []string{"1.5", "custom"} {
			if _, ok := multiplayer.FindMTAInstallation(installed, version); ok {
				addRow(version, "Existing installation", nil)
			}
		}
		var recovery fyne.CanvasObject
		if runtime.GOOS == "linux" && runtime.GOARCH == "amd64" {
			if active.Installed && active.Version == "1.6" {
				serial := appwidget.NewButton("Serial validation…", func() { action(func() { w.showMTAValidation(environment) }) })
				if runner == nil || !canChange {
					serial.Disable()
				}
				recovery = serial
			} else if len(installed) > 0 {
				recovery = appwidget.NewButton("Serial guide", func() {
					if err := w.app.OpenURL(mustURL(multiplayer.MTASerialGuideURL)); err != nil {
						appwidget.ShowError(w.window, err)
					}
				})
			}
		}
		modal = appwidget.ShowModal(w.window, appwidget.ModalOptions{Title: "MTA manager", Body: container.NewVScroll(body), Size: fyne.NewSize(700, 340), Extra: recovery, OnClosed: func(string) { done("", false) }})
		return func() { modal.Close("") }
	}})
}
func (w *Window) mtaManagerGuard(environment multiplayer.Environment) error {
	if !w.modTableReady() || w.scanPath != environment.GameDir {
		return fmt.Errorf("rescan the same writable, downgraded game before changing MTA")
	}
	return platform.RequireGameClosed()
}
func (w *Window) confirmMTAInstall(environment multiplayer.Environment, runner multiplayer.Runner, release multiplayer.MTARelease, reinstall bool) {
	if err := w.mtaManagerGuard(environment); err != nil {
		w.error(err)
		return
	}
	text := "Install MTA " + release.Version + " (" + release.Channel + ", " + release.Build + ")?\n\nUses the original, SHA-256-checked installer. Keep each version in its own folder; the saved launcher choice stays unchanged."
	if reinstall {
		text += "\n\nReinstall may replace a newer build with this catalog build. Review the official installer's settings-preservation prompts."
	}
	if release.Experimental {
		text += "\n\n" + multiplayer.MTABetaWarning
	}
	w.confirm("Install MTA "+release.Version, text, func(ok bool) {
		if ok {
			w.installMultiplayer(environment, runner, multiplayer.ClientMTA, release.Version)
		}
	})
}
func (w *Window) chooseMTAVersion(environment multiplayer.Environment, version string) {
	if err := w.mtaManagerGuard(environment); err != nil {
		w.error(err)
		return
	}
	text := "Use installed MTA " + version + " when choosing MTA in the Steam launcher? Only the version is saved; server, nickname and password are not stored. Other installations and client settings are retained."
	if version == "1.7" {
		text += "\n\n" + multiplayer.MTABetaWarning
	}
	w.confirm("Use MTA "+version, text, func(ok bool) {
		if !ok {
			return
		}
		err := multiplayer.SetMTAChoice(context.Background(), environment.GameDir, version, environment, func() error { return w.mtaManagerGuard(environment) })
		if err != nil {
			w.error(err)
			return
		}
		w.multiplayer()
	})
}
func (w *Window) removeMTAVersion(environment multiplayer.Environment, runner multiplayer.Runner, version string) {
	if err := w.mtaManagerGuard(environment); err != nil {
		w.error(err)
		return
	}
	w.confirm("Uninstall MTA "+version, "Open only this version's official uninstaller? Other MTA versions, GTA and prepared prefix fonts are retained. Do not choose removal of shared user settings unless you explicitly want that. If this is the active version, the MTA launcher card becomes unavailable until you select another.", func(ok bool) {
		if !ok {
			return
		}
		if err := w.mtaManagerGuard(environment); err != nil {
			w.error(err)
			return
		}
		w.run("Uninstalling MTA "+version, func(ctx context.Context) error {
			if err := platform.RequireGameClosed(); err != nil {
				return err
			}
			return multiplayer.UninstallMTA(ctx, version, environment, runner)
		}, func(error) { w.scan() })
	})
}
