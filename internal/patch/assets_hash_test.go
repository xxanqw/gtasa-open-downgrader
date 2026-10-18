package patch

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOfficialArchiveHashRejectsUnexpectedBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Patches_v2.1.zip")
	if err := os.WriteFile(path, []byte("wrong archive"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := verifyOfficialArchive(context.Background(), path); err == nil || !strings.Contains(err.Error(), "SHA-256 mismatch") {
		t.Fatalf("unexpected archive verification result: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := verifyOfficialArchive(ctx, path); err != context.Canceled {
		t.Fatalf("cancelled archive verification: %v", err)
	}
}
