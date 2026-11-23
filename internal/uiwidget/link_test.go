package uiwidget

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
)

func TestLinksUnderlineOnTextHover(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	app.Settings().SetTheme(CompactTheme{Theme: theme.DefaultTheme()})
	window := app.NewWindow("Links")
	defer window.Close()
	link := NewLink("Licenses", nil)
	modal := NewModalLink("Official source", nil)
	window.SetContent(container.NewVBox(link, modal))
	window.Resize(fyne.NewSize(320, 120))
	canvas := window.Canvas()
	canvas.Capture()

	for _, entry := range []struct {
		widget fyne.CanvasObject
		link   *Link
	}{{link, link}, {modal, &modal.Link}} {
		pos := app.Driver().AbsolutePositionForObject(entry.widget)
		test.MoveMouse(canvas, pos.Add(fyne.NewPos(16, 12)))
		if !entry.link.TextStyle.Underline || entry.link.Cursor() != desktop.PointerCursor {
			t.Fatalf("hover on %q did not underline its text", entry.link.Text)
		}
		if entry.link.Hyperlink.Cursor() != desktop.DefaultCursor {
			t.Fatalf("%q enabled Fyne's separate hover line", entry.link.Text)
		}
		test.MoveMouse(canvas, pos.Add(fyne.NewPos(260, 12)))
		if entry.link.TextStyle.Underline || entry.link.Cursor() != desktop.DefaultCursor {
			t.Fatalf("blank space still hovers %q", entry.link.Text)
		}
		test.MoveMouse(canvas, pos.Add(fyne.NewPos(16, 12)))
		test.MoveMouse(canvas, fyne.NewPos(1, 1))
		if entry.link.TextStyle.Underline {
			t.Fatalf("%q stayed underlined after the pointer left", entry.link.Text)
		}
	}
}

func TestWrappedModalLinkRetainsKeyboardActivation(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	window := app.NewWindow("Modal link")
	defer window.Close()
	link := NewModalLink("Read the official guide for installing this multiplayer client", nil)
	activations := 0
	link.OnTapped = func() { activations++ }
	window.SetContent(link)
	window.Resize(fyne.NewSize(175, 155))
	canvas := window.Canvas()
	canvas.Capture()
	singleLine := NewModalLink("Guide", nil).MinSize().Height
	if link.MinSize().Height <= singleLine {
		t.Fatalf("expected the link to wrap, got minimum height %g", link.MinSize().Height)
	}
	pos := app.Driver().AbsolutePositionForObject(link)
	test.MoveMouse(canvas, pos.Add(fyne.NewPos(16, singleLine+2)))
	if !link.TextStyle.Underline {
		t.Fatal("hover on the wrapped text did not underline it")
	}
	test.TapCanvas(canvas, pos.Add(fyne.NewPos(16, 12)))
	canvas.Focus(link)
	link.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if activations != 2 {
		t.Fatalf("click and Enter activated the link %d times, want 2", activations)
	}
}
