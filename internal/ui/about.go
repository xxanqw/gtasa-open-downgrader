package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/xxanqw/gtasa-open-downgrader/internal/assets"
	"github.com/xxanqw/gtasa-open-downgrader/internal/updater"
)

const aboutCredits = `This project is an open-source replication of the original RockstarNexus Downgrader.
Special thanks to the previous **RockstarNexus** developers for creating the initial application.

Developer: **Ivan Potiienko (xxanqw)**

[https://github.com/xxanqw](https://github.com/xxanqw)

Thanks to all of the **GTA SA community** for their continuous support and work!

This application is licensed under the **MIT License**.

Icons: [Font Awesome Free 6.7.2](https://fontawesome.com) by Fonticons, Inc. ([CC BY 4.0](https://creativecommons.org/licenses/by/4.0/)).`

func (w *Window) showAbout() {
	w.prompt(promptRequest{present: func(done func(string, bool)) func() {
		body, licenses := aboutContent()
		copyLicense := widget.NewButtonWithIcon("Copy license", theme.ContentCopyIcon(), func() {
			text := assets.AppLicense
			if licenses.SelectedIndex() == 1 {
				text = assets.XdeltaLicense
			} else if licenses.SelectedIndex() == 2 {
				text = assets.FontAwesomeLicense
			}
			w.app.Clipboard().SetContent(text)
		})
		return w.panel("About GTA SA Open Downgrader", body, fyne.NewSize(580, 520), "", "OK", copyLicense, func(bool) { done("", false) })
	}})
}
func aboutContent() (fyne.CanvasObject, *container.AppTabs) {
	title := widget.NewLabelWithStyle("GTA SA Open Downgrader", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	version := widget.NewLabel(updater.Version)
	credits := widget.NewRichTextFromMarkdown(aboutCredits)
	credits.Wrapping = fyne.TextWrapWord
	summary := container.NewVBox(container.NewBorder(nil, nil, nil, version, title), credits)
	licenses := container.NewAppTabs(
		container.NewTabItem("App License (MIT)", container.NewVScroll(promptText(assets.AppLicense))),
		container.NewTabItem("xdelta3 License", container.NewVScroll(promptText(assets.XdeltaLicense))),
		container.NewTabItem("Font Awesome", container.NewVScroll(promptText(assets.FontAwesomeLicense))),
	)
	body := container.NewVSplit(container.NewVScroll(summary), licenses)
	body.SetOffset(0.44)
	return body, licenses
}
