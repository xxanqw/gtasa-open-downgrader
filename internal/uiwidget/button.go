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

func (button *Button) Cursor() desktop.Cursor {
	if button.Disabled() {
		return desktop.DefaultCursor
	}
	return desktop.PointerCursor
}
