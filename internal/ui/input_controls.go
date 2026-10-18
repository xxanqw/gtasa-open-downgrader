package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	appwidget "github.com/xxanqw/gtasa-open-downgrader/internal/uiwidget"
	"image/color"
)

// Input focus must not inherit the dark filled-button Primary color.
type inputControlTheme = appwidget.InputTheme

func styleInput(object fyne.CanvasObject) fyne.CanvasObject {
	return container.NewThemeOverride(object, inputControlTheme{Theme: fyne.CurrentApp().Settings().Theme()})
}

// Preserve Select's data, keyboard arrows and change callbacks, but replace
// its default popup with square, filled application controls and an explicit
// selected marker. Outside click, Escape and choice all dismiss it.
type controlSelect struct {
	widget.Select
	focused, hovered bool
	popup            *fyne.Container
	popupButtons     []*selectOption
	popupIndex       int
	popupScroll      *container.Scroll
}

func newControlSelect(options []string, changed func(string)) *controlSelect {
	s := &controlSelect{}
	s.Options = options
	s.OnChanged = changed
	s.ExtendBaseWidget(s)
	return s
}
func (s *controlSelect) MinSize() fyne.Size {
	size := s.Select.MinSize()
	size.Width = min(size.Width, 260)
	return size
}
func (s *controlSelect) AccessibilityRole() fyne.AccessibleRole { return fyne.AccessibleRoleButton }
func (s *controlSelect) AccessibilityLabel() string {
	if s.Selected != "" {
		return "Selected option: " + s.Selected
	}
	return s.PlaceHolder
}
func (s *controlSelect) Cursor() desktop.Cursor {
	if s.Disabled() {
		return desktop.DefaultCursor
	}
	return desktop.PointerCursor
}
func (s *controlSelect) FocusGained() { s.focused = true; s.Select.FocusGained(); s.Refresh() }
func (s *controlSelect) FocusLost() {
	s.closePopup()
	s.focused = false
	s.Select.FocusLost()
	s.Refresh()
}
func (s *controlSelect) MouseIn(e *desktop.MouseEvent) {
	s.hovered = true
	s.Select.MouseIn(e)
	s.Refresh()
}
func (s *controlSelect) MouseOut()               { s.hovered = false; s.Select.MouseOut(); s.Refresh() }
func (s *controlSelect) Hide()                   { s.closePopup(); s.Select.Hide() }
func (s *controlSelect) Disable()                { s.closePopup(); s.Select.Disable() }
func (s *controlSelect) Tapped(*fyne.PointEvent) { s.showPopup() }

// Desktop Space produces both a key and a rune event. Handle only the key,
// otherwise one press opens the list and immediately accepts/closes it.
func (s *controlSelect) TypedRune(rune) {}
func (s *controlSelect) TypedKey(e *fyne.KeyEvent) {
	if s.Disabled() || !s.Visible() {
		return
	}
	if s.popup != nil && !s.popupValid() {
		s.closePopup()
		return
	}
	if s.popup != nil {
		switch e.Name {
		case fyne.KeyEscape:
			s.closePopup()
		case fyne.KeyDown:
			s.popupIndex = (s.popupIndex + 1) % len(s.Options)
			s.refreshPopupSelection()
		case fyne.KeyUp:
			s.popupIndex = (s.popupIndex + len(s.Options) - 1) % len(s.Options)
			s.refreshPopupSelection()
		case fyne.KeyHome:
			s.popupIndex = 0
			s.refreshPopupSelection()
		case fyne.KeyEnd:
			s.popupIndex = len(s.Options) - 1
			s.refreshPopupSelection()
		case fyne.KeyPageDown:
			s.popupIndex = min(len(s.Options)-1, s.popupIndex+max(1, int(s.popupScroll.Size().Height/(36+theme.Padding()))))
			s.refreshPopupSelection()
		case fyne.KeyPageUp:
			s.popupIndex = max(0, s.popupIndex-max(1, int(s.popupScroll.Size().Height/(36+theme.Padding()))))
			s.refreshPopupSelection()
		case fyne.KeyReturn, fyne.KeyEnter, fyne.KeySpace:
			value := s.Options[s.popupIndex]
			s.closePopup()
			s.SetSelected(value)
		}
		return
	}
	if e.Name == fyne.KeyEscape {
		if c := fyne.CurrentApp().Driver().CanvasForObject(s); c != nil && c.OnTypedKey() != nil {
			c.OnTypedKey()(e)
		}
		return
	}
	if e.Name == fyne.KeyReturn || e.Name == fyne.KeyEnter || e.Name == fyne.KeySpace {
		s.showPopup()
		return
	}
	s.Select.TypedKey(e)
}
func (s *controlSelect) closePopup() {
	if s.popup == nil {
		return
	}
	p := s.popup
	s.popup = nil
	if c := fyne.CurrentApp().Driver().CanvasForObject(s); c != nil {
		c.Overlays().Remove(p)
	}
	s.popupButtons = nil
	s.popupScroll = nil
}
func (s *controlSelect) popupValid() bool {
	if len(s.Options) == 0 || len(s.Options) != len(s.popupButtons) || s.popupIndex < 0 || s.popupIndex >= len(s.Options) {
		return false
	}
	for i, b := range s.popupButtons {
		if b.text != s.Options[i] {
			return false
		}
	}
	return true
}

