package uiwidget

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

func (b *codeEditorBody) Cursor() desktop.Cursor { return b.editor.Cursor() }
func (b *codeEditorBody) Tapped(p *fyne.PointEvent) {
	e := b.editor
	if e.Disabled() {
		return
	}
	if e.pointerHandled {
		e.pointerHandled = false
		return
	}
	e.focus()
	e.MoveCursor(e.pointOffset(p.Position), false)
}
func (b *codeEditorBody) DoubleTapped(p *fyne.PointEvent) {
	e := b.editor
	if e.Disabled() {
		return
	}
	e.focus()
	offset := e.pointOffset(p.Position)
	start, end := offset, offset
	if offset < len(e.buffer) && wordRune(e.buffer[offset]) {
		for start > 0 && wordRune(e.buffer[start-1]) {
			start--
		}
		for end < len(e.buffer) && wordRune(e.buffer[end]) {
			end++
		}
	} else if offset < len(e.buffer) {
		end = e.nextBoundary(offset)
	}
	e.SelectRange(start, end)
	e.lastDouble = time.Now()
	e.pointerHandled = false
}
func (b *codeEditorBody) MouseDown(p *desktop.MouseEvent) {
	e := b.editor
	if e.Disabled() || p.Button != desktop.MouseButtonPrimary {
		return
	}
	e.focus()
	e.pointerHandled = true
	e.dragging = true
	offset := e.pointOffset(p.Position)
	if !e.lastDouble.IsZero() && time.Since(e.lastDouble) < 400*time.Millisecond {
		row, _ := e.rowCol(offset)
		end := e.lineEnd(row)
		if end < len(e.buffer) {
			end++
		}
		e.SelectRange(e.starts[row], end)
		e.lastDouble = time.Time{}
		return
	}
	e.MoveCursor(offset, p.Modifier&fyne.KeyModifierShift != 0)
}
func (b *codeEditorBody) MouseUp(p *desktop.MouseEvent) {
	if p.Button == desktop.MouseButtonPrimary {
		b.editor.dragging = false
	}
}
func (b *codeEditorBody) Dragged(p *fyne.DragEvent) {
	e := b.editor
	if e.Disabled() {
		return
	}
	if !e.dragging {
		e.focus()
		e.anchor = e.CursorTextOffset()
		e.dragging = true
	}
	e.setCursor(e.pointOffset(p.Position))
	e.merge = false
	e.keepX = false
	e.moved()
}
func (b *codeEditorBody) DragEnd() { b.editor.dragging = false; b.editor.pointerHandled = false }
func (b *codeEditorBody) TappedSecondary(p *fyne.PointEvent) {
	e := b.editor
	if e.Disabled() {
		return
	}
	e.focus()
	if e.SelectedText() == "" {
		e.MoveCursor(e.pointOffset(p.Position), false)
	}
	c := fyne.CurrentApp().Driver().CanvasForObject(e)
	if c == nil {
		return
	}
	clip := fyne.CurrentApp().Clipboard()
	menu := fyne.NewMenu("", fyne.NewMenuItem("Undo", e.Undo), fyne.NewMenuItem("Redo", e.Redo), fyne.NewMenuItemSeparator(), fyne.NewMenuItem("Cut", func() { e.TypedShortcut(&fyne.ShortcutCut{Clipboard: clip}) }), fyne.NewMenuItem("Copy", func() { e.TypedShortcut(&fyne.ShortcutCopy{Clipboard: clip}) }), fyne.NewMenuItem("Paste", func() { e.TypedShortcut(&fyne.ShortcutPaste{Clipboard: clip}) }), fyne.NewMenuItemSeparator(), fyne.NewMenuItem("Select all", func() { e.SelectRange(0, len(e.buffer)) }))
	widget.NewPopUpMenu(menu, c).ShowAtRelativePosition(p.Position, b)
}
func (g *codeEditorGutter) Tapped(p *fyne.PointEvent) {
	e := g.editor
	if e.Disabled() || e.scroll == nil {
		return
	}
	e.focus()
	position := p.Position
	position.Y += e.scroll.Offset.Y
	position.X = 0
	offset := e.pointOffset(position)
	row, _ := e.rowCol(offset)
	end := e.lineEnd(row)
	if end < len(e.buffer) {
		end++
	}
	e.SelectRange(e.starts[row], end)
}
func (g *codeEditorGutter) Cursor() desktop.Cursor {
	if g.editor.Disabled() {
		return desktop.DefaultCursor
	}
	return desktop.DefaultCursor
}
