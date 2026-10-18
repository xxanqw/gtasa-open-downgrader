package ui

import (
	"sort"
	"strings"

	"github.com/xxanqw/gtasa-open-downgrader/internal/mods"
)

func modRequirements(name string) string {
	if name == "ASI Loader" {
		return ""
	}
	if name == "ModLoader" {
		return "Requires ASI Loader"
	}
	required := []string{"ASI Loader", "ModLoader"}
	required = append(required, mods.RequiredMods(name)...)
	return "Requires " + strings.Join(required, " + ")
}

func (w *Window) modDependents(name string) []string {
	seen := map[string]bool{}
	var dependents []string
	for _, installed := range w.batchTracker().Installed {
		if installed == name || seen[installed] {
			continue
		}
		if mods.UninstallBlocker(mods.Tracker{Installed: []string{name, installed}}, name, nil) != "" {
			seen[installed] = true
			dependents = append(dependents, installed)
		}
	}
	sort.Strings(dependents)
	return dependents
}

func modPurpose(name string) string {
	switch name {
	case "ASI Loader":
		return "ASI plugin support"
	case "ModLoader":
		return "Separate mod folders"
	case "SilentPatch":
		return "Fixes original game bugs"
	case "Widescreen Fixes":
		return "Widescreen scaling"
	case "SkyGFX":
		return "Console-style graphics"
	case "Frontend Mods":
		return "HD loading screens"
	case mods.ExtendedFonts:
		return "HD fonts + 11 languages"
	case mods.HDRadarMap:
		return "Colored HD map tiles"
	case mods.OriginalHQIcons:
		return "HD radar and HUD icons"
	case mods.AtmosphereHUD:
		return "HD buttons and minigames"
	case mods.NadalaoWeapons:
		return "Weapon icons (beta)"
	case mods.Framerate:
		return "Fixes higher-FPS issues"
	case "GInput":
		return "Improved controller support"
	case "Project 2DFX":
		return "Distant lights and effects"
	case mods.SanLtdRussian:
		return "Russian translation"
	}
	return "Optional game modification"
}

func (w *Window) setModLabel(name, full string) {
	w.modDetails[name] = full
	w.renderModLabel(name)
	if w.modTable != nil && !w.dependencies {
		w.rebuildModTable()
	}
}
