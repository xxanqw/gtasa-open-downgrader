package uiwidget

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// CompactTheme is the shared dark-only palette for both executables.
// Fonts, icons and sizing remain independent of OS/Fyne appearance preferences.
type CompactTheme struct{ fyne.Theme }

func (t CompactTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	neutral := func(value uint8) color.Color {
		return color.NRGBA{R: value, G: value, B: value, A: 255}
	}
	switch name {
	case theme.ColorNamePrimary:
		return neutral(54)
	case theme.ColorNameHyperlink:
		return color.NRGBA{R: 133, G: 208, B: 250, A: 255}
	case theme.ColorNameForegroundOnPrimary:
		return neutral(245)
	case theme.ColorNameForegroundOnError, theme.ColorNameForegroundOnSuccess, theme.ColorNameForegroundOnWarning:
		return neutral(20)
	case theme.ColorNameError:
		return color.NRGBA{R: 255, G: 132, B: 132, A: 255}
	case theme.ColorNameWarning:
		return color.NRGBA{R: 239, G: 190, B: 113, A: 255}
	case theme.ColorNameSuccess:
		return color.NRGBA{R: 142, G: 207, B: 152, A: 255}
	case theme.ColorNameForeground:
		return neutral(245)
	case theme.ColorNameDisabled:
		return DisabledControlColor(false)
	case theme.ColorNamePlaceHolder:
		return neutral(174)
	case theme.ColorNameBackground:
		return neutral(26)
	case theme.ColorNameInputBackground:
		return neutral(33)
	case theme.ColorNameButton:
		return neutral(75)
	case theme.ColorNameDisabledButton:
		return DisabledControlColor(true)
	case theme.ColorNameHeaderBackground:
		return neutral(37)
	case theme.ColorNameMenuBackground, theme.ColorNameOverlayBackground:
		return neutral(36)
	case theme.ColorNameInputBorder, theme.ColorNameInnerWindowBorder, theme.ColorNameInnerWindowBorderInactive:
		return neutral(100)
	case theme.ColorNameSeparator:
		return neutral(91)
	case theme.ColorNameScrollBar:
		return neutral(120)
	case theme.ColorNameScrollBarBackground:
		return neutral(36)
	case theme.ColorNameSelection:
		return neutral(64)
	case theme.ColorNameFocus:
		return neutral(62)
	case theme.ColorNameHover:
		return neutral(90)
	case theme.ColorNamePressed:
		return color.Transparent
	case theme.ColorNameShadow:
		return color.NRGBA{A: 40}
	}
	// Fallback roles must also be dark, even when a caller supplies a light base.
	return theme.DefaultTheme().Color(name, theme.VariantDark)
}
func (t CompactTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNameText:
		return 14
	case theme.SizeNamePadding, theme.SizeNameInnerPadding:
		return 6
	case theme.SizeNameInlineIcon:
		return 16
	case theme.SizeNameButtonRadius, theme.SizeNameInputRadius, theme.SizeNameCardRadius, theme.SizeNameDialogRadius, theme.SizeNameSelectionRadius, theme.SizeNameScrollBarRadius, theme.SizeNameModalBlurRadius:
		return 0
	case theme.SizeNameInputBorder:
		return 1
	}
	return t.Theme.Size(name)
}
func DisabledControlColor(background bool) color.Color {
	value := uint8(145)
	if background {
		value = 40
	}
	return color.NRGBA{R: value, G: value, B: value, A: 255}
}
