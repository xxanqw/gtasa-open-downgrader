package launcherapp

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/xxanqw/gtasa-open-downgrader/internal/entrypoint"
	"github.com/xxanqw/gtasa-open-downgrader/internal/testutil"
)

func TestDesktopMTAScriptUsesExistingExplorerAndSafeLiterals(t *testing.T) {
	directory := `C:\Users\O'Neil\MTA тест`
	executable := directory + `\Multi Theft Auto.exe`
	script := desktopMTAScript(directory, executable, []string{`quoted"value`, `space value`, `$(not executable)`})
	for _, part := range []string{"FindWindowSW(0,0,8,[ref]$h,1)", "$d.Document.Application.ShellExecute(", "O''Neil", "'open',1"} {
		if !strings.Contains(script, part) {
			t.Fatalf("desktop dispatch missing %q", part)
		}
	}
	if strings.Contains(script, "$s.ShellExecute(") {
		t.Fatal("dispatch runs in the injected caller instead of Explorer")
	}
	data, err := base64.StdEncoding.DecodeString(encodePowerShell(script))
	if err != nil {
		t.Fatal(err)
	}
	units := make([]uint16, len(data)/2)
	for i := range units {
		units[i] = binary.LittleEndian.Uint16(data[i*2:])
	}
	if string(utf16.Decode(units)) != script {
		t.Fatal("UTF-16 script encoding changed paths or arguments")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := executeDesktopMTA(ctx, directory, executable); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestMTAHostCacheIsNeutralAndContentVerified(t *testing.T) {
	root := t.TempDir()
	source := testutil.Write(t, root, "gta-sa.exe", "MZsynthetic own executable")
	cache := t.TempDir()
	host, err := stageMTAHost(context.Background(), source, cache)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(host) != "gtasa-mta-host.exe" || host == source {
		t.Fatal("MTA can mistake the host for GTA")
	}
	expected, err := entrypoint.Hash(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(filepath.Dir(host)) != expected {
		t.Fatal("host cache is not content addressed")
	}
	if err := os.WriteFile(host, []byte("modified host"), 0600); err != nil {
		t.Fatal(err)
	}
	reused, err := stageMTAHost(context.Background(), source, cache)
	if err != nil || reused != host {
		t.Fatal(reused, err)
	}
	actual, err := entrypoint.Hash(context.Background(), host)
	if err != nil || actual != expected {
		t.Fatal("unverified cached host reused", err)
	}
}

func TestMTAHostArgumentsDoNotFallThroughToGame(t *testing.T) {
	if handled, err := HandleMTAHost(nil); handled || err != nil {
		t.Fatal(handled, err)
	}
	if handled, err := HandleMTAHost([]string{mtaHostArgument}); !handled || err == nil {
		t.Fatal("invalid host invocation accepted", handled, err)
	}
}
