package ui

import (
	_ "embed"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
)

//go:embed modicons/preset.png
var modIconPreset []byte

var modIconBase = fyne.NewStaticResource("mod-preset.png", modIconPreset)

func newClassicIcon(resource fyne.Resource) *fyne.Container {
	base := canvas.NewImageFromResource(modIconBase)
	base.FillMode = canvas.ImageFillContain
	base.ScaleMode = canvas.ImageScaleSmooth
	base.SetMinSize(fyne.NewSquareSize(56))
	glyph := canvas.NewImageFromResource(resource)
	glyph.FillMode = canvas.ImageFillContain
	glyph.SetMinSize(fyne.NewSquareSize(32))
	return container.NewStack(base, container.New(layout.NewCustomPaddedLayout(12, 12, 12, 12), glyph))
}
