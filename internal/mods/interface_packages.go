package mods

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/xxanqw/gtasa-open-downgrader/internal/fsutil"
	"github.com/xxanqw/gtasa-open-downgrader/internal/platform"
)

const weaponHelperURL = "https://sharemods.com/6pk21r0b4ds0/Weapon_Icons_TXD.7z.html"
const weaponHelperSHA = "3d2e8d0d6a459859ee5d9ff0fed85d649b649465842db21c3393b7fc43d61622"
const interfaceMarker = "downgrader-package.json"

type interfaceSpec struct {
	folder, checksum string
	members, hashes  map[string]string
}
type interfaceManifest struct {
	Schema    int               `json:"schema"`
	Name      string            `json:"name"`
	SourceURL string            `json:"source_url"`
	Files     map[string]string `json:"files"`
}

func isInterfaceMod(name string) bool          { _, ok := interfacePacks[name]; return ok }
func hasTransactionalTracker(name string) bool { return name == ExtendedFonts || isInterfaceMod(name) }
func interfaceArchivePath(name, member string) string {
	spec := interfacePacks[name]
	for from, to := range spec.members {
		if strings.EqualFold(member, from) {
			return spec.folder + "/" + to
		}
	}
	return ""
}

// Atmosphere's hud.txd is BYTE IDENTICAL to Original HQ Icons. Install it once,
// owned by that dependency; omit Atmosphere's radar/menu replacements entirely.
func RequiredMods(name string) []string {
	if name == AtmosphereHUD {
		return []string{OriginalHQIcons}
	}
	return nil
}

