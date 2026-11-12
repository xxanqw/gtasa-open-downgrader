package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/xxanqw/gtasa-open-downgrader/internal/inieditor"
	"github.com/xxanqw/gtasa-open-downgrader/internal/mods"
	appwidget "github.com/xxanqw/gtasa-open-downgrader/internal/uiwidget"
	"image/color"
	"net/url"
	"strings"
)

// A small, bounded catalog does not need a cell grid: reuse one complete row per
// package. Header and body share a layout, while only the body scrolls.
type modWebTable struct {
	widget.BaseWidget
	window   *Window
	header   *modWebRow
	expanded string
	rows     map[string]*modWebRow
	names    []string
	body     *fyne.Container
	scroll   *container.Scroll
}

func newModWebTable(w *Window) *modWebTable {
	t := &modWebTable{window: w, rows: map[string]*modWebRow{}, body: container.New(modWebRowsLayout{})}
	t.header = newModWebRow(w, "")
	t.scroll = container.NewVScroll(t.body)
	t.scroll.OnScrolled = func(fyne.Position) { t.dismissDownloadHints() }
	t.ExtendBaseWidget(t)
	return t
}
func (t *modWebTable) SetNames(names []string) {
	t.dismissDownloadHints()
	changed := len(names) != len(t.names)
	if !changed {
		for i, name := range names {
			if name != t.names[i] {
				changed = true
				break
			}
		}
	}
	if changed {
		objects := make([]fyne.CanvasObject, 0, len(names))
		for _, name := range names {
			row := t.rows[name]
			if row == nil {
				row = newModWebRow(t.window, name)
				t.rows[name] = row
			}
			objects = append(objects, row)
		}
		if t.expanded != "" {
			found := false
			for _, name := range names {
				if name == t.expanded {
					found = true
				}
			}
			if !found {
				t.expanded = ""
			}
		}
		t.names = append(t.names[:0], names...)
		t.body.Objects = objects
	}
	// Hidden pages keep their model current; render their rows when shown.
	// Refreshing the scroll already refreshes its body, so do not refresh each
	// row separately before refreshing the table a second time.
	if t.window.hubSelected == hubMods {
		t.Refresh()
	}
}
func (t *modWebTable) CreateRenderer() fyne.WidgetRenderer { return &modWebTableRenderer{table: t} }

type modWebTableRenderer struct{ table *modWebTable }

func (r *modWebTableRenderer) Destroy()           {}
func (r *modWebTableRenderer) MinSize() fyne.Size { return fyne.NewSize(500, 220) }
func (r *modWebTableRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.table.header, r.table.scroll}
}
func (r *modWebTableRenderer) Layout(s fyne.Size) {
	r.table.header.Move(fyne.NewPos(0, 0))
	r.table.header.Resize(fyne.NewSize(max(float32(500), s.Width-theme.ScrollBarSize()-theme.Padding()), 32))
	r.table.scroll.Move(fyne.NewPos(0, 32))
	r.table.scroll.Resize(fyne.NewSize(s.Width, max(0, s.Height-32)))
}
func (r *modWebTableRenderer) Refresh() {
	r.table.header.Refresh()
	// Dependency/version changes can change the open description's height.
	// Update that row before measuring, without redrawing every row twice.
	if row := r.table.rows[r.table.expanded]; row != nil {
		row.Refresh()
	}
	r.table.body.Layout.Layout(r.table.body.Objects, r.table.body.Size())
	r.table.scroll.Refresh()
}

type modWebRow struct {
	widget.BaseWidget
	window                                         *Window
	name                                           string
	check                                          *modCheck
	title, purpose                                 *widget.Label
	version, versionHeading                        *widget.Label
	detailText                                     *widget.Label
	downloadHint                                   *modDownloadHint
	detailBody                                     *fyne.Container
	state                                          *widget.Label
	icon                                           *widget.Icon
	status                                         *fyne.Container
	details, settings, notice, sortName, sortState *appwidget.Button
	actions                                        *fyne.Container
	hovered, focused                               bool
}

