//go:build linux

package ui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ncruces/zenity"
)

// A local stand-in for the desktop helper validates the real library boundary
// without opening windows, touching the user's clipboard, or requiring a display.
func TestSystemDialogHelper(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$DIALOG_ARGS\"\nif [ \"$DIALOG_WAIT\" = 1 ]; then exec /bin/sleep 30; fi\nprintf '%s' \"$DIALOG_VALUE\"\nexit \"$DIALOG_EXIT\"\n"
	if err := os.WriteFile(filepath.Join(dir, "qarma"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("DIALOG_ARGS", filepath.Join(dir, "args"))
	t.Setenv("DIALOG_EXIT", "0")
	t.Setenv("DIALOG_VALUE", "/games/GTA SA/patches.zip")
	path, err := systemDialog(context.Background(), promptRequest{kind: promptFile, title: "Import patches", extensions: []string{".zip", ".7z"}}, int(42))
	if err != nil || path != "/games/GTA SA/patches.zip" {
		t.Fatalf("picker: %q, %v", path, err)
	}
	args, err := os.ReadFile(filepath.Join(dir, "args"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"--file-selection", "--title", "Import patches", "--attach", "42", "*.[zZ][iI][pP]", "*.7[zZ]"} {
		if !strings.Contains(string(args), want) {
			t.Fatalf("missing %s from %s", want, args)
		}
	}
	t.Setenv("DIALOG_EXIT", "1")
	_, err = systemDialog(context.Background(), promptRequest{kind: promptConfirm, text: "Delete backups?"}, nil)
	if !errors.Is(err, zenity.ErrCanceled) {
		t.Fatalf("cancel: %v", err)
	}
	t.Setenv("DIALOG_EXIT", "2")
	_, err = systemDialog(context.Background(), promptRequest{kind: promptConfirm}, nil)
	if err == nil || errors.Is(err, zenity.ErrCanceled) {
		t.Fatalf("helper failure was not distinguished: %v", err)
	}
	t.Setenv("DIALOG_WAIT", "1")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err = systemDialog(ctx, promptRequest{kind: promptFolder}, nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cancellation did not stop helper: %v", err)
	}
}
