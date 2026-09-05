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

// Table column widths follow the viewport; long paths and statuses can be read
// in Details instead of forcing the primary information off the right edge.
type fileTableLayout struct {
	table *widget.Table
	width float32
}

func (l *fileTableLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(360, 150) }
func (l *fileTableLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if l.width != size.Width {
		l.width = size.Width
		usable := max(float32(0), size.Width-24)
		l.table.SetColumnWidth(0, usable*0.57)
		l.table.SetColumnWidth(1, usable*0.43)
	}
	l.table.Move(fyne.NewPos(0, 0))
	l.table.Resize(size)
}

func fileStatus(row game.Row) string {
	switch row.Status {
	case "Original (Needs Patch)":
		return "Needs downgrade"
	case "Already Downgraded":
		return "Downgraded"
	case "Already Downgraded (LAA)":
		return "Downgraded · 4GB enabled"
	case "Modified":
		return "Modified · verification required"
	default:
		return row.Status
	}
}
func fileDetails(row game.Row) string {
	return fmt.Sprintf("File: %s\nStatus: %s\nNeeds patch: %s\n\nCurrent MD5: %s\nTarget MD5: %s", row.Path, row.Status, row.NeedsPatch, row.CurrentHash, row.TargetHash)
}
func (w *Window) showFileDetails() {
	if w.selectedRow < 0 || w.selectedRow >= len(w.rows) {
		return
	}
	text := fileDetails(w.rows[w.selectedRow])
	entry := widget.NewLabel(text)
	entry.Selectable = true
	entry.Wrapping = fyne.TextWrapBreak
	d := dialog.NewCustom("File details", "Close", container.NewBorder(nil, widget.NewButtonWithIcon("Copy details", theme.ContentCopyIcon(), func() { w.window.Clipboard().SetContent(text) }), nil, nil, container.NewVScroll(entry)), w.window)
	size := w.window.Canvas().Size()
	d.Resize(fyne.NewSize(min(float32(560), size.Width-40), min(float32(320), size.Height-60)))
	d.Show()
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
