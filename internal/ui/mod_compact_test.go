package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/xxanqw/gtasa-open-downgrader/internal/mods"
	appwidget "github.com/xxanqw/gtasa-open-downgrader/internal/uiwidget"
	"strings"
	"testing"
)

func seedCompactMods(t *testing.T) *Window {
	w := newTestWindow(t)
	verifiedTestGame(t, w)
	w.modTracker = mods.Tracker{Schema: 2, Installed: []string{"ASI Loader", "ModLoader", "SilentPatch"}, Versions: map[string]mods.Release{
		"ASI Loader": {Version: "1.3"}, "ModLoader": {Version: "0.3.8"}, "SilentPatch": {Version: "1.1"},
	}}
	for _, name := range w.modTracker.Installed {
		w.installedMods[name] = true
		w.setModLabel(name, "Installed · "+w.modTracker.Versions[name].Version)
	}
	w.installedModCount = 3
	w.refreshEnabled()
	w.showHub(hubMods)
	return w
}

func TestCompactModsFitFiveRowsAtSmallSize(t *testing.T) {
	w := seedCompactMods(t)
	w.window.Resize(fyne.NewSize(640, 480))
	w.window.Canvas().Capture()
	if w.modTable.scroll.Size().Height < 5*modRowHeight {
		t.Fatalf("only %.1f rows fit", w.modTable.scroll.Size().Height/modRowHeight)
	}
	for _, name := range w.modTableNames {
		row := w.modTable.rows[name]
		if row.Size().Height != 48 {
			t.Fatalf("%s occupies %v", name, row.Size())
		}
		if row.version.Visible() {
			t.Fatal("compact view should put versions in details")
		}
		if row.actions.Position().X < row.status.Position().X+row.status.Size().Width {
			t.Fatal("row actions overlap status")
		}
	}
	if w.bulkActions.Visible() {
		t.Fatal("empty batch actions reserve a lane")
	}
	captureUX(t, w, "mods-compact")
	w.window.Resize(fyne.NewSize(960, 720))
	w.window.Canvas().Capture()
	if !w.modTable.rows["SilentPatch"].version.Visible() {
		t.Fatal("desktop version is hidden")
	}
	captureUX(t, w, "mods-desktop")
}

func TestModExpansionDoesNotSelectAndOnlyOneIsOpen(t *testing.T) {
	w := seedCompactMods(t)
	first, second := w.modTable.rows["SilentPatch"], w.modTable.rows[mods.NadalaoWeapons]
	test.Tap(first.details)
	if first.detailBody.Position().Y < modRowHeight {
		t.Fatal("description overlaps header")
	}
	if first.Position().Y-w.modTable.scroll.Offset.Y > 1 {
		t.Fatal("expanded header did not come into view")
	}
	first.Tapped(&fyne.PointEvent{Position: fyne.NewPos(50, modRowHeight+10)})
	if w.checks["SilentPatch"].Checked {
		t.Fatal("inspecting a mod selected it")
	}
	if !first.detailBody.Visible() || first.Size().Height <= modRowHeight {
		t.Fatal("details did not expand")
	}
	test.Tap(second.details)
	if first.detailBody.Visible() || !second.detailBody.Visible() {
		t.Fatal("more than one expanded mod")
	}
	if !strings.Contains(second.detailText.Text, "helper archive") {
		t.Fatal("helper download is missing")
	}
	if !strings.Contains(second.detailText.Text, "Browser download") {
		t.Fatal("browser requirement is missing")
	}
	w.modFilter.SetSelected("Installed")
	if w.modTable.expanded != "" {
		t.Fatal("filtered-out row retained expansion")
	}
	w.modFilter.SetSelected("All")
	if second.detailBody.Visible() {
		t.Fatal("filtered-out expansion returned")
	}
}

func TestExpandedModKeepsSourceAndDependencyContext(t *testing.T) {
	w := seedCompactMods(t)
	w.modExplicit[mods.AtmosphereHUD] = true
	w.refreshModBatch()
	row := w.modTable.rows[mods.OriginalHQIcons]
	test.Tap(row.details)
	if !strings.Contains(row.detailText.Text, "Selected automatically for") {
		t.Fatal("missing reason for locked dependency")
	}
	if w.checks[mods.OriginalHQIcons].Checked != true || !w.checks[mods.OriginalHQIcons].Disabled() {
		t.Fatal("expansion changed dependency lock")
	}
	w.window.Resize(fyne.NewSize(640, 480))
	w.window.Canvas().Capture()
	if row.detailBody.Size().Width > w.modTable.scroll.Size().Width {
		t.Fatal("details escape compact viewport")
	}
	captureUX(t, w, "mods-required-details")
}

func TestModSelectionActionsAppearInHeader(t *testing.T) {
	w := seedCompactMods(t)
	if w.bulkActions.Visible() {
		t.Fatal("unselected list should not reserve selection actions")
	}
	w.window.Resize(fyne.NewSize(640, 480))
	before := w.app.Driver().AbsolutePositionForObject(w.modTable)
	updatePosition := w.updateModsButton.Position()
	w.modExplicit["SilentPatch"] = true
	w.refreshModBatch()
	if w.updateModsButton.Position().X >= updatePosition.X {
		t.Fatal("Update did not move beside selection actions")
	}
	if w.app.Driver().AbsolutePositionForObject(w.modTable) != before {
		t.Fatal("selection moved the list")
	}
	if w.bulkActions.Position().Y != w.updateModsButton.Position().Y {
		t.Fatal("actions are not beside Update")
	}
	for _, button := range []*appwidget.Button{w.reinstall, w.bulkRemove, w.bulkClear} {
		if !button.Visible() || button.Disabled() {
			t.Fatalf("selection action unavailable: %s", button.Text)
		}
		position := w.app.Driver().AbsolutePositionForObject(button)
		if position.X+button.Size().Width > w.window.Canvas().Size().Width {
			t.Fatal("header action escaped compact view")
		}
	}
	test.Tap(w.bulkClear)
	if len(w.selected()) != 0 || w.bulkActions.Visible() {
		t.Fatal("clear selection did not collapse contextual header")
	}
	if w.updateModsButton.Position() != updatePosition || w.updateModsButton.Text != "Update installed" {
		t.Fatal("Update installed did not return to its original position")
	}
	if w.app.Driver().AbsolutePositionForObject(w.modTable) != before {
		t.Fatal("clearing selection moved the list")
	}
}