func newModWebRow(w *Window, name string) *modWebRow {
	r := &modWebRow{window: w, name: name, check: newModCheck("", nil), title: compactLabel(modDisplayName(name)), purpose: compactLabel(modPurpose(name)), state: widget.NewLabel(""), icon: widget.NewIcon(theme.ConfirmIcon())}
	r.downloadHint = newModDownloadHint(w)
	r.downloadHint.Hide()
	r.title.TextStyle.Bold = true
	r.purpose.Importance = widget.LowImportance
	r.status = container.New(modStatusLayout{}, r.icon, r.state)
	r.version = compactLabel("")
	r.version.Importance = widget.LowImportance
	r.versionHeading = compactLabel("Version")
	r.detailText = widget.NewLabel("")
	r.detailText.Wrapping = fyne.TextWrapWord
	r.detailText.Selectable = true
	r.detailBody = container.NewVBox(r.detailText)
	for _, source := range mods.Catalog {
		if source.Name == name {
			if strings.Contains(source.URL, "sharemods.com/") {
				r.downloadHint.Show()
			}
			address := source.Page
			if address == "" && source.Repository != "" {
				address = "https://github.com/" + source.Repository
			}
			if address == "" {
				address = source.URL
			}
			if address != "" {
				parsed, _ := url.Parse(address)
				link := appwidget.NewModalLink("Official source", parsed)
				link.OnFocused = func() { appwidget.RevealInScroll(w.modTable.scroll, link) }
				r.detailBody.Add(link)
			}
			break
		}
	}
	r.detailBody.Hide()
	r.details = appwidget.NewButtonWithIcon("", theme.MenuDropDownIcon(), func() { w.modTable.toggleDetails(name) })
	r.details.AccessibleLabel = "Details for " + modDisplayName(name)
	r.details.Importance = widget.LowImportance
	r.notice = appwidget.NewButtonWithIcon("", theme.WarningIcon(), w.showFramerateNotice)
	r.notice.AccessibleLabel = "Read Framerate Vigilante compatibility notice"
	r.notice.Importance = widget.WarningImportance
	r.settings = appwidget.NewButtonWithIcon("", theme.SettingsIcon(), func() { w.showINISettings(name) })
	r.settings.AccessibleLabel = "Edit " + modDisplayName(name) + " INI settings"
	r.settings.Importance = widget.LowImportance
	for _, button := range []*appwidget.Button{r.details, r.settings, r.notice} {
		button.OnFocused = func() { appwidget.RevealInScroll(w.modTable.scroll, button) }
	}
	r.actions = container.New(modRowActionsLayout{}, r.notice, r.settings, r.details)
	r.sortName = appwidget.NewButton("Mod", func() { r.sort(1) })
	r.sortName.Importance = widget.LowImportance
	r.sortName.Alignment = widget.ButtonAlignLeading
	r.sortState = appwidget.NewButton("Status", func() { r.sort(2) })
	r.sortState.Importance = widget.LowImportance
	r.sortState.Alignment = widget.ButtonAlignLeading
	if name == "" {
		r.check.accessibleLabel = "Select all visible mods"
		r.check.OnChanged = func(value bool) {
			if !w.modTableReady() {
				return
			}
			w.setVisibleModSelection(value)
		}
	} else {
		r.check.accessibleLabel = "Select " + modDisplayName(name)
		r.check.OnChanged = func(value bool) {
			if w.modTableReady() && !w.checks[name].Disabled() {
				w.checks[name].SetChecked(value)
			}
		}
	}
	r.ExtendBaseWidget(r)
	return r
}
func (r *modWebRow) sort(col int) {
	w := r.window
	if w.modSortColumn == col {
		w.modSortDescending = !w.modSortDescending
	} else {
		w.modSortColumn = col
		w.modSortDescending = false
	}
	w.rebuildModTable()
}
func (r *modWebRow) selectable() bool {
	return r.name != "" && r.Visible() && r.window.modTableReady() && r.window.checks[r.name].Visible() && !r.window.checks[r.name].Disabled()
}
func (r *modWebRow) Tapped(event *fyne.PointEvent) {
	if event != nil && event.Position.Y >= modRowHeight {
		return
	}
	if r.selectable() {
		c := r.window.checks[r.name]
		c.SetChecked(!c.Checked)
	}
}
func (r *modWebRow) Cursor() desktop.Cursor {
	if r.selectable() {
		return desktop.PointerCursor
	}
	return desktop.DefaultCursor
}
func (r *modWebRow) MouseIn(*desktop.MouseEvent)    { r.hovered = true; r.Refresh() }
func (r *modWebRow) MouseMoved(*desktop.MouseEvent) {}
func (r *modWebRow) MouseOut()                      { r.hovered = false; r.Refresh() }
func (r *modWebRow) FocusGained() {
	r.focused = true
	r.Refresh()
	if r.name != "" {
		appwidget.RevealInScroll(r.window.modTable.scroll, r.details)
	}
}
func (r *modWebRow) FocusLost() { r.focused = false; r.Refresh() }
func (r *modWebRow) TypedRune(ch rune) {
	if ch == ' ' {
		r.Tapped(nil)
	}
}
func (r *modWebRow) TypedKey(e *fyne.KeyEvent) {
	if r.name != "" && (e.Name == fyne.KeyRight || e.Name == fyne.KeyLeft) {
		if (e.Name == fyne.KeyRight) != (r.window.modTable.expanded == r.name) {
			r.window.modTable.toggleDetails(r.name)
		}
		return
	}
	if e.Name == fyne.KeyReturn || e.Name == fyne.KeyEnter {
		r.Tapped(nil)
	}
}
func (r *modWebRow) AccessibilityRole() fyne.AccessibleRole {
	if r.name == "" {
		return fyne.AccessibleRoleContainer
	}
	return fyne.AccessibleRoleButton
}
func (r *modWebRow) AccessibilityLabel() string {
	if r.name == "" {
		return "Mod table header"
	}
	label := modDisplayName(r.name) + ". " + modPurpose(r.name) + ". " + r.window.modLabels[r.name].Text + ". "
	if r.window.modTable.expanded == r.name {
		label += "Details expanded. "
	} else {
		label += "Details collapsed. "
	}
	if required := r.window.modRequiredBy[r.name]; len(required) > 0 {
		return label + "Selected and locked. Required by " + strings.Join(required, ", ")
	}
	if !r.selectable() {
		return label + "Selection unavailable. Use Details for more information."
	}
	if r.window.checks[r.name].Checked {
		return label + "Selected. Activate to deselect."
	}
	return label + "Activate to select."
}
func (r *modWebRow) CreateRenderer() fyne.WidgetRenderer {
	out := &modWebRowRenderer{row: r, background: canvas.NewRectangle(color.Transparent), line: canvas.NewRectangle(color.NRGBA{R: 53, G: 53, B: 53, A: 255}), badge: canvas.NewRectangle(color.Transparent)}
	out.Refresh()
	return out
}

