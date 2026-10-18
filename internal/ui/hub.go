package ui

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/xxanqw/gtasa-open-downgrader/internal/game"
	modpkg "github.com/xxanqw/gtasa-open-downgrader/internal/mods"
	appwidget "github.com/xxanqw/gtasa-open-downgrader/internal/uiwidget"
)

const (
	downgradeVerifyPending = "Patch applied. Verifying the target hashes…"
	hubDowngrade           = "Downgrade"
	hubMods                = "Mods"
	hubMultiplayer         = "Multiplayer"
	hubRecovery            = "Recovery"
	hubTools               = "Tools"
	hubAbout               = "About"
)

func hubText(text string) *widget.Label {
	label := widget.NewLabel(text)
	label.Wrapping = fyne.TextWrapWord
	return label
}

func hubSection(title string, objects ...fyne.CanvasObject) fyne.CanvasObject {
	heading := widget.NewLabelWithStyle(strings.ToUpper(title), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	body := container.NewVBox(append([]fyne.CanvasObject{heading}, objects...)...)
	return container.New(layout.NewCustomPaddedLayout(4, 4, 0, 0),
		newFramedPanel(container.New(layout.NewCustomPaddedLayout(12, 12, 14, 14), body)))
}

func (w *Window) hubPage(body ...fyne.CanvasObject) fyne.CanvasObject {
	scroll := container.NewVScroll(container.New(layout.NewCustomPaddedLayout(14, 12, 14, 12), container.NewVBox(body...)))
	bindPageFocus(scroll, scroll.Content)
	return scroll
}

// Traverse our own composition only, never Fyne's private renderer state.
func bindPageFocus(scroll *container.Scroll, object fyne.CanvasObject) {
	switch item := object.(type) {
	case *fyne.Container:
		for _, child := range item.Objects {
			bindPageFocus(scroll, child)
		}
	case *framedPanel:
		bindPageFocus(scroll, item.content)
	case *appwidget.Button:
		item.OnFocused = func() { appwidget.RevealInScroll(scroll, item) }
	}
}

func (w *Window) buildHub() fyne.CanvasObject {
	w.hubReadiness = hubText("Select a game folder and scan it to get started.")
	w.hubReadiness.Wrapping = fyne.TextWrapOff
	w.hubReadiness.Truncation = fyne.TextTruncateEllipsis
	w.hubBackups = hubText("Scan a game to check for downgrade backups.")
	w.patchButton = appwidget.NewButtonWithIcon("Download patches", theme.DownloadIcon(), w.downloadPatches)
	fileResults := container.NewThemeOverride(container.New(&fileTableLayout{table: w.table, detailed: func() bool { return w.checksums }}, w.table), fileTableTheme{w.app.Settings().Theme()})
	copyDetails := appwidget.NewButtonWithIcon("Copy file report", theme.ContentCopyIcon(), func() {
		w.window.Clipboard().SetContent(fileReport(w.rows))
		w.setStatus("File report copied.")
	})
	w.checksumCheck = newModCheck("Show checksums", func(on bool) { w.checksums = on; w.table.Refresh(); fileResults.Refresh() })
	downgradeBody := container.NewBorder(
		container.NewVBox(
			container.NewBorder(nil, nil, w.checksumCheck, container.NewHBox(copyDetails), layout.NewSpacer()),
			w.patchButton, w.hubReadiness,
		),
		nil, nil, nil, fileResults,
	)
	downgrade := container.NewBorder(w.gameHeader, nil, nil, nil,
		container.New(layout.NewCustomPaddedLayout(10, 8, 14, 14),
			newFramedPanel(container.New(layout.NewCustomPaddedLayout(12, 12, 14, 14), downgradeBody))))

	modToolbar := container.New(layout.NewCustomPaddedLayout(4, 0, 14, 14), container.NewVBox(
		container.New(modSelectionToolbarLayout{}, styleInput(w.modFilter), w.updateModsButton, w.modSelection, w.bulkActions),
		w.modEmpty,
	))
	mods := container.NewBorder(modToolbar, nil, nil, nil, container.New(layout.NewCustomPaddedLayout(0, 0, 14, 14), w.modPanel))

	w.multiplayerBody = container.NewVBox(hubText("Scan a game to view multiplayer options."))
	multiplayerPage := w.hubPage(w.multiplayerBody)

	restore := appwidget.NewButtonWithIcon("Restore original files", theme.HistoryIcon(), func() { w.runHubTool("Restore downgrade backups") })
	deleteBackups := appwidget.NewButton("Delete backups", func() { w.runHubTool("Delete downgrade backups") })
	clearData := appwidget.NewButton("Clear saves & settings", func() { w.runHubTool("Clear saves, settings and gallery") })
	w.recoveryActions = []fyne.Disableable{restore, deleteBackups, clearData}
	w.legacyRussianRemove = appwidget.NewButton("Remove legacy SanLtd Russian", func() { w.uninstallMod(modpkg.SanLtdRussian) })
	w.legacyRussianSection = hubSection("Retired translation", hubText("SanLtd Russian is no longer offered. ELL includes Russian. Existing files are left untouched unless you remove them here."), w.legacyRussianRemove)
	w.legacyRussianSection.Hide()
	recovery := w.hubPage(
		hubSection("Downgrade backups", w.hubBackups, container.NewHBox(restore, deleteBackups)),
		hubSection("Saves & settings", hubText("Deleting user data cannot be undone. Back up files first."), container.NewHBox(clearData)),
		w.legacyRussianSection,
	)
	tools := w.buildToolsPage()
	about := w.buildAboutPage()

	w.hubPages = map[string]fyne.CanvasObject{hubDowngrade: downgrade, hubMods: mods, hubMultiplayer: multiplayerPage, hubRecovery: recovery, hubTools: tools, hubAbout: about}
	w.hubNav = map[string]*appwidget.Button{}
	icons := map[string]fyne.Resource{
		hubDowngrade:   appwidget.GameIcon(),
		hubMods:        appwidget.ModsIcon(),
		hubMultiplayer: appwidget.MultiplayerIcon(),
		hubRecovery:    theme.HistoryIcon(),
		hubTools:       appwidget.ToolsIcon(),
	}
	pages := []string{hubDowngrade, hubMods, hubMultiplayer, hubRecovery, hubTools}
	for _, name := range pages {
		page := name
		label := page
		if page == hubDowngrade {
			label = "Game"
		}
		button := appwidget.NewButtonWithIcon(label, icons[page], func() { w.showHub(page) })
		w.hubNav[page] = button
	}
	aboutButton := appwidget.NewButtonWithIcon(hubAbout, theme.InfoIcon(), func() { w.showHub(hubAbout) })
	aboutButton.Alignment = widget.ButtonAlignLeading
	w.hubNav[hubAbout] = aboutButton
	w.controls = append(w.controls, w.patchButton)
	w.hubContent = container.NewStack(downgrade, mods, multiplayerPage, recovery, tools, about)
	w.showHub(hubDowngrade)
	// Keep every destination visible, with flexible space between the groups.
	left := container.NewHBox(w.hubNav[hubDowngrade], w.hubNav[hubMods], w.hubNav[hubMultiplayer])
	right := container.NewHBox(w.hubNav[hubTools], w.hubNav[hubRecovery], aboutButton)
	w.navigation = container.New(layout.NewCustomPaddedLayout(6, 0, 14, 14), container.NewBorder(nil, nil, left, right, nil))
	return w.hubContent
}

func (w *Window) showHub(page string) {
	if w.modFilter != nil {
		w.modFilter.closePopup()
	}
	if w.hubPages == nil || w.hubPages[page] == nil || requiresDowngradePage(page) && !w.downgradeComplete() {
		return
	}
	for name, content := range w.hubPages {
		if name == page {
			content.Show()
		} else {
			content.Hide()
		}
	}
	w.hubSelected = page
	if page == hubDowngrade {
		w.gameHeader.Show()
	} else {
		w.gameHeader.Hide()
	}
	for name, button := range w.hubNav {
		button.Importance = widget.LowImportance
		if name == page {
			button.Importance = widget.HighImportance
		}
		button.Refresh()
	}
	w.hubContent.Refresh()
	w.refreshFooterActions()
	if page == hubMultiplayer && w.scanPath != "" && !w.busy && w.multiplayerBody != nil {
		w.multiplayer()
	}
}

func (w *Window) refreshFooterActions() {
	if w.footerActions == nil {
		return
	}
	if w.hubSelected == hubDowngrade && !w.busy {
		w.downgrade.Show()
	} else {
		w.downgrade.Hide()
	}
	w.footerLogButton.Show()
	w.footerActions.Refresh()
	w.footer.Refresh()
}

func requiresDowngradePage(page string) bool {
	return page == hubMods || page == hubMultiplayer
}

// A successful operation is not enough: use a fresh scan of this folder's
// target hashes, including the supported LAA executable, before unlocking.
func verifiedDowngrade(rows []game.Row) bool {
	if len(rows) == 0 {
		return false
	}
	verified := false
	for _, row := range rows {
		if row.Status == "Not in this edition" && row.NeedsPatch == "No" {
			continue
		}
		if !strings.HasPrefix(row.Status, "Already Downgraded") || row.NeedsPatch != "No" || !validMD5(row.CurrentHash) || !validMD5(row.TargetHash) {
			return false
		}
		if row.CurrentHash != row.TargetHash && !(game.IsEXE(row.Path) && row.CurrentHash == game.LAAHash) {
			return false
		}
		verified = true
	}
	return verified
}

func validMD5(hash string) bool {
	decoded, err := hex.DecodeString(hash)
	return err == nil && len(decoded) == 16
}

func (w *Window) downgradeComplete() bool {
	return w.scanPath != "" && w.scanPath == w.path.Text && verifiedDowngrade(w.rows)
}

func (w *Window) refreshHubNavigation() {
	if w.hubNav == nil {
		return
	}
	complete := w.downgradeComplete()
	if !complete && requiresDowngradePage(w.hubSelected) {
		w.showHub(hubDowngrade)
	}
	for _, page := range []string{hubMods, hubMultiplayer} {
		if complete {
			w.hubNav[page].Enable()
		} else {
			w.hubNav[page].Disable()
		}
	}
}

func (w *Window) runHubTool(name string) {
	if toolNeedsDowngrade(name) && !w.downgradeComplete() {
		w.info("Downgrade required", "Finish downgrading and scan the game before using this tool.")
		return
	}
	for _, action := range w.toolActions() {
		if action.label == name {
			action.run()
			return
		}
	}
}

func fileCounts(rows []game.Row) (pending, modified, missing, done, unknown int) {
	for _, row := range rows {
		switch {
		case row.Status == "Missing":
			missing++
		case row.Status == "Source edition unknown":
			unknown++
		case row.Status == "Modified":
			modified++
		case strings.HasPrefix(row.Status, "Already Downgraded"):
			done++
		case row.NeedsPatch == "Yes":
			pending++
		}
	}
	return
}

func hasDowngradeBackups(root string) bool {
	entries, err := os.ReadDir(filepath.Join(root, "backups"))
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.Name() != "mods" {
			return true
		}
	}
	return false
}

