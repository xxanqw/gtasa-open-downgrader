package ui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/xxanqw/gtasa-open-downgrader/internal/assets"
	"github.com/xxanqw/gtasa-open-downgrader/internal/game"
	"github.com/xxanqw/gtasa-open-downgrader/internal/updater"
)

// Keep the user's scaling and theme colours, with denser desktop controls.
type compactTheme struct{ fyne.Theme }

func (t compactTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNameText:
		return 13
	case theme.SizeNamePadding:
		return 3
	case theme.SizeNameInnerPadding:
		return 5
	case theme.SizeNameInlineIcon:
		return 16
	}
	return t.Theme.Size(name)
}

func compactLabel(text string) *widget.Label {
	l := widget.NewLabel(text)
	l.Truncation = fyne.TextTruncateEllipsis
	return l
}

// The file grid uses tighter typography and padding than the surrounding controls.
// Leave Fyne's scale setting intact for high-DPI displays.
type fileTableTheme struct{ fyne.Theme }

func (t fileTableTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNameText:
		return 11
	case theme.SizeNameInnerPadding:
		return 2
	}
	return t.Theme.Size(name)
}

var fileColumns = [5]string{"File", "Patch", "Status", "Current MD5", "Target MD5"}

func fileValues(row game.Row) [5]string {
	return [5]string{row.Path, row.NeedsPatch, row.Status, row.CurrentHash, row.TargetHash}
}

// Keep all five columns in the viewport. Long values elide like the original
// Qt table; selecting a cell allows copying its complete value.
type fileTableLayout struct {
	table *widget.Table
	width float32
}

func (l *fileTableLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(360, 150) }
func (l *fileTableLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if l.width != size.Width {
		l.width = size.Width
		usable := max(float32(0), size.Width-24)
		for col, fraction := range []float32{0.25, 0.07, 0.24, 0.22, 0.22} {
			l.table.SetColumnWidth(col, usable*fraction)
		}
	}
	l.table.Move(fyne.NewPos(0, 0))
	l.table.Resize(size)
}

func (w *Window) refreshSummary() {
	if w.summary == nil {
		return
	}
	if len(w.rows) == 0 {
		w.summary.SetText("Scan a game folder to check its files.")
		return
	}
	done, missing, modified, need, failed := 0, 0, 0, 0, 0
	for _, r := range w.rows {
		switch {
		case strings.HasPrefix(r.Status, "Already") || r.Status == "Success":
			done++
		case r.Status == "Missing":
			missing++
		case r.Status == "Modified":
			modified++
		case strings.HasPrefix(r.Status, "Failed") || strings.HasPrefix(r.Status, "Error"):
			failed++
		default:
			need++
		}
	}
	parts := []string{fmt.Sprintf("%d files", len(w.rows))}
	for _, v := range []struct {
		n     int
		label string
	}{{need, "need downgrade"}, {done, "ready"}, {missing, "missing"}, {modified, "modified"}, {failed, "failed"}} {
		if v.n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", v.n, v.label))
		}
	}
	w.summary.SetText(strings.Join(parts, "  ·  "))
}
func (w *Window) refreshModSelection() {
	if w.tabs == nil {
		return
	}
	n := len(w.selected())
	title := "Mods"
	action := "Downgrade"
	button := "Choose mods"
	if n > 0 {
		title = fmt.Sprintf("Mods (%d)", n)
		action = fmt.Sprintf("Downgrade + %d mods", n)
		button = fmt.Sprintf("%d mods selected", n)
	}
	w.tabs.Items[1].Text = title
	w.tabs.Refresh()
	w.downgrade.SetText(action)
	w.modsButton.SetText(button)
}

func (w *Window) showAbout() {
	license := widget.NewMultiLineEntry()
	license.SetText(assets.XdeltaLicense)
	license.Wrapping = fyne.TextWrapWord
	header := container.NewVBox(widget.NewLabel("GTA SA Open Downgrader · "+updater.Version), widget.NewLabel("MIT licensed · Bundled xdelta3: Apache License 2.0"), widget.NewHyperlink("Source and releases", mustURL("https://github.com/xxanqw/gtasa-open-downgrader")))
	d := dialog.NewCustom("About", "Close", container.NewBorder(header, nil, nil, nil, license), w.window)
	size := w.window.Canvas().Size()
	d.Resize(fyne.NewSize(min(float32(540), size.Width-40), min(float32(400), size.Height-40)))
	d.Show()
}
