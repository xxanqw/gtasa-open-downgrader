package uiwidget

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// ModalAction is explicit: Return never silently confirms a destructive action.
// An empty ID is cancellation. Actions may keep a guide open for async work.
type ModalAction struct {
	ID, Text   string
	Importance widget.Importance
	KeepOpen   bool
	Disabled   bool
	Run        func(*Modal)
}
type ModalOptions struct {
	Title, Subtitle string
	Body            fyne.CanvasObject
	Size            fyne.Size
	Actions         []ModalAction
	Extra           fyne.CanvasObject
	// Accessory stays visible below the scrolling body (e.g. explicit consent).
	Accessory    fyne.CanvasObject
	InitialFocus fyne.Focusable
	// Nested is for results/errors produced by an open sheet (e.g. a license export).
	Nested   bool
	OnClosed func(string)
}

// Modal is a theme-aware, canvas-bounded sheet. Calls belong on Fyne's UI
// goroutine; async jobs post updates with fyne.Do. Sheets queue per window.
type Modal struct {
	window               fyne.Window
	parent               *Modal
	options              ModalOptions
	overlay              *modalOverlay
	header, body, footer fyne.CanvasObject
	title, subtitle      *widget.Label
	closeButton          *Button
	actions              []*Button
	closed, shown        bool
	oldKey               func(*fyne.KeyEvent)
	oldFocus             fyne.Focusable
	cancelAction         func()
}
type modalQueue struct {
	active  *Modal
	pending []*Modal
}

var modalQueues = map[fyne.Window]*modalQueue{}

