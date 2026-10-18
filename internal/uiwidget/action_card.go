package uiwidget

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"image/color"
)

// ActionCard uses the same panel surface as the downgrader, with one hit target.
type ActionCard struct {
	widget.DisableableWidget
	Content          fyne.CanvasObject
	Title            *widget.Label
	Label            string
	OnTapped         func()
	focused, hovered bool
}

func NewActionCard(label string, content fyne.CanvasObject, action func()) *ActionCard {
	c := &ActionCard{Content: content, Label: label, OnTapped: action}
	c.ExtendBaseWidget(c)
	return c
}
func (c *ActionCard) Tapped(*fyne.PointEvent) {
	if !c.Disabled() && c.Visible() && c.OnTapped != nil {
		c.OnTapped()
	}
}
func (c *ActionCard) Cursor() desktop.Cursor {
	if c.Disabled() {
		return desktop.DefaultCursor
	}
	return desktop.PointerCursor
}
func (c *ActionCard) MouseIn(*desktop.MouseEvent)    { c.hovered = true; c.Refresh() }
func (c *ActionCard) MouseMoved(*desktop.MouseEvent) {}
func (c *ActionCard) MouseOut()                      { c.hovered = false; c.Refresh() }
func (c *ActionCard) FocusGained()                   { c.focused = true; c.Refresh() }
func (c *ActionCard) FocusLost()                     { c.focused = false; c.Refresh() }
func (c *ActionCard) TypedRune(r rune) {
	if r == ' ' {
		c.Tapped(nil)
	}
}
func (c *ActionCard) TypedKey(e *fyne.KeyEvent) {
	if e.Name == fyne.KeyReturn || e.Name == fyne.KeyEnter {
		c.Tapped(nil)
	}
}
func (c *ActionCard) AccessibilityRole() fyne.AccessibleRole { return fyne.AccessibleRoleButton }
func (c *ActionCard) AccessibilityLabel() string {
	if c.Disabled() {
		return c.Label + ", unavailable"
	}
	return c.Label
}
func (c *ActionCard) CreateRenderer() fyne.WidgetRenderer {
	r := &actionCardRenderer{card: c, background: canvas.NewRectangle(color.Transparent)}
	r.Refresh()
	return r
}

type actionCardRenderer struct {
	card       *ActionCard
	background *canvas.Rectangle
}

func (r *actionCardRenderer) Destroy() {}
func (r *actionCardRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.background, r.card.Content}
}
func (r *actionCardRenderer) MinSize() fyne.Size {
	return r.card.Content.MinSize().Add(fyne.NewSquareSize(2))
}
func (r *actionCardRenderer) Layout(s fyne.Size) {
	r.background.Resize(s)
	r.card.Content.Move(fyne.NewPos(1, 1))
	r.card.Content.Resize(fyne.NewSize(max(0, s.Width-2), max(0, s.Height-2)))
}
func (r *actionCardRenderer) Refresh() {
	StylePanel(r.background, r.card.Theme())
	if r.card.hovered && !r.card.Disabled() {
		r.background.FillColor = r.card.Theme().Color(theme.ColorNameSelection, theme.VariantDark)
		r.background.StrokeColor = r.card.Theme().Color(theme.ColorNameDisabled, theme.VariantDark)
	}
	if r.card.Title != nil {
		importance := widget.MediumImportance
		if r.card.Disabled() {
			importance = widget.LowImportance
		}
		r.card.Title.Importance = importance
	}
	if r.card.focused && !r.card.Disabled() {
		r.background.StrokeColor = r.card.Theme().Color(theme.ColorNameForeground, theme.VariantDark)
		r.background.StrokeWidth = 2
	}
	r.background.Refresh()
	r.card.Content.Refresh()
}
