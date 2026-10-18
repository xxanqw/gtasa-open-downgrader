package main

import (
	"context"
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/xxanqw/gtasa-open-downgrader/internal/assets"
	"github.com/xxanqw/gtasa-open-downgrader/internal/launcherprefs"
	"github.com/xxanqw/gtasa-open-downgrader/internal/launcherversion"
	"github.com/xxanqw/gtasa-open-downgrader/internal/multiplayer"
	appwidget "github.com/xxanqw/gtasa-open-downgrader/internal/uiwidget"
)

type launcherActions struct {
	single        func(context.Context) error
	multiplayer   func(context.Context, multiplayer.ClientID) error
	openMP        func(context.Context, multiplayer.OpenMPConnection) error
	downgrader    func() error
	hasDowngrader bool
}
type launcherWindow struct {
	app             fyne.App
	window          fyne.Window
	gameDir         string
	actions         launcherActions
	status          *widget.Label
	buttons         map[string]*appwidget.Button
	available       map[*appwidget.Button]bool
	busy            bool
	closed          bool
	checkingUpdates bool
	updateCancel    context.CancelFunc
	browser         *serverBrowser
	dispatch        func(func())
	cards           []*appwidget.ActionCard
	grid            *fyne.Container
	settingsPopup   *widget.PopUpMenu
	settingsMenu    *fyne.Menu
	details         string
	modeCards       map[launcherprefs.Choice]*appwidget.ActionCard
	modeAvailable   map[launcherprefs.Choice]bool
	activate        map[launcherprefs.Choice]func()
	remember        *appwidget.Check
	remembered      launcherprefs.Choice
	preferenceError error
}

