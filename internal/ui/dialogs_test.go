package ui

import (
	"context"
	"errors"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/ncruces/zenity"
)

// Fyne's test driver runs Do immediately on the calling goroutine. Queue those
// callbacks so dialog tests reproduce the real desktop event loop's ordering.
type dialogTestDriver struct {
	fyne.Driver
	calls chan func()
}

func (d *dialogTestDriver) DoFromGoroutine(fn func(), wait bool) {
	if !wait {
		d.calls <- fn
		return
	}
	done := make(chan struct{})
	d.calls <- func() { fn(); close(done) }
	<-done
}

type dialogTestApp struct {
	fyne.App
	driver *dialogTestDriver
}

func (a *dialogTestApp) Driver() fyne.Driver   { return a.driver }
func (a *dialogTestApp) dispatch(t *testing.T) { t.Helper(); receive(t, a.driver.calls)() }
func newDialogTestApp() *dialogTestApp {
	base := test.NewApp()
	a := &dialogTestApp{App: base, driver: &dialogTestDriver{Driver: base.Driver(), calls: make(chan func(), 16)}}
	fyne.SetCurrentApp(a)
	return a
}

func receive[T any](t *testing.T, c <-chan T) T {
	t.Helper()
	select {
	case value := <-c:
		return value
	case <-time.After(3 * time.Second):
		t.Fatal("dialog did not complete")
		var zero T
		return zero
	}
}
func TestNativePromptsSerializeAndRespectCancel(t *testing.T) {
	a := newDialogTestApp()
	defer a.Quit()
	w := New(a)
	defer w.jobs.Close()
	requests := make(chan promptRequest, 2)
	replies := make(chan error, 2)
	done := make(chan bool, 2)
	w.dialogs = dialogBackend{available: func() bool { return true }, show: func(ctx context.Context, r promptRequest, parent any) (string, error) {
		requests <- r
		select {
		case err := <-replies:
			return "", err
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}}
	w.confirm("First", "Delete backups?", func(ok bool) { done <- ok })
	receive(t, requests)
	if !w.path.Disabled() {
		t.Fatal("game path can change during confirmation")
	}
	w.confirm("Second", "Download patches?", func(ok bool) { done <- ok })
	select {
	case <-requests:
		t.Fatal("overlapping native prompts")
	default:
	}
	replies <- zenity.ErrCanceled
	a.dispatch(t)
	if receive(t, done) {
		t.Fatal("Cancel approved an action")
	}
	if r := receive(t, requests); r.title != "Second" {
		t.Fatal("lost queued prompt")
	}
	replies <- nil
	a.dispatch(t)
	if !receive(t, done) {
		t.Fatal("confirmation was lost")
	}
	w.jobs.Close() // Wait for final widget updates before reading them.
	if w.promptActive || w.path.Disabled() {
		t.Fatal("controls stayed blocked after dialog closed")
	}
}
func TestClosingCancelsNativeDialog(t *testing.T) {
	a := newDialogTestApp()
	defer a.Quit()
	w := New(a)
	started := make(chan bool)
	dismissed := make(chan bool, 1)
	called := make(chan bool, 1)
	w.dialogs = dialogBackend{available: func() bool { return true }, show: func(ctx context.Context, r promptRequest, parent any) (string, error) {
		close(started)
		<-ctx.Done()
		dismissed <- true
		return "", ctx.Err()
	}}
	w.confirm("Confirm", "Continue?", func(ok bool) { called <- ok })
	receive(t, started)
	w.closing = true
	w.jobs.Close()
	a.dispatch(t)
	receive(t, dismissed)
	select {
	case <-called:
		t.Fatal("dialog callback ran after shutdown")
	default:
	}
}
func TestMissingNativeHelperUsesExplicitFallbackConfirmation(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := New(a)
	defer w.jobs.Close()
	w.dialogs = dialogBackend{available: func() bool { return false }}
	w.window.Show()
	for _, answer := range []string{"Cancel", "Continue"} {
		called, approved := false, false
		w.confirm("Restore originals", "Overwrite current files?", func(ok bool) { called, approved = true, ok })
		if called {
			t.Fatal("missing dialog helper approved an action")
		}
		popup := w.window.Canvas().Overlays().Top()
		button := promptButton(popup, answer)
		if button == nil {
			t.Fatalf("fallback has no %s button", answer)
		}
		test.Tap(button)
		if !called || approved != (answer == "Continue") {
			t.Fatal("fallback returned the wrong decision")
		}
	}
}
func promptButton(o fyne.CanvasObject, text string) *widget.Button {
	switch v := o.(type) {
	case *widget.Button:
		if v.Text == text {
			return v
		}
	case *widget.PopUp:
		return promptButton(v.Content, text)
	case fyne.Widget:
		for _, child := range test.WidgetRenderer(v).Objects() {
			if b := promptButton(child, text); b != nil {
				return b
			}
		}
	case *fyne.Container:
		for _, child := range v.Objects {
			if b := promptButton(child, text); b != nil {
				return b
			}
		}
	}
	return nil
}

func TestBrokenNativeHelperFallsBackWithoutApproving(t *testing.T) {
	a := newDialogTestApp()
	defer a.Quit()
	w := New(a)
	w.window.Show()
	w.dialogs = dialogBackend{available: func() bool { return true }, show: func(context.Context, promptRequest, any) (string, error) {
		return "", errors.New("desktop helper failed")
	}}
	called := false
	w.confirm("Delete backups", "Permanently delete backups?", func(ok bool) {
		called = true
		if ok {
			t.Error("failed helper approved deletion")
		}
	})
	w.jobs.Close()
	a.dispatch(t)
	if called {
		t.Fatal("helper error bypassed confirmation")
	}
	cancel := promptButton(w.window.Canvas().Overlays().Top(), "Cancel")
	if cancel == nil {
		t.Fatal("no fallback after helper error")
	}
	test.Tap(cancel)
	if !called {
		t.Fatal("fallback Cancel did not complete the prompt")
	}
}
