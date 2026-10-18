package mods

import (
	"context"
	"fmt"
	"sort"
)

// UninstallBlocker reports dependents outside the proposed removal selection.
// The executor still validates/ordering-checks the complete plan before writes.
func UninstallBlocker(t Tracker, name string, selected []string) string {
	name = canonicalModName(name)
	removing := map[string]bool{}
	for _, n := range selected {
		removing[canonicalModName(n)] = true
	}
	remaining := []string{}
	for _, n := range t.Installed {
		if canonicalModName(n) == name || !removing[canonicalModName(n)] {
			remaining = append(remaining, n)
		}
	}
	return uninstallDependency(name, Tracker{Installed: remaining})
}

// UninstallPlan validates the entire selection before a single installed file
// moves. Required packages are ordered after their selected dependents.
func UninstallPlan(t Tracker, selected []string) ([]string, error) {
	pending := map[string]bool{}
	for _, name := range selected {
		name = canonicalModName(name)
		if !t.Has(name) {
			return nil, fmt.Errorf("%s is not installed by the downgrader", name)
		}
		if len(uninstallPaths(name)) == 0 {
			return nil, fmt.Errorf("%s has no known install location", name)
		}
		pending[name] = true
	}
	remaining := append([]string(nil), t.Installed...)
	var order []string
	for len(pending) > 0 {
		names := make([]string, 0, len(pending))
		for name := range pending {
			names = append(names, name)
		}
		sort.Strings(names)
		progress := false
		for _, name := range names {
			if uninstallDependency(name, Tracker{Installed: remaining}) != "" {
				continue
			}
			order = append(order, name)
			delete(pending, name)
			progress = true
			next := make([]string, 0, len(remaining))
			for _, installed := range remaining {
				if canonicalModName(installed) != name {
					next = append(next, installed)
				}
			}
			remaining = next
		}
		if !progress {
			name := names[0]
			return nil, fmt.Errorf("%s: %s; select its installed dependents for removal too", name, uninstallDependency(name, Tracker{Installed: remaining}))
		}
	}
	return order, nil
}

// Each removal retains Uninstall's game-closed checks, tracker reload, backup
// and per-mod rollback. Completed removals are returned even on cancellation;
// this is not an all-or-nothing transaction across the entire selection.
func UninstallSelected(ctx context.Context, root string, selected []string, appVersion string) (map[string]UninstallResult, error) {
	t, err := LoadTracker(root)
	if err != nil {
		return nil, err
	}
	order, err := UninstallPlan(t, selected)
	if err != nil {
		return nil, err
	}
	done := map[string]UninstallResult{}
	for _, name := range order {
		if err = ctx.Err(); err != nil {
			return done, err
		}
		result, removeErr := Uninstall(ctx, root, name, appVersion)
		if removeErr != nil {
			return done, fmt.Errorf("remove %s after %d completed removals: %w", name, len(done), removeErr)
		}
		done[name] = result
	}
	return done, nil
}