type modWebRowRenderer struct {
	row                     *modWebRow
	background, line, badge *canvas.Rectangle
}

func (r *modWebRowRenderer) Destroy() {}
func (r *modWebRowRenderer) MinSize() fyne.Size {
	if r.row.name == "" {
		return fyne.NewSize(500, 32)
	}
	height := modRowHeight
	if r.row.detailBody.Visible() {
		height += r.row.detailBody.MinSize().Height + 20
	}
	return fyne.NewSize(500, height)
}
func (r *modWebRowRenderer) Objects() []fyne.CanvasObject {
	row := r.row
	if row.name == "" {
		return []fyne.CanvasObject{r.background, r.line, row.check, row.sortName, row.versionHeading, row.sortState}
	}
	return []fyne.CanvasObject{r.background, r.line, r.badge, row.check, row.title, row.downloadHint, row.purpose, row.version, row.status, row.actions, row.detailBody}
}
func (r *modWebRowRenderer) Layout(s fyne.Size) {
	row := r.row
	r.background.Resize(s)
	r.line.Move(fyne.NewPos(0, s.Height-1))
	r.line.Resize(fyne.NewSize(s.Width, 1))
	actionWidth := float32(104)
	statusWidth := float32(136)
	versionWidth := float32(0)
	if s.Width >= 780 {
		versionWidth = 132
		row.version.Show()
		row.versionHeading.Show()
	} else {
		row.version.Hide()
		row.versionHeading.Hide()
	}
	identityWidth := max(float32(180), s.Width-44-statusWidth-versionWidth-actionWidth-12)
	versionX := 44 + identityWidth
	statusX := versionX + versionWidth
	actionX := s.Width - actionWidth - 4
	row.check.Move(fyne.NewPos(6, (modRowHeight-28)/2))
	row.check.Resize(fyne.NewSquareSize(28))
	if row.name == "" {
		label := "Mod"
		if len(row.window.selected()) > 0 && row.window.window.Canvas().Size().Width < 800 {
			label += " · " + row.window.modSelection.Text
		}
		if row.window.modSortColumn == 1 {
			if row.window.modSortDescending {
				label += " ↓"
			} else {
				label += " ↑"
			}
		}
		row.sortName.SetText(label)
		row.check.Move(fyne.NewPos(6, 2))
		row.sortName.Move(fyne.NewPos(38, 0))
		row.sortName.Resize(fyne.NewSize(identityWidth, 32))
		row.versionHeading.Move(fyne.NewPos(versionX, 0))
		row.versionHeading.Resize(fyne.NewSize(versionWidth, 32))
		row.sortState.Move(fyne.NewPos(statusX-6, 0))
		row.sortState.Resize(fyne.NewSize(statusWidth, 32))
		return
	}
	row.title.Move(fyne.NewPos(42, 0))
	titleWidth := identityWidth - 8
	if row.downloadHint.Visible() {
		titleWidth -= 24
		nameWidth := fyne.MeasureText(row.title.Text, row.title.Theme().Size(theme.SizeNameText), row.title.TextStyle).Width
		row.downloadHint.Move(fyne.NewPos(42+min(titleWidth, nameWidth+12)+2, 3))
		row.downloadHint.Resize(fyne.NewSquareSize(16))
	}
	row.title.Resize(fyne.NewSize(titleWidth, 23))
	row.purpose.Move(fyne.NewPos(42, 20))
	row.purpose.Resize(fyne.NewSize(identityWidth-8, 22))
	row.version.Move(fyne.NewPos(versionX, 8))
	row.version.Resize(fyne.NewSize(max(0, versionWidth-8), 32))
	statusHeight := max(float32(28), row.status.MinSize().Height)
	statusY := (modRowHeight - statusHeight) / 2
	r.badge.Move(fyne.NewPos(statusX, statusY))
	r.badge.Resize(fyne.NewSize(min(statusWidth-6, row.status.MinSize().Width+8), statusHeight))
	row.status.Move(fyne.NewPos(statusX+4, statusY))
	row.status.Resize(fyne.NewSize(statusWidth-12, statusHeight))
	row.actions.Move(fyne.NewPos(actionX, (modRowHeight-row.actions.MinSize().Height)/2))
	row.actions.Resize(fyne.NewSize(actionWidth, row.actions.MinSize().Height))
	if row.detailBody.Visible() {
		row.detailBody.Move(fyne.NewPos(42, modRowHeight+6))
		row.detailBody.Resize(fyne.NewSize(max(1, s.Width-58), row.detailBody.MinSize().Height))
	}

}
func (r *modWebRowRenderer) Refresh() {
	row := r.row
	w := row.window
	th := row.Theme()
	r.background.FillColor = th.Color(theme.ColorNameBackground, theme.VariantDark)
	if row.name == "" {
		total, checked := 0, 0
		for _, name := range w.modTableNames {
			if !w.checks[name].Disabled() {
				total++
				if w.checks[name].Checked {
					checked++
				}
			}
		}
		row.check.Checked = checked > 0
		row.check.Partial = checked > 0 && checked < total
		if total > 0 && w.modTableReady() {
			row.check.Enable()
		} else {
			row.check.Disable()
		}
		for col, button := range map[int]*appwidget.Button{1: row.sortName, 2: row.sortState} {
			label := "Mod"
			if col == 2 {
				label = "Status"
			}
			if col == 1 && len(w.selected()) > 0 && w.window.Canvas().Size().Width < 800 {
				label += " · " + w.modSelection.Text
			}
			if w.modSortColumn == col {
				if w.modSortDescending {
					label += " ↓"
				} else {
					label += " ↑"
				}
			}
			button.SetText(label)
		}
	} else {
		logical := w.checks[row.name]
		row.check.Checked = logical.Checked
		row.check.Partial = false
		if logical.Disabled() {
			row.check.Disable()
		} else {
			row.check.Enable()
		}
		row.check.required = len(w.modRequiredBy[row.name]) > 0
		if logical.Checked {
			r.background.FillColor = color.NRGBA{R: 53, G: 45, B: 27, A: 255}
			if row.check.required {
				r.background.FillColor = color.NRGBA{R: 35, G: 35, B: 35, A: 255}
			}
		} else {
			if row.hovered && row.selectable() {
				r.background.FillColor = color.NRGBA{R: 34, G: 34, B: 34, A: 255}
			}
		}
		row.version.SetText(w.modInstalledVersion(row.name))
		if w.modTable.expanded == row.name {
			row.detailText.SetText(w.modDetailText(row.name))
			row.detailBody.Show()
			row.details.SetIcon(theme.MenuDropUpIcon())
			row.details.AccessibleLabel = "Collapse details for " + modDisplayName(row.name)
		} else {
			row.detailBody.Hide()
			row.details.SetIcon(theme.MenuDropDownIcon())
			row.details.AccessibleLabel = "Expand details for " + modDisplayName(row.name)
		}
		state := w.modLabels[row.name]
		row.state.Importance = state.Importance
		row.state.SetText(state.Text)
		row.icon.SetResource(modStateSymbol(state.Text, state.Importance))
		role := theme.ColorNameForeground
		if state.Importance == widget.DangerImportance {
			role = theme.ColorNameError
		}
		if state.Importance == widget.SuccessImportance {
			role = theme.ColorNameSuccess
		} else if state.Importance == widget.WarningImportance {
			role = theme.ColorNameWarning
		}
		tint := color.NRGBAModel.Convert(th.Color(role, theme.VariantDark)).(color.NRGBA)
		tint.A = 22
		r.badge.FillColor = tint
		if len(w.modRequiredBy[row.name]) > 0 {
			row.purpose.SetText("Required automatically · " + modPurpose(row.name))
		} else {
			row.purpose.SetText(modPurpose(row.name))
		}
		row.purpose.Importance = widget.LowImportance
		if row.name == mods.Framerate {
			row.purpose.SetText("Compatibility warning · Higher-FPS fixes")
			row.purpose.Importance = widget.WarningImportance
		}
		if inieditor.Supports(row.name) && w.installedMods[row.name] {
			row.settings.Show()
			if w.modTableReady() {
				row.settings.Enable()
			} else {
				row.settings.Disable()
			}
		} else {
			row.settings.Hide()
		}
		if row.name == mods.Framerate {
			row.notice.Show()
		} else {
			row.notice.Hide()
		}
	}
	if row.focused && row.Visible() {
		r.background.StrokeColor = th.Color(theme.ColorNameForeground, theme.VariantDark)
		r.background.StrokeWidth = 1
	} else {
		r.background.StrokeWidth = 0
	}
	row.check.Refresh()
	r.background.Refresh()
	r.line.Refresh()
	r.badge.Refresh()
	row.status.Refresh()
	row.title.Refresh()
	row.purpose.Refresh()
	row.actions.Refresh()
}

