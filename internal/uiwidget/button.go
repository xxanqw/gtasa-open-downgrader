// Package uiwidget contains the small widget adaptations shared by the two
// Fyne applications.
package uiwidget

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// Button is a regular Fyne button that uses the platform pointer cursor while
// it can be clicked.
type Button struct {
	widget.Button
	AccessibleLabel string
	// OnFocused lets a scroll owner reveal an action reached by keyboard.
	OnFocused func()
	focused   bool
}

func NewButton(label string, tapped func()) *Button {
	button := &Button{}
	button.Text = label
	button.OnTapped = tapped
	button.ExtendBaseWidget(button)
	return button
}

func NewButtonWithIcon(label string, icon fyne.Resource, tapped func()) *Button {
	button := NewButton(label, tapped)
	button.Icon = icon
	return button
}

func (button *Button) AccessibilityLabel() string {
	label := button.AccessibleLabel
	if label == "" {
		label = button.Text
	}
	if label == "" && button.Icon != nil {
		label = button.Icon.Name()
	}
	if button.Disabled() {
		label += ", unavailable"
	}
	return label
}

func (button *Button) FocusGained() {
	button.focused = true
	button.Button.FocusGained()
	if button.OnFocused != nil {
		button.OnFocused()
	}
}
func (button *Button) FocusLost() { button.focused = false; button.Button.FocusLost() }

// Native Button handles Space only and clears focus when activated. Keyboard
// activation supports Enter too, keeping focus unless the action moves it.
func (button *Button) TypedKey(key *fyne.KeyEvent) {
	if key.Name == fyne.KeyReturn || key.Name == fyne.KeyEnter || key.Name == fyne.KeySpace {
		if !button.Disabled() && button.Visible() && button.OnTapped != nil {
			button.OnTapped()
		}
		return
	}
	if key.Name == fyne.KeyEscape {
		if c := fyne.CurrentApp().Driver().CanvasForObject(button); c != nil && c.OnTypedKey() != nil {
			c.OnTypedKey()(key)
			return
		}
	}
	button.Button.TypedKey(key)
}
func (button *Button) CreateRenderer() fyne.WidgetRenderer {
	return newFocusOutline(button.Button.CreateRenderer(), button, func() bool { return button.focused })
}

func (button *Button) Cursor() desktop.Cursor {
	if button.Disabled() {
		return desktop.DefaultCursor
	}
	return desktop.PointerCursor
}
