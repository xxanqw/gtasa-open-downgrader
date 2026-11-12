# Publishing releases

Development and stable releases use `main`. The `v0.2.3` branch preserves the final Python implementation. Release tags are immutable; do not move an existing tag or replace its published binaries.

## Downgrader

1. Update the default version in `internal/updater/updater.go`, `scripts/build.sh`, `scripts/build.ps1`, and `FyneApp.toml`.
2. Write `docs/releases/vX.Y.Z.md`, complete [the checks](development.md#checks), and merge into `main`.
3. Push the matching `vX.Y.Z` tag. The tag workflow validates the official patches and builds Windows and Linux online/offline distributions, notices, and checksums.
4. Wait for every build and publishing job to succeed. A stable `vX.Y.Z` release is non-prerelease and becomes GitHub's Latest release. Tags such as `vX.Y.Z-rc.N` remain prereleases and never become Latest.
5. Verify all release assets and both managed update channels. Refresh the update server's pinned release snapshot after publication.

## Steam launcher

Launcher versions are independent positive integers in `internal/launcherversion/VERSION`.

1. For a new launcher build, increment that integer and write `docs/releases/launcher-N.md`.
2. Merge into `main`, then push the matching `launcher-N` tag.
3. The workflow publishes `gta-sa.exe`, third-party notices, and checksums as a **stable** release with **`make_latest: false`**. It never replaces the repository's Latest downgrader release.
4. Verify `/api/v1/launcher/latest` reports the integer version and matching checksum, and `/api/v1/releases/latest` still reports the stable downgrader.

Launcher v4 was initially published as a prerelease. Promoting it changes its release metadata and notes; its existing tag and artifacts remain intact. Future launcher tags publish stable automatically.

## Storage and validation

The release workflow caches compressed Go module downloads only. Compiled objects and embedded offline patch data stay on the temporary runner. After successful publication, the workflow deletes its temporary build artifacts; a one-day retention limit is the fallback if cleanup is unavailable. Public releases retain the distributable files and `SHA256SUMS`.

The `ci` build tag is only for headless tests. Release builds use the desktop driver. No game files, generated patches, private logs, temporary screenshots, or local build products belong in commits.
