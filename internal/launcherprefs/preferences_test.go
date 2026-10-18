package launcherprefs

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChoicesRoundTripAndResetOnlyMode(t *testing.T) {
	root := t.TempDir()
	other := filepath.Join(root, "gta_sa.set")
	os.WriteFile(other, []byte("keep settings"), 0600)
	if choice, err := Load(root); err != nil || choice != None {
		t.Fatal(choice, err)
	}
	for _, choice := range []Choice{SinglePlayer, SAMP, OpenMP, MTA, None} {
		if err := Save(context.Background(), root, choice); err != nil {
			t.Fatal(err)
		}
		got, err := Load(root)
		if err != nil || got != choice {
			t.Fatal(got, err)
		}
	}
	if err := Reset(root); err != nil {
		t.Fatal(err)
	}
	if err := Reset(root); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(other)
	if string(data) != "keep settings" {
		t.Fatal("reset changed unrelated preferences")
	}
}
func TestMalformedPreferencesRefuseSaveButCanReset(t *testing.T) {
	for _, data := range []string{`{}`, `{"schema_version":2,"remembered_choice":"samp"}`, `{"schema_version":1,"remembered_choice":"unknown"}`, `{"schema_version":1,"remembered_choice":"samp","password":"secret"}`, `{"schema_version":1,"remembered_choice":"samp"} {}`, strings.Repeat("x", 4097)} {
		t.Run(data[:min(30, len(data))], func(t *testing.T) {
			root := t.TempDir()
			p := filepath.Join(root, Path)
			os.MkdirAll(filepath.Dir(p), 0755)
			os.WriteFile(p, []byte(data), 0600)
			if _, err := Load(root); err == nil {
				t.Fatal("malformed record accepted")
			}
			if err := Save(context.Background(), root, SinglePlayer); err == nil {
				t.Fatal("invalid record silently overwritten")
			}
			if err := Reset(root); err != nil {
				t.Fatal(err)
			}
			if choice, err := Load(root); err != nil || choice != None {
				t.Fatal(choice, err)
			}
		})
	}
}
func TestPreferencesRefuseSymlinksAndDirectories(t *testing.T) {
	for _, variant := range []string{"parent-link", "file-link", "directory"} {
		t.Run(variant, func(t *testing.T) {
			root := t.TempDir()
			outside := t.TempDir()
			target := filepath.Join(outside, "saved.json")
			os.WriteFile(target, []byte("preserve"), 0600)
			p := filepath.Join(root, Path)
			switch variant {
			case "parent-link":
				if err := os.Symlink(outside, filepath.Join(root, ".downgrader")); err != nil {
					t.Fatal(err)
				}
			case "file-link":
				os.MkdirAll(filepath.Dir(p), 0755)
				if err := os.Symlink(target, p); err != nil {
					t.Fatal(err)
				}
			case "directory":
				os.MkdirAll(p, 0755)
			}
			if _, err := Load(root); err == nil {
				t.Fatal("unsafe path accepted")
			}
			if err := Save(context.Background(), root, SAMP); err == nil {
				t.Fatal("unsafe save accepted")
			}
			if err := Reset(root); err == nil {
				t.Fatal("unsafe reset accepted")
			}
			data, _ := os.ReadFile(target)
			if string(data) != "preserve" {
				t.Fatal("external target changed")
			}
		})
	}
}
func TestCancelledPreferenceSaveKeepsPreviousMode(t *testing.T) {
	root := t.TempDir()
	if err := Save(context.Background(), root, SAMP); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Save(ctx, root, MTA); err == nil {
		t.Fatal("cancelled write committed")
	}
	choice, err := Load(root)
	if err != nil || choice != SAMP {
		t.Fatal(choice, err)
	}
	if err := Save(context.Background(), root, Choice("cmd.exe")); err == nil {
		t.Fatal("arbitrary executable persisted")
	}
}
