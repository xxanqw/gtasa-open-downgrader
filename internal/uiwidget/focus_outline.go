package uiwidget

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	"image/color"
)

type focusOutline struct {
	fyne.WidgetRenderer
	object  fyne.Widget
	focused func() bool
	outline *canvas.Rectangle
}

func newFocusOutline(renderer fyne.WidgetRenderer, object fyne.Widget, focused func() bool) fyne.WidgetRenderer {
	return &focusOutline{WidgetRenderer: renderer, object: object, focused: focused, outline: canvas.NewRectangle(color.Transparent)}
}
func (r *focusOutline) Objects() []fyne.CanvasObject {
	return append(append([]fyne.CanvasObject(nil), r.WidgetRenderer.Objects()...), r.outline)
}
func (r *focusOutline) Layout(size fyne.Size) {
	r.WidgetRenderer.Layout(size)
	r.outline.Move(fyne.NewPos(1, 1))
	r.outline.Resize(fyne.NewSize(max(0, size.Width-2), max(0, size.Height-2)))
}
func (r *focusOutline) Refresh() {
	r.WidgetRenderer.Refresh()
	r.outline.StrokeWidth = 1
	r.outline.StrokeColor = color.Transparent
	disabled := false
	if d, ok := r.object.(fyne.Disableable); ok {
		disabled = d.Disabled()
	}
	if r.focused() && !disabled && r.object.Visible() {
		th := fyne.CurrentApp().Settings().Theme()
		if themed, ok := r.object.(interface{ Theme() fyne.Theme }); ok {
			th = themed.Theme()
		}
		r.outline.StrokeColor = th.Color(theme.ColorNameForeground, theme.VariantDark)
	}
	r.outline.Refresh()
}
