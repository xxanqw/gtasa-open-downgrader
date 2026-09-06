package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/xxanqw/gtasa-open-downgrader/internal/game"
)

func TestFileTableIgnoresPointerAndKeyboardSelection(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := New(a)
	defer w.jobs.Close()
	w.rows = []game.Row{{Path: "gta-sa.exe", Status: "Missing"}}
	w.table.Refresh()
	w.window.Show()
	changed := false
	w.table.OnSelected = func(widget.TableCellID) { changed = true }
	w.table.OnHighlighted = func(widget.TableCellID) { changed = true }
	w.table.MouseIn(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(20, 40)}})
	w.table.MouseMoved(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(40, 40)}})
	test.Tap(w.table)
	w.table.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(40, 40)}})
	w.table.TypedKey(&fyne.KeyEvent{Name: fyne.KeyDown})
	w.table.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	w.table.Select(widget.TableCellID{Row: 1, Col: 0})
	w.window.Canvas().Focus(w.table)
	if changed || w.window.Canvas().Focused() == w.table {
		t.Fatal("display-only table accepted selection or keyboard focus")
	}
	// Newly created rows retain the compact typography after refresh/scrolling.
	w.table.ScrollToBottom()
	cell := w.table.CreateCell().(*container.ThemeOverride)
	if cell.MinSize().Height > 22 {
		t.Fatalf("file rows grew beyond compact height: %v", cell.MinSize())
	}
}
