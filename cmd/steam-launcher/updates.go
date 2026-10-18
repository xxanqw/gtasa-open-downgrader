package main

import (
	"context"
	"errors"
	"net/url"
	"time"

	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/xxanqw/gtasa-open-downgrader/internal/download"
	"github.com/xxanqw/gtasa-open-downgrader/internal/launcherupdate"
	"github.com/xxanqw/gtasa-open-downgrader/internal/launcherversion"
	"github.com/xxanqw/gtasa-open-downgrader/internal/service"
	appwidget "github.com/xxanqw/gtasa-open-downgrader/internal/uiwidget"
)

func (ui *launcherWindow) checkUpdates() {
	if ui.busy || ui.closed || ui.checkingUpdates {
		return
	}
	ui.checkingUpdates = true
	ui.status.SetText("Checking launcher updates…")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	ui.updateCancel = cancel
	go func() {
		release, err := launcherupdate.Check(ctx, download.New().HTTP)
		ui.dispatch(func() {
			cancel()
			ui.updateCancel = nil
			ui.checkingUpdates = false
			if ui.closed || ui.busy || errors.Is(err, context.Canceled) {
				return
			}
			ui.status.SetText("Ready")
			if err != nil {
				appwidget.ShowError(ui.window, err)
				return
			}
			if !launcherupdate.Newer(release.Version, launcherversion.Version()) {
				appwidget.ShowMessage(ui.window, "Steam launcher", "No newer launcher is available. Installed: "+launcherversion.Version()+". Latest published: "+release.Version+".")
				return
			}
			body := container.NewVBox(appwidget.ModalText("Steam launcher " + release.Version + " is available. Installed: " + launcherversion.Version() + ".\n\nClose this launcher, then use Multiplayer → Steam launcher → Check updates in the downgrader. It verifies the download and preserves your original Steam entrypoint and remembered choice."))
			actions := []appwidget.ModalAction{{Text: "Close"}}
			if ui.actions.hasDowngrader {
				actions = append(actions, appwidget.ModalAction{ID: "downgrader", Text: "Open Downgrader", Importance: widget.HighImportance, Run: func(*appwidget.Modal) {
					if err := ui.actions.downgrader(); err != nil {
						appwidget.ShowError(ui.window, err)
						return
					}
					ui.requestClose()
				}})
			} else {
				link, _ := url.Parse(service.BaseURL)
				body.Add(appwidget.NewModalLink("Download the downgrader", link))
			}
			appwidget.ShowModal(ui.window, appwidget.ModalOptions{Title: "Launcher update available", Body: body, Actions: actions})
		})
	}()
}
