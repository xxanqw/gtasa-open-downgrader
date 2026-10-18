package ui

import (
	"context"
	"fmt"
	"strings"

	"fyne.io/fyne/v2/widget"
	"github.com/xxanqw/gtasa-open-downgrader/internal/mods"
	"github.com/xxanqw/gtasa-open-downgrader/internal/updater"
)

func (w *Window) batchTracker() mods.Tracker {
	if w.modTracker.Installed != nil {
		return w.modTracker
	}
	t := mods.Tracker{}
	for name, installed := range w.installedMods {
		if installed {
			t.Installed = append(t.Installed, name)
		}
	}
	return t
}
func (w *Window) refreshModBatch() {
	if w.dependencies || w.modTable == nil {
		return
	}
	w.refreshModTableBatch()
}
func (w *Window) renderModLabel(name string) {
	if w.modLabels[name] == nil {
		return
	}
	if w.installedMods[name] || strings.HasPrefix(w.modDetails[name], "Installed") {
		text := "Installed"
		if strings.Contains(w.modDetails[name], "Update available:") {
			text = "Installed (update)"
		}
		w.setModState(name, text, widget.SuccessImportance)
	} else {
		w.setModState(name, "Not installed", widget.DangerImportance)
	}
}
func (w *Window) removeSelectedMods() {
	if !w.downgradeComplete() || w.readonly || w.busy || w.promptActive {
		return
	}
	root := w.scanPath
	selection := w.modSelectedInstalled(true)
	if len(selection) == 0 {
		return
	}
	tracker, err := mods.LoadTracker(root)
	if err != nil {
		w.error(err)
		return
	}
	order, err := mods.UninstallPlan(tracker, selection)
	if err != nil {
		w.error(err)
		return
	}
	w.confirm("Remove selected mods", strings.Join(order, "\n")+"\n\nDependents are removed before their loaders. Each mod is backed up in backups/mods. If cancelled or failed, completed removals remain removed; remaining mods are kept.", func(ok bool) {
		if !ok {
			return
		}
		var done map[string]mods.UninstallResult
		w.run("Removing selected mods", func(ctx context.Context) error {
			var e error
			done, e = mods.UninstallSelected(ctx, root, selection, updater.Version)
			return e
		}, func(error) {
			for name, result := range done {
				w.logs.Record("INFO", fmt.Sprintf("Removed %s; backup: %s", name, result.BackupDir))
			}
			w.scan()
		})
	})
}
