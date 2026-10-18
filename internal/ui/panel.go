package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
	appwidget "github.com/xxanqw/gtasa-open-downgrader/internal/uiwidget"
)

// framedPanel supplies the thin square outline used for grouped tasks. Unlike
// a fixed-color canvas shape it derives its border from the dark app palette.
type framedPanel struct {
	widget.BaseWidget
	content fyne.CanvasObject
}

func newFramedPanel(content fyne.CanvasObject) *framedPanel {
	panel := &framedPanel{content: content}
	panel.ExtendBaseWidget(panel)
	return panel
}

func (p *framedPanel) CreateRenderer() fyne.WidgetRenderer {
	r := &framedPanelRenderer{
		background: canvas.NewRectangle(color.Transparent),
		content:    p.content,
	}
	r.objects = []fyne.CanvasObject{r.background, r.content}
	r.Refresh()
	return r
}

type framedPanelRenderer struct {
	background *canvas.Rectangle
	content    fyne.CanvasObject
	objects    []fyne.CanvasObject
}

func (r *framedPanelRenderer) Destroy()                     {}
func (r *framedPanelRenderer) Objects() []fyne.CanvasObject { return r.objects }
func (r *framedPanelRenderer) MinSize() fyne.Size {
	return r.content.MinSize().Add(fyne.NewSquareSize(2))
}
func (r *framedPanelRenderer) Layout(size fyne.Size) {
	r.background.Resize(size)
	r.content.Move(fyne.NewPos(1, 1))
	r.content.Resize(fyne.NewSize(max(0, size.Width-2), max(0, size.Height-2)))
}
func (r *framedPanelRenderer) Refresh() {
	if app := fyne.CurrentApp(); app != nil {
		appwidget.StylePanel(r.background, app.Settings().Theme())
	}
	r.background.Refresh()
	r.content.Refresh()
}
