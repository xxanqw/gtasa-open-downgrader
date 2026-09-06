package ui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
	"github.com/ncruces/zenity"
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
}
type dialogBackend struct {
	available func() bool
	show      func(context.Context, promptRequest, any) (string, error)
}

func systemDialog(ctx context.Context, r promptRequest, parent any) (string, error) {
	opts := []zenity.Option{zenity.Title(r.title), zenity.Context(ctx), zenity.Modal()}
	if r.accept != "" {
		opts = append(opts, zenity.OKLabel(r.accept))
	}
	if r.dismiss != "" {
		opts = append(opts, zenity.CancelLabel(r.dismiss))
	}
	if parent != nil {
		opts = append(opts, zenity.Attach(parent))
	}
	switch r.kind {
	case promptInfo:
		return "", zenity.Info(r.text, opts...)
	case promptError:
		return "", zenity.Error(r.text, opts...)
	case promptConfirm:
		return "", zenity.Question(r.text, append(opts, zenity.OKLabel("Continue"), zenity.CancelLabel("Cancel"), zenity.DefaultCancel())...)
	case promptFolder, promptFile:
		if r.initial != "" {
			opts = append(opts, zenity.Filename(r.initial+string(os.PathSeparator)))
		}
		if r.kind == promptFolder {
			opts = append(opts, zenity.Directory())
		}
		if len(r.extensions) > 0 {
			patterns := make([]string, len(r.extensions))
			for i, ext := range r.extensions {
				patterns[i] = "*" + ext
			}
			opts = append(opts, zenity.FileFilter{Name: "Supported files", Patterns: patterns, CaseFold: true})
		}
		return zenity.SelectFile(opts...)
	case promptEntry:
		return zenity.Entry(r.text, append(opts, zenity.EntryText(r.initial))...)
	case promptChoice:
		return zenity.List(r.text, r.choices, append(opts, zenity.DisallowEmpty(), zenity.DefaultItems(r.initial))...)
	}
	return "", zenity.ErrUnsupported
}

// Prompts are serialized and owned by the same cancellable job group as other
// window work. No system dialog blocks Fyne's event goroutine or outlives Quit.
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
	complete := func(value string, ok bool) {
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
	if w.dialogs.available == nil || !w.dialogs.available() {
		w.fallbackPrompt(r, complete)
		return
	}
	var parent any
	if native, ok := w.window.(driver.NativeWindow); ok {
		native.RunNative(func(raw any) {
			switch c := raw.(type) {
			case driver.WindowsWindowContext:
				if c.HWND != 0 {
					parent = c.HWND
				}
			case driver.X11WindowContext:
				if c.WindowHandle != 0 {
					parent = int(c.WindowHandle)
				}
			}
		})
	}
	w.jobs.Go(func(ctx context.Context) {
		value, err := w.dialogs.show(ctx, r, parent)
		w.post(func() {
			if err != nil && !errors.Is(err, zenity.ErrCanceled) && !errors.Is(err, context.Canceled) {
				// A missing or broken desktop helper must never approve an operation.
				w.fallbackPrompt(r, complete)
				return
			}
			ok := err == nil
			if r.kind == promptChoice {
				ok = ok && slices.Contains(r.choices, value)
			}
			complete(value, ok)
		})
	})
}

func (w *Window) fallbackPrompt(r promptRequest, done func(string, bool)) {
	accept, dismiss := r.accept, r.dismiss
	if accept == "" {
		accept = "Continue"
	}
	if dismiss == "" {
		dismiss = "Cancel"
	}
	switch r.kind {
	case promptInfo, promptError:
		var d dialog.Dialog
		if r.kind == promptError {
			d = dialog.NewError(errors.New(r.text), w.window)
		} else {
			d = dialog.NewInformation(r.title, r.text, w.window)
		}
		d.SetOnClosed(func() { done("", true) })
		d.Show()
	case promptConfirm:
		d := dialog.NewConfirm(r.title, r.text, func(ok bool) { done("", ok) }, w.window)
		d.SetConfirmText("Continue")
		d.SetDismissText("Cancel")
		d.Show()
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
		w.setPickerLocation(d, r.initial)
		d.Show()
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
		w.setPickerLocation(d, r.initial)
		d.Show()
	case promptEntry:
		entry := widget.NewEntry()
		entry.SetText(r.initial)
		dialog.ShowForm(r.title, accept, dismiss, []*widget.FormItem{widget.NewFormItem(r.text, entry)}, func(ok bool) { done(entry.Text, ok) }, w.window)
	case promptChoice:
		choice := widget.NewSelect(r.choices, nil)
		choice.SetSelected(r.initial)
		dialog.ShowForm(r.title, accept, dismiss, []*widget.FormItem{widget.NewFormItem(r.text, choice)}, func(ok bool) { done(choice.Selected, ok && choice.Selected != "") }, w.window)
	}
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
