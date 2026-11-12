package uiwidget

import (
	"fmt"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

func TestReadOnlyEditorBlocksEditsButAllowsSelectionAndCopy(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	e := NewCodeEditor()
	e.SetText("αβ example\nsecond")
	e.ReadOnly = true
	e.SelectRange(3, 10)
	clip := a.Clipboard()
	e.TypedShortcut(&fyne.ShortcutCopy{Clipboard: clip})
	if clip.Content() != "example" {
		t.Fatal("copy failed", clip.Content())
	}
	for _, mutate := range []func(){func() { e.TypedRune('x') }, func() { e.TypedKey(&fyne.KeyEvent{Name: fyne.KeyDelete}) }, func() { e.TypedShortcut(&fyne.ShortcutCut{Clipboard: clip}) }, func() { e.TypedShortcut(&fyne.ShortcutPaste{Clipboard: clip}) }, e.Undo, e.Redo} {
		mutate()
		if e.Text != "αβ example\nsecond" {
			t.Fatal("read-only text changed", e.Text)
		}
	}
}

func TestLargeLiveDocumentPreservesViewportAndBuildsVisibleRows(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	var text strings.Builder
	for i := 0; i < 6000; i++ {
		fmt.Fprintf(&text, "2026-11-12 [DEBUG] file.write op=%d path=\"/mods/пример-%d.asi\"\n", i, i)
	}
	e := NewCodeEditor()
	e.ReadOnly = true
	e.SetText(text.String())
	w := a.NewWindow("log")
	defer w.Close()
	w.SetContent(e)
	w.Resize(fyne.NewSize(740, 400))
	w.Show()
	e.ScrollToBottom()
	ready := 0
	for _, line := range e.layouts {
		if line.ready {
			ready++
		}
	}
	if ready > 100 {
		t.Fatalf("materialized %d of %d rows", ready, len(e.layouts))
	}
	e.SelectRange(5, 10)
	e.scroll.ScrollToOffset(fyne.NewPos(60, 1000))
	offset, selected := e.scroll.Offset, e.SelectedText()
	oldHeight := e.body.Size().Height
	e.UpdateText(e.Text + "tail\n")
	if e.scroll.Offset != offset || e.SelectedText() != selected || e.CanUndo() {
		t.Fatal("live update reset reader state", e.scroll.Offset, offset)
	}
	if e.body.Size().Height <= oldHeight {
		t.Fatal("scroll geometry did not grow")
	}
	e.ScrollToBottom()
	if e.scroll.Offset.Y < oldHeight-e.scroll.Size().Height {
		t.Fatal("follow did not reach tail")
	}
	// Retention drops a prefix without losing a retained selection.
	e.SetText("old\nkeep αβ\ntail\n")
	e.SelectRange(9, 11)
	e.UpdateText("keep αβ\ntail\nnew\n")
	if e.SelectedText() != "αβ" {
		t.Fatal("retention shifted selection", e.SelectedText())
	}
}
