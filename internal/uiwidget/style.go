package uiwidget

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	"image/color"
)

// InputTheme is scoped so input focus does not recolor ordinary action buttons.
type InputTheme struct{ fyne.Theme }

func (t InputTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNamePrimary:
		return color.NRGBA{R: 225, G: 225, B: 225, A: 255}
	case theme.ColorNameFocus:
		return color.NRGBA{R: 82, G: 82, B: 82, A: 255}
	}
	return t.Theme.Color(name, theme.VariantDark)
}

// StylePanel is shared by downgrader groups and the launcher's actionable cards.
func StylePanel(rect *canvas.Rectangle, th fyne.Theme) {
	rect.FillColor = th.Color(theme.ColorNameHeaderBackground, theme.VariantDark)
	rect.StrokeColor = th.Color(theme.ColorNameInputBorder, theme.VariantDark)
	rect.StrokeWidth = 1
	rect.CornerRadius = 0
}
