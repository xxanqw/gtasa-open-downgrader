package uiwidget

import (
	_ "embed"
	"fyne.io/fyne/v2"
)

// Hack is covered by internal/ui/fonts/LICENSE and the embedded notices.
//
//go:embed fonts/Hack-Regular.ttf
var hackRegular []byte

//go:embed fonts/Hack-Bold.ttf
var hackBold []byte

//go:embed fonts/Hack-Italic.ttf
var hackItalic []byte

//go:embed fonts/Hack-BoldItalic.ttf
var hackBoldItalic []byte

var (
	hackRegularFont    = fyne.NewStaticResource("Hack-Regular.ttf", hackRegular)
	hackBoldFont       = fyne.NewStaticResource("Hack-Bold.ttf", hackBold)
	hackItalicFont     = fyne.NewStaticResource("Hack-Italic.ttf", hackItalic)
	hackBoldItalicFont = fyne.NewStaticResource("Hack-BoldItalic.ttf", hackBoldItalic)
)

func (t CompactTheme) Font(style fyne.TextStyle) fyne.Resource {
	if style.Symbol || !style.Monospace {
		return t.Theme.Font(style)
	}
	switch {
	case style.Bold && style.Italic:
		return hackBoldItalicFont
	case style.Bold:
		return hackBoldFont
	case style.Italic:
		return hackItalicFont
	default:
		return hackRegularFont
	}
}
