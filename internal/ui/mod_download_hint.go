package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	appwidget "github.com/xxanqw/gtasa-open-downgrader/internal/uiwidget"
)

const shareModsDownloadHint = "Start this ShareMods download manually in your browser. The downgrader detects and checks the completed archive, then continues installation automatically."

// A quiet inline hint, separate from selection and the row's action columns.
// Keyboard focus and touch expose the same explanation as mouse hover.
type modDownloadHint struct {
	widget.BaseWidget
	window           *Window
	layer            *fyne.Container
	popup            *fyne.Container
	focused, hovered bool
}

func newModDownloadHint(window *Window) *modDownloadHint {
	hint := &modDownloadHint{window: window, layer: window.modTooltipLayer}
	hint.ExtendBaseWidget(hint)
	return hint
}
func (h *modDownloadHint) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(widget.NewIcon(theme.InfoIcon()))
}
func (h *modDownloadHint) AccessibilityRole() fyne.AccessibleRole { return fyne.AccessibleRoleButton }
func (h *modDownloadHint) AccessibilityLabel() string             { return shareModsDownloadHint }
func (h *modDownloadHint) show() {
	if h.popup != nil || !h.Visible() {
		return
	}
	c := fyne.CurrentApp().Driver().CanvasForObject(h)
	if c == nil {
		return
	}
	h.window.modTable.dismissDownloadHints()
	label := widget.NewLabel(shareModsDownloadHint)
	label.Wrapping = fyne.TextWrapWord
	background := canvas.NewRectangle(fyne.CurrentApp().Settings().Theme().Color(theme.ColorNameOverlayBackground, theme.VariantDark))
	h.popup = container.NewStack(background, container.NewPadded(label))
	width := min(float32(340), max(float32(1), c.Size().Width-24))
	// Set the wrapping width before measuring the tooltip's natural height.
	label.Resize(fyne.NewSize(max(1, width-24), 1))
	h.popup.Resize(fyne.NewSize(width, label.MinSize().Height+24))
	position := fyne.CurrentApp().Driver().AbsolutePositionForObject(h)
	x := max(float32(12), min(position.X, c.Size().Width-width-12))
	y := position.Y + h.Size().Height + 6
	if y+h.popup.Size().Height > c.Size().Height-12 {
		y = max(float32(12), position.Y-h.popup.Size().Height-6)
	}
	h.popup.Move(fyne.NewPos(x, y))
	h.layer.Add(h.popup)
}
func (h *modDownloadHint) dismiss() {
	if h.popup != nil {
		h.layer.Remove(h.popup)
		h.layer.Refresh()
		h.popup = nil
	}
}
func (h *modDownloadHint) MouseIn(*desktop.MouseEvent)  { h.hovered = true; h.show() }
func (*modDownloadHint) MouseMoved(*desktop.MouseEvent) {}
func (h *modDownloadHint) MouseOut() {
	h.hovered = false
	if !h.focused {
		h.dismiss()
	}
}
func (h *modDownloadHint) FocusGained() {
	h.focused = true
	appwidget.RevealInScroll(h.window.modTable.scroll, h)
	h.show()
}
func (h *modDownloadHint) FocusLost() {
	h.focused = false
	if !h.hovered {
		h.dismiss()
	}
}
func (h *modDownloadHint) TypedRune(rune) {}
func (h *modDownloadHint) TypedKey(key *fyne.KeyEvent) {
	switch key.Name {
	case fyne.KeyEscape:
		h.dismiss()
	case fyne.KeyReturn, fyne.KeyEnter, fyne.KeySpace:
		h.show()
	}
}
func (h *modDownloadHint) Tapped(*fyne.PointEvent) { h.show() }

func (h *modDownloadHint) Hide() { h.dismiss(); h.BaseWidget.Hide() }

func (t *modWebTable) dismissDownloadHints() {
	for _, row := range t.rows {
		row.downloadHint.dismiss()
	}
}
