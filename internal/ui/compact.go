package ui

import (
	"strings"

	"github.com/xxanqw/gtasa-open-downgrader/internal/mods"
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
	case mods.WindowedMode:
		return "ThirteenAG's windowed and borderless display plugin for GTA SA v1.0 US. Alt+Enter or Ctrl+Enter switches modes. Settings are created next to the plugin in III.VC.SA.WindowedMode.cfg after the first game launch; updates preserve them."
	case mods.OpenLimitAdjuster:
		return "Grows supported engine limits as needed. Installed in scripts so it starts before ModLoader. Shared by Project 2DFX and LOD Vegetation; its bundled copy is not installed twice. Do not combine with another limit adjuster without checking compatibility."
	case mods.LODVegetation:
		return "Junior_Djjr's LOD Vegetation v1.2 adds low-detail distant trees. Requires Open Limit Adjuster. Replaces many IPL map files: conflicts with other IPL/map replacements such as Z PC Map, and is already included in Proper Fixes. Keep Project 2DFX's LoadAllBinaryIPLs at 0. SA-MP requires manual IDE setup described on the source page; multiplayer compatibility is not automatic."
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
func (w *Window) modInstalledVersion(name string) string {
	if !w.installedMods[name] && !strings.HasPrefix(w.modDetails[name], "Installed") {
		return ""
	}
	if version := w.modTracker.Versions[name].Version; version != "" {
		return version
	}
	if parts := strings.Split(w.modDetails[name], " · "); len(parts) > 1 && parts[1] != "" {
		return parts[1]
	}
	return "Version unknown"
}

func (w *Window) modDetailText(name string) string {
	state := w.modDetails[name]
	if state == "" {
		if w.installedMods[name] {
			state = "Installed · " + w.modInstalledVersion(name)
		} else {
			state = "Not installed"
		}
	}
	parts := []string{state, modInfo(name)}
	if requirements := modRequirements(name); requirements != "" {
		parts = append(parts, requirements)
	}
	if required := w.modRequiredBy[name]; len(required) > 0 {
		parts = append(parts, "Selected automatically for "+strings.Join(required, ", ")+". Clear those selections to unlock this requirement.")
	}
	if dependents := w.modDependents(name); len(dependents) > 0 {
		parts = append(parts, "Required by installed mods: "+strings.Join(dependents, ", ")+". Select these dependents too before removal.")
	}
	for _, source := range mods.Catalog {
		if source.Name == name {
			if source.Version != "" && !strings.HasPrefix(state, "Installed") {
				parts = append(parts, "Catalog version: "+source.Version)
			}
			if strings.Contains(source.URL, "sharemods.com/") {
				parts = append(parts, "Browser download: complete ShareMods confirmation in the download queue. Verified cached archives are reused.")
			}
			break
		}
	}
	if name == mods.NadalaoWeapons {
		parts = append(parts, "Downloads include the weapon icons and the Weapon Icons TXD helper archive.")
	}
	if name == mods.AtmosphereHUD || name == mods.HDRadarMap {
		parts = append(parts, "Shares its download archive with "+map[string]string{mods.AtmosphereHUD: modDisplayName(mods.HDRadarMap), mods.HDRadarMap: modDisplayName(mods.AtmosphereHUD)}[name]+".")
	}
	if name == mods.Framerate {
		parts = append(parts, "Compatibility warning: read the warning beside this row before installing the upstream package.")
	}
	return strings.Join(parts, "\n\n")
}
