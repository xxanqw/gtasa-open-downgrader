package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/xxanqw/gtasa-open-downgrader/internal/assets"
)

// Center the label's natural height beside its icon, accounting for inner padding.
type modStatusLayout struct{ alignment fyne.TextAlign }

func (modStatusLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	label := objects[1].MinSize()
	return fyne.NewSize(22+label.Width, max(float32(16), label.Height))
}

func (l modStatusLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	label := objects[1].(*widget.Label)
	th := label.Theme()
	padding := th.Size(theme.SizeNameInnerPadding)
	textWidth := max(float32(0), label.MinSize().Width-2*padding)
	// Pin the visible group independently of action-button size or count.
	left := float32(0)
	if l.alignment == fyne.TextAlignTrailing {
		left = max(float32(0), size.Width-22-textWidth-padding)
	} else if l.alignment == fyne.TextAlignCenter {
		left = (size.Width - 22 - textWidth) / 2
	}
	objects[0].Resize(fyne.NewSquareSize(16))
	objects[0].Move(fyne.NewPos(left, (size.Height-16)/2))
	labelHeight := label.MinSize().Height
	label.Resize(fyne.NewSize(textWidth+2*padding, labelHeight))
	label.Move(fyne.NewPos(left+22-padding, (size.Height-labelHeight)/2))
}

var modStateSymbols = loadModStateSymbols()

func loadModStateSymbols() map[string]fyne.Resource {
	symbols := map[string]fyne.Resource{}
	for _, file := range []string{"circle-check", "circle-minus", "download", "trash-can", "arrows-rotate", "link"} {
		name := "solid-" + file + ".svg"
		data, err := assets.FontAwesome.ReadFile("fontawesome/" + name)
		if err != nil {
			panic(err)
		}
		symbols[file] = fyne.NewStaticResource("mod-state-"+name, data)
	}
	return symbols
}

// Distinct shapes supplement status words and colors; no color-only meaning.
func modStateSymbol(text string, importance widget.Importance) fyne.Resource {
	file := "circle-minus"
	switch text {
	case "Installed":
		file = "circle-check"
	case "Installed (update)", "To reinstall":
		file = "arrows-rotate"
	case "To install":
		file = "download"
	case "To remove":
		file = "trash-can"
	case "Required":
		file = "link"
	}
	role := theme.ColorNameError
	if importance == widget.SuccessImportance {
		role = theme.ColorNameSuccess
	}
	if importance == widget.WarningImportance {
		role = theme.ColorNameWarning
	}
	return theme.NewColoredResource(modStateSymbols[file], role)
}

func (w *Window) setModState(name, text string, importance widget.Importance) {
	label := w.modLabels[name]
	label.Importance = importance
	label.SetText(text)
	label.Refresh()
}
