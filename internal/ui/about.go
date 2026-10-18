package ui

import (
	"net/url"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/xxanqw/gtasa-open-downgrader/internal/assets"
	"github.com/xxanqw/gtasa-open-downgrader/internal/licensereader"
	legal "github.com/xxanqw/gtasa-open-downgrader/internal/licenses"
	appwidget "github.com/xxanqw/gtasa-open-downgrader/internal/uiwidget"
	"github.com/xxanqw/gtasa-open-downgrader/internal/updater"
)

const aboutCreditsIntro = `This project is an open-source replication of the original RockstarNexus Downgrader.
Special thanks to the previous **RockstarNexus** developers for creating the initial application.

Developer: **Ivan Potiienko (xxanqw)**`

const aboutCreditsDetails = `Thanks to all of the **GTA SA community** for their continuous support and work!

Original application code is licensed under the **MIT License**. Third-party components retain their own terms.

Fyne UI toolkit: BSD 3-Clause. Complete dependency notices and MPL source availability are under Licenses.

Adapted icons: Font Awesome Free 6.7.2 by Fonticons, Inc. (CC BY 4.0).

Interface font: Noto Sans by Google (SIL Open Font License), bundled with Fyne.
Technical text: Hack by the Source Foundry Authors (MIT / Bitstream Vera licenses).`

func (w *Window) buildAboutPage() fyne.CanvasObject {
	splash := canvas.NewImageFromResource(fyne.NewStaticResource("splash.png", assets.Splash))
	splash.FillMode = canvas.ImageFillContain
	splash.ScaleMode = canvas.ImageScaleSmooth
	splash.SetMinSize(fyne.NewSize(256, 144))

	licenses := &aboutHyperlink{}
	licenses.Text = "Licenses"
	licenses.TextStyle.Underline = true
	licenses.OnTapped = w.showAboutLicenses
	licenses.ExtendBaseWidget(licenses)
	links := container.NewVBox(splash,
		aboutLink("GitHub repository", "https://github.com/xxanqw/gtasa-open-downgrader"),
		aboutLink("Developer on GitHub", "https://github.com/xxanqw"),
		container.NewHBox(licenses),
		widget.NewSeparator(),
		widget.NewLabelWithStyle("RESOURCES", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		aboutLink("Font Awesome Free 6.7.2", "https://fontawesome.com"),
		aboutLink("CC BY 4.0", "https://creativecommons.org/licenses/by/4.0/"),
		aboutLink("Noto Sans", "https://fonts.google.com/noto/specimen/Noto+Sans"),
		aboutLink("Hack font", "https://sourcefoundry.org/hack/"))
	left := container.NewVScroll(container.New(layout.NewCustomPaddedLayout(0, 12, 0, 0), links))

	title := widget.NewLabelWithStyle("GTA SA Open Downgrader", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	title.SizeName = theme.SizeNameHeadingText
	title.Wrapping = fyne.TextWrapWord
	version := widget.NewLabel("Version " + updater.Version)
	version.Importance = widget.LowImportance
	intro := widget.NewRichTextFromMarkdown(aboutCreditsIntro)
	intro.Wrapping = fyne.TextWrapWord
	details := widget.NewRichTextFromMarkdown(aboutCreditsDetails)
	details.Wrapping = fyne.TextWrapWord
	description := container.NewVBox(title, version,
		hubText("Open-source game downgrader and mod installer"),
		widget.NewSeparator(), intro, details)
	right := container.NewVScroll(container.New(layout.NewCustomPaddedLayout(0, 12, 0, 0), description))
	return container.New(layout.NewCustomPaddedLayout(16, 12, 14, 14),
		container.New(aboutColumnsLayout{}, left, right))
}

// The artwork and links stay on the left; only the description column grows.
// Both columns scroll independently on compact windows without clipping links.
type aboutColumnsLayout struct{}

func (aboutColumnsLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	left := min(float32(280), size.Width*0.44)
	gap := float32(24)
	objects[0].Move(fyne.NewPos(0, 0))
	objects[0].Resize(fyne.NewSize(left, size.Height))
	objects[1].Move(fyne.NewPos(left+gap, 0))
	objects[1].Resize(fyne.NewSize(max(0, size.Width-left-gap), size.Height))
}
func (aboutColumnsLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(620, 240) }

func (w *Window) showAboutLicenses() {
	w.prompt(promptRequest{title: "Open-source licenses", present: func(done func(string, bool)) func() {
		reader := licensereader.New(aboutLicenses())
		copy := appwidget.NewButtonWithIcon("Copy all licenses", theme.ContentCopyIcon(), func() {
			w.app.Clipboard().SetContent(aboutLicenses())
		})
		var close func()
		source := appwidget.NewButton("Save MPL source", func() {
			close()
			w.chooseDestination("Save MPL-covered source", legal.MPLSourceName, []string{".zip"}, func(path string) {
				if err := os.WriteFile(path, legal.MPLSource, 0644); err != nil {
					w.error(err)
				}
			})
		})
		body := container.NewBorder(container.NewHBox(copy, source), nil, nil, nil, reader)
		close = w.panel("Open-source licenses", body, fyne.NewSize(660, 520), "", "Close", nil, func(bool) { done("", false) })
		return close
	}})
}

func aboutLicenses() string { return legal.Text }

// Fyne's standard hyperlink draws a second underline on hover. Keep its
// accessible tap and keyboard handling, but use only the static text underline.
type aboutHyperlink struct{ widget.Hyperlink }

func plainLink(text, address string) *aboutHyperlink {
	parsed, _ := url.Parse(address) // fixed, valid application links
	link := &aboutHyperlink{}
	link.Text, link.URL = text, parsed
	link.ExtendBaseWidget(link)
	return link
}

func aboutLink(text, address string) fyne.CanvasObject {
	link := plainLink(text, address)
	link.TextStyle.Underline = true
	return container.NewHBox(link)
}

func (*aboutHyperlink) MouseIn(*desktop.MouseEvent)    {} // no hover-only underline
func (*aboutHyperlink) MouseMoved(*desktop.MouseEvent) {} // or on pointer movement
func (*aboutHyperlink) Cursor() desktop.Cursor         { return desktop.PointerCursor }
