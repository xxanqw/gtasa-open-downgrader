package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Keep the expanded controls on a distinct surface without changing their theme
// or stealing additional space when the section is collapsed.
func newModPanel(content fyne.CanvasObject) *fyne.Container {
	background := &sectionBackground{}
	background.ExtendBaseWidget(background)
	inset := container.New(layout.NewCustomPaddedLayout(7, 7, 9, 9), content)
	return container.New(layout.NewCustomPaddedLayout(0, 6, 0, 0), container.NewStack(background, inset))
}

type sectionBackground struct {
	widget.BaseWidget
	rectangle *canvas.Rectangle
}

func (b *sectionBackground) CreateRenderer() fyne.WidgetRenderer {
	b.rectangle = canvas.NewRectangle(color.Transparent)
	b.rectangle.CornerRadius = 6
	b.rectangle.StrokeWidth = 1
	b.applyColors()
	return widget.NewSimpleRenderer(b.rectangle)
}

func (b *sectionBackground) Refresh() {
	if b.rectangle != nil {
		b.applyColors()
	}
	b.BaseWidget.Refresh()
}

func (b *sectionBackground) applyColors() {
	th := b.Theme()
	variant := fyne.CurrentApp().Settings().ThemeVariant()
	base := color.NRGBAModel.Convert(th.Color(theme.ColorNameBackground, variant)).(color.NRGBA)
	foreground := color.NRGBAModel.Convert(th.Color(theme.ColorNameForeground, variant)).(color.NRGBA)
	tint := func(weight uint16) color.Color {
		mix := func(bg, fg uint8) uint8 { return uint8((uint16(bg)*(100-weight) + uint16(fg)*weight) / 100) }
		return color.NRGBA{R: mix(base.R, foreground.R), G: mix(base.G, foreground.G), B: mix(base.B, foreground.B), A: 255}
	}
	b.rectangle.FillColor = tint(7)
	b.rectangle.StrokeColor = tint(15)
}
