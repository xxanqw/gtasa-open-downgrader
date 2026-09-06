package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// Keep Table's virtualized rendering and scrollbars, without cell selection,
// hover highlights, column resizing, or a stop in keyboard focus navigation.
type readOnlyTable struct{ widget.Table }

func newReadOnlyTable(length func() (int, int), create func() fyne.CanvasObject, update func(widget.TableCellID, fyne.CanvasObject)) *readOnlyTable {
	t := &readOnlyTable{Table: widget.Table{Length: length, CreateCell: create, UpdateCell: update}}
	t.ExtendBaseWidget(t)
	return t
}
func (*readOnlyTable) Tapped(*fyne.PointEvent)        {}
func (*readOnlyTable) MouseIn(*desktop.MouseEvent)    {}
func (*readOnlyTable) MouseMoved(*desktop.MouseEvent) {}
func (*readOnlyTable) MouseOut()                      {}
func (*readOnlyTable) MouseDown(*desktop.MouseEvent)  {}
func (*readOnlyTable) MouseUp(*desktop.MouseEvent)    {}
func (*readOnlyTable) Dragged(*fyne.DragEvent)        {}
func (*readOnlyTable) DragEnd()                       {}
func (*readOnlyTable) FocusGained()                   {}
func (*readOnlyTable) FocusLost()                     {}
func (*readOnlyTable) TypedKey(*fyne.KeyEvent)        {}
func (*readOnlyTable) TypedRune(rune)                 {}
func (*readOnlyTable) Select(widget.TableCellID)      {}
func (*readOnlyTable) Cursor() desktop.Cursor         { return desktop.DefaultCursor }
func (*readOnlyTable) Disabled() bool                 { return true }
func (*readOnlyTable) Disable()                       {}
func (*readOnlyTable) Enable()                        {}

// A quiet, full-width progress track. The operation text conveys the task;
// the bar shows progress without a second, button-shaped percentage readout.
type thinProgress struct{ widget.ProgressBar }

func newThinProgress() *thinProgress {
	p := &thinProgress{ProgressBar: widget.ProgressBar{Max: 1, TextFormatter: func() string { return "" }}}
	p.ExtendBaseWidget(p)
	return p
}
func (*thinProgress) MinSize() fyne.Size { return fyne.NewSize(1, 3) }
