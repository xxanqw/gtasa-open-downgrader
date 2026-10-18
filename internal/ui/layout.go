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

// Short footer labels only; the journal retains the original diagnostics.
func compactStatusText(text string) string {
	text = strings.TrimSpace(text)
	if strings.HasSuffix(text, " failed") {
		return "Failed | see log"
	}
	complete := strings.HasSuffix(text, " complete")
	operation := strings.TrimSuffix(text, " complete")
	for _, item := range []struct{ operation, active, done string }{
		{"Detecting Steam and multiplayer", "Checking Steam…", "Steam checked"},
		{"Preparing patch assets", "Preparing patches…", "Patches ready"},
		{"Scanning game files", "Scanning…", "Scan done"},
		{"Installing mods", "Installing mods…", "Mods installed"},
		{"Updating installed mods", "Updating mods…", "Mods updated"},
		{"Downgrading game", "Downgrading…", "Downgrade done"},
		{"Downloading patch assets", "Downloading patches…", "Patches downloaded"},
		{"Downloading app update", "Downloading update…", "Update downloaded"},
		{"Importing patch assets", "Importing patches…", "Patches imported"},
		{"Validating patches", "Checking patches…", "Patches checked"},
		{"Restoring backups", "Restoring…", "Backups restored"},
		{"Deleting backups", "Deleting backups…", "Backups deleted"},
		{"Clearing user data", "Clearing user data…", "User data cleared"},
		{"Stopping background work…", "Stopping…", "Stopped"},
		{"Steam / Proton launch options copied.", "Options copied", "Options copied"},
		{"Application log exported.", "Log exported", "Log exported"},
		{"File report copied.", "Report copied", "Report copied"},
	} {
		if operation == item.operation {
			if complete {
				return item.done
			}
			return item.active
		}
	}
	if complete {
		return "Done"
	}
	if strings.HasPrefix(text, "Scan complete: ") {
		var files int
		if _, err := fmt.Sscanf(text, "Scan complete: %d manifest files checked.", &files); err == nil {
			return fmt.Sprintf("Scanned %d files", files)
		}
	}
	if strings.HasPrefix(text, "Cancelled.") {
		return "Cancelled"
	}
	if strings.HasPrefix(text, "Cancelling;") {
		return "Cancelling…"
	}
	if parts := strings.SplitN(text, " · ", 2); len(parts) == 2 && strings.ContainsAny(parts[0], `/\`) {
		path := strings.TrimRight(parts[0], `/\`)
		if index := strings.LastIndexAny(path, `/\`); index >= 0 {
			path = path[index+1:]
		}
		text = path + " | " + parts[1]
	}
	text = strings.ReplaceAll(text, " · ", " | ")
	const limit = 24
	// Keep download percentages visible even when the asset name is long.
	if index := strings.LastIndex(text, " | "); index >= 0 && strings.HasSuffix(text, "%") {
		suffix := text[index:]
		if len([]rune(suffix)) < limit {
			return shortStatusLabel(text[:index], limit-len([]rune(suffix))) + suffix
		}
	}
	return shortStatusLabel(text, limit)
}

func shortStatusLabel(text string, limit int) string {
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
