package main

import (
	"bytes"
	"errors"
	"io"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"github.com/xxanqw/gtasa-open-downgrader/internal/licensereader"
	legal "github.com/xxanqw/gtasa-open-downgrader/internal/licenses"
	appwidget "github.com/xxanqw/gtasa-open-downgrader/internal/uiwidget"
)

func launcherLicenseContent(w fyne.Window) fyne.CanvasObject {
	reader := licensereader.New(legal.Text)
	copy := appwidget.NewButtonWithIcon("Copy all licenses", theme.ContentCopyIcon(), func() {
		fyne.CurrentApp().Clipboard().SetContent(legal.Text)
	})
	source := appwidget.NewButton("Save MPL source", func() {
		picker := dialog.NewFileSave(func(file fyne.URIWriteCloser, err error) {
			if err != nil {
				appwidget.ShowError(w, err)
				return
			}
			if file == nil {
				return
			}
			_, writeErr := io.Copy(file, bytes.NewReader(legal.MPLSource))
			if err := errors.Join(writeErr, file.Close()); err != nil {
				appwidget.ShowError(w, err)
			}
		}, w)
		picker.SetTitleText("Save MPL-covered source")
		picker.SetFileName(legal.MPLSourceName)
		picker.Show()
	})
	return container.NewBorder(container.NewHBox(copy, source), nil, nil, nil, reader)
}

func showLauncherLicenses(w fyne.Window) {
	appwidget.ShowModal(w, appwidget.ModalOptions{Title: "Open-source licenses", Subtitle: "Selectable notices | full-text copy | MPL source export", Body: launcherLicenseContent(w), Size: fyne.NewSize(640, 460)})
}
