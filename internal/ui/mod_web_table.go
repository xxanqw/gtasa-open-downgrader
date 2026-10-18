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
	"strings"
)

// A small, bounded catalog does not need a cell grid: reuse one complete row per
// package. Header and body share a layout, while only the body scrolls.
type modWebTable struct {
	widget.BaseWidget
	window *Window
	header *modWebRow
	rows   map[string]*modWebRow
	names  []string
	body   *fyne.Container
	scroll *container.Scroll
}

func newModWebTable(w *Window) *modWebTable {
	t := &modWebTable{window: w, rows: map[string]*modWebRow{}, body: container.New(modWebRowsLayout{})}
	t.header = newModWebRow(w, "")
	t.scroll = container.NewVScroll(t.body)
	t.ExtendBaseWidget(t)
	return t
}
func (t *modWebTable) SetNames(names []string) {
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
		t.names = append(t.names[:0], names...)
		t.body.Objects = objects
		t.body.Refresh()
	}
	t.header.Refresh()
	for _, name := range t.names {
		t.rows[name].Refresh()
	}
	t.Refresh()
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
	r.table.header.Resize(fyne.NewSize(max(float32(500), s.Width-theme.ScrollBarSize()-theme.Padding()), 40))
	r.table.scroll.Move(fyne.NewPos(0, 40))
	r.table.scroll.Resize(fyne.NewSize(s.Width, max(0, s.Height-40)))
}
func (r *modWebTableRenderer) Refresh() { r.table.header.Refresh(); r.table.scroll.Refresh() }

type modWebRow struct {
	widget.BaseWidget
	window                                         *Window
	name                                           string
	check                                          *modCheck
	title, purpose                                 *widget.Label
	state                                          *widget.Label
	icon                                           *widget.Icon
	status                                         *fyne.Container
	details, settings, notice, sortName, sortState *appwidget.Button
	actions                                        *fyne.Container
	hovered, focused                               bool
}

func newModWebRow(w *Window, name string) *modWebRow {
	r := &modWebRow{window: w, name: name, check: newModCheck("", nil), title: compactLabel(modDisplayName(name)), purpose: compactLabel(modPurpose(name)), state: widget.NewLabel(""), icon: widget.NewIcon(theme.ConfirmIcon())}
	r.title.TextStyle.Bold = true
	r.purpose.Importance = widget.LowImportance
	r.status = container.New(modStatusLayout{}, r.icon, r.state)
	r.details = appwidget.NewButton("Details", func() { w.showModDetails(name) })
	r.details.AccessibleLabel = "Details for " + modDisplayName(name)
	r.details.Importance = widget.LowImportance
	r.notice = appwidget.NewButton("Read first", w.showFramerateNotice)
	r.notice.AccessibleLabel = "Read Framerate Vigilante compatibility notice"
	r.notice.Importance = widget.LowImportance
	r.settings = appwidget.NewButtonWithIcon("", theme.SettingsIcon(), func() { w.showINISettings(name) })
	r.settings.AccessibleLabel = "Edit " + modDisplayName(name) + " INI settings"
	r.settings.Importance = widget.LowImportance
	for _, button := range []*appwidget.Button{r.details, r.settings, r.notice} {
		button.OnFocused = func() { appwidget.RevealInScroll(w.modTable.scroll, r) }
	}
	r.actions = container.NewHBox(r.details, r.settings, r.notice)
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
func (r *modWebRow) Tapped(*fyne.PointEvent) {
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
		appwidget.RevealInScroll(r.window.modTable.scroll, r)
	}
}
func (r *modWebRow) FocusLost() { r.focused = false; r.Refresh() }
func (r *modWebRow) TypedRune(ch rune) {
	if ch == ' ' {
		r.Tapped(nil)
	}
}
func (r *modWebRow) TypedKey(e *fyne.KeyEvent) {
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
	label := modDisplayName(r.name) + ". " + r.window.modLabels[r.name].Text + ". "
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
	out := &modWebRowRenderer{row: r, background: canvas.NewRectangle(color.Transparent), line: canvas.NewRectangle(color.NRGBA{R: 53, G: 53, B: 53, A: 255}), badge: canvas.NewRectangle(color.Transparent), marker: canvas.NewRectangle(color.Transparent)}
	out.Refresh()
	return out
}

type modWebRowRenderer struct {
	row                             *modWebRow
	background, line, badge, marker *canvas.Rectangle
}

