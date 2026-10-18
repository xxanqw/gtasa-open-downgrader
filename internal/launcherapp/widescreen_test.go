package launcherapp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xxanqw/gtasa-open-downgrader/internal/entrypoint"
	"github.com/xxanqw/gtasa-open-downgrader/internal/multiplayer"
)

func wsWrite(t *testing.T, root, name, contents string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0640); err != nil {
		t.Fatal(err)
	}
}
func wsRuntime(root string) *Runtime {
	return &Runtime{GameDir: root, snapshotGameProcesses: func() (gameProcessSet, error) { return gameProcessSet{}, nil }}
}
func wsExpect(t *testing.T, root, name, want string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
	if err != nil || string(data) != want {
		t.Fatalf("%s = %q, %v; want %q", name, data, err, want)
	}
}
func wsAbsent(t *testing.T, root, name string) {
	t.Helper()
	if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(name))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("%s still present: %v", name, err)
	}
}

func TestWidescreenOnlyPausedAcrossAutoloadLocations(t *testing.T) {
	root := t.TempDir()
	r := wsRuntime(root)
	asis := []string{"GTASA.WidescreenFix.asi", "scripts/nested/gtasa.widescreenfix.ASI", "plugins/GTASA.WidescreenFixLegacy.asi", "modloader/WidescreenFix/GTASA.WidescreenFix.asi", "modloader/renamed/copy.asi"}
	for _, p := range asis {
		wsWrite(t, root, p, "widescreen")
	}
	untouched := map[string]string{"modloader/WidescreenFix/GTASA.WidescreenFix.ini": "preferences", "modloader.asi": "loader", "modloader/ExtendedLanguageLoader/ExtendedLanguageLoader.asi": "ell", "modloader/ExtendedLanguageLoader/fonts.txd": "fonts", "scripts/unrelated.asi": "other", "modloader/FrontendMods/GTASA.WidescreenFrontend.asi": "frontend"}
	for p, v := range untouched {
		wsWrite(t, root, p, v)
	}
	if err := r.disableWidescreen(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, p := range asis {
		wsAbsent(t, root, p)
	}
	for p, v := range untouched {
		wsExpect(t, root, p, v)
	}
	if err := r.restoreWidescreen(); err != nil {
		t.Fatal(err)
	}
	for _, p := range asis {
		wsExpect(t, root, p, "widescreen")
		info, _ := os.Stat(filepath.Join(root, p))
		if info.Mode().Perm() != 0640 {
			t.Fatal("permissions not preserved")
		}
	}
	if err := r.restoreWidescreen(); err != nil {
		t.Fatal("repeat recovery", err)
	}
	state, backup := r.widescreenPaths()
	for _, p := range []string{state, backup} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatal("recovery residue", p, err)
		}
	}
}

