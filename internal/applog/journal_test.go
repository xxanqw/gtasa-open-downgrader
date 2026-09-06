package applog

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
)

func TestConcurrentBoundedJournal(t *testing.T) {
	j := &Journal{}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := 0; n < 100; n++ {
				j.Record("INFO", strings.Repeat("x", 2048))
				j.Snapshot()
			}
		}()
	}
	wg.Wait()
	j.Record("ERROR", "last failure")
	text, revision := j.Snapshot()
	if len(text) > maxBytes || revision != 801 || !strings.HasSuffix(text, "[ERROR] last failure\n") {
		t.Fatal("journal lost ordering, bounds or entries", len(text), revision)
	}
	if !strings.Contains(strings.SplitN(text, "\n", 2)[0], "[INFO]") {
		t.Fatal("retention cut through a line")
	}
}
func TestGuardRecordsPanicAndPreservesErrors(t *testing.T) {
	j := &Journal{}
	cleaned := false
	err := j.Guard(context.Background(), func(context.Context) error { defer func() { cleaned = true }(); panic("broken patch") })
	text, _ := j.Snapshot()
	if err == nil || !cleaned || !strings.Contains(text, "broken patch") || !strings.Contains(text, "goroutine") {
		t.Fatal("panic details or cleanup missing")
	}
	err = j.Guard(context.Background(), func(context.Context) error { return context.Canceled })
	if !errors.Is(err, context.Canceled) {
		t.Fatal("lost cancellation")
	}
}

type destination struct {
	buffer             strings.Builder
	writeErr, closeErr error
	closed             bool
}

func (d *destination) Write(p []byte) (int, error) {
	if d.writeErr != nil {
		return 0, d.writeErr
	}
	return d.buffer.Write(p)
}
func (d *destination) Close() error { d.closed = true; return d.closeErr }
func TestExportClosesAndReportsFailures(t *testing.T) {
	for _, d := range []*destination{{}, {writeErr: io.ErrShortWrite}, {closeErr: io.ErrClosedPipe}} {
		err := Export(d, "complete log\n")
		if !d.closed {
			t.Fatal("export left file open")
		}
		if d.writeErr != nil && !errors.Is(err, d.writeErr) || d.closeErr != nil && !errors.Is(err, d.closeErr) {
			t.Fatal("export hid failure", err)
		}
		if d.writeErr == nil && d.closeErr == nil && (err != nil || d.buffer.String() != "complete log\n") {
			t.Fatal("incomplete export")
		}
	}
}
