package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/xxanqw/gtasa-open-downgrader/internal/assets"
	appwidget "github.com/xxanqw/gtasa-open-downgrader/internal/uiwidget"
	"github.com/xxanqw/gtasa-open-downgrader/internal/updater"
)

const aboutCredits = `This project is an open-source replication of the original RockstarNexus Downgrader.
Special thanks to the previous **RockstarNexus** developers for creating the initial application.

Developer: **Ivan Potiienko (xxanqw)**

[https://github.com/xxanqw](https://github.com/xxanqw)

Thanks to all of the **GTA SA community** for their continuous support and work!

This application is licensed under the **MIT License**.

Adapted icons: [Font Awesome Free 6.7.2](https://fontawesome.com) by Fonticons, Inc. ([CC BY 4.0](https://creativecommons.org/licenses/by/4.0/)).`

const (
	appLicenseName         = "App License (MIT)"
	xdeltaLicenseName      = "xdelta3 License"
	fontAwesomeLicenseName = "Font Awesome License"
)

func (w *Window) showAbout() {
	w.prompt(promptRequest{present: func(done func(string, bool)) func() {
		body, licenses := aboutContent()
		copyLicense := appwidget.NewButtonWithIcon("Copy license", theme.ContentCopyIcon(), func() {
			w.app.Clipboard().SetContent(aboutLicense(licenses.Selected))
		})
		return w.panel("About", body, fyne.NewSize(640, 540), "", "Close", copyLicense, func(bool) { done("", false) })
	}})
}

func aboutContent() (fyne.CanvasObject, *widget.Select) {
	icon := canvas.NewImageFromResource(fyne.NewStaticResource("icon.png", assets.Icon))
	icon.FillMode = canvas.ImageFillContain
	icon.SetMinSize(fyne.NewSquareSize(56))

	title := widget.NewLabelWithStyle("GTA SA Open Downgrader", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	title.SizeName = theme.SizeNameHeadingText
	version := widget.NewLabel("Version " + updater.Version)
	version.Importance = widget.LowImportance
	subtitle := widget.NewLabel("Open-source game downgrader and mod installer")
	subtitle.Importance = widget.LowImportance
	headerText := container.NewVBox(title, container.NewHBox(version, layout.NewSpacer()), subtitle)
	header := container.NewBorder(nil, nil, icon, nil, headerText)

	projectHeading := widget.NewLabelWithStyle("Project and credits", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	credits := widget.NewRichTextFromMarkdown(aboutCredits)
	credits.Wrapping = fyne.TextWrapWord
	project := container.NewVBox(header, widget.NewSeparator(), projectHeading, credits)

	licenseHeading := widget.NewLabelWithStyle("Licenses", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	licenseText := promptText(assets.AppLicense)
	licenseScroll := container.NewVScroll(licenseText)
	licenses := widget.NewSelect([]string{appLicenseName, xdeltaLicenseName, fontAwesomeLicenseName}, func(name string) {
		licenseText.SetText(aboutLicense(name))
		licenseScroll.ScrollToTop()
	})
	licenses.PlaceHolder = "Choose a license"
	licenses.SetSelected(appLicenseName)
	licenseHeader := container.NewBorder(nil, nil, licenseHeading, nil, licenses)
	licenseSection := container.NewBorder(container.NewVBox(widget.NewSeparator(), licenseHeader), nil, nil, nil, licenseScroll)

	return container.NewBorder(project, nil, nil, nil, licenseSection), licenses
}

func aboutLicense(name string) string {
	switch name {
	case xdeltaLicenseName:
		return assets.XdeltaLicense
	case fontAwesomeLicenseName:
		return assets.FontAwesomeLicense
	default:
		return assets.AppLicense
	}
}
