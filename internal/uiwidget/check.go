package uiwidget

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"image/color"
)

// Check has no pointer halo, only a square keyboard-focus outline.
type Check struct {
	widget.Check
	focused, pointerFocus bool
	OnFocused             func()
}

func NewCheck(text string, changed func(bool)) *Check {
	c := &Check{}
	c.Text = text
	c.OnChanged = changed
	c.ExtendBaseWidget(c)
	return c
}
func (c *Check) Cursor() desktop.Cursor {
	if c.Disabled() {
		return desktop.DefaultCursor
	}
	return desktop.PointerCursor
}
func (*Check) MouseIn(*desktop.MouseEvent)    {}
func (*Check) MouseMoved(*desktop.MouseEvent) {}
func (*Check) MouseOut()                      {}
func (c *Check) Tapped(e *fyne.PointEvent) {
	if c.Disabled() || !c.Visible() {
		return
	}
	c.pointerFocus = true
	c.focused = false
	defer func() { c.pointerFocus = false }()
	c.Check.Tapped(e)
	c.Refresh()
}
func (c *Check) FocusGained() {
	c.focused = !c.pointerFocus && !c.Disabled()
	c.Check.FocusGained()
	if c.OnFocused != nil {
		c.OnFocused()
	}
}
func (c *Check) FocusLost() { c.focused = false; c.Check.FocusLost() }
func (c *Check) Hide()      { c.focused = false; c.Check.Hide(); c.Refresh() }
func (c *Check) TypedKey(key *fyne.KeyEvent) {
	if !ModalEscape(c, key) {
		c.Check.TypedKey(key)
	}
}
func (c *Check) CreateRenderer() fyne.WidgetRenderer {
	r := &checkRenderer{WidgetRenderer: c.Check.CreateRenderer(), check: c, focus: canvas.NewRectangle(color.Transparent)}
	r.focus.StrokeWidth = 1
	r.Refresh()
	return r
}

type checkRenderer struct {
	fyne.WidgetRenderer
	check *Check
	focus *canvas.Rectangle
}

func (r *checkRenderer) Objects() []fyne.CanvasObject {
	objs := append([]fyne.CanvasObject(nil), r.WidgetRenderer.Objects()...)
	objs[0] = r.focus
	return objs
}
func (r *checkRenderer) Layout(s fyne.Size) {
	r.WidgetRenderer.Layout(s)
	icon := r.WidgetRenderer.Objects()[2]
	r.focus.Move(icon.Position().Subtract(fyne.NewPos(2, 2)))
	r.focus.Resize(icon.Size().Add(fyne.NewSquareSize(4)))
}
func (r *checkRenderer) Refresh() {
	r.WidgetRenderer.Refresh()
	if r.check.focused && !r.check.Disabled() && r.check.Visible() {
		r.focus.StrokeColor = r.check.Theme().Color(theme.ColorNameForeground, theme.VariantDark)
		r.focus.Show()
	} else {
		r.focus.StrokeColor = color.Transparent
		r.focus.Hide()
	}
	r.focus.Refresh()
	role := theme.ColorNameForeground
	if r.check.Disabled() {
		role = theme.ColorNameScrollBar
	}
	name := theme.IconNameCheckButton
	if r.check.Checked {
		name = theme.IconNameCheckButtonChecked
	}
	if r.check.Partial {
		name = theme.IconNameCheckButtonPartial
	}
	icon := r.WidgetRenderer.Objects()[2].(*canvas.Image)
	icon.Resource = theme.NewColoredResource(r.check.Theme().Icon(name), role)
	icon.Refresh()
}