func ShowModal(window fyne.Window, options ModalOptions) *Modal {
	m := &Modal{window: window, options: options}
	if m.options.Size.Width <= 0 {
		m.options.Size.Width = 600
	}
	if m.options.Body == nil {
		m.options.Body = ModalText("")
	}
	if len(m.options.Actions) == 0 {
		m.options.Actions = []ModalAction{{Text: "Close"}}
	}
	q := modalQueues[window]
	if q == nil {
		q = &modalQueue{}
		modalQueues[window] = q
	}
	if options.Nested && q.active != nil {
		m.parent = q.active
		q.active = m
		m.show()
	} else {
		q.pending = append(q.pending, m)
		showNextModal(window)
	}
	return m
}
func showNextModal(window fyne.Window) {
	q := modalQueues[window]
	if q == nil || q.active != nil {
		return
	}
	for len(q.pending) > 0 {
		m := q.pending[0]
		q.pending = q.pending[1:]
		if m.closed {
			continue
		}
		q.active = m
		m.show()
		return
	}
	delete(modalQueues, window)
}
func (m *Modal) show() {
	c := m.window.Canvas()
	m.oldKey = c.OnTypedKey()
	m.oldFocus = c.Focused()
	m.title = widget.NewLabelWithStyle(m.options.Title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	m.title.Wrapping = fyne.TextWrapWord
	m.title.SizeName = theme.SizeNameSubHeadingText
	m.subtitle = widget.NewLabel(m.options.Subtitle)
	m.subtitle.Wrapping = fyne.TextWrapWord
	m.subtitle.Importance = widget.LowImportance
	if m.options.Subtitle == "" {
		m.subtitle.Hide()
	}
	m.closeButton = NewButtonWithIcon("", theme.CancelIcon(), func() { m.Cancel() })
	m.closeButton.AccessibleLabel = "Close " + m.options.Title
	m.closeButton.Importance = widget.LowImportance
	m.header = container.NewVBox(container.NewBorder(nil, nil, nil, m.closeButton, container.NewVBox(m.title, m.subtitle)), widget.NewSeparator())
	m.body = m.options.Body
	if scroll, ok := m.body.(*container.Scroll); ok {
		bindGuideFocus(scroll, scroll.Content)
	}
	m.footer = m.actionBar()
	m.overlay = &modalOverlay{modal: m}
	m.overlay.ExtendBaseWidget(m.overlay)
	m.shown = true
	c.Overlays().Add(m.overlay)
	m.overlay.Resize(c.Size())
	c.SetOnTypedKey(func(k *fyne.KeyEvent) {
		if k.Name == fyne.KeyEscape {
			m.Cancel()
		}
	})
	focus := m.options.InitialFocus
	if focus == nil && len(m.actions) > 0 {
		focus = m.actions[0]
	}
	if focus == nil {
		focus = m.closeButton
	}
	c.Focus(focus)
}
func (m *Modal) actionBar() fyne.CanvasObject {
	var objects []fyne.CanvasObject
	if m.options.Extra != nil {
		objects = append(objects, m.options.Extra)
	}
	m.actions = nil
	for _, spec := range m.options.Actions {
		action := spec
		button := NewButton(action.Text, func() {
			if m.closed {
				return
			}
			if !action.KeepOpen {
				m.Close(action.ID)
			}
			if action.Run != nil {
				action.Run(m)
			}
		})
		button.Importance = action.Importance
		if action.Disabled {
			button.Disable()
		}
		m.actions = append(m.actions, button)
		objects = append(objects, button)
	}
	return container.New(&modalActionsLayout{}, objects...)
}
func (m *Modal) Cancel() {
	if !m.closed {
		if m.cancelAction != nil {
			m.cancelAction()
		} else {
			m.Close("")
		}
	}
}

// SetCancel lets a running guide request cancellation without abandoning cleanup.
func (m *Modal) SetCancel(cancel func()) { m.cancelAction = cancel }
func (m *Modal) SetBody(body fyne.CanvasObject) {
	if m.closed || body == nil {
		return
	}
	m.body = body
	m.options.Body = body
	if scroll, ok := body.(*container.Scroll); ok {
		bindGuideFocus(scroll, scroll.Content)
	}
	if m.shown {
		m.overlay.Refresh()
	}
}
func (m *Modal) SetAccessory(object fyne.CanvasObject) {
	m.options.Accessory = object
	if m.shown && !m.closed {
		m.overlay.Refresh()
	}
}
func (m *Modal) SetSubtitle(text string) {
	if m.closed {
		return
	}
	m.options.Subtitle = text
	if m.subtitle != nil {
		m.subtitle.SetText(text)
		if text == "" {
			m.subtitle.Hide()
		} else {
			m.subtitle.Show()
		}
		m.overlay.Refresh()
	}
}
func (m *Modal) SetActions(actions ...ModalAction) {
	m.options.Actions = actions
	if m.shown && !m.closed {
		focus := m.window.Canvas().Focused()
		replaceFocus := false
		for _, button := range m.actions {
			if focus == button {
				replaceFocus = true
			}
		}
		m.footer = m.actionBar()
		m.overlay.Refresh()
		if replaceFocus && len(m.actions) > 0 {
			m.window.Canvas().Focus(m.actions[0])
		}
	}
}
func (m *Modal) SetActionEnabled(id string, enabled bool) {
	for i := range m.options.Actions {
		if m.options.Actions[i].ID == id {
			m.options.Actions[i].Disabled = !enabled
			if i < len(m.actions) {
				if enabled {
					m.actions[i].Enable()
				} else {
					m.actions[i].Disable()
				}
			}
		}
	}
}
func (m *Modal) Close(result string) {
	if m.closed {
		return
	}
	q := modalQueues[m.window]
	// Never leave an active child pointing at a removed parent, or restore its
	// key handler over the child. An async child may defer closure for cleanup.
	if m.shown && q != nil {
		for q.active != nil && q.active != m {
			descendant := false
			for p := q.active.parent; p != nil; p = p.parent {
				if p == m {
					descendant = true
					break
				}
			}
			if !descendant {
				break
			}
			active := q.active
			active.Cancel()
			if q.active == active || m.closed {
				return
			}
		}
	}
	m.closed = true
	if m.shown {
		c := m.window.Canvas()
		c.Overlays().Remove(m.overlay)
		c.SetOnTypedKey(m.oldKey)
		if q != nil && q.active == m {
			q.active = m.parent
		}
	}
	if m.options.OnClosed != nil {
		m.options.OnClosed(result)
	}
	if q = modalQueues[m.window]; q != nil && q.active == nil {
		showNextModal(m.window)
	}
	canRestore := modalQueues[m.window] == nil || (modalQueues[m.window] != nil && modalQueues[m.window].active == m.parent)
	if m.shown && canRestore && m.oldFocus != nil {
		if o, ok := m.oldFocus.(fyne.CanvasObject); ok && o.Visible() && fyne.CurrentApp().Driver().CanvasForObject(o) == m.window.Canvas() {
			if d, ok := o.(fyne.Disableable); !ok || !d.Disabled() {
				m.window.Canvas().Focus(m.oldFocus)
			}
		}
	}
}
func DismissModals(window fyne.Window) {
	q := modalQueues[window]
	if q == nil {
		return
	}
	pending := append([]*Modal(nil), q.pending...)
	q.pending = nil
	for _, m := range pending {
		m.Close("")
	}
	for q.active != nil {
		active := q.active
		active.Cancel()
		if q.active == active {
			break
		}
	}
}
func ModalOpen(window fyne.Window) bool { q := modalQueues[window]; return q != nil && q.active != nil }

// ModalEscape is used by shared focused inputs, avoiding private Fyne key state.
func ModalEscape(object fyne.CanvasObject, key *fyne.KeyEvent) bool {
	if key.Name != fyne.KeyEscape {
		return false
	}
	c := fyne.CurrentApp().Driver().CanvasForObject(object)
	if c == nil {
		return false
	}
	for window, q := range modalQueues {
		if window.Canvas() == c && q.active != nil {
			q.active.Cancel()
			return true
		}
	}
	return false
}

type modalOverlay struct {
	widget.BaseWidget
	modal *Modal
}

func (o *modalOverlay) AccessibilityRole() fyne.AccessibleRole { return fyne.AccessibleRoleContainer }
func (o *modalOverlay) AccessibilityLabel() string             { return o.modal.options.Title + ", dialog" }
func (o *modalOverlay) Tapped(*fyne.PointEvent)                {} // Consume outside taps, never confirm or abandon work.
func (o *modalOverlay) CreateRenderer() fyne.WidgetRenderer {
	r := &modalRenderer{overlay: o, shade: canvas.NewRectangle(color.NRGBA{A: 140}), surface: canvas.NewRectangle(color.Transparent), body: container.NewWithoutLayout()}
	r.Refresh()
	return r
}

type modalRenderer struct {
	overlay        *modalOverlay
	shade, surface *canvas.Rectangle
	body           *fyne.Container
}

func (r *modalRenderer) Destroy()           {}
func (r *modalRenderer) MinSize() fyne.Size { return fyne.NewSize(1, 1) }
func (r *modalRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.shade, r.surface, r.body}
}
func (r *modalRenderer) Layout(size fyne.Size) {
	m := r.overlay.modal
	r.shade.Resize(size)
	width := max(1, min(m.options.Size.Width, size.Width-24))
	available := max(1, size.Height-24)
	inner := max(1, width-32)
	m.header.Resize(fyne.NewSize(inner, m.header.MinSize().Height))
	// Wrapped title/subtitle minima change after the final width reaches their
	// text providers; settle them before reserving body and footer space.
	m.header.Refresh()
	headerHeight := m.header.MinSize().Height
	footerHeight := modalActionsHeight(m.footer.(*fyne.Container).Objects, inner)
	accessoryHeight := float32(0)
	if m.options.Accessory != nil && m.options.Accessory.Visible() {
		accessoryHeight = m.options.Accessory.MinSize().Height + 10
	}
	m.body.Resize(fyne.NewSize(inner, max(1, available-headerHeight-footerHeight-accessoryHeight-52)))
	naturalBody := m.body.MinSize().Height
	if scroll, ok := m.body.(*container.Scroll); ok {
		naturalBody = scroll.Content.MinSize().Height
	} else if form, ok := m.body.(*fyne.Container); ok {
		for _, child := range form.Objects {
			if scroll, ok := child.(*container.Scroll); ok {
				naturalBody += scroll.Content.MinSize().Height - scroll.MinSize().Height
			}
		}
	}
	height := m.options.Size.Height
	if height <= 0 {
		height = naturalBody + headerHeight + footerHeight + accessoryHeight + 52
	}
	height = max(1, min(available, height))
	pos := fyne.NewPos((size.Width-width)/2, (size.Height-height)/2)
	contentSize := fyne.NewSize(width, height)
	r.body.Move(pos)
	r.body.Resize(contentSize)
	r.surface.Move(pos)
	r.surface.Resize(contentSize)
	m.header.Move(fyne.NewPos(16, 14))
	m.header.Resize(fyne.NewSize(inner, headerHeight))
	m.footer.Move(fyne.NewPos(16, height-14-footerHeight))
	m.footer.Resize(fyne.NewSize(inner, footerHeight))
	if accessoryHeight > 0 {
		m.options.Accessory.Move(fyne.NewPos(16, height-14-footerHeight-accessoryHeight))
		m.options.Accessory.Resize(fyne.NewSize(inner, accessoryHeight-10))
	}
	m.body.Move(fyne.NewPos(16, 14+headerHeight+12))
	m.body.Resize(fyne.NewSize(inner, max(1, height-headerHeight-footerHeight-accessoryHeight-52)))
}
func (r *modalRenderer) Refresh() {
	m := r.overlay.modal
	th := r.overlay.Theme()
	r.surface.FillColor = th.Color(theme.ColorNameOverlayBackground, theme.VariantDark)
	r.surface.StrokeColor = th.Color(theme.ColorNameInputBorder, theme.VariantDark)
	r.surface.StrokeWidth = 1
	r.surface.CornerRadius = 0
	r.body.Objects = []fyne.CanvasObject{m.header, m.body}
	if m.options.Accessory != nil {
		r.body.Objects = append(r.body.Objects, m.options.Accessory)
	}
	r.body.Objects = append(r.body.Objects, m.footer)
	r.Layout(r.overlay.Size())
	r.body.Refresh()
	r.Layout(r.overlay.Size())
	r.surface.Refresh()
	r.shade.Refresh()
}