func TestCaptureCompactModsExpandedAndMixed(t *testing.T) {
	w := seedCompactMods(t)
	w.setModLabel("SilentPatch", "Installed · 1.1 · Update available: 1.2")
	w.modExplicit["SilentPatch"] = true
	w.modExplicit[mods.NadalaoWeapons] = true
	w.refreshModBatch()
	for _, width := range []float32{640, 960} {
		w.window.Resize(fyne.NewSize(width, 480))
		if width == 640 {
			captureUX(t, w, "mods-mixed-compact")
		} else {
			captureUX(t, w, "mods-mixed-desktop")
		}
	}
	w.window.Resize(fyne.NewSize(640, 480))
	w.modTable.toggleDetails(mods.ExtendedFonts)
	captureUX(t, w, "mods-long-details")
	w.modTable.toggleDetails(mods.Framerate)
	captureUX(t, w, "mods-warning-details")
}

func TestModKeyboardInspectionIsSeparateFromSelection(t *testing.T) {
	w := seedCompactMods(t)
	row := w.modTable.rows["SilentPatch"]
	row.TypedKey(&fyne.KeyEvent{Name: fyne.KeyRight})
	if w.modTable.expanded != "SilentPatch" || w.checks["SilentPatch"].Checked {
		t.Fatal("right arrow should inspect without selecting")
	}
	row.TypedKey(&fyne.KeyEvent{Name: fyne.KeyRight})
	if w.modTable.expanded != "SilentPatch" {
		t.Fatal("right arrow collapsed an open description")
	}
	row.TypedKey(&fyne.KeyEvent{Name: fyne.KeyLeft})
	if w.modTable.expanded != "" {
		t.Fatal("left arrow did not collapse")
	}
	row.TypedRune(' ')
	if !w.modExplicit["SilentPatch"] {
		t.Fatal("space did not select")
	}
	row.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if w.modExplicit["SilentPatch"] {
		t.Fatal("enter did not deselect")
	}
}

func TestExpandedDetailsRelayoutWhenDependencyReasonChanges(t *testing.T) {
	w := seedCompactMods(t)
	w.modExplicit[mods.AtmosphereHUD] = true
	w.refreshModBatch()
	row := w.modTable.rows[mods.OriginalHQIcons]
	w.modTable.toggleDetails(mods.OriginalHQIcons)
	w.bulkClear.OnTapped()
	if row.Size().Height != row.MinSize().Height {
		t.Fatalf("expanded row has stale height: got %v want %v", row.Size().Height, row.MinSize().Height)
	}
	next := w.modTable.rows[mods.AtmosphereHUD]
	if next.Position().Y < row.Position().Y+row.MinSize().Height {
		t.Fatal("updated description overlaps next row")
	}
}

func TestShareModsTooltipIsInformational(t *testing.T) {
	w := seedCompactMods(t)
	w.window.Resize(fyne.NewSize(640, 480))
	for _, source := range mods.Catalog {
		hint := w.modTable.rows[source.Name].downloadHint
		if hint.Visible() != strings.Contains(source.URL, "sharemods.com/") {
			t.Fatalf("incorrect download hint on %s", source.Name)
		}
	}
	row := w.modTable.rows[mods.HDRadarMap]
	w.modTable.scroll.ScrollToOffset(fyne.NewPos(0, row.Position().Y))
	row.downloadHint.FocusGained()
	test.Tap(row.downloadHint)
	if row.downloadHint.popup == nil {
		t.Fatal("tooltip was not shown")
	}
	if w.checks[mods.HDRadarMap].Checked {
		t.Fatal("showing a hint selected the mod")
	}
	captureUX(t, w, "mods-sharemods-tooltip")
	w.showHub(hubTools)
	if row.downloadHint.popup != nil {
		t.Fatal("tooltip survived navigation")
	}
}

func TestModCatalogPlacesWindowedModeAfterWidescreenAndCentersHint(t *testing.T) {
	w := seedCompactMods(t)
	index := -1
	for i, name := range w.modTableNames {
		if name == "Widescreen Fixes" {
			index = i
		}
	}
	if index < 0 || index+1 >= len(w.modTableNames) || w.modTableNames[index+1] != mods.WindowedMode {
		t.Fatal("Windowed Mode is not next to Widescreen Fixes")
	}
	row := w.modTable.rows[mods.NadalaoWeapons]
	hintCenter := row.downloadHint.Position().Y + row.downloadHint.Size().Height/2
	titleCenter := row.title.Position().Y + row.title.MinSize().Height/2
	if hintCenter != titleCenter {
		t.Fatalf("hint center %v differs from title center %v", hintCenter, titleCenter)
	}
	captureUX(t, w, "mods-new-packages")
}
