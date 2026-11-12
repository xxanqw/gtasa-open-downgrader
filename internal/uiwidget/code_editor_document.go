package uiwidget

import (
	"strings"

	"fyne.io/fyne/v2"
)

// UpdateText refreshes an external document without resetting selection or scroll.
// It does not create undo history or invoke edit callbacks.
func (e *CodeEditor) UpdateText(text string) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if text == e.Text {
		return
	}
	cursor, anchor := e.CursorTextOffset(), e.anchor
	// Retention may remove complete lines from the beginning of a live log.
	removed := 0
	if len(e.Text) > 0 && !strings.HasPrefix(text, e.Text) {
		first, _, _ := strings.Cut(text, "\n")
		if first != "" {
			if offset := strings.Index(e.Text, first+"\n"); offset > 0 {
				removed = len([]rune(e.Text[:offset]))
			}
		}
	}
	var offset fyne.Position
	if e.scroll != nil {
		offset = e.scroll.Offset
		if removed > 0 {
			offset.Y = max(0, offset.Y-float32(strings.Count(string(e.buffer[:removed]), "\n"))*e.metrics().height)
		}
	}
	e.Text, e.buffer = text, []rune(text)
	e.reindex()
	e.setCursor(max(0, min(cursor-removed, len(e.buffer))))
	e.anchor = max(0, min(anchor-removed, len(e.buffer)))
	e.history, e.redo = nil, nil
	e.historyBytes = 0
	e.revision++
	e.syncScrollGeometry()
	e.refreshInput()
	if e.scroll != nil {
		e.scroll.ScrollToOffset(offset)
	}
}

func (e *CodeEditor) ScrollToBottom() {
	if e.scroll != nil {
		x := e.scroll.Offset.X
		e.scroll.ScrollToBottom()
		e.scroll.ScrollToOffset(fyne.NewPos(x, e.scroll.Offset.Y))
	}
}
