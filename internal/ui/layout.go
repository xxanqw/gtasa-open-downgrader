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
		return t.Theme.Size(theme.SizeNameText)
	case theme.SizeNameInnerPadding:
		return 2
	}
	return t.Theme.Size(name)
}

var fileColumns = [5]string{"File", "Patch", "Status", "Current MD5", "Target MD5"}

func fileValues(row game.Row) [5]string {
	return [5]string{row.Path, row.NeedsPatch, row.Status, row.CurrentHash, row.TargetHash}
}

// Resize File/Status columns, or all five columns when checksums are enabled.
type fileTableLayout struct {
	table        *readOnlyTable
	width        float32
	detailed     func() bool
	lastDetailed bool
}

func (l *fileTableLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(360, 150) }
func (l *fileTableLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	detailed := l.detailed == nil || l.detailed()
	if l.width != size.Width || detailed != l.lastDetailed {
		l.width = size.Width
		l.lastDetailed = detailed
		usable := max(float32(0), size.Width-24)
		fractions := []float32{0.58, 0.42}
		if detailed {
			fractions = []float32{0.25, 0.07, 0.24, 0.22, 0.22}
		}
		for col, fraction := range fractions {
			l.table.SetColumnWidth(col, usable*fraction)
		}
	}
	l.table.Move(fyne.NewPos(0, 0))
	l.table.Resize(size)
}

// Only the active progress fill uses the light foreground; keep primary
// buttons and other components on the existing neutral palette.
type progressBarTheme struct{ fyne.Theme }

func (t progressBarTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if name == theme.ColorNamePrimary {
		return t.Theme.Color(theme.ColorNameForeground, variant)
	}
	return t.Theme.Color(name, variant)
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
