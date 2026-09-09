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
// Explicit surface shades keep controls, collection headers and separators
// distinct instead of flattening the interface into one grey plane.
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
		return neutral(32, 238)
	case theme.ColorNameForegroundOnPrimary, theme.ColorNameForegroundOnError, theme.ColorNameForegroundOnSuccess, theme.ColorNameForegroundOnWarning:
		return neutral(255, 18)
	case theme.ColorNameError, theme.ColorNameWarning, theme.ColorNameSuccess:
		return neutral(45, 220)
	case theme.ColorNameForeground:
		return neutral(24, 242)
	case theme.ColorNameDisabled, theme.ColorNamePlaceHolder:
		return neutral(104, 154)
	case theme.ColorNameBackground:
		return neutral(244, 18)
	case theme.ColorNameInputBackground:
		return neutral(255, 30)
	case theme.ColorNameButton:
		return neutral(218, 48)
	case theme.ColorNameDisabledButton:
		return neutral(234, 30)
	case theme.ColorNameHeaderBackground:
		return neutral(222, 40)
	case theme.ColorNameMenuBackground, theme.ColorNameOverlayBackground:
		return neutral(250, 28)
	case theme.ColorNameInputBorder, theme.ColorNameSeparator, theme.ColorNameInnerWindowBorder, theme.ColorNameInnerWindowBorderInactive:
		return neutral(170, 68)
	case theme.ColorNameScrollBar:
		return neutral(96, 160)
	case theme.ColorNameScrollBarBackground:
		return neutral(230, 35)
	case theme.ColorNameSelection:
		return neutral(200, 76)
	case theme.ColorNameFocus:
		if dark {
			return color.NRGBA{R: 255, G: 255, B: 255, A: 112}
		}
		return color.NRGBA{A: 72}
	case theme.ColorNameHover:
		if dark {
			return color.NRGBA{R: 255, G: 255, B: 255, A: 36}
		}
		return color.NRGBA{A: 28}
	case theme.ColorNamePressed:
		if dark {
			return color.NRGBA{R: 255, G: 255, B: 255, A: 72}
		}
		return color.NRGBA{A: 52}
	case theme.ColorNameShadow:
		return color.NRGBA{A: 104}
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

// Keep the footer glanceable while the journal retains the complete message.
// Paths are reduced to their final component because the full source path is
// available from the log button beside the status bar.
func compactStatusText(text string) string {
	if strings.HasPrefix(text, "Scan complete: ") {
		var files int
		if _, err := fmt.Sscanf(text, "Scan complete: %d manifest files checked.", &files); err == nil {
			return fmt.Sprintf("Scan complete · %d files", files)
		}
	}
	if strings.HasPrefix(text, "Cancelled.") {
		return "Cancelled · backups preserved"
	}
	if strings.HasPrefix(text, "Cancelling;") {
		return "Cancelling…"
	}
	if parts := strings.SplitN(text, " · ", 2); len(parts) == 2 && strings.ContainsAny(parts[0], `/\`) {
		path := strings.TrimRight(parts[0], `/\`)
		if index := strings.LastIndexAny(path, `/\`); index >= 0 {
			path = path[index+1:]
		}
		text = path + " · " + parts[1]
	}
	const limit = 30
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit-1]) + "…"
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