func (s *controlSelect) showPopup() {
	if s.Disabled() || len(s.Options) == 0 {
		return
	}
	if s.popup != nil {
		s.closePopup()
		return
	}
	c := fyne.CurrentApp().Driver().CanvasForObject(s)
	if c == nil {
		return
	}
	s.popupIndex = max(0, s.SelectedIndex())
	buttons := make([]fyne.CanvasObject, 0, len(s.Options))
	for _, option := range s.Options {
		value := option
		index := len(buttons)
		b := &selectOption{text: value, selected: value == s.Selected, OnTapped: func() { s.closePopup(); s.SetSelected(value) }, onHover: func() { s.popupIndex = index; s.refreshPopupSelection() }}
		b.ExtendBaseWidget(b)
		buttons = append(buttons, b)
		s.popupButtons = append(s.popupButtons, b)
	}
	s.popupScroll = container.NewVScroll(container.NewVBox(buttons...))
	content := newFramedPanel(s.popupScroll)
	dismiss := &selectDismissArea{dismiss: s.closePopup}
	dismiss.ExtendBaseWidget(dismiss)
	s.popup = container.New(selectPopupLayout{selectBox: s}, dismiss, content)
	s.popup.Resize(c.Size())
	c.Overlays().Add(s.popup)
	c.Focus(s)
	s.refreshPopupSelection()
}
func (s *controlSelect) refreshPopupSelection() {
	for i, b := range s.popupButtons {
		b.active = i == s.popupIndex
		b.Refresh()
	}
	if s.popupScroll != nil && s.popupIndex >= 0 && s.popupIndex < len(s.popupButtons) {
		scroll := s.popupScroll
		item := s.popupButtons[s.popupIndex]
		y := scroll.Offset.Y
		if item.Position().Y < y {
			y = item.Position().Y
		} else if bottom := item.Position().Y + item.Size().Height; bottom > y+scroll.Size().Height {
			y = bottom - scroll.Size().Height
		}
		scroll.ScrollToOffset(fyne.NewPos(0, y))
	}
}

type selectDismissArea struct {
	widget.BaseWidget
	dismiss func()
}

func (d *selectDismissArea) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(canvas.NewRectangle(color.Transparent))
}
func (d *selectDismissArea) Tapped(*fyne.PointEvent) { d.dismiss() }

type selectPopupLayout struct{ selectBox *controlSelect }

func (selectPopupLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(1, 1) }
func (l selectPopupLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	objects[0].Move(fyne.NewPos(0, 0))
	objects[0].Resize(size)
	content := objects[1]
	s := l.selectBox
	if s.popupScroll == nil {
		return
	}
	width := s.Size().Width
	for _, b := range s.popupButtons {
		width = max(width, fyne.MeasureText(b.text, theme.TextSize(), fyne.TextStyle{Bold: true}).Width+56)
	}
	width = min(max(0, size.Width-16), min(600, width))
	height := min(max(0, size.Height-16), min(360, s.popupScroll.Content.MinSize().Height+2))
	anchor := fyne.CurrentApp().Driver().AbsolutePositionForObject(s)
	x := max(float32(0), min(anchor.X, size.Width-width))
	y := anchor.Y + s.Size().Height + 2
	if y+height > size.Height {
		y = max(float32(0), anchor.Y-height-2)
	}
	content.Move(fyne.NewPos(x, y))
	content.Resize(fyne.NewSize(width, height))
}

func (s *controlSelect) CreateRenderer() fyne.WidgetRenderer {
	return &controlSelectRenderer{WidgetRenderer: s.Select.CreateRenderer(), selectBox: s, border: canvas.NewRectangle(color.Transparent)}
}

type controlSelectRenderer struct {
	fyne.WidgetRenderer
	selectBox *controlSelect
	border    *canvas.Rectangle
}

func (r *controlSelectRenderer) Objects() []fyne.CanvasObject {
	return append(r.WidgetRenderer.Objects(), r.border)
}
func (r *controlSelectRenderer) Layout(size fyne.Size) {
	r.WidgetRenderer.Layout(size)
	r.border.Resize(size)
}
func (r *controlSelectRenderer) Refresh() {
	r.WidgetRenderer.Refresh()
	s := r.selectBox
	th := s.Theme()
	v := fyne.CurrentApp().Settings().ThemeVariant()
	role := theme.ColorNameInputBorder
	r.border.StrokeWidth = 1
	if s.Disabled() {
		role = theme.ColorNameDisabled
	} else if s.focused {
		role = theme.ColorNamePrimary
		r.border.StrokeWidth = 2
	} else if s.hovered {
		role = theme.ColorNameForeground
	}
	r.border.StrokeColor = th.Color(role, v)
	r.border.Refresh()
}