const modRowHeight float32 = 48

type modSelectionToolbarLayout struct{}

func (modSelectionToolbarLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(500, 36) }
func (modSelectionToolbarLayout) Layout(objects []fyne.CanvasObject, s fyne.Size) {
	filter, update, summary, actions := objects[0], objects[1], objects[2], objects[3]
	filterWidth := float32(112)
	filter.Move(fyne.NewPos(0, 0))
	filter.Resize(fyne.NewSize(filterWidth, s.Height))
	right := s.Width
	// One stable line, with selection controls immediately beside Update.
	for _, object := range []fyne.CanvasObject{actions, update} {
		if !object.Visible() {
			continue
		}
		width := object.MinSize().Width
		right -= width
		object.Move(fyne.NewPos(right, 0))
		object.Resize(fyne.NewSize(width, s.Height))
		right -= 8
	}
	if actions.Visible() && s.Width < 772 {
		summary.Hide()
	} else {
		summary.Show()
	}
	summary.Move(fyne.NewPos(filterWidth+10, 0))
	summary.Resize(fyne.NewSize(max(0, right-filterWidth-10), s.Height))
}

type modWebRowsLayout struct{}

func (modWebRowsLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	height := float32(0)
	for _, object := range objects {
		height += object.MinSize().Height
	}
	return fyne.NewSize(500, height)
}
func (modWebRowsLayout) Layout(objects []fyne.CanvasObject, s fyne.Size) {
	y := float32(0)
	for _, object := range objects {
		row := object.(*modWebRow)
		if row.detailBody.Visible() {
			row.detailBody.Resize(fyne.NewSize(max(1, s.Width-58), row.detailBody.MinSize().Height))
		}
		height := row.MinSize().Height
		object.Move(fyne.NewPos(0, y))
		object.Resize(fyne.NewSize(s.Width, height))
		y += height
	}
}

func (t *modWebTable) toggleDetails(name string) {
	if name == "" {
		return
	}
	previous := t.expanded
	t.expanded = name
	if previous == name {
		t.expanded = ""
	}
	for _, key := range []string{previous, name} {
		if row := t.rows[key]; row != nil {
			row.Refresh()
		}
	}
	t.Refresh()
	if row := t.rows[name]; row != nil && t.expanded == name {
		// Reveal the description immediately, rather than only the bottom-most
		// expand button. Tall descriptions retain their header at the top.
		t.scroll.ScrollToOffset(fyne.NewPos(0, row.Position().Y))
	}
}

// Keep expansion at the right edge on every row. Optional settings and warning
// actions have their own slots, so missing actions never move the chevron.
type modRowActionsLayout struct{}

func (modRowActionsLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(104, 32) }
func (modRowActionsLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for index, object := range objects {
		if !object.Visible() {
			continue
		}
		object.Move(fyne.NewPos(size.Width-104+float32(index)*36, (size.Height-32)/2))
		object.Resize(fyne.NewSquareSize(32))
	}
}
