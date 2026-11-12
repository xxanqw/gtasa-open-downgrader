package ui

import (
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"github.com/xxanqw/gtasa-open-downgrader/internal/mods"
	appwidget "github.com/xxanqw/gtasa-open-downgrader/internal/uiwidget"
	"sort"
	"strings"
)

func (w *Window) newModTable() fyne.CanvasObject {
	w.modExplicit = map[string]bool{}
	w.modRequiredBy = map[string][]string{}
	w.modFilter = newControlSelect([]string{"All", "Installed", "Available", "Updates"}, func(string) { w.rebuildModTable() })
	w.modFilter.Selected = "All"
	w.bulkRemove = appwidget.NewButton("Remove", w.removeSelectedMods)
	w.bulkClear = appwidget.NewButton("Clear", func() {
		if !w.modTableReady() {
			return
		}
		w.modExplicit = map[string]bool{}
		w.refreshModBatch()
	})
	w.bulkClear.AccessibleLabel = "Clear selection"
	w.bulkActions = container.NewHBox(w.install, w.reinstall, w.bulkRemove, w.bulkClear)
	w.modTable = newModWebTable(w)
	w.rebuildModTable()
	return w.modTable
}
func modDisplayName(name string) string {
	if name == mods.ExtendedFonts {
		return "Extended Fonts (ELL)"
	}
	if name == "Frontend Mods" {
		return "Frontend pack"
	}
	return strings.TrimSuffix(name, " (60fps fix)")
}
func (w *Window) modTableReady() bool {
	return w.scanPath != "" && w.scanPath == w.path.Text && w.downgradeComplete() && !w.busy && !w.promptActive && !w.readonly
}
func (w *Window) rebuildModTable() {
	if w.modTable == nil {
		return
	}
	names := []string{}
	for _, source := range mods.Catalog {
		name := source.Name
		switch w.modFilter.Selected {
		case "Installed":
			if !w.installedMods[name] {
				continue
			}
		case "Available":
			if w.installedMods[name] {
				continue
			}
		case "Updates":
			if !strings.Contains(w.modDetails[name], "Update available:") {
				continue
			}
		}
		names = append(names, name)
	}
	if w.modSortColumn != 0 {
		sort.SliceStable(names, func(i, j int) bool {
			a, b := modDisplayName(names[i]), modDisplayName(names[j])
			if w.modSortColumn == 2 {
				a = w.modLabels[names[i]].Text + " " + a
				b = w.modLabels[names[j]].Text + " " + b
			}
			if w.modSortDescending {
				return strings.ToLower(a) > strings.ToLower(b)
			}
			return strings.ToLower(a) < strings.ToLower(b)
		})
	}
	w.modTableNames = names
	if len(names) == 0 {
		w.modEmpty.Show()
	} else {
		w.modEmpty.Hide()
	}
	w.modTable.SetNames(names)
	w.refreshModCatalogSummary()
}
func (w *Window) modSelectedInstalled(installed bool) []string {
	var names []string
	for _, name := range w.selected() {
		// Automatically checked prerequisites are not destructive-action targets.
		if w.installedMods[name] == installed && (!installed || w.modExplicit[name]) {
			names = append(names, name)
		}
	}
	return names
}
func (w *Window) refreshModTableBatch() {
	ready := w.modTableReady()
	w.dependencies = true
	w.syncModRequirements()
	for name, c := range w.checks {
		c.Show()
		if ready && len(w.modRequiredBy[name]) == 0 {
			c.Enable()
		} else {
			c.Disable()
		}
		w.renderModLabel(name)
	}
	w.dependencies = false
	missing, installed := w.modSelectedInstalled(false), w.modSelectedInstalled(true)
	total := len(w.selected())
	explicit, required := 0, 0
	for _, name := range w.selected() {
		if w.modExplicit[name] {
			explicit++
		} else {
			required++
		}
	}
	w.modSelection.SetText(fmt.Sprintf("%d selected", explicit))
	if required > 0 {
		w.modSelection.SetText(fmt.Sprintf("%d selected + %d required", explicit, required))
	}
	w.install.SetText(fmt.Sprintf("Install %d", len(missing)))
	w.reinstall.SetText("Reinstall")
	w.reinstall.AccessibleLabel = fmt.Sprintf("Reinstall %d selected mods", len(installed))
	w.bulkRemove.SetText("Remove")
	w.bulkRemove.AccessibleLabel = fmt.Sprintf("Remove %d selected mods", len(installed))
	for _, item := range []struct {
		button *appwidget.Button
		count  int
	}{{w.install, len(missing)}, {w.reinstall, len(installed)}, {w.bulkRemove, len(installed)}} {
		if item.count == 0 {
			item.button.Hide()
		} else {
			item.button.Show()
		}
		if ready && item.count > 0 {
			item.button.Enable()
		} else {
			item.button.Disable()
		}
	}
	if ready && total > 0 {
		w.bulkClear.Enable()
	} else {
		w.bulkClear.Disable()
	}
	blocker := ""
	tracker := w.batchTracker()
	removing := map[string]bool{}
	for _, name := range installed {
		removing[name] = true
	}
	for _, name := range installed {
		for _, dependent := range w.modRequiredBy[name] {
			if !removing[dependent] {
				blocker = "Required by selected " + modDisplayName(dependent)
				break
			}
		}
		if blocker != "" {
			break
		}
		if reason := mods.UninstallBlocker(tracker, name, installed); reason != "" {
			blocker = reason
			break
		}
	}
	if blocker != "" {
		w.bulkRemove.Disable()

		w.bulkRemove.AccessibleLabel = "Removal blocked: " + blocker
		w.modSelection.SetText(fmt.Sprintf("%d selected · Removal blocked", explicit))
	}
	if total == 0 {
		w.bulkActions.Hide()
		w.refreshModCatalogSummary()
	} else {
		w.bulkActions.Show()
	}
	if w.modFilter != nil {
		if w.busy || w.promptActive {
			w.modFilter.Disable()
		} else {
			w.modFilter.Enable()
		}
	}
	// Keep this control visible and give it a fixed slot in the main toolbar.
	if total > 0 {
		w.updateModsButton.SetText("Update selected")
		if ready && len(installed) > 0 {
			w.updateModsButton.Enable()
		} else {
			w.updateModsButton.Disable()
		}
	} else {
		w.updateModsButton.SetText("Update installed")
		if ready && w.installedModCount > 0 {
			w.updateModsButton.Enable()
		} else {
			w.updateModsButton.Disable()
		}
	}
	if total > 0 && len(installed) == 0 {
		w.updateModsButton.Hide()
	} else {
		w.updateModsButton.Show()
	}
	w.rebuildModTable()
	// Visibility changes do not alter the header's fixed minimum size, so Fyne
	// will not necessarily rerun its layout. Reposition the surviving actions.
	w.refreshWorkspace()
}

// Keep routine counts and background version checks in the existing toolbar.
func (w *Window) refreshModCatalogSummary() {
	if w.modSelection == nil || len(w.selected()) > 0 {
		return
	}
	if w.modStatus != nil && strings.HasPrefix(w.modStatus.Text, "Checking installed mod versions") {
		w.modSelection.SetText("Checking versions…")
		return
	}
	installed, updates := 0, 0
	for _, source := range mods.Catalog {
		if w.installedMods[source.Name] {
			installed++
		}
		if strings.Contains(w.modDetails[source.Name], "Update available:") {
			updates++
		}
	}
	text := fmt.Sprintf("%d mods · %d installed", len(mods.Catalog), installed)
	if updates == 1 {
		text += " · 1 update"
	} else if updates > 1 {
		text += fmt.Sprintf(" · %d updates", updates)
	}
	w.modSelection.SetText(text)
}