func newLauncherWindow(a fyne.App, w fyne.Window, gameDir, hostOS string, underProton bool, clients []multiplayer.Client, actions launcherActions) *launcherWindow {
	ui := &launcherWindow{app: a, window: w, gameDir: gameDir, actions: actions, buttons: make(map[string]*appwidget.Button), available: make(map[*appwidget.Button]bool), dispatch: fyne.Do}
	ui.modeCards = make(map[launcherprefs.Choice]*appwidget.ActionCard)
	ui.modeAvailable = make(map[launcherprefs.Choice]bool)
	ui.activate = make(map[launcherprefs.Choice]func())
	ui.remembered, ui.preferenceError = launcherprefs.Load(gameDir)
	ui.remember = appwidget.NewCheck("Remember choice", func(checked bool) {
		if !checked && !ui.busy {
			if err := launcherprefs.Reset(gameDir); err != nil {
				ui.remember.SetChecked(true)
				appwidget.ShowError(w, err)
				return
			}
			ui.remembered = launcherprefs.None
		}
	})
	ui.remember.Checked = ui.remembered != launcherprefs.None
	w.Resize(fyne.NewSize(540, 410))
	ui.status = widget.NewLabel("Ready")
	ui.status.Truncation = fyne.TextTruncateEllipsis
	heading := widget.NewLabelWithStyle("San Andreas", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	platform := "Windows | native clients"
	if underProton {
		platform = "Linux / Proton | multiplayer experimental"
	}
	ui.details = fmt.Sprintf("Steam launcher %s\n%s\n\nGame folder:\n%s\n\n", launcherversion.Version(), platform, gameDir)
	mtaWarning := ""
	if underProton || hostOS == "linux" {
		mtaWarning = multiplayer.MTAProtonWarning(multiplayer.RunningProtonTool())
		ui.details += "MTA on Linux: experimental. Uses the same prepared Proton prefix; required fonts are prepared automatically by MTA Install / Repair in the downgrader.\nOfficial guide: " + multiplayer.MTALinuxGuideURL + "\n\n"
		if mtaWarning != "" {
			ui.details += mtaWarning + "\n\n"
		}
	}
	ui.addCard(launcherprefs.SinglePlayer, "Single Player", "Ready", widget.SuccessImportance, launcherArtwork("steam-launcher-icon.png", assets.Icon), true, func() { ui.launch(actions.single) })
	for _, id := range []multiplayer.ClientID{multiplayer.ClientSAMP, multiplayer.ClientOpenMP, multiplayer.ClientMTA} {
		client, _ := multiplayer.Find(clients, id)
		client.ID = id
		client.Name = map[multiplayer.ClientID]string{multiplayer.ClientSAMP: "SA-MP", multiplayer.ClientOpenMP: "open.mp", multiplayer.ClientMTA: "MTA"}[id]
		supported, reason := multiplayer.LaunchSupport(client.ID, hostOS, underProton)
		state := "Installed"
		importance := widget.SuccessImportance
		description := "Installed"
		if client.ID == multiplayer.ClientMTA && client.Installed && client.Version != "" {
			state = "Installed | " + client.Version
			description = "Active version: " + client.Version
			if client.Version == "1.7" {
				state = "1.7 development"
				importance = widget.WarningImportance
				description = "Active: 1.7 development / untested"
			}
		}
		if client.ID == multiplayer.ClientOpenMP && underProton {
			description = "Browse servers or connect directly. Widescreen Fix pauses; other mods stay enabled (experimental)."
		}
		if !supported {
			state = "Unavailable"
			importance = widget.WarningImportance
			description = reason
		} else if !client.Installed {
			state = "Not installed"
			importance = widget.DangerImportance
			description = "Install this client from Multiplayer in the downgrader."
			if client.ID == multiplayer.ClientMTA && client.Detail != "" {
				state = "Choose version"
				importance = widget.WarningImportance
				description = client.Detail
			}
		}
		ui.addCard(launcherprefs.Choice(client.ID), client.Name, state, importance, clientArtwork(client.ID), supported && client.Installed, func() {
			if client.ID == multiplayer.ClientOpenMP && underProton {
				ui.showServers()
				return
			}
			ui.launch(func(ctx context.Context) error { return actions.multiplayer(ctx, client.ID) })
		})
		ui.details += fmt.Sprintf("%s: %s\n", client.Name, description)
	}
	ui.button("downgrader", "Open Downgrader", actions.hasDowngrader, func() {
		if err := actions.downgrader(); err != nil {
			appwidget.ShowError(w, err)
		}
	})
	ui.details += "\nInstall or repair clients and manage mods in the downgrader.\n\nWidescreen Fix pauses for SA-MP/open.mp and restores after GTA exits. open.mp keeps other mods enabled (experimental).\n\nThe launcher waits for GTA before restoring multiplayer files. API browsing is optional; direct connection works offline."
	if underProton || hostOS == "linux" {
		ui.button("mta-help", "MTA on Linux…", true, ui.showMTAHelp)
	}
	ui.button("details", "Details", true, func() {
		appwidget.ShowMessage(w, "Launcher details", ui.details)
	})
	ui.button("licenses", "Licenses", true, func() { showLauncherLicenses(w) })
	ui.button("updates", "Check for launcher updates", true, ui.checkUpdates)
	settings := ui.button("settings", "Settings", true, ui.showSettings)
	settings.SetIcon(theme.SettingsIcon())
	top := container.NewHBox(heading, layout.NewSpacer(), settings)
	var footer fyne.CanvasObject = container.NewBorder(nil, nil, nil, ui.remember, ui.status)
	if mtaWarning != "" {
		warning := widget.NewLabel(mtaWarning)
		warning.Wrapping = fyne.TextWrapWord
		warning.Importance = widget.WarningImportance
		footer = container.NewVBox(warning, footer)
	}
	objects := make([]fyne.CanvasObject, len(ui.cards))
	for i, card := range ui.cards {
		objects[i] = card
	}
	ui.grid = container.NewGridWithColumns(2, objects...)
	w.SetContent(container.NewPadded(container.NewBorder(top, footer, nil, nil, ui.grid)))
	// Reserve the advisory's natural height before showing the native window;
	// do not let minimum-size growth clip the initial GL viewport.
	if mtaWarning != "" {
		w.Resize(fyne.NewSize(540, max(410, w.Content().MinSize().Height+2*theme.Padding())))
	}
	w.SetCloseIntercept(ui.requestClose)
	w.SetOnClosed(func() {
		ui.closed = true
		if ui.updateCancel != nil {
			ui.updateCancel()
		}
		if ui.browser != nil {
			ui.browser.close()
		}
	})
	w.CenterOnScreen()
	return ui
}
func (ui *launcherWindow) addCard(choice launcherprefs.Choice, name, state string, importance widget.Importance, icon fyne.CanvasObject, available bool, action func()) {
	card := launcherCard(name, state, importance, icon, func() { ui.choose(choice) })
	ui.cards = append(ui.cards, card)
	ui.modeCards[choice] = card
	ui.modeAvailable[choice] = available
	ui.activate[choice] = action
	if !available {
		card.Disable()
	}
}

func (ui *launcherWindow) choose(choice launcherprefs.Choice) {
	if ui.busy || appwidget.ModalOpen(ui.window) || !ui.modeAvailable[choice] {
		return
	}
	if ui.remember.Checked {
		if err := launcherprefs.Save(context.Background(), ui.gameDir, choice); err != nil {
			appwidget.ShowError(ui.window, err)
			return
		}
		ui.remembered = choice
	}
	ui.activate[choice]()
}

// Remember only the mode. open.mp on Proton still opens its server browser:
// never automatically replay a server address, nickname or password.
func (ui *launcherWindow) startRemembered() {
	if ui.preferenceError != nil {
		ui.status.SetText("Saved choice invalid | choose a mode")
		ui.details += "\n\nRemembered choice: " + ui.preferenceError.Error()
		return
	}
	choice := ui.remembered
	if choice == launcherprefs.None {
		return
	}
	if !ui.modeAvailable[choice] {
		ui.status.SetText("Saved choice unavailable | choose a mode")
		return
	}
	ui.activate[choice]()
}

func (ui *launcherWindow) showSettings() {
	if ui.busy || appwidget.ModalOpen(ui.window) {
		return
	}
	if ui.settingsPopup != nil {
		ui.settingsPopup.Hide()
	}
	items := []*fyne.MenuItem{}
	for _, key := range []string{"downgrader", "updates", "mta-help", "details", "licenses"} {
		button := ui.buttons[key]
		if button == nil {
			continue
		}
		item := fyne.NewMenuItem(button.Text, button.OnTapped)
		item.Disabled = !ui.available[button]
		items = append(items, item)
	}
	ui.settingsMenu = fyne.NewMenu("Settings", items...)
	ui.settingsPopup = widget.NewPopUpMenu(ui.settingsMenu, ui.window.Canvas())
	button := ui.buttons["settings"]
	pos := ui.app.Driver().AbsolutePositionForObject(button)
	pos.X += button.Size().Width - ui.settingsPopup.MinSize().Width
	pos.Y += button.Size().Height
	ui.settingsPopup.ShowAtPosition(pos)
}

func (ui *launcherWindow) requestClose() {
	if ui.busy {
		ui.status.SetText("Game active | waiting for GTA and safe restoration")
		return
	}
	if ui.browser != nil {
		ui.browser.close()
	}
	appwidget.DismissModals(ui.window)
	ui.window.SetCloseIntercept(nil)
	ui.window.Close()
}

func (ui *launcherWindow) button(key, text string, available bool, tap func()) *appwidget.Button {
	button := appwidget.NewButton(text, func() {
		if ui.busy || appwidget.ModalOpen(ui.window) || !ui.available[ui.buttons[key]] {
			return
		}
		tap()
	})
	ui.buttons[key] = button
	ui.available[button] = available
	if !available {
		button.Disable()
	}
	return button
}
func (ui *launcherWindow) setBusy(busy bool) {
	ui.busy = busy
	if busy && ui.updateCancel != nil {
		ui.updateCancel()
	}
	if ui.settingsPopup != nil {
		ui.settingsPopup.Hide()
	}
	if busy {
		ui.remember.Disable()
	} else {
		ui.remember.Enable()
	}
	for choice, card := range ui.modeCards {
		if busy || !ui.modeAvailable[choice] {
			card.Disable()
		} else {
			card.Enable()
		}
	}
	for button, available := range ui.available {
		if busy || !available {
			button.Disable()
		} else {
			button.Enable()
		}
	}
}
func (ui *launcherWindow) launch(run func(context.Context) error) {
	if ui.busy || appwidget.ModalOpen(ui.window) {
		return
	}
	if ui.browser != nil {
		ui.browser.close()
	}
	ui.setBusy(true)
	ui.status.SetText("Starting | waiting for GTA and safe restoration")
	go func() {
		err := run(context.Background())
		ui.dispatch(func() {
			ui.setBusy(false)
			if err != nil {
				ui.status.SetText("Launch failed | see details")
				appwidget.ShowError(ui.window, err)
				return
			}
			ui.window.Close()
		})
	}()
}
func (ui *launcherWindow) showServers() {
	if ui.busy || appwidget.ModalOpen(ui.window) {
		return
	}
	if ui.browser != nil && !ui.browser.closed {
		return
	}
	ui.browser = newServerBrowser(ui)
	ui.browser.show()
}
func launcherArtwork(name string, data []byte) *canvas.Image {
	image := canvas.NewImageFromResource(fyne.NewStaticResource(name, data))
	image.FillMode = canvas.ImageFillContain
	image.ScaleMode = canvas.ImageScaleSmooth
	image.SetMinSize(fyne.NewSquareSize(56))
	return image
}
func clientArtwork(id multiplayer.ClientID) *canvas.Image {
	switch id {
	case multiplayer.ClientSAMP:
		return launcherArtwork("samp.png", assets.SAMPClientIcon)
	case multiplayer.ClientOpenMP:
		return launcherArtwork("openmp.png", assets.OpenMPClientIcon)
	case multiplayer.ClientMTA:
		return launcherArtwork("mta.png", assets.MTAClientIcon)
	}
	return launcherArtwork("steam-launcher-icon.png", assets.Icon)
}
func launcherCard(name, state string, importance widget.Importance, icon fyne.CanvasObject, action func()) *appwidget.ActionCard {
	title := widget.NewLabelWithStyle(name, fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	label := widget.NewLabel(state)
	label.Importance = importance
	symbol := theme.ConfirmIcon()
	if importance == widget.DangerImportance {
		symbol = theme.CancelIcon()
	} else if importance == widget.WarningImportance {
		symbol = theme.WarningIcon()
	}
	status := container.NewHBox(widget.NewIcon(symbol), label)
	body := container.NewCenter(container.NewVBox(container.NewCenter(icon), title))
	content := container.NewPadded(container.NewBorder(nil, container.NewCenter(status), nil, nil, body))
	card := appwidget.NewActionCard(name+" | "+state, content, action)
	card.Title = title
	return card
}

// Plain labels, never interpreted as HTML, executable links, or markup.
func detailLabel(text string) *widget.Label {
	label := widget.NewLabel(strings.TrimSpace(text))
	label.Selectable = true
	label.Wrapping = fyne.TextWrapWord
	return label
}

// Inputs use a scoped high-contrast focus color without recoloring action buttons.
type launcherInputTheme = appwidget.InputTheme

func launcherInput(object fyne.CanvasObject) fyne.CanvasObject {
	return container.NewThemeOverride(object, launcherInputTheme{Theme: fyne.CurrentApp().Settings().Theme()})
}
