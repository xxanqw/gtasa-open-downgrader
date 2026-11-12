package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/xxanqw/gtasa-open-downgrader/internal/mods"
	appwidget "github.com/xxanqw/gtasa-open-downgrader/internal/uiwidget"
)

func utilityHeading(title string) fyne.CanvasObject {
	heading := widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	heading.SizeName = theme.SizeNameHeadingText
	return heading
}

func utilitySection(title string, rows ...fyne.CanvasObject) fyne.CanvasObject {
	heading := widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	objects := []fyne.CanvasObject{container.New(layout.NewCustomPaddedLayout(8, 0, 0, 0), heading)}
	for index, row := range rows {
		if index > 0 {
			objects = append(objects, widget.NewSeparator())
		}
		objects = append(objects, row)
	}
	return container.NewVBox(objects...)
}

// Actions keep their natural height. On narrow windows the buttons move below
// the explanation instead of squeezing it or stretching over the whole row.
func utilityRow(title, description string, actions ...fyne.CanvasObject) fyne.CanvasObject {
	heading := widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	detail := hubText(description)
	detail.Importance = widget.LowImportance
	return container.New(&utilityRowLayout{}, heading, detail, container.NewHBox(actions...))
}

type utilityRowLayout struct{ stacked bool }

func (l *utilityRowLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	textHeight := objects[0].MinSize().Height + objects[1].MinSize().Height - 12
	height := max(textHeight, objects[2].MinSize().Height)
	if l.stacked {
		height = textHeight + objects[2].MinSize().Height + 4
	}
	return fyne.NewSize(max(320, objects[2].MinSize().Width), height)
}
func (l *utilityRowLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	heading, detail, actions := objects[0], objects[1], objects[2]
	actionSize := actions.MinSize()
	l.stacked = size.Width-actionSize.Width-20 < 320
	textWidth := size.Width - actionSize.Width - 20
	if l.stacked {
		textWidth = size.Width
	}
	heading.Resize(fyne.NewSize(max(1, textWidth), heading.MinSize().Height))
	detail.Resize(fyne.NewSize(max(1, textWidth), detail.MinSize().Height))
	headingHeight, detailHeight := heading.MinSize().Height, detail.MinSize().Height
	heading.Move(fyne.NewPos(0, 0))
	detail.Move(fyne.NewPos(0, headingHeight-12))
	detail.Resize(fyne.NewSize(max(1, textWidth), detailHeight))
	textHeight := headingHeight + detailHeight - 12
	actions.Resize(actionSize)
	if l.stacked {
		actions.Move(fyne.NewPos(8, textHeight+4))
	} else {
		actions.Move(fyne.NewPos(size.Width-actionSize.Width, max(0, (textHeight-actionSize.Height)/2)))
	}
	appwidget.Relayout(actions)
}

func (w *Window) buildRecoveryPage() fyne.CanvasObject {
	restore := appwidget.NewButtonWithIcon("Restore original files", theme.HistoryIcon(), func() { w.runHubTool("Restore downgrade backups") })
	deleteBackups := appwidget.NewButton("Delete backups…", func() { w.runHubTool("Delete downgrade backups") })
	clearData := appwidget.NewButton("Delete user data…", func() { w.runHubTool("Clear saves, settings and gallery") })
	deleteBackups.Importance = widget.DangerImportance
	clearData.Importance = widget.DangerImportance
	w.recoveryActions = []fyne.Disableable{restore, deleteBackups, clearData}
	w.legacyRussianRemove = appwidget.NewButton("Remove translation…", func() { w.uninstallMod(mods.SanLtdRussian) })
	w.legacyRussianSection = utilitySection("Retired translation", utilityRow("SanLtd Russian", "ELL includes Russian. Remove this older translation only if you no longer need it.", w.legacyRussianRemove))
	w.legacyRussianSection.Hide()
	return w.hubPage(
		utilityHeading("Recovery"),
		utilitySection("Restore game files", w.hubBackups, utilityRow("Original files", "Restore pre-downgrade files. Installed mods remain.", restore)),
		utilitySection("Permanent deletion",
			utilityRow("Downgrade backups", "Delete downgrade originals. Keep mod-removal backups.", deleteBackups),
			utilityRow("Saves, settings & gallery", "Permanently delete user data. Back it up first.", clearData)),
		w.legacyRussianSection,
	)
}