func (r *modWebRowRenderer) Destroy() {}
func (r *modWebRowRenderer) MinSize() fyne.Size {
	if r.row.name == "" {
		return fyne.NewSize(500, 40)
	}
	return fyne.NewSize(500, 64)
}
func (r *modWebRowRenderer) Objects() []fyne.CanvasObject {
	row := r.row
	if row.name == "" {
		return []fyne.CanvasObject{r.background, r.line, row.check, row.sortName, row.sortState, row.window.modStatus}
	}
	return []fyne.CanvasObject{r.background, r.line, r.marker, r.badge, row.check, row.title, row.purpose, row.status, row.actions}
}
func (r *modWebRowRenderer) Layout(s fyne.Size) {
	row := r.row
	r.background.Resize(s)
	r.line.Move(fyne.NewPos(0, s.Height-1))
	r.line.Resize(fyne.NewSize(s.Width, 1))
	r.marker.Resize(fyne.NewSize(2, s.Height))
	actionWidth := float32(198)
	statusWidth := float32(156)
	identityWidth := max(float32(140), s.Width-60-statusWidth-actionWidth-20)
	statusX := 60 + identityWidth
	actionX := s.Width - actionWidth - 8
	row.check.Move(fyne.NewPos(12, (s.Height-28)/2))
	row.check.Resize(fyne.NewSquareSize(28))
	if row.name == "" {
		row.sortName.Move(fyne.NewPos(50, 4))
		row.sortName.Resize(fyne.NewSize(identityWidth, 32))
		row.sortState.Move(fyne.NewPos(statusX-8, 4))
		row.sortState.Resize(fyne.NewSize(statusWidth, 32))
		row.window.modStatus.Move(fyne.NewPos(actionX, 4))
		row.window.modStatus.Resize(fyne.NewSize(actionWidth, 32))
		return
	}
	row.title.Move(fyne.NewPos(54, 5))
	row.title.Resize(fyne.NewSize(identityWidth-10, 26))
	row.purpose.Move(fyne.NewPos(54, 30))
	row.purpose.Resize(fyne.NewSize(identityWidth-10, 26))
	r.badge.Move(fyne.NewPos(statusX, 18))
	r.badge.Resize(fyne.NewSize(min(statusWidth-12, row.status.MinSize().Width+8), 28))
	row.status.Move(fyne.NewPos(statusX+5, 18))
	row.status.Resize(fyne.NewSize(statusWidth-16, 28))
	row.actions.Move(fyne.NewPos(actionX, (s.Height-row.actions.MinSize().Height)/2))
	row.actions.Resize(fyne.NewSize(actionWidth, row.actions.MinSize().Height))
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
			r.marker.FillColor = color.NRGBA{R: 245, G: 213, B: 87, A: 255}
			if row.check.required {
				r.background.FillColor = color.NRGBA{R: 35, G: 35, B: 35, A: 255}
				r.marker.FillColor = color.NRGBA{R: 135, G: 135, B: 135, A: 255}
			}
		} else {
			r.marker.FillColor = color.Transparent
			if row.hovered && row.selectable() {
				r.background.FillColor = color.NRGBA{R: 34, G: 34, B: 34, A: 255}
			}
		}
		state := w.modLabels[row.name]
		row.state.Importance = state.Importance
		row.state.SetText(state.Text)
		row.icon.SetResource(modStateSymbol(state.Text, state.Importance))
		role := theme.ColorNameError
		if state.Importance == widget.SuccessImportance {
			role = theme.ColorNameSuccess
		} else if state.Importance == widget.WarningImportance {
			role = theme.ColorNameWarning
		}
		tint := color.NRGBAModel.Convert(th.Color(role, theme.VariantDark)).(color.NRGBA)
		tint.A = 22
		r.badge.FillColor = tint
		if required := w.modRequiredBy[row.name]; len(required) > 0 {
			row.purpose.SetText("Required (locked) | " + strings.Join(required, ", "))
		} else {
			row.purpose.SetText(modPurpose(row.name))
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
	r.marker.Refresh()
	row.status.Refresh()
	row.title.Refresh()
	row.purpose.Refresh()
	row.actions.Refresh()
}

// Selection controls occupy a permanent lane: changing selection never moves
// the table. Only horizontal space allocation changes with applicable actions.
type modSelectionToolbarLayout struct{}

func (modSelectionToolbarLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(600, 36) }

type modWebRowsLayout struct{}

func (modWebRowsLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(500, float32(len(objects))*64)
}
func (modWebRowsLayout) Layout(objects []fyne.CanvasObject, s fyne.Size) {
	for i, obj := range objects {
		obj.Move(fyne.NewPos(0, float32(i)*64))
		obj.Resize(fyne.NewSize(s.Width, 64))
	}
}
func (modSelectionToolbarLayout) Layout(objects []fyne.CanvasObject, s fyne.Size) {
	filter, update, summary, actions := objects[0], objects[1], objects[2], objects[3]
	filter.Move(fyne.NewPos(0, 2))
	filter.Resize(fyne.NewSize(100, 32))
	update.Move(fyne.NewPos(108, 2))
	update.Resize(fyne.NewSize(130, 32))
	width := float32(0)
	if actions.Visible() {
		width = actions.MinSize().Width
	}
	actions.Move(fyne.NewPos(max(246, s.Width-width), 2))
	actions.Resize(fyne.NewSize(width, 32))
	summary.Move(fyne.NewPos(246, 2))
	summary.Resize(fyne.NewSize(max(0, s.Width-width-254), 32))
}
