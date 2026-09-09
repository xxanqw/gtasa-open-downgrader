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

// A local desktop-helper stand-in verifies selection arguments without opening
// a real system window during tests.
func TestSystemPickerUsesNativeSelectionModes(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$PICKER_ARGS\"\nif [ \"$PICKER_WAIT\" = 1 ]; then exec /bin/sleep 30; fi\nprintf '%s' \"$PICKER_VALUE\"\nexit \"$PICKER_EXIT\"\n"
	if err := os.WriteFile(filepath.Join(dir, "qarma"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("PICKER_ARGS", filepath.Join(dir, "args"))
	t.Setenv("PICKER_EXIT", "0")
	t.Setenv("PICKER_VALUE", "/games/GTA SA/patches.zip")
	path, err := systemPicker(context.Background(), promptRequest{kind: promptFile, title: "Import patches", initial: dir, extensions: []string{".zip", ".7z"}}, int(42))
	if err != nil || path != "/games/GTA SA/patches.zip" {
		t.Fatalf("open picker: %q, %v", path, err)
	}
	args, err := os.ReadFile(filepath.Join(dir, "args"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"--file-selection", "--title", "Import patches", "--attach", "42", "*.[zZ][iI][pP]", "*.7[zZ]"} {
		if !strings.Contains(string(args), want) {
			t.Fatalf("missing %q from %s", want, args)
		}
	}

	t.Setenv("PICKER_VALUE", "/home/player/gtasa.log")
	path, err = systemPicker(context.Background(), promptRequest{kind: promptSave, title: "Export log", initial: "/home/player/gtasa.log", extensions: []string{".log"}}, nil)
	if err != nil || path != "/home/player/gtasa.log" {
		t.Fatalf("save picker: %q, %v", path, err)
	}
	args, err = os.ReadFile(filepath.Join(dir, "args"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"--save", "--confirm-overwrite", "/home/player/gtasa.log"} {
		if !strings.Contains(string(args), want) {
			t.Fatalf("missing %q from %s", want, args)
		}
	}
}

func TestSystemPickerCancellation(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\nif [ \"$PICKER_WAIT\" = 1 ]; then exec /bin/sleep 30; fi\nexit \"$PICKER_EXIT\"\n"
	if err := os.WriteFile(filepath.Join(dir, "qarma"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("PICKER_EXIT", "1")
	_, err := systemPicker(context.Background(), promptRequest{kind: promptFolder}, nil)
	if !errors.Is(err, zenity.ErrCanceled) {
		t.Fatalf("cancel = %v", err)
	}
	t.Setenv("PICKER_WAIT", "1")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err = systemPicker(ctx, promptRequest{kind: promptFolder}, nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("context cancellation = %v", err)
	}
}
