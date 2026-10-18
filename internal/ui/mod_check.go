package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Fyne colors a disabled checked box with Disabled (bright for readable text),
// but an enabled checked box with Primary (dark for filled buttons). Give only
// mod checkmarks a distinct enabled/locked appearance; keep their labels and
// keyboard/checkbox behavior supplied by the stock widget.
type modCheck struct {
	widget.Check
	required        bool // locked prerequisite: retain a clear checked glyph
	accessibleLabel string
	focused         bool // visible keyboard-focus indicator, not mouse focus
	pointerFocus    bool
}

func newModCheck(text string, changed func(bool)) *modCheck {
	check := &modCheck{}
	check.Text, check.OnChanged = text, changed
	check.ExtendBaseWidget(check)
	return check
}

func (check *modCheck) AccessibilityLabel() string {
	label := check.accessibleLabel
	if label == "" {
		label = check.Text
	}
	if check.Partial {
		label += ", partially selected"
	} else if check.Checked {
		label += ", checked"
	} else {
		label += ", unchecked"
	}
	if check.required {
		label += ", required and locked"
	} else if check.Disabled() {
		label += ", unavailable"
	}
	return label
}

func (check *modCheck) Cursor() desktop.Cursor {
	if check.Disabled() {
		return desktop.DefaultCursor
	}
	return desktop.PointerCursor
}

// Hover/click never paints a checkbox halo. Only keyboard focus gets an outline.
func (*modCheck) MouseIn(*desktop.MouseEvent)    {}
func (*modCheck) MouseMoved(*desktop.MouseEvent) {}
func (*modCheck) MouseOut()                      {}

func (check *modCheck) Tapped(event *fyne.PointEvent) {
	if check.Disabled() || !check.Visible() {
		return
	}
	// Check.Tapped focuses before toggling. Mark that focus as pointer-driven,
	// also clearing an existing keyboard outline when clicking a focused box.
	check.pointerFocus = true
	check.focused = false
	defer func() { check.pointerFocus = false }()
	check.Check.Tapped(event)
	check.Refresh()
}

func (check *modCheck) FocusGained() {
	check.focused = !check.pointerFocus && !check.Disabled()
	check.Check.FocusGained()
}

func (check *modCheck) FocusLost() {
	check.focused = false
	check.Check.FocusLost()
}

func (check *modCheck) Hide() {
	check.focused = false
	check.Check.Hide()
	check.Refresh()
}

func (check *modCheck) CreateRenderer() fyne.WidgetRenderer {
	r := &modCheckRenderer{WidgetRenderer: check.Check.CreateRenderer(), check: check,
		focus: canvas.NewRectangle(color.Transparent)}
	r.focus.StrokeWidth = 1
	r.Refresh()
	return r
}

type modCheckRenderer struct {
	fyne.WidgetRenderer
	check *modCheck
	focus *canvas.Rectangle
}

func (r *modCheckRenderer) Objects() []fyne.CanvasObject {
	// Preserve the existing background/icon/text object indices, but never
	// expose the stock circle to the canvas, including after mouse clicks.
	objects := append([]fyne.CanvasObject(nil), r.WidgetRenderer.Objects()...)
	objects[0] = r.focus
	return objects
}

func (r *modCheckRenderer) Layout(size fyne.Size) {
	r.WidgetRenderer.Layout(size)
	icon := r.WidgetRenderer.Objects()[2]
	r.focus.Move(icon.Position().Subtract(fyne.NewPos(2, 2)))
	r.focus.Resize(icon.Size().Add(fyne.NewSquareSize(4)))
}

func (r *modCheckRenderer) Refresh() {
	r.WidgetRenderer.Refresh()
	if r.check.focused && !r.check.Disabled() && r.check.Visible() {
		r.focus.StrokeColor = r.check.Theme().Color(theme.ColorNameForeground, fyne.CurrentApp().Settings().ThemeVariant())
		r.focus.Show()
	} else {
		r.focus.StrokeColor = color.Transparent
		r.focus.Hide()
	}
	r.focus.Refresh()
	for _, object := range r.WidgetRenderer.Objects()[1:3] {
		object.Show()
	}
	if r.check.required && r.check.Checked {
		icon := r.WidgetRenderer.Objects()[2].(*canvas.Image)
		icon.Resource = theme.NewColoredResource(r.check.Theme().Icon(theme.IconNameCheckButtonChecked), theme.ColorNameForeground)
		icon.Refresh()
		bg := r.WidgetRenderer.Objects()[1].(*canvas.Image)
		bg.Resource = theme.NewColoredResource(r.check.Theme().Icon(theme.IconNameCheckButtonFill), theme.ColorNameDisabledButton)
		bg.Refresh()
		return
	}
	if r.check.Disabled() {
		icon := r.WidgetRenderer.Objects()[2].(*canvas.Image)
		name := theme.IconNameCheckButton
		if r.check.Partial {
			name = theme.IconNameCheckButtonPartial
		} else if r.check.Checked {
			name = theme.IconNameCheckButtonChecked
		}
		icon.Resource = theme.NewColoredResource(r.check.Theme().Icon(name), theme.ColorNameScrollBar)
		icon.Refresh()
		bg := r.WidgetRenderer.Objects()[1].(*canvas.Image)
		bg.Resource = theme.NewColoredResource(r.check.Theme().Icon(theme.IconNameCheckButtonFill), theme.ColorNameDisabledButton)
		bg.Refresh()
		return
	}
	if !r.check.Checked || r.check.Partial {
		icon := r.WidgetRenderer.Objects()[2].(*canvas.Image)
		name := theme.IconNameCheckButton
		if r.check.Partial {
			name = theme.IconNameCheckButtonPartial
		}
		icon.Resource = theme.NewColoredResource(r.check.Theme().Icon(name), theme.ColorNameForeground)
		icon.Refresh()
		return
	}
	icon := r.WidgetRenderer.Objects()[2].(*canvas.Image)
	icon.Resource = theme.NewColoredResource(theme.CheckButtonCheckedIcon(), theme.ColorNameForeground)
	icon.Refresh()
}
