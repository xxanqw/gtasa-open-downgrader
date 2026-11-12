package uiwidget

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// ModalSection is optional information revealed without opening another dialog.
type ModalSection struct {
	Title    string
	Content  fyne.CanvasObject
	Expanded bool
}

func (m *Modal) sectionBody() fyne.CanvasObject {
	body := container.NewVBox()
	if m.options.Body != nil {
		body.Add(m.options.Body)
	}
	for _, section := range m.options.Sections {
		if section.Content == nil {
			continue
		}
		content := section.Content
		expanded := section.Expanded
		if !expanded {
			content.Hide()
		}
		button := NewButtonWithIcon(section.Title, theme.MenuDropDownIcon(), nil)
		button.Alignment = widget.ButtonAlignLeading
		button.Importance = widget.LowImportance
		if expanded {
			button.SetIcon(theme.MenuDropUpIcon())
		}
		button.OnTapped = func() {
			expanded = !expanded
			if expanded {
				content.Show()
				button.SetIcon(theme.MenuDropUpIcon())
			} else {
				content.Hide()
				button.SetIcon(theme.MenuDropDownIcon())
			}
			if m.overlay != nil {
				m.overlay.Refresh()
			}
			m.window.Canvas().Focus(button)
		}
		body.Add(container.NewVBox(button, content))
	}
	return container.NewVScroll(body)
}

// ModalFact presents a readable label/value pair. Paths remain selectable.
type ModalFact struct{ Label, Value string }

func ModalFacts(facts ...ModalFact) fyne.CanvasObject {
	rows := container.NewVBox()
	for _, fact := range facts {
		label := widget.NewLabel(fact.Label)
		label.Importance = widget.LowImportance
		value := widget.NewLabel(fact.Value)
		value.Wrapping = fyne.TextWrapWord
		value.Selectable = true
		rows.Add(container.New(modalFactLayout{}, label, value))
	}
	return rows
}

type modalFactLayout struct{}

func (modalFactLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(280, max(objects[0].MinSize().Height, objects[1].MinSize().Height))
}
func (modalFactLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	label, value := objects[0], objects[1]
	labelWidth := min(float32(96), size.Width/3)
	valueWidth := max(1, size.Width-labelWidth-8)
	value.Resize(fyne.NewSize(valueWidth, value.MinSize().Height))
	height := max(label.MinSize().Height, value.MinSize().Height)
	label.Move(fyne.NewPos(0, 0))
	label.Resize(fyne.NewSize(labelWidth, height))
	value.Move(fyne.NewPos(labelWidth+8, 0))
	value.Resize(fyne.NewSize(valueWidth, height))
}

func ModalNotice(text string, importance widget.Importance) fyne.CanvasObject {
	label := widget.NewLabel(text)
	label.Wrapping = fyne.TextWrapWord
	label.Importance = importance
	icon := theme.InfoIcon()
	if importance == widget.WarningImportance {
		icon = theme.WarningIcon()
	}
	return container.NewBorder(nil, nil, widget.NewIcon(icon), nil, label)
}
