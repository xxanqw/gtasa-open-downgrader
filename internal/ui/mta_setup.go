package ui

import (
	"context"
	"fmt"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	appwidget "github.com/xxanqw/gtasa-open-downgrader/internal/uiwidget"

	"github.com/xxanqw/gtasa-open-downgrader/internal/mtafonts"
	"github.com/xxanqw/gtasa-open-downgrader/internal/multiplayer"
	"github.com/xxanqw/gtasa-open-downgrader/internal/platform"
)

// Check UI ownership before starting a worker (which makes the UI busy).
// Workers independently recheck the closed-game guard before prefix writes.
func (w *Window) requireMTAEnvironment(environment multiplayer.Environment) error {
	if !w.modTableReady() || environment.GameDir != w.scanPath {
		return fmt.Errorf("rescan the same writable, downgraded game before preparing MTA")
	}
	return nil
}
func (w *Window) showMTAInstallerGuideFor(environment multiplayer.Environment, runner multiplayer.Runner, start func(), version string) {
	w.prompt(promptRequest{done: func(_ string, accepted bool) {
		if accepted {
			w.ensureMTAFonts(environment, runner, start)
		}
	}, present: func(done func(string, bool)) func() {
		setup := container.NewVBox(appwidget.GuideNotice("Same Steam prefix", "Uses the existing detected Proton prefix without recreating it or changing its architecture.", widget.MediumImportance), appwidget.GuideSection("Automatic fonts", appwidget.ModalText("Missing Tahoma regular/bold and Verdana are prepared automatically from original SourceForge packages. Packages and fonts are SHA-256 verified; existing fonts are not overwritten.")), appwidget.NewModalLink("Microsoft font terms", mustURL(mtafonts.FontTermsURL)))
		if warning := multiplayer.MTAProtonWarning(environment.CompatibilityTool); warning != "" {
			setup.Add(appwidget.GuideNotice("Proton advisory", warning, widget.WarningImportance))
		}
		installer := container.NewVBox(appwidget.GuideSection("GTA folder", appwidget.GuideCode(w.window, environment.WindowsGameDir)), appwidget.GuideBullets("Keep MTA in its normal separate installation folder, not inside the GTA folder.", "Select the GTA folder shown above.", "Uncheck ‘Launch MTA’ when the installer finishes.", "Start MTA through the Steam launcher afterwards."), appwidget.GuideNotice("Best-effort Linux support", "Fonts, older-Proton HTTP problems, serial validation and server anti-cheat acceptance are separate issues. Do not apply old CEF/DLL workarounds indiscriminately.", widget.WarningImportance), appwidget.NewModalLink("Official Linux guide", mustURL(multiplayer.MTALinuxGuideURL)))
		title := "Install MTA with Proton"
		if version != "" {
			title = "Install MTA " + version + " with Proton"
			installer.Add(appwidget.GuideNotice("Selected client: "+version, "Keep MTA San Andreas "+version+" separate from other installed versions. After setup, return to Manage… and choose Use "+version+" if you want it to become the active launcher client. Keep the game-client component enabled; install a dedicated server only if you intend to host.", widget.MediumImportance))
		}
		g := appwidget.ShowGuide(w.window, title, []appwidget.GuideStep{{Title: "Preparation", Summary: "Nothing is installed until you explicitly open setup.", Body: setup}, {Title: "In the official installer", Summary: "Keep these choices when setup opens.", Body: installer}})
		accepted := false
		g.OnClose = func() { done("", accepted) }
		g.OnNext = func(g *appwidget.Guide) {
			g.Show(1)
			g.Actions(appwidget.ModalAction{ID: "install", Text: "Open installer", Importance: widget.HighImportance, KeepOpen: true, Run: func(m *appwidget.Modal) { accepted = true; m.Close("install") }})
		}
		return func() { g.Modal.Cancel() }
	}})
}
func (w *Window) ensureMTAFonts(environment multiplayer.Environment, runner multiplayer.Runner, after func()) {
	if err := w.requireMTAEnvironment(environment); err != nil {
		w.error(err)
		return
	}
	missing, err := multiplayer.MissingMTAFonts(environment)
	if err != nil {
		w.error(err)
		return
	}
	if len(missing) == 0 {
		if after != nil {
			after()
		}
		return
	}
	w.run("Preparing MTA fonts automatically", func(ctx context.Context) error {
		return mtafonts.Ensure(ctx, w.client.HTTP, w.cache, environment, runner, platform.RequireGameClosed, w.report)
	}, func(err error) {
		if err == nil && after != nil {
			after()
		}
	})
}
