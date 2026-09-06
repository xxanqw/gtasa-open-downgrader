package ui

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func walkObjects(o fyne.CanvasObject, visit func(fyne.CanvasObject) bool) bool {
	if o == nil {
		return false
	}
	if visit(o) {
		return true
	}
	switch v := o.(type) {
	case *widget.PopUp:
		return walkObjects(v.Content, visit)
	case *fyne.Container:
		for _, child := range v.Objects {
			if walkObjects(child, visit) {
				return true
			}
		}
	case fyne.Widget:
		for _, child := range test.WidgetRenderer(v).Objects() {
			if walkObjects(child, visit) {
				return true
			}
		}
	}
	return false
}
func promptButton(o fyne.CanvasObject, text string) *widget.Button {
	var button *widget.Button
	walkObjects(o, func(o fyne.CanvasObject) bool {
		if b, ok := o.(*widget.Button); ok && b.Text == text {
			button = b
			return true
		}
		return false
	})
	return button
}
func tapPrompt(t *testing.T, w *Window, label string) {
	t.Helper()
	button := promptButton(w.window.Canvas().Overlays().Top(), label)
	if button == nil {
		t.Fatalf("missing dialog button %q", label)
	}
	test.Tap(button)
}
func TestFynePromptsSerializeAndRequireExplicitConfirmation(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := New(a)
	defer w.jobs.Close()
	w.window.Show()
	var decisions []bool
	w.confirm("Restore files", "Overwrite current files?", func(ok bool) { decisions = append(decisions, ok) })
	w.confirm("Delete backups", "Permanently delete backups?", func(ok bool) { decisions = append(decisions, ok) })
	if !w.path.Disabled() || len(w.prompts) != 1 {
		t.Fatal("prompt did not block path changes or queue the next question")
	}
	tapPrompt(t, w, "Cancel")
	if len(decisions) != 1 || decisions[0] {
		t.Fatal("Cancel approved the operation")
	}
	tapPrompt(t, w, "Continue")
	if len(decisions) != 2 || !decisions[1] || w.promptActive || w.path.Disabled() {
		t.Fatal("confirmation did not complete and restore controls")
	}
}
func TestPromptEscapeAndShutdownDoNotRunActions(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := New(a)
	defer w.jobs.Close()
	w.window.Show()
	called := false
	w.confirm("Confirm", "Continue?", func(ok bool) {
		called = true
		if ok {
			t.Error("Escape approved an action")
		}
	})
	w.window.Canvas().OnTypedKey()(&fyne.KeyEvent{Name: fyne.KeyEscape})
	if !called || w.promptActive {
		t.Fatal("Escape did not dismiss the prompt")
	}
	called = false
	w.confirm("Confirm", "Continue?", func(bool) { called = true })
	w.closing = true
	w.activePrompt()
	if called {
		t.Fatal("dialog callback ran after shutdown")
	}
}
func TestPickerCancelAndLongPromptsFitWindow(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := New(a)
	defer w.jobs.Close()
	w.window.Resize(fyne.NewSize(720, 480))
	w.window.Show()
	picked := false
	w.chooseFolder("Game directory", t.TempDir(), func(string) { picked = true })
	w.activePrompt()
	if picked || w.promptActive {
		t.Fatal("cancelled folder selection started an action")
	}
	w.confirm("Long message", strings.Repeat("A long file path and operation message. ", 100), func(bool) {})
	button := promptButton(w.window.Canvas().Overlays().Top(), "Continue")
	pos := a.Driver().AbsolutePositionForObject(button)
	if pos.X+button.Size().Width > 720 || pos.Y+button.Size().Height > 480 {
		t.Fatal("confirmation action is outside the window")
	}
	snapshot(t, w, "GTASA_PROMPT_SNAPSHOT")
	w.activePrompt()
}
