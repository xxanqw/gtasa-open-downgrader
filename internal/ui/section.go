package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
)

// Separate the mod controls with spacing only; no card fill or border.
func newModPanel(content fyne.CanvasObject) *fyne.Container {
	return container.New(layout.NewCustomPaddedLayout(2, 6, 4, 4), content)
}

func sectionTint(th fyne.Theme, variant fyne.ThemeVariant, weight uint16) color.Color {
	base := color.NRGBAModel.Convert(th.Color(theme.ColorNameBackground, variant)).(color.NRGBA)
	foreground := color.NRGBAModel.Convert(th.Color(theme.ColorNameForeground, variant)).(color.NRGBA)
	mix := func(bg, fg uint8) uint8 { return uint8((uint16(bg)*(100-weight) + uint16(fg)*weight) / 100) }
	return color.NRGBA{R: mix(base.R, foreground.R), G: mix(base.G, foreground.G), B: mix(base.B, foreground.B), A: 255}
}

// Distinguish the secondary mod actions from the window background without
// competing with the main Downgrade action's high-importance treatment.
type modActionTheme struct{ fyne.Theme }

func (t modActionTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameButton:
		return sectionTint(t.Theme, variant, 18)
	case theme.ColorNameDisabledButton:
		return sectionTint(t.Theme, variant, 7)
	}
	return t.Theme.Color(name, variant)
}
