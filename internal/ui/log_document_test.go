package ui

import (
	"strings"
	"testing"
	"unicode/utf8"

	"fyne.io/fyne/v2"
)

func TestLogDisplayWrapsAndFiltersWithoutChangingRawLog(t *testing.T) {
	raw := "2026-11-12 16:28:49.555 +0100 [DEBUG] http.response op=51 status=200 url=\"https://example.test/" + strings.Repeat("archive-", 30) + "\"\n" +
		"2026-11-12 16:28:50.000 +0100 [ERROR] patch.file.end op=52 result=\"failed\"\n"
	e := newLogEntry(raw)
	e.Resize(fyne.NewSize(640, 400))
	if e.raw != raw || strings.Contains(e.Text, "2026-11-12") || !e.HideLineNumbers {
		t.Fatal("log mode did not shorten timestamp/hide gutter or changed raw data")
	}
	for _, line := range strings.Split(e.Text, "\n") {
		if utf8.RuneCountInString(line) > e.columns {
			t.Fatal("unwrapped log line", line)
		}
	}
	e.SetLevel("Warnings & errors")
	if strings.Contains(e.Text, "http.response") || !strings.Contains(e.Text, "patch.file.end") || e.raw != raw {
		t.Fatal("filter lost raw log or kept debug events")
	}
	e.SetLevel("All events")
	if !strings.Contains(e.Text, "http.response") {
		t.Fatal("clearing filter did not restore events")
	}
}

func TestIssueUploadIsOfferedWithoutSendingLog(t *testing.T) {
	w := newTestWindow(t)
	w.proposeIssueLogs("private example log")
	if !w.promptActive || findButton(w.window.Canvas().Overlays().Top(), "Upload log") == nil || findButton(w.window.Canvas().Overlays().Top(), "Continue without upload") == nil {
		t.Fatal("missing explicit upload choice")
	}
	text, _ := w.logs.Snapshot()
	if strings.Contains(text, "http.request") || strings.Contains(text, "log.upload.start") {
		t.Fatal("opened offer uploaded log automatically")
	}
	captureUX(t, w, "issue-log-offer")
}
