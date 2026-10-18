package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"math"
)

// Rebalance visible columns on resize instead of requiring horizontal scrolling
// at normal window sizes. Table still pools only the visible cells.
type serverTableLayout struct {
	table  *widget.Table
	widths [4]float32
}

func (*serverTableLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(340, 160) }
func (l *serverTableLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) == 0 {
		return
	}
	width := max(float32(300), size.Width-3*l.table.Theme().Size(theme.SizeNamePadding)-l.table.Theme().Size(theme.SizeNameScrollBar))
	players := float32(80)
	mode := width * 0.22
	language := width * 0.18
	widths := [4]float32{width - players - mode - language, players, mode, language}
	for column, value := range widths {
		if math.Abs(float64(l.widths[column]-value)) > 0.5 {
			l.widths[column] = value
			l.table.SetColumnWidth(column, value)
		}
	}
	objects[0].Move(fyne.NewPos(0, 0))
	objects[0].Resize(size)
}
