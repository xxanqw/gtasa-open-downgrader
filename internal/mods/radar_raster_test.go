package mods

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/xxanqw/gtasa-open-downgrader/internal/testutil"
)

// The raster is copied unchanged, including the upstream sampler/mask fields.
// This checks the reviewed native layout, not a newly authored TXD encoder.
func checkOriginalRadarTxd(t *testing.T, b []byte, n int) {
	t.Helper()
	if len(b) != 131240 {
		t.Fatal("unexpected original radar size", n, len(b))
	}
	u32 := func(p int) uint32 { return binary.LittleEndian.Uint32(b[p:]) }
	if u32(0) != 0x16 || u32(4) != uint32(len(b)-12) || u32(28) != 0x15 || u32(52) != 9 || binary.LittleEndian.Uint16(b[24:]) != 1 {
		t.Fatal("invalid original radar dictionary", n)
	}
	name := fmt.Sprintf("radar%02d", n)
	if string(b[60:60+len(name)]) != name || b[60+len(name)] != 0 {
		t.Fatal("wrong texture name", n)
	}
	if u32(128) != 0x31545844 || binary.LittleEndian.Uint16(b[132:]) != 512 || binary.LittleEndian.Uint16(b[134:]) != 512 || b[137] != 1 || u32(140) != 512*512/2 {
		t.Fatal("not native 512px D3D9 DXT1", n)
	}
}
func TestOriginalRadarRejectsIncompleteOrMismatchedImport(t *testing.T) {
	root := t.TempDir()
	testutil.Write(t, root, "models/hud.txd", "native HUD")
	archive := testutil.Zip(t, map[string]string{"Atmosphere Interface Pack/HQ Radar/radar00.txd": "unreviewed", "Readme (or die).txt": "missing others"})
	if err := Import(context.Background(), root, HDRadarMap, archive, "test"); err == nil {
		t.Fatal("partial/unreviewed map accepted")
	}
	if _, err := os.Stat(filepath.Join(root, "modloader/HDRadarMap")); !os.IsNotExist(err) {
		t.Fatal("failed import activated partial map")
	}
	if testutil.Read(t, root, "models/hud.txd") != "native HUD" {
		t.Fatal("root HUD changed")
	}
}
func TestLegacySVGRadarReplacedWithBackupAndCanonicalTracker(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	old := syntheticInterface(t, HDRadarMap)
	data, err := os.ReadFile(filepath.Join(old, interfaceMarker))
	if err != nil {
		t.Fatal(err)
	}
	var m interfaceManifest
	if err = json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	m.Name = legacySVGRadarMap
	data, err = json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	testutil.Write(t, old, interfaceMarker, string(data))
	if err = copyExtendedDir(ctx, old, filepath.Join(root, "modloader/HDRadarMap")); err != nil {
		t.Fatal(err)
	}
	testutil.Write(t, root, TrackerPath, `{"schema_version":2,"installed_mods":["HD Radar Map (SVG)","SilentPatch"],"mod_versions":{"HD Radar Map (SVG)":{"version":"old","fingerprint":"svg"}},"custom":{"keep":true}}`)
	if err = commitInterface(ctx, root, HDRadarMap, syntheticInterface(t, HDRadarMap), Release{Version: "raster", Fingerprint: "raster"}, "test", closedTest, nil); err != nil {
		t.Fatal(err)
	}
	tracker, err := LoadTracker(root)
	if err != nil {
		t.Fatal(err)
	}
	var keep map[string]bool
	_ = json.Unmarshal(tracker.Extra["custom"], &keep)
	if !tracker.Has(HDRadarMap) || !tracker.Has("SilentPatch") || tracker.Versions[HDRadarMap].Version != "raster" || !keep["keep"] {
		t.Fatal("tracker migration lost unrelated state", tracker)
	}
	if _, ok := tracker.Versions[legacySVGRadarMap]; ok {
		t.Fatal("duplicate old map version")
	}
	backups, err := filepath.Glob(filepath.Join(root, "backups/mods/"+backupPrefix(HDRadarMap)+"-*/previous/"+interfaceMarker))
	if err != nil || len(backups) != 1 {
		t.Fatal("SVG map not backed up", backups, err)
	}
	b, err := os.ReadFile(backups[0])
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(b, &m); err != nil || m.Name != legacySVGRadarMap {
		t.Fatal("old map inventory was not preserved", err)
	}
	if _, err = Uninstall(ctx, root, legacySVGRadarMap, "test"); err != nil {
		t.Fatal("legacy-name removal failed", err)
	}
}
