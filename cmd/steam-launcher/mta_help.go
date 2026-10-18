package main

import (
	"net/url"

	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/xxanqw/gtasa-open-downgrader/internal/multiplayer"
	appwidget "github.com/xxanqw/gtasa-open-downgrader/internal/uiwidget"
)

func (ui *launcherWindow) showMTAHelp() {
	if ui.busy || appwidget.ModalOpen(ui.window) {
		return
	}
	link := func(text, address string) *appwidget.ModalLink {
		u, _ := url.Parse(address)
		return appwidget.NewModalLink(text, u)
	}
	var guide *appwidget.Guide
	open := appwidget.NewButton("Open Downgrader", func() {
		guide.Modal.Close("")
		if err := ui.actions.downgrader(); err != nil {
			appwidget.ShowError(ui.window, err)
		}
	})
	if !ui.actions.hasDowngrader {
		open.Disable()
	}
	setup := container.NewVBox(appwidget.GuideBullets("Install / Repair MTA from Multiplayer in the downgrader.", "Missing Tahoma regular/bold and Verdana fonts are prepared automatically from verified original SourceForge packages.", "Keep MTA and GTA in the same existing Steam Proton prefix, but separate installation folders.", "Keep the prefix's current architecture; launch MTA through this Steam launcher."), link("Official MTA Linux guide", multiplayer.MTALinuxGuideURL))
	serial := container.NewVBox(appwidget.GuideNotice("‘MTA was unable to validate your serial’", "This is separate from missing fonts or Proton HTTP issues. In the native Linux downgrader, open Multiplayer > MTA > Manage… > Serial validation…", widget.WarningImportance), appwidget.GuideBullets("The optional recovery downloads a SHA-256-verified official native server.", "A local authorization dialog approves only its bounded helper workflow, never the launcher UI.", "It temporarily uses normal networking and UDP 22003 / 22126, with a random password and no resources or HTTP. Your address can be advertised to MTA.", "Wait for confirmed master-server contact, retry this same Steam / Proton client, then stop the helper.", "No prefix reset, identifier changes or stored credentials."), open, link("Official serial-validation instructions", multiplayer.MTASerialGuideURL))
	downloads := container.NewVBox(appwidget.GuideNotice("Best-effort compatibility", "Master-server success is not proof of client validation, and no universal server or anti-cheat acceptance is guaranteed. Avoid indiscriminate DLL/CEF changes or old serial workarounds.", widget.WarningImportance))
	if warning := multiplayer.MTAProtonWarning(multiplayer.RunningProtonTool()); warning != "" {
		downloads.Add(appwidget.GuideNotice("Proton advisory", warning, widget.WarningImportance))
	} else {
		downloads.Add(appwidget.ModalText("For HTTP problems during longer downloads, check your active Proton build and network. GE-Proton versions below 11-8 have a specific advisory, not an installation or launch block."))
	}
	guide = appwidget.ShowGuide(ui.window, "MTA on Linux / Proton", []appwidget.GuideStep{{Title: "Setup & fonts", Summary: "Use the prepared Steam environment.", Body: setup}, {Title: "Serial validation", Summary: "Only for the specific serial-validation error.", Body: serial}, {Title: "Download & server issues", Summary: "Keep these issues separate when troubleshooting.", Body: downloads}})
}
