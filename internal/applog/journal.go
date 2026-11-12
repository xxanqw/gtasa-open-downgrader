// Package applog keeps a bounded, concurrent application journal for diagnostics.
package applog

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

const maxBytes = 1 << 20

type Journal struct {
	mu            sync.Mutex
	text          bytes.Buffer
	revision      uint64
	nextOperation uint64
	file          *os.File
	filePath      string
	fileBytes     int64
}

func (j *Journal) Record(level, message string) {
	j.Write([]byte(time.Now().Format("2006-01-02 15:04:05.000 -0700") + " [" + level + "] " + message + "\n"))
}

// Write also accepts diagnostics from Go's standard logger (including Fyne).
func (j *Journal) Write(p []byte) (int, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.text.Write(p)
	j.writeFile(p)
	if j.text.Len() > maxBytes {
		retained := j.text.Bytes()[j.text.Len()-maxBytes*3/4:]
		if i := bytes.IndexByte(retained, '\n'); i >= 0 {
			retained = retained[i+1:]
		}
		j.text = *bytes.NewBuffer(bytes.Clone(retained))
	}
	j.revision++
	return len(p), nil
}

func (j *Journal) Snapshot() (string, uint64) {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.text.String(), j.revision
}

// ChangesSince avoids copying a large idle journal on every UI polling tick.
func (j *Journal) ChangesSince(revision uint64) (string, uint64) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if revision == j.revision {
		return "", revision
	}
	return j.text.String(), j.revision
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
	if _, ok := ctx.Value(scopeKey{}).(scope); !ok {
		ctx = j.Context(ctx)
	}
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("unexpected operation error: %v (see application log for details)", p)
			Event(ctx, "ERROR", "worker.panic", "panic", fmt.Sprint(p), "stack", string(debug.Stack()))
		}
	}()
	return work(ctx)
}