// Wrap footer actions without allowing long buttons to escape a small canvas.
func modalActionsHeight(objects []fyne.CanvasObject, width float32) float32 {
	height := float32(0)
	rowHeight := float32(32)
	used := float32(0)
	for _, o := range objects {
		if !o.Visible() {
			continue
		}
		w := min(width, o.MinSize().Width)
		if used > 0 && used+6+w > width {
			height += rowHeight + 6
			used = 0
			rowHeight = 32
		}
		if used > 0 {
			used += 6
		}
		used += w
		rowHeight = max(rowHeight, o.MinSize().Height)
	}
	return height + rowHeight
}

type modalActionsLayout struct{}

func (*modalActionsLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	h := float32(32)
	w := float32(0)
	for _, o := range objects {
		if o.Visible() {
			h = max(h, o.MinSize().Height)
			w = max(w, o.MinSize().Width)
		}
	}
	return fyne.NewSize(w, h)
}
func (*modalActionsLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	var rows [][]fyne.CanvasObject
	var row []fyne.CanvasObject
	used := float32(0)
	for _, o := range objects {
		if !o.Visible() {
			continue
		}
		w := min(size.Width, o.MinSize().Width)
		if len(row) > 0 && used+6+w > size.Width {
			rows = append(rows, row)
			row = nil
			used = 0
		}
		row = append(row, o)
		if used > 0 {
			used += 6
		}
		used += w
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}
	y := float32(0)
	for _, row := range rows {
		width := float32(0)
		height := float32(32)
		for i, o := range row {
			width += min(size.Width, o.MinSize().Width)
			if i > 0 {
				width += 6
			}
			height = max(height, o.MinSize().Height)
		}
		x := max(0, size.Width-width)
		for _, o := range row {
			w := min(size.Width, o.MinSize().Width)
			o.Move(fyne.NewPos(x, y))
			o.Resize(fyne.NewSize(w, height))
			x += w + 6
		}
		y += height + 6
	}
}
