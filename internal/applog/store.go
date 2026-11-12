package applog

import (
	"fmt"
	"os"
	"path/filepath"
)

const maxFileBytes = 4 << 20

// Store keeps current.log and two older segments, each limited to 4 MiB.
// Rotate at startup too, retaining the previous run for post-exit diagnostics.
func (j *Journal) Store(path string) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.file != nil {
		return fmt.Errorf("journal already has a log file")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	j.filePath = path
	return j.rotateFile()
}

func (j *Journal) rotateFile() error {
	if j.file != nil {
		if err := j.file.Close(); err != nil {
			j.file = nil
			return err
		}
		j.file = nil
	}
	for _, pair := range [][2]string{{j.filePath + ".1", j.filePath + ".2"}, {j.filePath, j.filePath + ".1"}} {
		if err := os.Remove(pair[1]); err != nil && !os.IsNotExist(err) {
			return err
		}
		if err := os.Rename(pair[0], pair[1]); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	f, err := os.OpenFile(j.filePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	j.file, j.fileBytes = f, 0
	return nil
}

// Called under the journal mutex. A full disk cannot break game operations.
func (j *Journal) writeFile(p []byte) {
	if j.file == nil {
		return
	}
	var err error
	for len(p) > 0 && err == nil {
		if j.fileBytes == maxFileBytes {
			err = j.rotateFile()
			if err != nil {
				break
			}
		}
		size := min(int64(len(p)), maxFileBytes-j.fileBytes)
		var n int
		n, err = j.file.Write(p[:size])
		j.fileBytes += int64(n)
		p = p[n:]
	}
	if err != nil {
		if j.file != nil {
			j.file.Close()
			j.file = nil
		}
		fmt.Fprintf(&j.text, "[WARN] Persistent logging disabled: %s\n", err)
	}
}

func (j *Journal) Close() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.file == nil {
		return nil
	}
	err := j.file.Close()
	j.file = nil
	return err
}
