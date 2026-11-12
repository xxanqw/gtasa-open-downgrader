package applog

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestNestedOperationsAndErrorCauses(t *testing.T) {
	j := &Journal{}
	ctx, finish := Begin(j.Context(context.Background()), "install", "mod", "example")
	child, end := Begin(ctx, "write", "path", "line\nname")
	Event(child, "DEBUG", "decision", "preserve", true)
	err := errors.Join(fmt.Errorf("commit: %w", os.ErrPermission), fmt.Errorf("cleanup: %w", context.DeadlineExceeded))
	end(err)
	end(err)
	finish(context.Canceled)
	text, _ := j.Snapshot()
	for _, expected := range []string{"install.start op=1 parent=0", "write.start op=2 parent=1", "decision op=2 preserve=true", `path="line\nname"`, "duration_ms=", "permission denied", "context.deadlineExceededError", `result="cancelled"`} {
		if !strings.Contains(text, expected) {
			t.Errorf("missing %q in %s", expected, text)
		}
	}
	if strings.Count(text, "write.end") != 1 {
		t.Fatal("completion logged more than once")
	}
}

func TestRequestSecretsExcluded(t *testing.T) {
	raw := "https://person:password@example.test/archive?token=secret#private"
	err := fmt.Errorf("download: %w", &url.Error{Op: "Get", URL: raw, Err: context.DeadlineExceeded})
	j := &Journal{}
	ctx, finish := Begin(j.Context(context.Background()), "download", "url", URL(raw))
	Event(ctx, "ERROR", "request", "error", err)
	finish(err)
	text, _ := j.Snapshot()
	for _, secret := range []string{"person", "password", "token", "secret", "private"} {
		if strings.Contains(text, secret) {
			t.Errorf("leaked %q", secret)
		}
	}
	if !strings.Contains(text, "https://example.test/archive") {
		t.Fatal("lost useful request URL")
	}
}

func TestStoredLogsSurviveExitAndRotateWithinBounds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "current.log")
	j := &Journal{}
	if err := j.Store(path); err != nil {
		t.Fatal(err)
	}
	j.Record("INFO", "previous run")
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}
	j = &Journal{}
	if err := j.Store(path); err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	previous, err := os.ReadFile(path + ".1")
	if err != nil || !strings.Contains(string(previous), "previous run") {
		t.Fatal("previous session lost", err)
	}
	// One oversized write also obeys the disk budget.
	j.Write([]byte(strings.Repeat("x", maxFileBytes*4)))
	j.Record("ERROR", "latest failure")
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 3 {
		t.Fatal("unexpected retained segments", len(entries), err)
	}
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil || info.Size() > maxFileBytes {
			t.Fatal("unbounded segment", entry.Name(), err)
		}
		if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
			t.Fatal("log permissions", info.Mode())
		}
	}
	latest, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(latest), "latest failure") {
		t.Fatal("latest diagnostics missing", err)
	}
}
