package ui

import (
	"context"
	"fmt"
	"image/color"
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
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
		var versions []mtaVersionOption
		selectedVersion := active.Version
		if !active.Installed && len(installed) > 0 {
			selectedVersion = installed[0].Version
		}
		selected := 0
		var modal *appwidget.Modal
		action := func(f func()) { after = f; modal.Close("") }
		button := func(text, label string, importance widget.Importance, disabled bool, run func()) appwidget.ModalAction {
			return appwidget.ModalAction{
				Text: text, AccessibleLabel: label, Importance: importance, Disabled: disabled, KeepOpen: true,
				Run: func(*appwidget.Modal) { action(run) },
			}
		}
		addVersion := func(version string, release *multiplayer.MTARelease) {
			client, present := multiplayer.FindMTAInstallation(installed, version)
			isActive := active.Installed && active.Version == version
			name := mtaVersionName(version)
			channel := "Existing installation"
			if release != nil {
				channel = release.Channel
				if release.Experimental {
					channel = "Development"
				}
			}
			if version == selectedVersion {
				selected = len(versions)
			}

			available := []appwidget.ModalAction{{Text: "Close"}}
			cannotRun := runner == nil || !canChange
			if !present && release != nil {
				available = append(available, button("Install "+version+"…", "Install "+name, widget.HighImportance, cannotRun, func() {
					w.confirmMTAInstall(environment, runner, *release, false)
				}))
			}
			if present {
				if release != nil {
					available = append(available, button("Reinstall…", "Reinstall "+name, widget.LowImportance, cannotRun, func() {
						w.confirmMTAInstall(environment, runner, *release, true)
					}))
				}
				available = append(available, button("Uninstall…", "Uninstall "+name, widget.LowImportance, cannotRun, func() {
					w.removeMTAVersion(environment, runner, version)
				}))
				if !isActive {
					available = append(available, button("Use in Steam…", "Use "+name+" in the Steam launcher", widget.HighImportance, !canChange, func() {
						w.chooseMTAVersion(environment, version)
					}))
				}
			}
			versions = append(versions, mtaVersionOption{
				name: name, channel: channel,
				detail:  mtaVersionDetails(name, client, isActive, release),
				actions: available,
			})
		}
		for _, release := range multiplayer.MTAReleases {
			addVersion(release.Version, &release)
		}
		for _, version := range []string{"1.5", "custom"} {
			if _, ok := multiplayer.FindMTAInstallation(installed, version); ok {
				addVersion(version, nil)
			}
		}
		picker := mtaVersionPicker(versions)
		picker.Select(selected)
		detail := container.NewStack(versions[selected].detail)
		// Reserve the list's full height when the modal measures its body.
		reserved := canvas.NewRectangle(color.Transparent)
		padding := w.app.Settings().Theme().Size(theme.SizeNamePadding)
		listSize := picker.MinSize()
		reserved.SetMinSize(fyne.NewSize(max(156, listSize.Width), float32(len(versions))*(listSize.Height+padding)-padding))
		rail := newFramedPanel(container.NewStack(reserved, picker))
		body := container.NewBorder(nil, nil, rail, nil, detail)
		var recovery fyne.CanvasObject
		if runtime.GOOS == "linux" && runtime.GOARCH == "amd64" {
			if active.Installed && active.Version == "1.6" {
				serial := appwidget.NewButton("Serial validation…", func() { action(func() { w.showMTAValidation(environment) }) })
				serial.Importance = widget.LowImportance
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
		modal = appwidget.ShowModal(w.window, appwidget.ModalOptions{
			Title:    "Manage MTA",
			Summary:  mtaManagerSummary(active, len(installed) > 0, choiceErr),
			Body:     container.NewVScroll(body),
			Size:     fyne.NewSize(630, 0),
			Actions:  versions[selected].actions,
			Extra:    recovery,
			OnClosed: func(string) { done("", false) },
		})
		picker.OnSelected = func(index widget.ListItemID) {
			version := versions[index]
			detail.Objects[0] = version.detail
			detail.Refresh()
			modal.SetActions(version.actions...)
		}
		return func() { modal.Close("") }
	}})
}

type mtaVersionOption struct {
	name, channel string
	detail        fyne.CanvasObject
	actions       []appwidget.ModalAction
}

type mtaVersionList struct{ widget.List }

func (list *mtaVersionList) TypedKey(key *fyne.KeyEvent) {
	if appwidget.ModalEscape(list, key) {
		return
	}
	if key.Name == fyne.KeyReturn || key.Name == fyne.KeyEnter {
		key = &fyne.KeyEvent{Name: fyne.KeySpace}
	}
	list.List.TypedKey(key)
}

