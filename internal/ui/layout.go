package ui

import (
	"fmt"
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/xxanqw/gtasa-open-downgrader/internal/game"
)

// Preserve the user's light/dark preference and scaling with a neutral palette.
type compactTheme struct{ fyne.Theme }

func (t compactTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	dark := variant != theme.VariantLight
	neutral := func(light, darkValue uint8) color.Color {
		value := light
		if dark {
			value = darkValue
		}
		return color.NRGBA{R: value, G: value, B: value, A: 255}
	}
	switch name {
	case theme.ColorNamePrimary, theme.ColorNameHyperlink:
		return neutral(36, 224)
	case theme.ColorNameForegroundOnPrimary, theme.ColorNameForegroundOnError, theme.ColorNameForegroundOnSuccess, theme.ColorNameForegroundOnWarning:
		return neutral(255, 24)
	case theme.ColorNameError, theme.ColorNameWarning, theme.ColorNameSuccess:
		return neutral(48, 224)
	case theme.ColorNameForeground:
		return neutral(32, 232)
	case theme.ColorNameDisabled, theme.ColorNamePlaceHolder:
		return neutral(116, 136)
	case theme.ColorNameSelection:
		return neutral(215, 64)
	case theme.ColorNameFocus:
		if dark {
			return color.NRGBA{R: 255, G: 255, B: 255, A: 64}
		}
		return color.NRGBA{A: 40}
	}
	// Neutralize every remaining theme role, including translucent overlays.
	c := color.NRGBAModel.Convert(t.Theme.Color(name, variant)).(color.NRGBA)
	value := uint8((uint32(c.R)*299 + uint32(c.G)*587 + uint32(c.B)*114) / 1000)
	return color.NRGBA{R: value, G: value, B: value, A: c.A}
}

func (t compactTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNameText:
		return 13
	case theme.SizeNamePadding:
		return 3
	case theme.SizeNameInnerPadding:
		return 5
	case theme.SizeNameInlineIcon:
		return 12
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
// Qt table, with the grid used only to display scan and patch results.
type fileTableLayout struct {
	table *readOnlyTable
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

// Match the original compact mod grid while retaining readable version details.
type modGridTheme struct{ fyne.Theme }

func (t modGridTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNameText:
		return 11
	case theme.SizeNameInnerPadding:
		return 1
	case theme.SizeNameInlineIcon:
		return 14
	}
	return t.Theme.Size(name)
}

func (w *Window) refreshModSelection() {
	if w.modsTitle == nil {
		return
	}
	n := len(w.selected())
	action := "Downgrade game"
	if n > 0 {
		action = fmt.Sprintf("Downgrade game + %d mods", n)
	}
	w.downgrade.SetText(action)
}

// Keep status text out of the progress lane, even with long paths or at narrow
// window sizes. A hidden progress bar leaves the footer's geometry unchanged.
type statusLayout struct{}

func (statusLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(360, max(objects[0].MinSize().Height, objects[2].MinSize().Height))
}
func (statusLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	status, progress, actions := objects[0], objects[1], objects[2]
	right := actions.MinSize().Width
	available := max(float32(0), size.Width-right-12)
	left := min(size.Width*0.25, available*0.45)
	status.Move(fyne.NewPos(0, 0))
	status.Resize(fyne.NewSize(left, size.Height))
	progress.Move(fyne.NewPos(left+10, (size.Height-3)/2))
	progress.Resize(fyne.NewSize(max(float32(0), available-left-10), 3))
	actions.Move(fyne.NewPos(size.Width-right, (size.Height-actions.MinSize().Height)/2))
	actions.Resize(actions.MinSize())
}

func modCheckSummary(updates, unknown, failed int) string {
	if updates+unknown+failed == 0 {
		return "All tracked mods are up to date."
	}
	var parts []string
	for _, item := range []struct {
		count int
		label string
	}{
		{updates, "update"}, {unknown, "unknown version"}, {failed, "unavailable check"},
	} {
		if item.count > 0 {
			suffix := ""
			if item.count != 1 {
				suffix = "s"
			}
			parts = append(parts, fmt.Sprintf("%d %s%s", item.count, item.label, suffix))
		}
	}
	return strings.Join(parts, " · ") + "."
}
