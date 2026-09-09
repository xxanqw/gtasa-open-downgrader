package ui

import (
	"math"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

func TestMonochromeThemePreservesTextContrast(t *testing.T) {
	th := compactTheme{theme.DefaultTheme()}
	for _, variant := range []fyne.ThemeVariant{theme.VariantDark, theme.VariantLight, fyne.ThemeVariant(2)} {
		for _, pair := range [][2]fyne.ThemeColorName{
			{theme.ColorNameForeground, theme.ColorNameBackground},
			{theme.ColorNameForeground, theme.ColorNameSelection},
			{theme.ColorNameForegroundOnPrimary, theme.ColorNamePrimary},
			{theme.ColorNameSuccess, theme.ColorNameBackground},
			{theme.ColorNameWarning, theme.ColorNameBackground},
			{theme.ColorNameError, theme.ColorNameBackground},
		} {
			luminance := func(name fyne.ThemeColorName) float64 {
				r, g, b, _ := th.Color(name, variant).RGBA()
				if r != g || g != b {
					t.Errorf("colored theme role %s", name)
				}
				value := float64(r) / 65535
				if value <= 0.04045 {
					return value / 12.92
				}
				return math.Pow((value+0.055)/1.055, 2.4)
			}
			a, b := luminance(pair[0]), luminance(pair[1])
			contrast := (max(a, b) + 0.05) / (min(a, b) + 0.05)
			if contrast < 4.5 {
				t.Errorf("variant %v: %v contrast is %.2f", variant, pair, contrast)
			}
		}
	}
}

func TestMonochromeThemeSeparatesInterfaceSurfaces(t *testing.T) {
	themeValue := compactTheme{theme.DefaultTheme()}
	for _, variant := range []fyne.ThemeVariant{theme.VariantDark, theme.VariantLight} {
		shade := func(name fyne.ThemeColorName) uint32 {
			red, green, blue, _ := themeValue.Color(name, variant).RGBA()
			if red != green || green != blue {
				t.Fatalf("variant %v role %s is not monochrome", variant, name)
			}
			return red
		}
		background := shade(theme.ColorNameBackground)
		for _, role := range []fyne.ThemeColorName{theme.ColorNameInputBackground, theme.ColorNameButton, theme.ColorNameHeaderBackground, theme.ColorNameSeparator} {
			if got := shade(role); got == background {
				t.Errorf("variant %v role %s blends into the background", variant, role)
			}
		}
	}
}
