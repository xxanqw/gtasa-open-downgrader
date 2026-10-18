package ui

import (
	"github.com/xxanqw/gtasa-open-downgrader/internal/mods"
	"strings"
)

func modInfo(name string) string {
	switch name {
	case "ASI Loader":
		return "Loads ASI plugins. Required automatically when selected mods need it."
	case "ModLoader":
		return "Loads mods from separate folders instead of replacing original game assets. Selected automatically when needed."
	case "SilentPatch":
		return "Fixes original game bugs."
	case "Widescreen Fixes":
		return "Corrects widescreen scaling and proportions."
	case mods.Framerate:
		return "Fixes several problems caused by higher frame rates. Not a guarantee of compatibility with every mod."
	case "GInput":
		return "Improved controller support."
	case "SkyGFX":
		return "Classic console-style graphics."
	case "Project 2DFX":
		return "Distant lights and other visual effects; may affect performance."
	case "Frontend Mods":
		return "Widescreen loading screens; no fonts or radar replacement."
	case mods.ExtendedFonts:
		return "This matched Extended Fonts (ELL) integration is exclusive to GTA SA Open Downgrader. The best way to install this package is through the downgrader: it installs the matching loader, fonts and translations together and checks for conflicts. The ELL loader itself is publicly available.\n\nFour HD fonts and eleven translations with ELL 0.4.2 Beta. CLEO support is experimental; Romanian is transliterated and arbitrary Unicode is unsupported.\n\nSelect English and close GTA first. Tracked HD Fonts are backed up; untracked/customized ELL installations and duplicate loaders are refused."
	case mods.HDRadarMap:
		return "Colored original-style Yojo2 HQ radar: 144 native 512px tiles, copied unchanged from the reviewed Atmosphere archive. Map only; no new ASI, fonts or extra HUD replacement. Live appearance remains unverified."
	case mods.OriginalHQIcons:
		return "Original-style HD HUD/radar icons. Refuses competing ModLoader hud.txd files."
	case mods.AtmosphereHUD:
		return "Selected HD button/race/minigame textures. Requires Original HQ Radar Icons. Does not add fonts, map tiles or replacement menus."
	case mods.NadalaoWeapons:
		return "Faithful 128px weapon icons plus Weapon Icons TXD 1.0 helper ASI. Preserves weapon models.\n\nLive compatibility with ELL, CLEO and weapon wheels is experimental/unverified. The helper's empty placeholder TXD is excluded."
	case mods.SanLtdRussian:
		return "Standalone Russian translation with its own fonts/HUD. Conflicts with HQ HUD. ELL already includes Russian."
	}
	return name
}
func (w *Window) showModDetails(name string) {
	state := w.modDetails[name]
	if state == "" {
		state = "Not installed"
	}
	requirements := modRequirements(name)
	if required := w.modRequiredBy[name]; len(required) > 0 {
		requirements += "\n\nSelected automatically and locked: required by " + strings.Join(required, ", ") + ". Clear those selections to unlock this row. Automatic prerequisite selection does not request removal or an update. Reinstall review lists its required packages."
	}
	if dependents := w.modDependents(name); len(dependents) > 0 {
		requirements += "\nRequired by: " + strings.Join(dependents, ", ") + ".\nTo remove this package, also select its installed dependents."
	}
	w.info(name, strings.ReplaceAll(state, " · ", " | ")+"\n\n"+modInfo(name)+"\n\n"+strings.TrimSpace(requirements)+"\n\nClose GTA before changing mods. Installation and removal retain their review/backup safeguards.")
}
