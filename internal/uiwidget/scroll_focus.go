package uiwidget

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
)

// RevealInScroll keeps a keyboard-focused descendant inside its known scroll
// owner. Owners bind this explicitly, avoiding guesses about widget ancestry.
func RevealInScroll(scroll *container.Scroll, object fyne.CanvasObject) {
	if scroll == nil || object == nil || !object.Visible() || scroll.Size().Height <= 0 {
		return
	}
	driver := fyne.CurrentApp().Driver()
	top := driver.AbsolutePositionForObject(object).Y - driver.AbsolutePositionForObject(scroll).Y
	offset := scroll.Offset.Y
	if top < 6 {
		offset += top - 6
	} else if bottom := top + object.Size().Height + 6; bottom > scroll.Size().Height {
		offset += bottom - scroll.Size().Height
	}
	if offset != scroll.Offset.Y {
		scroll.ScrollToOffset(fyne.NewPos(scroll.Offset.X, offset))
	}
}
