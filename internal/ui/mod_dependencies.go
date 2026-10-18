package ui

import "github.com/xxanqw/gtasa-open-downgrader/internal/mods"

// Keep user choices separate from their dependency closure. Shared/transitive
// requirements remain checked until the last selecting dependent is cleared.
// Manually chosen prerequisites remain selected after their dependents clear.
func (w *Window) syncModRequirements() {
	w.modRequiredBy = map[string][]string{}
	for _, source := range mods.Catalog {
		root := source.Name
		if !w.modExplicit[root] {
			continue
		}
		for _, dependency := range mods.Selection([]string{root}, mods.Tracker{}) {
			if dependency != root {
				w.modRequiredBy[dependency] = append(w.modRequiredBy[dependency], root)
			}
		}
	}
	for name, check := range w.checks {
		check.SetChecked(w.modExplicit[name] || len(w.modRequiredBy[name]) > 0)
	}
}
func (w *Window) setVisibleModSelection(value bool) {
	if !w.modTableReady() {
		return
	}
	for _, name := range w.modTableNames {
		// Bulk clear can release explicitly chosen prerequisites too. Dependencies
		// needed by selected rows outside the filter are rechecked by the closure.
		if !value || len(w.modRequiredBy[name]) == 0 {
			w.modExplicit[name] = value
		}
	}
	w.refreshModBatch()
}
