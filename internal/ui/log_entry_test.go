package ui

import (
	"strings"
	"testing"
)

func TestLogSearchUsesRuneOffsetsAndWraps(t *testing.T) {
	e := newLogEntry("αβ first\nneedle\nmore needle\n")
	if !findLog(e, "needle", false) || e.SelectedText() != "needle" {
		t.Fatal("first match missing")
	}
	first, _ := e.SelectionRange()
	if !findLog(e, "needle", false) {
		t.Fatal("second match missing")
	}
	second, _ := e.SelectionRange()
	if second <= first {
		t.Fatal("search did not advance")
	}
	findLog(e, "needle", false)
	wrapped, _ := e.SelectionRange()
	if wrapped != first {
		t.Fatal("search did not wrap")
	}
	findLog(e, "needle", true)
	previous, _ := e.SelectionRange()
	if previous != second || findLog(e, "absent", false) {
		t.Fatal("reverse wrap or absent result wrong")
	}
}

func TestLogViewKeepsCompleteJournalAndHighlightsStructuredFields(t *testing.T) {
	text := strings.Repeat("[DEBUG] item\n", 10000) + "[ERROR] failure\n"
	e := newLogEntry(text)
	if e.raw != text || !e.ReadOnly || !strings.Contains(e.Text, "[ERROR] failure") {
		t.Fatal("log viewer truncated text or allowed editing")
	}
	line := `2026-11-12 05:48:32.123 +0100 [ERROR] patch.file.end op=12 path="mods/пример.asi" result="failed"`
	tokens := logTokens(line)
	seen := map[string]bool{}
	runes := []rune(line)
	for _, token := range tokens {
		if token.Start < 0 || token.End > len(runes) || token.Start >= token.End {
			t.Fatal("invalid highlight", token)
		}
		seen[string(runes[token.Start:token.End])] = true
	}
	for _, required := range []string{"[ERROR]", "op", "12", "path", `"mods/пример.asi"`, "result", `"failed"`} {
		if !seen[required] {
			t.Error("missing highlight", required)
		}
	}
}

func TestApplicationLogUsesSharedEditor(t *testing.T) {
	w := newTestWindow(t)
	w.logs.Event("INFO", "session.start", "version", "v0.3.0", "os", "linux", "arch", "amd64")
	w.logs.Event("DEBUG", "operation.start", "op", 1, "title", "Installing mods", "game", "/games/Grand Theft Auto San Andreas")
	w.logs.Event("DEBUG", "mod.metadata", "op", 2, "url", "https://dwngrdr.xxanqw.me/api/v1/github/repos/ThirteenAG/III.VC.SA.WindowedMode/releases/latest", "cache_hit", false)
	w.logs.Event("DEBUG", "http.response", "op", 3, "status", 404, "request_id", "metadata-123")
	w.logs.Event("ERROR", "http.request.end", "op", 3, "result", "failed", "duration_ms", 103.42, "errors", []string{"Windowed Mode: HTTP 404"})
	w.showLogs()
	if !w.promptActive {
		t.Fatal("log panel did not open")
	}
	captureUX(t, w, "application-log")
}
