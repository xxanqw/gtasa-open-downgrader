package ui

import (
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type promptKind int

const (
	promptInfo promptKind = iota
	promptError
	promptConfirm
	promptFolder
	promptFile
	promptEntry
	promptChoice
)

type promptRequest struct {
	kind                 promptKind
	title, text, initial string
	accept, dismiss      string
	extensions, choices  []string
	done                 func(string, bool)
	present              func(func(string, bool)) func()
}

// Keep prompts ordered, including prompts arriving from background work while
// Tools or About is open. Only an explicit confirmation can start an action.
func (w *Window) prompt(r promptRequest) {
	if w.closing {
		return
	}
	w.prompts = append(w.prompts, r)
	w.nextPrompt()
}
func (w *Window) nextPrompt() {
	if w.closing || w.promptActive || len(w.prompts) == 0 {
		return
	}
	r := w.prompts[0]
	w.prompts = w.prompts[1:]
	w.promptActive = true
	w.refreshEnabled()
	finished := false
	complete := func(value string, ok bool) {
		if finished {
			return
		}
		finished = true
		w.activePrompt = nil
		w.promptActive = false
		if w.closing {
			return
		}
		w.refreshEnabled()
		if r.done != nil {
			r.done(value, ok)
		}
		w.nextPrompt()
	}
	if r.present != nil {
		w.activePrompt = r.present(complete)
	} else {
		w.activePrompt = w.fynePrompt(r, complete)
	}
}

// panel gives application dialogs a shared header, spacing and action bar.
// Content is bounded by the current window; scrollable bodies retain access to
// long messages and licenses even at the smallest supported window size.
func (w *Window) panel(title string, body fyne.CanvasObject, size fyne.Size, accept, dismiss string, extra fyne.CanvasObject, done func(bool)) func() {
	canvas := w.window.Canvas()
	oldKey, oldFocus := canvas.OnTypedKey(), canvas.Focused()
	var popup *widget.PopUp
	closed := false
	finish := func(ok bool) {
		if closed {
			return
		}
		closed = true
		popup.Hide()
		canvas.SetOnTypedKey(oldKey)
		if oldFocus != nil {
			canvas.Focus(oldFocus)
		} else {
			canvas.Unfocus()
		}
		done(ok)
	}
	closeButton := widget.NewButtonWithIcon("", theme.CancelIcon(), func() { finish(false) })
	heading := compactLabel(title)
	heading.TextStyle.Bold = true
	header := container.NewBorder(nil, nil, nil, closeButton, heading)
	cancel := widget.NewButton(dismiss, func() { finish(false) })
	buttons := container.NewHBox(cancel)
	if accept != "" {
		confirm := widget.NewButton(accept, func() { finish(true) })
		confirm.Importance = widget.HighImportance
		buttons.Add(confirm)
	}
	footer := container.NewBorder(nil, nil, extra, buttons, nil)
	frame := container.NewBorder(container.NewVBox(header, widget.NewSeparator()), container.NewVBox(widget.NewSeparator(), footer), nil, nil, body)
	content := container.New(layout.NewCustomPaddedLayout(10, 10, 14, 14), frame)
	popup = widget.NewModalPopUp(content, canvas)
	bounds := canvas.Size()
	popup.Resize(fyne.NewSize(min(size.Width, bounds.Width-24), min(size.Height, bounds.Height-24)))
	canvas.Unfocus()
	canvas.SetOnTypedKey(func(key *fyne.KeyEvent) {
		if key.Name == fyne.KeyEscape {
			finish(false)
		}
	})
	popup.Show()
	return func() { finish(false) }
}
func promptText(text string) *widget.Label {
	label := widget.NewLabel(text)
	label.Wrapping = fyne.TextWrapWord
	label.Selectable = true
	return label
}
func (w *Window) fynePrompt(r promptRequest, done func(string, bool)) func() {
	accept, dismiss := r.accept, r.dismiss
	if accept == "" {
		accept = "Continue"
	}
	if dismiss == "" {
		dismiss = "Cancel"
	}
	switch r.kind {
	case promptInfo, promptError:
		return w.panel(r.title, container.NewVScroll(promptText(r.text)), fyne.NewSize(500, 250), "", "Close", nil, func(bool) { done("", false) })
	case promptConfirm:
		return w.panel(r.title, container.NewVScroll(promptText(r.text)), fyne.NewSize(500, 245), accept, dismiss, nil, func(ok bool) { done("", ok) })
	case promptFolder:
		d := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil {
				done("", false)
				w.error(err)
				return
			}
			if uri == nil {
				done("", false)
				return
			}
			done(uri.Path(), true)
		}, w.window)
		d.SetTitleText(r.title)
		w.setPickerLocation(d, r.initial)
		w.showPicker(d)
		return func() { d.Hide(); done("", false) }
	case promptFile:
		d := dialog.NewFileOpen(func(file fyne.URIReadCloser, err error) {
			if err != nil {
				done("", false)
				w.error(err)
				return
			}
			if file == nil {
				done("", false)
				return
			}
			path := file.URI().Path()
			file.Close()
			done(path, true)
		}, w.window)
		d.SetFilter(storage.NewExtensionFileFilter(r.extensions))
		d.SetTitleText(r.title)
		w.setPickerLocation(d, r.initial)
		w.showPicker(d)
		return func() { d.Hide(); done("", false) }
	case promptEntry:
		entry := widget.NewEntry()
		entry.SetText(r.initial)
		body := container.NewBorder(nil, entry, nil, nil, container.NewVScroll(promptText(r.text)))
		close := w.panel(r.title, body, fyne.NewSize(500, 250), accept, dismiss, nil, func(ok bool) { done(entry.Text, ok) })
		w.window.Canvas().Focus(entry)
		return close
	case promptChoice:
		choice := widget.NewSelect(r.choices, nil)
		choice.SetSelected(r.initial)
		body := container.NewBorder(nil, choice, nil, nil, container.NewVScroll(promptText(r.text)))
		return w.panel(r.title, body, fyne.NewSize(520, 280), accept, dismiss, nil, func(ok bool) { done(choice.Selected, ok && choice.Selected != "") })
	}
	return func() { done("", false) }
}
func (w *Window) showPicker(d *dialog.FileDialog) {
	d.SetConfirmText("Select")
	d.Show()
	bounds := w.window.Canvas().Size()
	d.Resize(fyne.NewSize(min(float32(640), bounds.Width-24), min(float32(460), bounds.Height-24)))
}
func (w *Window) setPickerLocation(d *dialog.FileDialog, path string) {
	if path != "" {
		if uri, err := storage.ListerForURI(storage.NewFileURI(path)); err == nil {
			d.SetLocation(uri)
		}
	}
}
func (w *Window) info(title, message string) {
	w.prompt(promptRequest{kind: promptInfo, title: title, text: message})
}
func (w *Window) confirm(title, message string, done func(bool)) {
	w.prompt(promptRequest{kind: promptConfirm, title: title, text: message, done: func(_ string, ok bool) { done(ok) }})
}
func (w *Window) chooseFolder(title, initial string, chosen func(string)) {
	w.prompt(promptRequest{kind: promptFolder, title: title, initial: initial, done: func(path string, ok bool) {
		if ok && path != "" {
			chosen(filepath.Clean(path))
		}
	}})
}
func (w *Window) chooseArchive(title string, extensions []string, chosen func(string)) {
	w.prompt(promptRequest{kind: promptFile, title: title, initial: w.path.Text, extensions: extensions, done: func(path string, ok bool) {
		if ok && path != "" {
			chosen(path)
		}
	}})
}