func TestWidescreenConflictAndBackupModificationRefused(t *testing.T) {
	for _, variant := range []string{"active", "backup", "extra", "invalid-state", "symlink-backup"} {
		t.Run(variant, func(t *testing.T) {
			root := t.TempDir()
			r := wsRuntime(root)
			p := "modloader/WidescreenFix/GTASA.WidescreenFix.asi"
			wsWrite(t, root, p, "widescreen")
			if err := r.disableWidescreen(context.Background()); err != nil {
				t.Fatal(err)
			}
			state, backup := r.widescreenPaths()
			switch variant {
			case "active":
				wsWrite(t, root, p, "user replacement")
			case "backup":
				if err := os.WriteFile(filepath.Join(backup, "000.bin"), []byte("changed"), 0640); err != nil {
					t.Fatal(err)
				}
			case "extra":
				if err := os.WriteFile(filepath.Join(backup, "user.txt"), []byte("keep"), 0640); err != nil {
					t.Fatal(err)
				}
			case "invalid-state":
				if err := os.WriteFile(state, []byte(`{"schema_version":99}`), 0600); err != nil {
					t.Fatal(err)
				}
			case "symlink-backup":
				if err := os.Rename(backup, backup+".saved"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(backup+".saved", backup); err != nil {
					t.Fatal(err)
				}
			}
			if err := r.restoreWidescreen(); err == nil {
				t.Fatal("unsafe recovery accepted")
			}
			if _, err := os.Stat(state); err != nil {
				t.Fatal("lost recovery state", err)
			}
			if variant == "active" {
				wsExpect(t, root, p, "user replacement")
				data, _ := os.ReadFile(filepath.Join(backup, "000.bin"))
				if string(data) != "widescreen" {
					t.Fatal("lost preserved ASI")
				}
			}
			if variant == "extra" {
				data, _ := os.ReadFile(filepath.Join(backup, "user.txt"))
				if string(data) != "keep" {
					t.Fatal("unrecorded backup file removed")
				}
			}
		})
	}
}

func TestWidescreenInterruptedRenamesRecover(t *testing.T) {
	root := t.TempDir()
	r := wsRuntime(root)
	paths := []string{"GTASA.WidescreenFix.asi", "scripts/GTASA.WidescreenFix.asi"}
	for _, p := range paths {
		wsWrite(t, root, p, "widescreen")
	}
	entries, err := r.findWidescreen(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	state, backup := r.widescreenPaths()
	if err := os.MkdirAll(backup, 0700); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(context.Background(), state, widescreenState{Schema: 1, Entries: entries}); err != nil {
		t.Fatal(err)
	}
	// Crash after the first rename; the second entry still exists at its source.
	if err := os.Rename(filepath.Join(root, entries[0].Path), filepath.Join(backup, "000.bin")); err != nil {
		t.Fatal(err)
	}
	if err := r.restoreCompatibility(); err != nil {
		t.Fatal(err)
	}
	for _, p := range paths {
		wsExpect(t, root, p, "widescreen")
	}
}

func TestWidescreenRefusesRunningUnreadableAndSymlinkedState(t *testing.T) {
	for _, variant := range []string{"running", "enumeration", "symlink", "cancelled"} {
		t.Run(variant, func(t *testing.T) {
			root := t.TempDir()
			r := wsRuntime(root)
			p := "modloader/WidescreenFix/GTASA.WidescreenFix.asi"
			wsWrite(t, root, p, "widescreen")
			ctx := context.Background()
			switch variant {
			case "running":
				r.snapshotGameProcesses = func() (gameProcessSet, error) { return gameProcessSet{42: {}}, nil }
			case "enumeration":
				r.snapshotGameProcesses = func() (gameProcessSet, error) { return nil, errors.New("enumeration denied") }
			case "symlink":
				if err := os.Symlink(filepath.Join(root, p), filepath.Join(root, "renamed.asi")); err != nil {
					t.Fatal(err)
				}
			case "cancelled":
				c, cancel := context.WithCancel(ctx)
				cancel()
				ctx = c
			}
			if err := r.disableWidescreen(ctx); err == nil {
				t.Fatal("unsafe preparation accepted")
			}
			wsExpect(t, root, p, "widescreen")
			state, _ := r.widescreenPaths()
			if _, err := os.Stat(state); !os.IsNotExist(err) {
				t.Fatal("state unexpectedly created", err)
			}
		})
	}
}

func TestWidescreenWaitFailurePreservesUntilSafeSinglePlayerRecovery(t *testing.T) {
	root := t.TempDir()
	r := wsRuntime(root)
	p := "modloader/WidescreenFix/GTASA.WidescreenFix.asi"
	wsWrite(t, root, p, "widescreen")
	wsWrite(t, root, entrypoint.RealGameExecutable, "game")
	r.waitForGameProcess = func(_ context.Context, _ gameProcessSet, done <-chan struct{}) (bool, error) {
		<-done
		wsAbsent(t, root, p)
		return true, errors.New("cannot enumerate GTA")
	}
	err := r.runWithGameCleanup(context.Background(), func() error { return nil }, func() error { t.Fatal("cleanup ran despite unsafe wait"); return nil })
	if err == nil {
		t.Fatal("wait failure swallowed")
	}
	wsAbsent(t, root, p)
	r.snapshotGameProcesses = func() (gameProcessSet, error) { return gameProcessSet{42: {}}, nil }
	if err := r.restoreWidescreen(); err == nil {
		t.Fatal("restored with GTA active")
	}
	wsAbsent(t, root, p)
	r.snapshotGameProcesses = func() (gameProcessSet, error) { return gameProcessSet{}, nil }
	r.Execute = func(_ context.Context, _, _ string, _ ...string) error {
		wsExpect(t, root, p, "widescreen")
		return nil
	}
	if err := r.SinglePlayer(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestWidescreenRecoveryPreservesConcurrentChanges(t *testing.T) {
	for _, variant := range []string{"destination", "backup"} {
		t.Run(variant, func(t *testing.T) {
			root := t.TempDir()
			r := wsRuntime(root)
			p := "modloader/WidescreenFix/GTASA.WidescreenFix.asi"
			wsWrite(t, root, p, "widescreen")
			if err := r.disableWidescreen(context.Background()); err != nil {
				t.Fatal(err)
			}
			_, backup := r.widescreenPaths()
			r.snapshotGameProcesses = func() (gameProcessSet, error) {
				if variant == "destination" {
					wsWrite(t, root, p, "concurrent replacement")
				} else {
					if err := os.WriteFile(filepath.Join(backup, "000.bin"), []byte("concurrent change"), 0640); err != nil {
						t.Fatal(err)
					}
				}
				return gameProcessSet{}, nil
			}
			if err := r.restoreWidescreen(); err == nil {
				t.Fatal("concurrent change overwritten")
			}
			if variant == "destination" {
				wsExpect(t, root, p, "concurrent replacement")
			} else {
				wsAbsent(t, root, p)
			}
			if _, err := os.Stat(filepath.Join(backup, "000.bin")); err != nil {
				t.Fatal("lost preserved file", err)
			}
		})
	}
}

func TestWidescreenFailedBootstrapRestores(t *testing.T) {
	root := t.TempDir()
	r := wsRuntime(root)
	p := "scripts/GTASA.WidescreenFix.asi"
	wsWrite(t, root, p, "widescreen")
	r.waitForGameProcess = func(_ context.Context, _ gameProcessSet, done <-chan struct{}) (bool, error) {
		<-done
		return false, nil
	}
	err := r.runWithGameCleanup(context.Background(), func() error { wsAbsent(t, root, p); return errors.New("bootstrap failed") }, nil)
	if err == nil || !strings.Contains(err.Error(), "bootstrap failed") {
		t.Fatal(err)
	}
	wsExpect(t, root, p, "widescreen")
}

func TestNativeSAMPAndOfficialOpenMPPauseWidescreen(t *testing.T) {
	for _, id := range []multiplayer.ClientID{multiplayer.ClientSAMP, multiplayer.ClientOpenMP} {
		t.Run(string(id), func(t *testing.T) {
			root := t.TempDir()
			r := wsRuntime(root)
			p := "modloader/WidescreenFix/GTASA.WidescreenFix.asi"
			wsWrite(t, root, p, "widescreen")
			wsWrite(t, root, "scripts/GTASA.WidescreenFix.asi", "widescreen")
			wsWrite(t, root, "samp.exe", "client")
			wsWrite(t, root, "samp.dll", "dll")
			wsWrite(t, root, "clients/omp-launcher.exe", "launcher")
			wsWrite(t, root, multiplayer.ConfigPath, `{"schema_version":1,"clients":{"openmp":{"executable":"clients/omp-launcher.exe"}}}`)
			r.Execute = func(_ context.Context, _, _ string, _ ...string) error {
				wsAbsent(t, root, p)
				wsAbsent(t, root, "scripts/GTASA.WidescreenFix.asi")
				return nil
			}
			r.waitForGameProcess = func(_ context.Context, _ gameProcessSet, done <-chan struct{}) (bool, error) {
				<-done
				wsAbsent(t, root, p)
				return true, nil
			}
			if err := r.Multiplayer(context.Background(), id); err != nil {
				t.Fatal(err)
			}
			wsExpect(t, root, p, "widescreen")
			wsExpect(t, root, "scripts/GTASA.WidescreenFix.asi", "widescreen")
		})
	}
}
