package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"image/color"
)

// Text-only selection yellow for the dark interface.
// Keep the rest of the monochrome interface and generic warning palette intact.
type modStateTheme struct{ fyne.Theme }

func (t modStateTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if name == theme.ColorNameWarning {
		return color.NRGBA{R: 245, G: 213, B: 87, A: 255}
	}
	return t.Theme.Color(name, theme.VariantDark)
}
