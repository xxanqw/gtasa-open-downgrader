package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"image/color"
)

// Non-focusable menu rows keep keyboard focus on the Select, just like native
// menu items. Tab/focus loss therefore closes the list rather than stranding
// its overlay or sending Escape to a surrounding dialog.
type selectOption struct {
	widget.BaseWidget
	text              string
	selected, active  bool
	OnTapped, onHover func()
}

func (o *selectOption) AccessibilityRole() fyne.AccessibleRole { return fyne.AccessibleRoleButton }
func (o *selectOption) AccessibilityLabel() string {
	if o.selected {
		return o.text + ", selected"
	}
	return o.text
}
func (o *selectOption) Tapped(*fyne.PointEvent)        { o.OnTapped() }
func (o *selectOption) MouseIn(*desktop.MouseEvent)    { o.onHover() }
func (o *selectOption) MouseMoved(*desktop.MouseEvent) {}
func (o *selectOption) MouseOut()                      {}
func (o *selectOption) Cursor() desktop.Cursor         { return desktop.PointerCursor }
func (o *selectOption) CreateRenderer() fyne.WidgetRenderer {
	text := widget.NewLabelWithStyle(o.text, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	text.Truncation = fyne.TextTruncateEllipsis
	check := widget.NewIcon(theme.NewColoredResource(theme.ConfirmIcon(), theme.ColorNameForeground))
	r := &selectOptionRenderer{option: o, background: canvas.NewRectangle(color.Transparent), text: text, check: check}
	r.Refresh()
	return r
}

type selectOptionRenderer struct {
	option     *selectOption
	background *canvas.Rectangle
	text       *widget.Label
	check      *widget.Icon
}

func (r *selectOptionRenderer) Destroy() {}
func (r *selectOptionRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.background, r.text, r.check}
}
func (r *selectOptionRenderer) MinSize() fyne.Size {
	return fyne.NewSize(80, 36)
}
func (r *selectOptionRenderer) Layout(size fyne.Size) {
	r.background.Resize(size)
	r.check.Move(fyne.NewPos(8, (size.Height-16)/2))
	r.check.Resize(fyne.NewSquareSize(16))
	r.text.Move(fyne.NewPos(26, (size.Height-r.text.MinSize().Height)/2))
	r.text.Resize(fyne.NewSize(max(float32(0), size.Width-34), r.text.MinSize().Height))
}
func (r *selectOptionRenderer) Refresh() {
	th := inputControlTheme{Theme: r.option.Theme()}
	v := fyne.CurrentApp().Settings().ThemeVariant()
	fill := theme.ColorNameButton
	r.background.StrokeWidth = 0
	if r.option.active {
		fill = theme.ColorNameFocus
		r.background.StrokeWidth = 1
		r.background.StrokeColor = th.Color(theme.ColorNamePrimary, v)
	}
	r.background.FillColor = th.Color(fill, v)
	if r.option.selected {
		r.check.Show()
	} else {
		r.check.Hide()
	}
	r.Layout(r.option.Size())
	r.background.Refresh()
	r.text.Refresh()
	r.check.Refresh()
}
