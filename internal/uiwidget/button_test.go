package uiwidget

import (
	"testing"

	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
)

func TestButtonUsesPointerCursorOnlyWhileEnabled(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	button := NewButton("Continue", func() {})
	if got := button.Cursor(); got != desktop.PointerCursor {
		t.Fatalf("enabled cursor = %v", got)
	}
	button.Disable()
	if got := button.Cursor(); got != desktop.DefaultCursor {
		t.Fatalf("disabled cursor = %v", got)
	}
}
