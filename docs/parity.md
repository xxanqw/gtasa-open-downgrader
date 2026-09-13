# Go / Fyne migration

Baseline: Python v0.2.2, commit `cee6363`. All 21 existing unittest cases passed before the migration. The original source remains available in Git history; Go tests now cover the relevant behavior without Python or Qt.

| Area | Go implementation and behavior |
| --- | --- |
| Startup | Immediate native Fyne window; preparation runs in a background context. No onefile extraction splash or Tcl/Tk IPC. |
| Game detection | Windows Rockstar/Steam Registry entries; native/Flatpak Steam and SD-card paths on Linux. Steam `libraryfolders.vdf` adds other libraries. Manual folder selection remains available. |
| Scanning | All nine known executable MD5s, both executable spellings, source/target checks, LAA recognition, missing/modified/already-downgraded statuses and read-only detection. |
| Manifests | Existing generator schema, default `patch` action, `copy` for the executable, `MISSING` sources. Reject invalid hashes, empty manifests, duplicate or unsafe paths, and unsupported actions. |
| Patching | Embedded amd64 xdelta3, direct executable copy, verified temporary output, per-file status, cancellation and errors. Modified sources may be attempted but output must match the manifest. |
| Backups | Keep the first original backup, including distinct EXE aliases; record newly created aliases. Revert supports Python backups and the new journal. Cleanup requires confirmation. |
| Patches | Go iCloud resolver, streamed downloads, local ZIP import and folder selection, staged validation and replacement with rollback. Online builds use writable cache; offline builds embed validated assets. |
| Mods | Ten curated package sources and archive mappings; ZIP and 7z; ASI/ModLoader dependencies; skip existing, forced reinstall, manual archive import. Staged overlay rolls back on failure/cancellation. |
| Mod tracking | Compatible `modloader/.downgrader` schema 2; preserve custom fields and versions; release fingerprints and update labels. Legacy tracking prompts for configuration backups and reinstalls every supported recorded mod. Partial migrations stay legacy until complete. |
| Tools | LAA with PE validation and backup; Windows Registry repair and shortcut; backup restore/cleanup; save/settings/gallery deletion confirmation; patch management; Proton guidance and installer launch. |
| Updates | Semantic version comparison; exact OS/online/offline asset matching; cancellable staged download, size and published SHA-256 verification, helper waits for parent exit, replaces executable and restarts. |
| Shutdown | Cancel workers, HTTP requests, hashing/copying and xdelta, then wait before closing. UI changes are dispatched through `fyne.Do`. Stale mod checks cannot overwrite another scan's UI. |
| Releases | Native Linux/Windows builds and online/offline AppImages/EXEs. No Python, PyInstaller, Qt, Tcl/Tk or WebView dependency. |

## Intentional differences

- Missing patches do not force the app to exit: detection, tools and mod installation remain usable.
- Invalid downloaded manifests are rejected before replacing existing assets. Patch outputs are hash-checked before replacement; existing backups are never overwritten by retries.
- Corrupt mod trackers produce an actionable error instead of silently forgetting installed state.
- TLS certificate validation stays enabled. ShareMods anti-bot challenges are reported; use the official browser download and **Import mod archive** when challenged. Manual imports record an unknown upstream fingerprint until a normal reinstall.
- Native binaries remain linked to platform graphics libraries. Fyne uses CGO; setting `GOOS` alone is not sufficient for a native cross-build. The bundled xdelta3 binaries are amd64 only.
- SanLtd Russian uses SanLtd Team 0.56 from a manual-install ZIP mirror because the project's own download links are temporarily disabled. It is isolated under ModLoader; the executable readme and translated intro videos are excluded for safety and startup compatibility.
- Mod installation can overwrite customized INI files, as before. Forced reinstall and legacy migration display explicit warnings. Mods are not removed by reverting the downgrade.
- Individual files/packages are committed independently. Cancellation preserves completed work; reverting backups restores game files. The new package rollback is for runtime errors/cancellation, not a crash-safe multi-file transaction.
- App updates do not silently choose another distribution variant. Unsigned upstream assets have transport/size checks but no independent signature; GitHub's SHA-256 digest is verified when present. Failed helper updates are logged next to the executable.

## Validation and remaining manual checks

Automated checks cover the original tests' archive/tracker/version/shutdown contracts plus real xdelta round trips, backups/aliases, cancellation, PE validation, HTTP fixtures, path/link rejection, Fyne dependencies/read-only controls and release-asset selection.

A real licensed game installation is still required for a complete downgrade-and-launch acceptance test. Windows Registry/shortcut behavior and update replacement/restart require a Windows runtime check. Steam Deck, openSUSE and GPU/driver compatibility need representative machines. Upstream mod hosting, especially ShareMods, can change independently of this application.

## Migration verification on this branch

- Baseline Python: 21 tests passed before removal.
- Go unit/integration fixtures: `go test ./...`, `go test -race ./...` and `go vet ./...` passed.
- Native Linux Fyne window rendered under an isolated Xvfb display, scanned disposable files and exited cleanly after cancellation/shutdown. The actual offline binary also extracted its embedded patch set successfully.
- The official iCloud archive downloaded and validated: 17 manifest files (one executable copy, 16 xdelta patches), approximately 847 MiB extracted.
- All ten curated mod packages resolved, downloaded and installed into disposable directories successfully, including ShareMods and the SanLtd manual ZIP mirror, using the opt-in live test below.
- Windows amd64 online/offline GUI cross-builds succeeded with icon/manifest resources. Both online/offline AppImages packaged successfully; the online AppImage extraction launcher also ran successfully. Runtime Windows acceptance is still pending; cross-compilation does not validate Registry access, shortcuts, drivers or restart behavior.

To repeat the live upstream check (downloads current packages into test temporary directories):

```bash
go test -tags integration ./internal/mods -run TestOfficialPackages -v
```

Linux build and AppImage scripts stage large temporary files on the project disk; offline CGO linking needs several GiB of scratch space. The build downloads and generated assets remain ignored by Git.
