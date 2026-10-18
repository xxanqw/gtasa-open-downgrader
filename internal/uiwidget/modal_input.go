package uiwidget

import (
	"net/url"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

type ModalLink struct {
	widget.Hyperlink
	OnFocused func()
	focused   bool
}

func NewModalLink(text string, address *url.URL) *ModalLink {
	l := &ModalLink{}
	l.Text = text
	l.URL = address
	l.Wrapping = fyne.TextWrapWord
	l.ExtendBaseWidget(l)
	return l
}
func (l *ModalLink) FocusGained() {
	l.focused = true
	l.Hyperlink.FocusGained()
	if l.OnFocused != nil {
		l.OnFocused()
	}
}
func (l *ModalLink) FocusLost() { l.focused = false; l.Hyperlink.FocusLost() }
func (l *ModalLink) TypedKey(key *fyne.KeyEvent) {
	if ModalEscape(l, key) {
		return
	}
	if key.Name == fyne.KeyReturn || key.Name == fyne.KeyEnter {
		key = &fyne.KeyEvent{Name: fyne.KeySpace}
	}
	l.Hyperlink.TypedKey(key)
}
func (l *ModalLink) CreateRenderer() fyne.WidgetRenderer {
	return newFocusOutline(l.Hyperlink.CreateRenderer(), l, func() bool { return l.focused })
}

// Bind only the guide's explicit composition, not private renderer ancestry.
func bindGuideFocus(scroll *container.Scroll, object fyne.CanvasObject) {
	reveal := func() { RevealInScroll(scroll, object) }
	switch o := object.(type) {
	case *Button:
		o.OnFocused = reveal
	case *Check:
		o.OnFocused = reveal
	case *ModalLink:
		o.OnFocused = reveal
	case *ModalEntry:
		o.OnFocused = reveal
	case *fyne.Container:
		for _, child := range o.Objects {
			bindGuideFocus(scroll, child)
		}
	case *container.ThemeOverride:
		bindGuideFocus(scroll, o.Content)
	}
}
