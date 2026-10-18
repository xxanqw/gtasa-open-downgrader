package ui

import (
	"context"
	"errors"
	"fmt"

	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/xxanqw/gtasa-open-downgrader/internal/mtavalidation"
	"github.com/xxanqw/gtasa-open-downgrader/internal/multiplayer"
	appwidget "github.com/xxanqw/gtasa-open-downgrader/internal/uiwidget"
)

func (w *Window) showMTAValidation(environment multiplayer.Environment) {
	if !mtavalidation.Supported() {
		w.info("MTA serial validation", "The verified temporary helper is available on Linux x86-64 only.")
		return
	}
	if err := w.requireMTAEnvironment(environment); err != nil {
		w.error(err)
		return
	}
	w.prompt(promptRequest{present: func(done func(string, bool)) func() {
		var g *appwidget.Guide
		var cancel context.CancelFunc
		running, stopping, confirmed, ready := false, false, false, false
		consent := appwidget.NewCheck("Allow this temporary network helper", nil)
		status := appwidget.ModalText("Waiting to start.")
		spinner := widget.NewProgressBarInfinite()
		spinner.Hide()
		before := container.NewVBox(appwidget.GuideNotice("For this specific error only", "Use this when MTA says it was unable to validate your serial. This is not a font repair, an HTTP download fix or an anti-cheat bypass.", widget.WarningImportance), appwidget.GuideBullets("Uses the official native Linux MTA server, with both archives SHA-256 verified.", "Runs only its restricted helper workflow with local pkexec authorization, not the launcher UI.", "Keeps your current Steam prefix, settings and identifiers unchanged.", "Runtime is capped at five minutes, with up to three seconds for forced cleanup. Cancelling or exiting stops it sooner."), appwidget.NewModalLink("Official serial-validation instructions", mustURL(mtavalidation.GuideURL)))
		network := container.NewVBox(appwidget.GuideNotice("Normal networking is required", fmt.Sprintf("Temporarily binds UDP %d and %d on normal interfaces. The password-protected server may be reachable on your LAN or Internet if your existing network allows it. ASE communicates with MTA and can advertise your public address. A loopback-only helper cannot perform the proven master-server query.", mtavalidation.ServerPort, mtavalidation.ASEPort), widget.WarningImportance), appwidget.GuideBullets("Fresh random password, one player, no resources, HTTP, voice or LAN broadcasts.", "No firewall/router changes and no stored credentials.", "Authorization is local; never enter a password into this application.", "The helper's private files are removed after it stops."))
		retry := container.NewVBox(status, spinner, appwidget.GuideSection("After master-server success", appwidget.GuideBullets("Retry MTA through the same Steam / Proton prefix while the helper is still running.", "Do not connect to the temporary server; retry the action that produced the serial error.", "If the client succeeds, choose ‘It works’ to stop the helper.", "Master-server success alone does not prove that the client or every game server will accept your serial.")))
		g = appwidget.ShowGuide(w.window, "MTA serial validation on Linux", []appwidget.GuideStep{{Title: "What this does", Summary: "Optional recovery, never run automatically on launch.", Body: before}, {Title: "Network access & authorization", Summary: "Read this before approving the helper.", Body: network, Footer: container.NewVBox(widget.NewSeparator(), consent)}, {Title: "Run the helper & retry MTA", Summary: "Startup alone is not a successful validation.", Body: retry}})
		g.OnClose = func() {
			spinner.Stop()
			if cancel != nil {
				cancel()
			}
			done("", false)
		}
		requestStop := func() {
			if !running {
				g.Modal.Close("")
				return
			}
			if stopping {
				return
			}
			stopping = true
			status.Importance = widget.MediumImportance
			status.SetText("Stopping the helper and waiting for privileged cleanup… If an authorization dialog is still open, dismiss it.")
			g.Modal.SetActions(appwidget.ModalAction{Text: "Stopping…", Disabled: true, KeepOpen: true})
			if cancel != nil {
				cancel()
			}
		}
		g.BeforeClose = func(*appwidget.Guide) { requestStop() }
		showConsent := func() {
			consent.SetChecked(false)
			g.Show(1)
			g.Actions(appwidget.ModalAction{ID: "start", Text: "Authorize & start", Importance: widget.HighImportance, KeepOpen: true, Disabled: true, Run: func(*appwidget.Modal) {
				if !consent.Checked || running || w.busy || w.closing || w.scanPath != environment.GameDir {
					return
				}
				running, stopping, confirmed, ready = true, false, false, false
				ctx, stop := context.WithCancel(context.Background())
				cancel = stop
				g.Show(2)
				spinner.Show()
				spinner.Start()
				status.Importance = widget.MediumImportance
				status.SetText("Downloading and verifying the official helper…")
				g.Modal.SetActions(appwidget.ModalAction{Text: "Stop helper", KeepOpen: true, Run: func(*appwidget.Modal) { requestStop() }})
				var result mtavalidation.Result
				progress := func(event mtavalidation.Event) {
					w.post(func() {
						if stopping {
							return
						}
						if event.Stage == "error" {
							return
						} // Final result is shown after cleanup.
						status.SetText(event.Message)
						if event.Stage == "ready" {
							ready = true
							spinner.Stop()
							spinner.Hide()
							status.Importance = widget.SuccessImportance
							status.Refresh()
							g.Modal.SetActions(appwidget.ModalAction{Text: "Stop helper", KeepOpen: true, Run: func(*appwidget.Modal) { requestStop() }}, appwidget.ModalAction{Text: "It works", Importance: widget.HighImportance, KeepOpen: true, Run: func(*appwidget.Modal) { confirmed = true; requestStop() }})
						}
					})
				}
				w.runHandled("Temporary MTA serial validation", func(parent context.Context) error {
					stopParent := context.AfterFunc(parent, stop)
					defer stopParent()
					defer stop()
					packages, err := mtavalidation.Prepare(ctx, w.client.HTTP, progress)
					if err != nil {
						return err
					}
					result, err = mtavalidation.Run(ctx, packages, progress)
					return err
				}, func(err error) {
					running = false
					cancel = nil
					spinner.Stop()
					spinner.Hide()
					switch {
					case confirmed && (err == nil || errors.Is(err, context.Canceled)):
						status.Importance = widget.SuccessImportance
						status.SetText("Client success confirmed by you. The temporary helper has stopped. Your prefix and settings were not changed.")
					case err != nil && !errors.Is(err, context.Canceled):
						status.Importance = widget.DangerImportance
						status.SetText("Helper stopped. " + err.Error() + "\n\nNo client success was confirmed. Review the official guide before trying again.")
					case result.MasterReady || ready:
						status.Importance = widget.WarningImportance
						status.SetText("The helper contacted the master server and has now stopped. Client validation was not confirmed; retry through the same Steam prefix if needed.")
					default:
						status.Importance = widget.WarningImportance
						status.SetText("Stopped before successful master-server contact. No client validation was confirmed.")
					}
					status.Refresh()
					g.Modal.SetActions(appwidget.ModalAction{Text: "Close"})
					g.Modal.SetSubtitle("Temporary helper stopped | no automatic retry")
				})
			}})
		}
		consent.OnChanged = func(checked bool) { g.Modal.SetActionEnabled("start", checked) }
		g.OnNext = func(*appwidget.Guide) { showConsent() }
		return func() { requestStop() }
	}})
}
