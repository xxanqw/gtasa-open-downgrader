package uiwidget

import "fyne.io/fyne/v2"

// Relayout settles container geometry without refreshing every descendant.
// Container.Refresh also redraws hidden pages and reparses their text/icons;
// changing a footer's visibility or opening a prompt only needs layout.
func Relayout(object fyne.CanvasObject) {
	if object == nil || !object.Visible() {
		return
	}
	c, ok := object.(*fyne.Container)
	if !ok {
		return
	}
	if c.Layout != nil {
		c.Layout.Layout(c.Objects, c.Size())
	}
	for _, child := range c.Objects {
		Relayout(child)
	}
}
