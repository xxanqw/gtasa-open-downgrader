package ui

import (
	"image"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Exercise the actual SVG renderer: repeated arc commands previously turned the
// ellipsis into broken crescents and drew a stripe across the search icon's hole.
func TestFontAwesomeRoundIconsRenderCleanly(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	a.Settings().SetTheme(compactTheme{theme.DarkTheme()})
	capture := func(resource fyne.Resource) image.Image {
		w := a.NewWindow("Icon")
		defer w.Close()
		w.SetPadded(false)
		w.SetContent(widget.NewIcon(resource))
		w.Resize(fyne.NewSize(64, 64))
		w.Show()
		return w.Canvas().Capture()
	}
	ink := func(img image.Image, x, y int) bool { r, _, _, _ := img.At(x, y).RGBA(); return r > 32768 }
	dots := capture(theme.MoreHorizontalIcon())
	runs := 0
	previous := false
	for x := 0; x < 64; x++ {
		current := ink(dots, x, 32)
		if current && !previous {
			runs++
		}
		previous = current
	}
	if runs != 3 {
		t.Errorf("ellipsis has %d dots across its center, want 3", runs)
	}
	search := capture(theme.SearchIcon())
	for y := 20; y < 32; y++ {
		for x := 20; x < 32; x++ {
			if ink(search, x, y) {
				t.Fatalf("search icon's clear center contains a stray stroke at %d,%d", x, y)
			}
		}
	}
}
