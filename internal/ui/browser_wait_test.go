package ui

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xxanqw/gtasa-open-downgrader/internal/download"
)

func browserWaitZIP(t *testing.T, contents string) []byte {
	t.Helper()
	var archive bytes.Buffer
	z := zip.NewWriter(&archive)
	f, err := z.Create("mod.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte(contents)); err != nil {
		t.Fatal(err)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return archive.Bytes()
}

func TestBrowserWaitCanChangeFolderWithoutRelaxingVerification(t *testing.T) {
	archive := browserWaitZIP(t, "reviewed mod")
	oldDir, newDir := t.TempDir(), t.TempDir()
	name := "reviewed.zip"
	if err := os.WriteFile(filepath.Join(oldDir, name), archive, 0600); err != nil {
		t.Fatal(err)
	}
	watcher, err := download.WatchDownloads(oldDir, name)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	folders := make(chan string, 1)
	folders <- newDir
	changed, rejected := make(chan struct{}, 1), make(chan struct{}, 1)
	result := make(chan error, 1)
	destination := filepath.Join(t.TempDir(), "copied.zip")
	checksum := fmt.Sprintf("%x", sha256.Sum256(archive))
	go func() {
		result <- waitForBrowserArchive(ctx, watcher, oldDir, name, destination, checksum, folders, func(folder, message string) {
			if folder == newDir {
				select {
				case changed <- struct{}{}:
				default:
				}
			}
			if strings.Contains(message, "checksum") {
				select {
				case rejected <- struct{}{}:
				default:
				}
			}
		})
	}()
	select {
	case <-changed:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	// Even a valid ZIP must match the reviewed checksum in the new folder.
	if err := os.WriteFile(filepath.Join(newDir, name), browserWaitZIP(t, "unreviewed mod"), 0600); err != nil {
		t.Fatal(err)
	}
	select {
	case <-rejected:
	case err := <-result:
		t.Fatalf("unreviewed archive completed the wait: %v", err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatal("unreviewed archive was committed")
	}
	if err := os.WriteFile(filepath.Join(newDir, "reviewed (1).zip"), archive, 0600); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	data, err := os.ReadFile(destination)
	if err != nil || !bytes.Equal(data, archive) {
		t.Fatalf("incorrect destination: %v", err)
	}
	if _, err := os.Stat(filepath.Join(newDir, "reviewed (1).zip")); err != nil {
		t.Fatal("browser download was deleted")
	}
}

func TestBrowserWaitCancellationJoinsWatcher(t *testing.T) {
	directory := t.TempDir()
	watcher, err := download.WatchDownloads(directory, "mod.zip")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ready := make(chan struct{}, 1)
	result := make(chan error, 1)
	destination := filepath.Join(t.TempDir(), "copy.zip")
	go func() {
		result <- waitForBrowserArchive(ctx, watcher, directory, "mod.zip", destination, "", make(chan string), func(string, string) {
			select {
			case ready <- struct{}{}:
			default:
			}
		})
	}()
	select {
	case <-ready:
	case <-time.After(2 * time.Second):
		t.Fatal("watcher did not start")
	}
	cancel()
	select {
	case err := <-result:
		if err != context.Canceled {
			t.Fatalf("got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("watcher did not stop")
	}
}

func TestInvalidBrowserFolderKeepsOriginalWatch(t *testing.T) {
	directory := t.TempDir()
	watcher, err := download.WatchDownloads(directory, "mod.zip")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	folders := make(chan string, 1)
	folders <- filepath.Join(directory, "missing")
	notices := make(chan string, 8)
	result := make(chan error, 1)
	destination := filepath.Join(t.TempDir(), "copy.zip")
	go func() {
		result <- waitForBrowserArchive(ctx, watcher, directory, "mod.zip", destination, "", folders, func(folder, message string) { notices <- folder + "\n" + message })
	}()
	for {
		select {
		case notice := <-notices:
			if strings.Contains(notice, "Could not watch") {
				if !strings.Contains(notice, "Still watching "+directory) {
					t.Fatal("old folder was not retained")
				}
				cancel()
				if err := <-result; err != context.Canceled {
					t.Fatalf("got %v", err)
				}
				return
			}
		case err := <-result:
			t.Fatalf("invalid folder stopped the batch: %v", err)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
}