func (i Installer) installInterface(ctx context.Context, r *Resolver, root, name string, rel Release, event Event) error {
	if err := preflightInterface(root, name); err != nil {
		return err
	}
	work, err := os.MkdirTemp("", "gtasa-interface-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	stage := filepath.Join(work, "stage")
	spec := interfacePacks[name]
	fetch := func(source Release, sha, dest string) error {
		raw, err := r.DownloadURL(ctx, source)
		if err != nil {
			return err
		}
		if err = i.Client.File(ctx, raw, dest, func(done, total int64) {
			if event != nil {
				event(name, "Downloading reviewed package", done, total)
			}
		}); err != nil {
			return err
		}
		return checkSHA(dest, sha)
	}
	archive := filepath.Join(work, "package.7z")
	if err = fetch(rel, spec.checksum, archive); err != nil {
		return err
	}
	if err = fsutil.Extract(ctx, archive, stage, "7z", func(m string) string { return interfaceArchivePath(name, m) }); err != nil {
		return err
	}
	if name == NadalaoWeapons {
		helper := filepath.Join(work, "helper.7z")
		if err = fetch(Release{URL: weaponHelperURL, FileID: "6pk21r0b4ds0"}, weaponHelperSHA, helper); err != nil {
			return err
		}
		// Never install the helper's empty 40-byte WeaponIcons.txd over the real art.
		if err = fsutil.Extract(ctx, helper, stage, "7z", func(m string) string {
			switch m {
			case "Weapon Icons TXD/WeaponIconsTXD.SA.asi":
				return spec.folder + "/WeaponIconsTXD.SA.asi"
			case "Readme (or die).txt":
				return spec.folder + "/README-helper.txt"
			}
			return ""
		}); err != nil {
			return err
		}
	}
	staged := filepath.Join(stage, filepath.FromSlash(spec.folder))
	if event != nil {
		event(name, "Preparing isolated interface assets", 0, 0)
	}
	if err = prepareInterface(ctx, name, staged, event); err != nil {
		return err
	}
	return commitInterface(ctx, root, name, staged, rel, i.AppVersion, platform.RequireGameClosed, nil)
}
func importInterface(ctx context.Context, root, name, archive, kind, version string) error {
	if err := platform.RequireGameClosed(); err != nil {
		return err
	}
	if err := preflightInterface(root, name); err != nil {
		return err
	}
	work, err := os.MkdirTemp("", "gtasa-interface-import-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	if err = fsutil.Extract(ctx, archive, work, kind, func(m string) string { return interfaceArchivePath(name, m) }); err != nil {
		return err
	}
	staged := filepath.Join(work, filepath.FromSlash(interfacePacks[name].folder))
	if err = prepareInterface(ctx, name, staged, nil); err != nil {
		return err
	}
	return commitInterface(ctx, root, name, staged, Release{Version: "manual import"}, version, platform.RequireGameClosed, nil)
}
func prepareInterface(ctx context.Context, name, staged string, event Event) error {
	spec := interfacePacks[name]
	if err := validateExtended(staged, spec.hashes); err != nil {
		return fmt.Errorf("%s requires the complete reviewed package (weapon imports must include the helper ASI and readme): %w", name, err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if name == HDRadarMap {
		notice := "Original-style colored HQ Radar by yojo2, ernestin1 and Junior_Djjr.\nSource: Atmosphere Interface Pack, 20/07/21, HQ Radar directory.\nOnly the 144 reviewed 512px radar dictionaries are installed, byte-for-byte unchanged.\nNo SVG conversion, recoloring, upscaling, extra HUD/menu/font replacement or new ASI.\nOriginal pack README and credits retained in README-upstream.txt.\nThis is the HQ Radar bundled in Atmosphere, not a claim of byte identity with the separate Definitive Radar Map Collection download.\n"
		if err := os.WriteFile(filepath.Join(staged, "README-map.txt"), []byte(notice), 0644); err != nil {
			return err
		}
	}
	if name == NadalaoWeapons {
		if err := os.WriteFile(filepath.Join(staged, "LICENSE-helper.txt"), []byte(weaponHelperLicense), 0644); err != nil {
			return err
		}
	}
	files, err := interfaceFiles(staged)
	if err != nil {
		return err
	}
	source := ""
	for _, s := range Catalog {
		if s.Name == name {
			source = s.Page
		}
	}
	data, err := json.MarshalIndent(interfaceManifest{1, name, source, files}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(staged, interfaceMarker), data, 0644)
}
func interfaceFiles(folder string) (map[string]string, error) {
	files := map[string]string{}
	err := filepath.WalkDir(folder, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("interface package contains a link/special file")
		}
		rel, err := filepath.Rel(folder, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == interfaceMarker {
			return nil
		}
		sha, err := fileSHA(p)
		if err != nil {
			return err
		}
		files[rel] = sha
		return nil
	})
	return files, err
}
func validateInterfaceFolder(folder, name string) error {
	data, err := os.ReadFile(filepath.Join(folder, interfaceMarker))
	if err != nil {
		return err
	}
	var m interfaceManifest
	if err = json.Unmarshal(data, &m); err != nil {
		return err
	}
	if m.Schema != 1 || canonicalModName(m.Name) != name || len(m.Files) == 0 {
		return fmt.Errorf("invalid managed interface inventory")
	}
	files, err := interfaceFiles(folder)
	if err != nil {
		return err
	}
	if len(files) != len(m.Files) {
		return fmt.Errorf("managed interface package has added/removed files")
	}
	for p, sha := range m.Files {
		if files[p] != sha {
			return fmt.Errorf("customized interface asset %s; back up/reconcile manually", p)
		}
	}
	return nil
}

// Refuse unmanaged/overlapping replacements, not arbitrary ModLoader priorities.
// Vanilla root models/hud.txd is expected. Root custom WeaponIcons/ASI is not.
func checkInterfaceSelection(names []string, t Tracker) error {
	wanted := map[string]bool{}
	for _, name := range append(append([]string{}, names...), t.Installed...) {
		wanted[name] = true
	}
	if wanted[SanLtdRussian] && (wanted[OriginalHQIcons] || wanted[AtmosphereHUD]) {
		return fmt.Errorf("standalone SanLtd Russian includes its own hud.txd; remove/reconcile it before HQ/Atmosphere HUD, or use the Russian translation included with ELL")
	}
	return nil
}

// Ordinary incoming packages cannot silently add a second runtime dictionary
// after an HD owner has been installed (for example a later SanLtd import).
func checkInterfaceOverlap(root string, files []string) error {
	for name, spec := range interfacePacks {
		folder := filepath.Join(root, filepath.FromSlash(spec.folder))
		if _, err := os.Stat(folder); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return err
		}
		owned, err := interfaceFiles(folder)
		if err != nil {
			return err
		}
		basenames := map[string]bool{}
		for p := range owned {
			if strings.HasSuffix(strings.ToLower(p), ".txd") || strings.HasSuffix(strings.ToLower(p), ".asi") {
				basenames[strings.ToLower(path.Base(p))] = true
			}
		}
		for _, p := range files {
			if basenames[strings.ToLower(path.Base(p))] {
				return fmt.Errorf("incoming %s overlaps managed %s; reconcile these packages first", p, name)
			}
		}
	}
	return nil
}
func preflightInterface(root, name string) error {
	spec, ok := interfacePacks[name]
	if !ok {
		return fmt.Errorf("unknown interface package %s", name)
	}
	t, err := LoadTracker(root)
	if err != nil {
		return err
	}
	target, err := fsutil.Join(root, spec.folder)
	if err != nil {
		return err
	}
	if _, err = os.Lstat(target); err == nil {
		if !t.Has(name) {
			return fmt.Errorf("unmanaged interface folder %s; back up/reconcile manually", target)
		}
		if err = validateInterfaceFolder(target, name); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	wanted := map[string]bool{}
	for p := range spec.hashes {
		if strings.HasSuffix(strings.ToLower(p), ".txd") || strings.HasSuffix(strings.ToLower(p), ".asi") {
			wanted[strings.ToLower(path.Base(p))] = true
		}
	}
	if name == HDRadarMap {
		for n := 0; n < 144; n++ {
			wanted[fmt.Sprintf("radar%02d.txd", n)] = true
		}
	}
	base := filepath.Join(root, "modloader")
	err = filepath.WalkDir(base, func(p string, d os.DirEntry, e error) error {
		if os.IsNotExist(e) {
			return nil
		}
		if e != nil {
			return e
		}
		if filepath.Clean(p) == filepath.Clean(target) && d.IsDir() {
			return filepath.SkipDir
		}
		if !d.IsDir() && wanted[strings.ToLower(d.Name())] {
			return fmt.Errorf("%s conflicts with %s; remove/reconcile that texture/helper first", name, p)
		}
		if name == NadalaoWeapons && !d.IsDir() && strings.EqualFold(filepath.Ext(p), ".asi") {
			sha, e := fileSHA(p)
			if e != nil {
				return e
			}
			if sha == spec.hashes["WeaponIconsTXD.SA.asi"] {
				return fmt.Errorf("renamed Weapon Icons TXD helper at %s", p)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if name == NadalaoWeapons {
		for _, dir := range []string{".", "scripts"} {
			base := filepath.Join(root, dir)
			err = filepath.WalkDir(base, func(p string, d os.DirEntry, e error) error {
				if os.IsNotExist(e) {
					return nil
				}
				if e != nil {
					return e
				}
				if d.IsDir() {
					if p != base && dir == "." {
						return filepath.SkipDir
					}
					return nil
				}
				if wanted[strings.ToLower(d.Name())] {
					return fmt.Errorf("duplicate/custom weapon icon file at %s", p)
				}
				if strings.EqualFold(filepath.Ext(p), ".asi") {
					sha, e := fileSHA(p)
					if e != nil {
						return e
					}
					if sha == spec.hashes["WeaponIconsTXD.SA.asi"] {
						return fmt.Errorf("renamed weapon icon helper at %s", p)
					}
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// Replace one owned directory, with durable outside-autoload backup and atomic
// tracker write. Hooks exist only for deterministic cancellation/I/O fault tests.
func commitInterface(ctx context.Context, root, name, staged string, rel Release, version string, closed func() error, save func(Tracker) error) error {
	if err := validateInterfaceFolder(staged, name); err != nil {
		return err
	}
	if err := closed(); err != nil {
		return err
	}
	if err := preflightInterface(root, name); err != nil {
		return err
	}
	t, err := LoadTracker(root)
	if err != nil {
		return err
	}
	for _, dep := range RequiredMods(name) {
		if !t.Has(dep) {
			return fmt.Errorf("install %s first (shared HUD dependency)", dep)
		}
		folder, err := fsutil.Join(root, interfacePacks[dep].folder)
		if err != nil {
			return err
		}
		if err = validateInterfaceFolder(folder, dep); err != nil {
			return fmt.Errorf("repair %s dependency first: %w", dep, err)
		}
	}
	base, err := fsutil.Join(root, "backups/mods")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(base, 0755); err != nil {
		return err
	}
	backup, err := os.MkdirTemp(base, backupPrefix(name)+"-"+time.Now().UTC().Format("20060102T150405")+"-*")
	if err != nil {
		return err
	}
	incoming := filepath.Join(backup, "incoming")
	if err = copyExtendedDir(ctx, staged, incoming); err != nil {
		_ = os.RemoveAll(backup)
		return err
	}
	if err = validateInterfaceFolder(incoming, name); err != nil {
		_ = os.RemoveAll(backup)
		return err
	}
	// Copying/rasterizing must not turn an earlier ownership check into
	// permission to overwrite assets or unrelated tracker edits made meanwhile.
	if err = preflightInterface(root, name); err != nil {
		_ = os.RemoveAll(backup)
		return err
	}
	t, err = LoadTracker(root)
	if err != nil {
		_ = os.RemoveAll(backup)
		return err
	}
	trackerPath, err := fsutil.Join(root, TrackerPath)
	if err != nil {
		return err
	}
	if b, e := os.ReadFile(trackerPath); e == nil {
		if err = os.WriteFile(filepath.Join(backup, "tracker-before.json"), b, 0644); err != nil {
			return err
		}
	} else if !os.IsNotExist(e) {
		return e
	}
	target, err := fsutil.Join(root, interfacePacks[name].folder)
	if err != nil {
		return err
	}
	previous := filepath.Join(backup, "previous")
	moved, active := false, false
	rollback := func(cause error) error {
		var failures []error
		if active {
			if e := os.RemoveAll(target); e != nil {
				failures = append(failures, e)
			}
		}
		if moved {
			if e := os.Rename(previous, target); e != nil {
				failures = append(failures, e)
			}
		}
		if len(failures) > 0 {
			return fmt.Errorf("%w; rollback incomplete %v; originals at %s", cause, errors.Join(failures...), backup)
		}
		_ = os.RemoveAll(backup)
		return cause
	}
	if err = ctx.Err(); err != nil {
		return rollback(err)
	}
	if err = closed(); err != nil {
		return rollback(err)
	}
	if _, err = os.Lstat(target); err == nil {
		if err = os.Rename(target, previous); err != nil {
			return rollback(err)
		}
		moved = true
	} else if !os.IsNotExist(err) {
		return rollback(err)
	}
	if err = os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return rollback(err)
	}
	if err = ctx.Err(); err != nil {
		return rollback(err)
	}
	if err = closed(); err != nil {
		return rollback(err)
	}
	if err = os.Rename(incoming, target); err != nil {
		return rollback(err)
	}
	active = true
	if !t.Has(name) {
		t.Installed = append(t.Installed, name)
	}
	rel.InstalledAt = time.Now().UTC().Format(time.RFC3339)
	t.Versions[name] = rel
	if !t.Legacy() {
		t.Schema = 2
	}
	if err = ctx.Err(); err != nil {
		return rollback(err)
	}
	if err = closed(); err != nil {
		return rollback(err)
	}
	if save == nil {
		save = func(t Tracker) error { return t.Save(context.Background(), root, version) }
	}
	if err = save(t); err != nil {
		return rollback(err)
	}
	return nil
}

const weaponHelperLicense = `MIT License

Copyright (c) 2022 Valdir da Costa Júnior

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
`