func mtaVersionPicker(versions []mtaVersionOption) *mtaVersionList {
	list := &mtaVersionList{}
	list.Length = func() int { return len(versions) }
	list.CreateItem = func() fyne.CanvasObject {
		name := widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		channel := widget.NewLabel("")
		channel.SizeName = theme.SizeNameCaptionText
		return container.New(layout.NewCustomPaddedVBoxLayout(0), name, channel)
	}
	list.UpdateItem = func(index widget.ListItemID, row fyne.CanvasObject) {
		labels := row.(*fyne.Container).Objects
		labels[0].(*widget.Label).SetText(versions[index].name)
		labels[1].(*widget.Label).SetText(versions[index].channel)
	}
	list.ExtendBaseWidget(list)
	return list
}

func mtaVersionName(version string) string {
	if version == "custom" {
		return "Configured MTA"
	}
	return "MTA " + version
}

func mtaManagerSummary(active multiplayer.Client, hasInstallations bool, choiceErr error) fyne.CanvasObject {
	switch {
	case active.Installed && active.Version == "1.7":
		return appwidget.ModalNotice("The Steam launcher uses MTA 1.7, a development build.", widget.WarningImportance)
	case active.Installed:
		return appwidget.ModalNotice("The Steam launcher uses "+mtaVersionName(active.Version)+".", widget.MediumImportance)
	case choiceErr != nil:
		return appwidget.ModalNotice("The Steam launcher’s MTA selection is invalid. Choose an installed version.", widget.WarningImportance)
	case active.Version != "":
		return appwidget.ModalNotice("The selected "+mtaVersionName(active.Version)+" is unavailable or unverified. Choose another installed version.", widget.WarningImportance)
	case hasInstallations:
		return appwidget.ModalNotice("Choose an installed version to enable MTA in the Steam launcher.", widget.MediumImportance)
	default:
		return appwidget.ModalNotice("Install a version to enable MTA in the Steam launcher.", widget.MediumImportance)
	}
}

func mtaVersionDetails(name string, client multiplayer.Client, active bool, release *multiplayer.MTARelease) fyne.CanvasObject {
	title := widget.NewLabelWithStyle(name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	title.SizeName = theme.SizeNameSubHeadingText
	title.Wrapping = fyne.TextWrapWord
	status := widget.NewLabel("Not installed")
	status.Importance = widget.LowImportance
	if active {
		status.SetText("Active in Steam")
		status.Importance = widget.SuccessImportance
	} else if client.Installed {
		status.SetText("Installed")
		status.Importance = widget.MediumImportance
	}
	header := container.NewBorder(nil, nil, nil, container.NewCenter(status), title)
	details := container.New(layout.NewCustomPaddedVBoxLayout(0), header)
	if release != nil && release.Experimental {
		channel := widget.NewLabel(release.Channel)
		channel.Importance = widget.WarningImportance
		details.Add(channel)
	}
	if client.Installed {
		location := widget.NewLabel("Install folder")
		location.Importance = widget.LowImportance
		path := appwidget.ModalText(client.WorkingDir)
		path.Wrapping = fyne.TextWrapBreak
		folder := container.New(layout.NewCustomPaddedVBoxLayout(0), location, path)
		details.Add(container.New(layout.NewCustomPaddedLayout(4, 0, 0, 0), folder))
	}
	return container.New(layout.NewCustomPaddedLayout(0, 6, 12, 4), details)
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
	text := "Install MTA " + release.Version + " (" + release.Channel + ")?\n\nDownloads the current official installer and verifies its publisher signature and signed SHA-256. Keep each version in its own folder."
	if reinstall {
		text += "\n\nReview the official installer's prompts about keeping your settings."
	}
	if release.Experimental {
		text += "\n\n" + multiplayer.MTABetaWarning
	}
	w.confirm("Install MTA "+release.Version, text+"\n\nGame folder:\n"+environment.GameDir, func(ok bool) {
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
	w.confirm("Use MTA "+version, text+"\n\nGame folder:\n"+environment.GameDir, func(ok bool) {
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
	w.confirm("Uninstall MTA "+version, "Open only this version's official uninstaller? Other MTA versions, GTA and prepared prefix fonts are retained. Do not choose removal of shared user settings unless you explicitly want that. If this is the active version, the MTA launcher card becomes unavailable until you select another.\n\nGame folder:\n"+environment.GameDir, func(ok bool) {
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
