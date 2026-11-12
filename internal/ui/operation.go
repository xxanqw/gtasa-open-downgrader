package ui

import (
	"context"
	"errors"
	"strings"

	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	appwidget "github.com/xxanqw/gtasa-open-downgrader/internal/uiwidget"
)

// Every operation uses this footer: one full-width status and one thin track.
func (w *Window) buildProgressFooter() {
	w.outcomeDismiss = appwidget.NewButtonWithIcon("", theme.CancelIcon(), func() { w.operationOutcome = ""; w.refreshProgressFooter() })
	w.outcomeDismiss.AccessibleLabel = "Dismiss operation warning"
	w.outcomeDismiss.Importance = widget.LowImportance
	w.browserButton = appwidget.NewButton("Download options…", func() {
		if w.browserOptions != nil {
			w.browserOptions()
		}
	})
	w.indeterminate = newThinIndeterminate()
	w.indeterminate.Hide()
	w.progressTrack = container.NewStack(w.progress, w.indeterminate)
	w.footerActions = container.NewHBox(w.version, w.cancelButton, w.browserButton, w.footerLogButton, w.downgrade, w.outcomeDismiss)
	line := container.NewBorder(nil, nil, nil, container.NewCenter(w.footerActions), w.status)
	w.footer = container.NewVBox(line, container.NewThemeOverride(w.progressTrack, progressBarTheme{w.app.Settings().Theme()}))
	w.refreshProgressFooter()
}

func (w *Window) setIndeterminate(active bool) {
	if w.indeterminate == nil {
		return
	}
	if active && w.busy {
		w.indeterminate.Show()
		w.indeterminate.Start()
		w.progress.Hide()
	} else {
		w.indeterminate.Stop()
		w.indeterminate.Hide()
		if w.busy {
			w.progress.Show()
		} else {
			w.progress.Hide()
		}
	}
}

func (w *Window) refreshProgressFooter() {
	if w.footer == nil {
		return
	}
	text := w.fullStatus
	if text == "" {
		text = "Ready"
	}
	w.status.Importance = widget.MediumImportance
	if w.busy {
		if w.operationTitle != "" && !strings.HasPrefix(text, w.operationTitle) {
			text = w.operationTitle + ": " + text
		}
		w.version.Hide()
		w.progressTrack.Show()
		if w.indeterminate.Visible() {
			w.progress.Hide()
		} else {
			w.progress.Show()
		}
	} else {
		w.version.Show()
		w.setIndeterminate(false)
		w.progressTrack.Hide()
		if w.operationOutcome != "" {
			text = w.operationOutcome
			w.status.Importance = widget.WarningImportance
		}
	}
	w.status.SetText(text)
	if !w.busy && w.operationOutcome != "" {
		w.outcomeDismiss.Show()
	} else {
		w.outcomeDismiss.Hide()
	}
	if w.busy && w.browserOptions != nil && !w.promptActive {
		w.browserButton.Show()
	} else {
		w.browserButton.Hide()
	}
	w.footerActions.Refresh()
	w.refreshWorkspace()
}

// Visibility and wrapped-label height changes require ancestor layout, not only
// repainting the child. A second pass settles wrapping at the final width.
func (w *Window) refreshWorkspace() {
	if content := w.window.Content(); content != nil {
		appwidget.Relayout(content)
		appwidget.Relayout(content)
		w.window.Canvas().Refresh(content)
	}
}

// A verification scan must not erase the result of the operation it follows.
func (w *Window) recordOperationOutcome(title string, err error) {
	if passiveOperation(title) {
		return
	}
	w.outcomePath = w.path.Text
	switch {
	case errors.Is(err, context.Canceled):
		w.operationOutcome = title + " cancelled. Cancellation does not undo completed changes."
	case err != nil:
		w.operationOutcome = title + " failed. See Log for details before retrying."
	default:
		// Routine success belongs in the status bar, not a banner to dismiss.
		w.operationOutcome = ""
	}
}

func passiveOperation(title string) bool {
	switch title {
	case "Identifying game installation", "Scanning game files", "Preparing patch assets", "Detecting Steam and multiplayer":
		return true
	}
	return false
}

// Guidance is deliberately advisory: never automatically retry a file mutation.
func errorRecovery(details string) string {
	text := strings.ToLower(details)
	switch {
	case strings.Contains(text, "permission"), strings.Contains(text, "read-only"), strings.Contains(text, "access is denied"):
		return "The location may not be writable. Choose a writable game folder on Game and scan it again. Do not move or delete backups while work is running."
	case strings.Contains(text, "no space left"), strings.Contains(text, "not enough space"), strings.Contains(text, "disk full"):
		return "Check free space on both the game and download/cache drives. Keep backups until you have verified the game."
	case strings.Contains(text, "source edition"), strings.Contains(text, "unsupported game"), strings.Contains(text, "unsupported edition"):
		return "Check the detected edition on Game. Only NewSteam R2 Global and German are supported downgrade sources. Do not force an unsupported edition."
	case strings.Contains(text, "gta is running"), strings.Contains(text, "game is running"), strings.Contains(text, "close gta"):
		return "Close GTA and any installer or launcher that is still active, then try the action again. Do not interrupt file restoration."
	case strings.Contains(text, "http"), strings.Contains(text, "network"), strings.Contains(text, "timeout"), strings.Contains(text, "connection"):
		return "Check your internet connection and the download host. You can try again later or use Tools to import a matching official archive."
	case strings.Contains(text, "checksum"), strings.Contains(text, "hash mismatch"), strings.Contains(text, "archive"):
		return "The package may be incomplete or different from the reviewed release. Use the official matching archive and check the diagnostic details before downloading or importing again."
	default:
		return "Review the diagnostic details and log before retrying. Completed changes may remain; Recovery restores downgrade backups, not installed mods. Copy these details when reporting a problem."
	}
}
