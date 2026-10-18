package download

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/xxanqw/gtasa-open-downgrader/internal/testutil"
)

func browserFixture(t *testing.T) ([]byte, string) {
	t.Helper()
	file := testutil.Zip(t, map[string]string{"official/readme.txt": "reviewed package"})
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	return data, fmt.Sprintf("%x", sha256.Sum256(data))
}
func TestBrowserArchiveVerifiedPrivateCopy(t *testing.T) {
	data, checksum := browserFixture(t)
	root := t.TempDir()
	source := testutil.Write(t, root, "package.zip", string(data))
	destination := testutil.Write(t, root, "staged.zip", "original destination")
	if err := CopyBrowserArchive(context.Background(), source, destination, checksum); err != nil {
		t.Fatal(err)
	}
	if got := testutil.Read(t, root, "staged.zip"); got != string(data) {
		t.Fatal("wrong private copy")
	}
	if got := testutil.Read(t, root, "package.zip"); got != string(data) {
		t.Fatal("user download was changed")
	}
	if err := CopyBrowserArchive(context.Background(), source, destination, fmt.Sprintf("%064d", 0)); err == nil {
		t.Fatal("wrong checksum accepted")
	}
	if testutil.Read(t, root, "staged.zip") != string(data) {
		t.Fatal("failed checksum replaced staged file")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := CopyBrowserArchive(ctx, source, destination, checksum); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
	for _, contents := range []string{"<html>anti-bot page</html>", "PK\x03\x04truncated zip"} {
		if err := os.WriteFile(source, []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
		if err := CopyBrowserArchive(context.Background(), source, destination, ""); err == nil {
			t.Fatal("non-archive or incomplete ZIP accepted")
		}
	}
}
func TestWatchDownloadsIgnoresOldPartialAndUnrelatedFiles(t *testing.T) {
	data, checksum := browserFixture(t)
	root := t.TempDir()
	testutil.Write(t, root, "package.zip", string(data)) // old matching download
	watch, err := WatchDownloads(root, "package.zip")
	if err != nil {
		t.Fatal(err)
	}
	testutil.Write(t, root, "unrelated.zip", string(data))
	testutil.Write(t, root, "package.zip.exe", string(data))
	testutil.Write(t, root, "package (1).zip", string(data))
	partial := testutil.Write(t, root, "package (1).zip.crdownload", "still downloading")
	destination := filepath.Join(t.TempDir(), "staged.zip")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- watch.Wait(ctx, destination, checksum, nil) }()
	select {
	case err := <-result:
		t.Fatalf("accepted partial/old/unrelated download: %v", err)
	case <-time.After(2500 * time.Millisecond):
	}
	if err := os.Remove(partial); err != nil {
		t.Fatal(err)
	}
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(destination); err != nil || string(got) != string(data) {
		t.Fatalf("staged=%q,%v", got, err)
	}
	if _, err := os.Stat(filepath.Join(root, "package (1).zip")); err != nil {
		t.Fatal("user download removed", err)
	}
}
func TestWatchDownloadsCancellationAndFilenameBounds(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"../package.zip", "payload.exe", "", "."} {
		if _, err := WatchDownloads(root, name); err == nil {
			t.Fatalf("accepted unsafe filename %q", name)
		}
	}
	watch, err := WatchDownloads(root, "package.7z")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := watch.Wait(ctx, filepath.Join(t.TempDir(), "staged"), "", nil); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
func TestBrowserArchiveRejectsSymlink(t *testing.T) {
	data, checksum := browserFixture(t)
	root := t.TempDir()
	source := testutil.Write(t, root, "real.zip", string(data))
	link := filepath.Join(root, "package.zip")
	if err := os.Symlink(source, link); err != nil {
		t.Skip("symlinks unavailable", err)
	}
	if err := CopyBrowserArchive(context.Background(), link, filepath.Join(t.TempDir(), "staged"), checksum); err == nil {
		t.Fatal("symlink accepted")
	}
	watch, err := WatchDownloads(root, "package.zip")
	if err != nil {
		t.Fatal(err)
	}
	files, err := watch.files()
	if err != nil || len(files) != 0 {
		t.Fatalf("watched symlink: %v,%v", files, err)
	}
}
