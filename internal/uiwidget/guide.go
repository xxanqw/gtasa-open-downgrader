package uiwidget

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// ModalText preserves Fyne's native selectable text and copy behavior.
func ModalText(text string) *widget.Label {
	l := widget.NewLabel(text)
	l.Wrapping = fyne.TextWrapWord
	l.Selectable = true
	return l
}

type ModalEntry struct {
	widget.Entry
	OnFocused func()
}

func (e *ModalEntry) FocusGained() {
	e.Entry.FocusGained()
	if e.OnFocused != nil {
		e.OnFocused()
	}
}

func NewModalEntry() *ModalEntry { e := &ModalEntry{}; e.ExtendBaseWidget(e); return e }
func (e *ModalEntry) TypedKey(k *fyne.KeyEvent) {
	if !ModalEscape(e, k) {
		e.Entry.TypedKey(k)
	}
}

// GuideStep holds a guide page. Work starts only from explicit actions.
type GuideStep struct {
	Title, Summary string
	Body           fyne.CanvasObject
	Footer         fyne.CanvasObject
}
type Guide struct {
	Modal       *Modal
	Steps       []GuideStep
	Index       int
	OnNext      func(*Guide)
	OnClose     func()
	BeforeClose func(*Guide)
	nextText    string
}

func ShowGuide(window fyne.Window, title string, steps []GuideStep) *Guide {
	g := &Guide{Steps: steps, nextText: "Next"}
	g.Modal = ShowModal(window, ModalOptions{Title: title, Size: fyne.NewSize(680, 460), Body: container.NewStack(), OnClosed: func(string) {
		if g.OnClose != nil {
			g.OnClose()
		}
	}})
	g.Modal.SetCancel(func() {
		if g.BeforeClose != nil {
			g.BeforeClose(g)
		} else {
			g.Modal.Close("")
		}
	})
	g.Show(0)
	return g
}
func (g *Guide) Show(index int) {
	if len(g.Steps) == 0 {
		return
	}
	g.Index = max(0, min(index, len(g.Steps)-1))
	step := g.Steps[g.Index]
	title := widget.NewLabelWithStyle(fmt.Sprintf("%d / %d  |  %s", g.Index+1, len(g.Steps), step.Title), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	title.Wrapping = fyne.TextWrapWord
	summary := ModalText(step.Summary)
	summary.Importance = widget.LowImportance
	content := container.NewVBox(title)
	if step.Summary != "" {
		content.Add(summary)
	}
	if step.Body != nil {
		content.Add(step.Body)
	}
	scroll := container.NewVScroll(content)
	bindGuideFocus(scroll, content)
	g.Modal.SetBody(scroll)
	g.Modal.SetAccessory(step.Footer)
	g.Actions()
}
func (g *Guide) Actions(extra ...ModalAction) {
	actions := []ModalAction{{Text: "Close", KeepOpen: true, Run: func(*Modal) { g.Modal.Cancel() }}}
	if g.Index > 0 {
		actions = append(actions, ModalAction{Text: "Back", KeepOpen: true, Run: func(*Modal) { g.Show(g.Index - 1) }})
	}
	if len(extra) > 0 {
		actions = append(actions, extra...)
	} else if g.Index < len(g.Steps)-1 {
		actions = append(actions, ModalAction{Text: g.nextText, Importance: widget.HighImportance, KeepOpen: true, Run: func(*Modal) {
			if g.OnNext != nil {
				g.OnNext(g)
			} else {
				g.Show(g.Index + 1)
			}
		}})
	}
	g.Modal.SetActions(actions...)
}
func GuideSection(title string, objects ...fyne.CanvasObject) fyne.CanvasObject {
	heading := widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	return container.NewVBox(append([]fyne.CanvasObject{heading}, objects...)...)
}
func GuideBullets(items ...string) fyne.CanvasObject {
	return ModalText("• " + strings.Join(items, "\n• "))
}
func GuideNotice(title, text string, importance widget.Importance) fyne.CanvasObject {
	heading := widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	heading.Importance = importance
	body := ModalText(text)
	stripe := canvas.NewRectangle(theme.WarningColor())
	if importance == widget.SuccessImportance {
		stripe.FillColor = theme.SuccessColor()
	} else if importance == widget.DangerImportance {
		stripe.FillColor = theme.ErrorColor()
	} else if importance != widget.WarningImportance {
		stripe.FillColor = theme.DisabledColor()
	}
	stripe.SetMinSize(fyne.NewSize(3, 1))
	return container.NewBorder(nil, nil, stripe, nil, container.New(layout.NewCustomPaddedLayout(6, 6, 8, 6), container.NewVBox(heading, body)))
}
func GuideCode(window fyne.Window, text string) fyne.CanvasObject {
	label := ModalText(text)
	label.TextStyle.Monospace = true
	label.Wrapping = fyne.TextWrapBreak
	copy := NewButtonWithIcon("Copy", theme.ContentCopyIcon(), func() { window.Clipboard().SetContent(text) })
	copy.AccessibleLabel = "Copy guide text"
	return container.NewBorder(nil, nil, nil, copy, container.NewThemeOverride(label, InputTheme{Theme: fyne.CurrentApp().Settings().Theme()}))
}
func ShowMessage(window fyne.Window, title, text string) {
	ShowModal(window, ModalOptions{Title: title, Body: container.NewVScroll(ModalText(text))})
}
func ShowError(window fyne.Window, err error) {
	if err != nil {
		ShowModal(window, ModalOptions{Title: "Could not complete the action", Nested: true, Subtitle: "Nothing else will be attempted automatically.", Body: container.NewVScroll(GuideNotice("Details", err.Error(), widget.DangerImportance))})
	}
}
func ShowConfirm(window fyne.Window, title, text, accept string, done func(bool)) *Modal {
	return ShowModal(window, ModalOptions{Title: title, Body: container.NewVScroll(ModalText(text)), Actions: []ModalAction{{Text: "Cancel"}, {ID: "accept", Text: accept, Importance: widget.HighImportance}}, OnClosed: func(result string) { done(result == "accept") }})
}
