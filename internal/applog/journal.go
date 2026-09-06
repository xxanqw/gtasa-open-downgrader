// Package applog keeps a bounded, concurrent application journal for diagnostics.
package applog

import (
	"context"
	"fmt"
	"io"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

const maxBytes = 1 << 20

type Journal struct {
	mu       sync.Mutex
	text     string
	revision uint64
}

func (j *Journal) Record(level, message string) {
	j.Write([]byte(time.Now().Format("2006-01-02 15:04:05.000 -0700") + " [" + level + "] " + message + "\n"))
}

// Write also accepts diagnostics from Go's standard logger (including Fyne).
func (j *Journal) Write(p []byte) (int, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.text += string(p)
	if len(j.text) > maxBytes {
		j.text = j.text[len(j.text)-maxBytes:]
		if i := strings.IndexByte(j.text, '\n'); i >= 0 {
			j.text = j.text[i+1:]
		}
	}
	j.revision++
	return len(p), nil
}

func (j *Journal) Snapshot() (string, uint64) {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.text, j.revision
}

// Export closes the selected file even if writing fails, and reports both errors.
func Export(dst io.WriteCloser, text string) error {
	_, err := io.Copy(dst, strings.NewReader(text))
	closeErr := dst.Close()
	if err != nil {
		return err
	}
	return closeErr
}

// Guard turns a worker panic into an operation failure so its normal cleanup and
// completion path still run. The journal retains the stack for an exported report.
func (j *Journal) Guard(ctx context.Context, work func(context.Context) error) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("unexpected operation error: %v (see application log for details)", p)
			j.Record("ERROR", fmt.Sprintf("Recovered worker panic: %v\n%s", p, debug.Stack()))
		}
	}()
	return work(ctx)
}