func (w *Window) refreshHub() {
	if w.hubReadiness == nil {
		return
	}
	pending, modified, missing, _, unknown := fileCounts(w.rows)
	ready := w.scanPath != "" && w.scanPath == w.path.Text
	switch {
	case w.busy:
		w.hubReadiness.SetText(w.status.Text)
	case w.lastDowngradeOutcome != "":
		w.hubReadiness.SetText(w.lastDowngradeOutcome)
	case !ready:
		if strings.TrimSpace(w.path.Text) == "" {
			w.hubReadiness.SetText("Choose your GTA San Andreas folder, then scan it.")
		} else {
			w.hubReadiness.SetText("Scan this folder to check the game version and files.")
		}
	case w.readonly:
		w.hubReadiness.SetText("This game folder is read-only. Choose a writable installation before changing files.")
	case w.patchDir == "":
		w.hubReadiness.SetText("Patch assets are missing or outdated. Download v2.1 to downgrade this game.")
	case unknown > 0:
		w.hubReadiness.SetText("This source edition is not supported by the current patch set. No files will be changed.")
	case missing > 0:
		w.hubReadiness.SetText("Some game files are missing. Review file details before downgrading.")
	case modified > 0:
		w.hubReadiness.SetText("Modified files detected. Review the change plan; patch output must match the verified target.")
	case pending > 0:
		w.hubReadiness.SetText("Downgrade to unlock Mods and Multiplayer. Originals will be saved in backups/.")
	case w.downgradeComplete():
		w.hubReadiness.SetText("Downgrade verified. Mods and Multiplayer are available.")
	default:
		w.hubReadiness.SetText("Finish the downgrade to unlock Mods and Multiplayer.")
	}
	// The file state and footer action already express the normal ready state.
	// Reserve this line for blockers, unusual scans and active operations.
	wasVisible := w.hubReadiness.Visible()
	if !ready || w.busy || w.lastDowngradeOutcome != "" || (ready && !w.downgradeComplete()) || (ready && (w.readonly || w.patchDir == "")) {
		w.hubReadiness.Show()
	} else {
		w.hubReadiness.Hide()
	}
	if wasVisible != w.hubReadiness.Visible() && w.hubPages != nil {
		w.hubPages[hubDowngrade].Refresh() // Reclaim or reserve the row above the hash table.
	}
	if w.patchDir == "" && !w.busy {
		w.patchButton.Show()
	} else {
		w.patchButton.Hide()
	}
	if ready && !w.readonly && (len(w.rows) == 0 || unknown > 0 || missing > 0 || (pending == 0 && modified == 0)) {
		w.downgrade.Disable()
	}
	backups := ready && hasDowngradeBackups(w.scanPath)
	for index, action := range w.recoveryActions {
		if ready && !w.readonly && !w.busy && !w.promptActive && (index == 2 || backups) {
			action.Enable()
		} else {
			action.Disable()
		}
	}
	if !ready {
		w.hubBackups.SetText("Scan a game to check for backups.")
	} else if backups {
		w.hubBackups.SetText("Original files are stored under backups/ in this game folder. Restore does not remove installed mods.")
	} else {
		w.hubBackups.SetText("No downgrade backups found in this game folder.")
	}
}

func fileReport(rows []game.Row) string {
	var report strings.Builder
	for _, row := range rows {
		fmt.Fprintf(&report, "%s\nStatus: %s · Patch: %s\nCurrent MD5: %s\nTarget MD5: %s\n\n", row.Path, row.Status, row.NeedsPatch, row.CurrentHash, row.TargetHash)
	}
	return report.String()
}

func downgradeReview(rows []game.Row) string {
	pending, modified, missing, _, _ := fileCounts(rows)
	count := pending + modified
	name := "game file"
	if count != 1 {
		name += "s"
	}
	message := fmt.Sprintf("Downgrade %d %s to v1.0 US?", count, name)
	if modified > 0 {
		message += " Modified files may need to be restored first."
	}
	if missing > 0 {
		message += " Missing files may prevent completion."
	}
	return message + "\n\nOriginals are saved in backups/. Cancelling won't undo files already changed."
}
