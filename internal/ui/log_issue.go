package ui

import (
	"context"
	"fmt"
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/xxanqw/gtasa-open-downgrader/internal/applog"
	"github.com/xxanqw/gtasa-open-downgrader/internal/logshare"
	appwidget "github.com/xxanqw/gtasa-open-downgrader/internal/uiwidget"
	"github.com/xxanqw/gtasa-open-downgrader/internal/updater"
)

func (w *Window) openLogIssue(link string) {
	if err := w.app.OpenURL(logshare.IssueDraft(updater.Version, runtime.GOOS+"/"+runtime.GOARCH, link)); err != nil {
		w.error(err)
	}
}

func (w *Window) proposeIssueLogs(text string) {
	w.prompt(promptRequest{present: func(done func(string, bool)) func() {
		ctx, cancel := context.WithCancel(w.logs.Context(context.Background()))
		closed, uploading := false, false
		link := ""
		status := promptText("")
		body := container.NewVBox(
			promptText(fmt.Sprintf("Upload this session log (%.0f KiB) to files.xxanqw.me and include its link in your GitHub issue?", float64(len(text))/1024)),
			promptText("Anyone with the link can read it. The log includes local paths and diagnostic details. It is deleted after 30 days."),
			status,
		)
		modal := appwidget.ShowModal(w.window, appwidget.ModalOptions{
			Title: "Include a diagnostic log?", Subtitle: "You review and submit the issue on GitHub.", Body: body, Size: fyne.NewSize(620, 330),
			Actions: []appwidget.ModalAction{
				{Text: "Cancel"},
				{ID: "without", Text: "Continue without upload"},
				{ID: "upload", Text: "Upload log", Importance: widget.HighImportance, KeepOpen: true, Disabled: text == "", Run: func(m *appwidget.Modal) {
					if uploading {
						return
					}
					uploading = true
					status.SetText("Uploading log…")
					m.SetActionEnabled("upload", false)
					m.SetActionEnabled("without", false)
					w.uploadIssueLog(ctx, text, func(receipt logshare.Receipt, err error) {
						if closed {
							return
						}
						uploading = false
						if err != nil {
							status.SetText("Upload failed: " + applog.SafeError(err) + "\nRetry, or continue without an upload and attach an exported log.")
							m.SetActionEnabled("upload", true)
							m.SetActionEnabled("without", true)
							return
						}
						link = receipt.URL
						m.SetSubtitle("Log uploaded · expires " + receipt.ExpiresAt.Local().Format("2 Jan 2006"))
						m.SetBody(container.NewVBox(promptText("This link will be included in the issue draft. Describe the problem on GitHub before submitting it."), promptText(link)))
						m.SetActions(appwidget.ModalAction{Text: "Close"},
							appwidget.ModalAction{ID: "copy", Text: "Copy log link", KeepOpen: true, Run: func(*appwidget.Modal) { w.window.Clipboard().SetContent(link) }},
							appwidget.ModalAction{ID: "issue", Text: "Open GitHub issue", Importance: widget.HighImportance})
					})
				}},
			}, OnClosed: func(result string) {
				closed = true
				cancel()
				value := ""
				if result == "issue" {
					value = link
				}
				done(value, result == "issue" || result == "without")
			},
		})
		return func() { modal.Close("") }
	}, done: func(action string, ok bool) {
		if ok {
			w.openLogIssue(action)
		}
	}})
}

// Upload work owns only an immutable snapshot and posts its result to the UI.
// Closing the sheet or the application cancels the same request.
func (w *Window) uploadIssueLog(ctx context.Context, text string, done func(logshare.Receipt, error)) {
	w.jobs.Go(func(parent context.Context) {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		stop := context.AfterFunc(parent, cancel)
		defer stop()
		ctx, finish := applog.Begin(ctx, "log.upload", "bytes", len(text))
		var receipt logshare.Receipt
		err := w.logs.Guard(ctx, func(ctx context.Context) error {
			var err error
			receipt, err = logshare.Upload(ctx, w.client, logshare.UploadURL, text)
			return err
		})
		finish(err)
		w.post(func() { done(receipt, err) })
	})
}
