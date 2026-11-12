package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/xxanqw/gtasa-open-downgrader/internal/game"
	appwidget "github.com/xxanqw/gtasa-open-downgrader/internal/uiwidget"
)

func downgradeReviewModal(root, source string, rows []game.Row) *appwidget.ModalOptions {
	files := container.NewVBox()
	changed, unchanged, modified := 0, 0, 0
	for _, row := range rows {
		if row.NeedsPatch == "No" {
			unchanged++
			continue
		}
		changed++
		if strings.HasPrefix(row.Status, "Modified") || strings.Contains(row.NeedsPatch, "Force") {
			modified++
		}
		file := promptText(row.Path)
		file.Selectable = true
		state := widget.NewLabel(strings.TrimSpace(strings.TrimSuffix(row.Status, " (Needs Patch)")))
		state.Importance = widget.LowImportance
		files.Add(container.NewBorder(nil, nil, nil, state, file))
	}
	subtitle := fmt.Sprintf("%d files to patch · %d unchanged or not applicable", changed, unchanged)
	notice := "Originals are backed up. Cancelling stops further work; completed changes remain."
	if modified > 0 {
		subtitle += fmt.Sprintf(" · %d modified", modified)
		notice = "Includes modified files. Originals are backed up; cancelling does not undo completed changes."
	}
	return &appwidget.ModalOptions{
		Subtitle: subtitle,
		Size:     fyne.NewSize(620, 0),
		Summary: container.NewVBox(
			appwidget.ModalFacts(appwidget.ModalFact{Label: "Game folder", Value: root}),
			container.NewGridWithColumns(2,
				appwidget.ModalFacts(appwidget.ModalFact{Label: "Source", Value: strings.TrimSpace(strings.SplitN(source, "|", 2)[0])}),
				appwidget.ModalFacts(appwidget.ModalFact{Label: "Target", Value: "v1.0 US"}),
			),
		),
		Sections: []appwidget.ModalSection{
			{Title: fmt.Sprintf("Files to change (%d)", changed), Content: files},
			{Title: "Backup & recovery", Content: container.NewVBox(
				appwidget.ModalFacts(appwidget.ModalFact{Label: "Backup folder", Value: filepath.Join(root, "backups")}),
				promptText("Use Recovery to restore originals. Restoring downgrade backups does not uninstall mods."),
			)},
		},
		MaxBodyHeight: 220,
		Accessory:     appwidget.ModalNotice(notice, widget.WarningImportance),
	}
}

func modInstallReviewModal(root string, plan []string, reinstall bool) *appwidget.ModalOptions {
	notice := "Required packages are included automatically. Completed installations remain if cancelled."
	if reinstall {
		notice = "Reinstallation can overwrite customized settings. Back up settings you want to keep."
	}
	packages := container.NewVBox()
	for _, name := range plan {
		packages.Add(promptText(name))
	}
	return &appwidget.ModalOptions{
		Subtitle: fmt.Sprintf("%d packages in the plan, including requirements", len(plan)),
		Size:     fyne.NewSize(600, 0),
		Summary:  appwidget.ModalFacts(appwidget.ModalFact{Label: "Game folder", Value: root}),
		Sections: []appwidget.ModalSection{
			{Title: fmt.Sprintf("Review packages (%d)", len(plan)), Content: packages, Expanded: true},
		},
		MaxBodyHeight: 180,
		Accessory:     appwidget.ModalNotice(notice, widget.WarningImportance),
	}
}
