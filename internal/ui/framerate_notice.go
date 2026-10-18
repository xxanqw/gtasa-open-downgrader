package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
)

const framerateGymPR = "https://github.com/GTAmodding/FramerateVigilante/pull/30"
const framerateSwimPR = "https://github.com/GTAmodding/FramerateVigilante/pull/34"
const framerateNotice = `Read before installing upstream Framerate Vigilante.

PR #30 proposes a gym FPS cap and a short exit grace period to address high-FPS gym exit/re-entry loops.

PR #34 corrects a swim-pitch damping patch address associated with swimming crashes on GTA SA 1.0 US.

Both pull requests were open when this notice was added. Review their current status using the links below.

You can build your own .asi from a source tree containing both PR changes, following the upstream build instructions, or wait until the fixes are merged and included in an upstream release before installing. Merging a PR alone does not update an existing release archive.

The downgrader installs the upstream package; it does not apply these PRs or build a custom .asi for you. Proposed fixes and custom builds are not a guarantee of compatibility with your other mods.`

func framerateNoticeBody() fyne.CanvasObject {
	return container.NewVBox(promptText(framerateNotice),
		container.NewHBox(plainLink("PR #30 — Gym exit loop", framerateGymPR)),
		container.NewHBox(plainLink("PR #34 — Swimming crash patch address", framerateSwimPR)))
}

func (w *Window) showFramerateNotice() {
	w.prompt(promptRequest{title: "Framerate Vigilante — Read before installation", text: framerateNotice, present: func(done func(string, bool)) func() {
		return w.panel("Framerate Vigilante — Read before installation", container.NewVScroll(framerateNoticeBody()), fyne.NewSize(560, 0), "", "Close", nil, func(bool) { done("", false) })
	}})
}
