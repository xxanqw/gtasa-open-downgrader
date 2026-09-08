# Steam launcher and multiplayer integration

This document records the runtime architecture and the upstream assumptions used by the implementation. The end-user workflow is in the README.

## Executable roles and launcher ownership

`gta_sa.exe` is always the real downgraded game. The optional Windows Fyne launcher owns Steam's `gta-sa.exe` entrypoint. The launcher starts programs with ordinary Windows process creation; it contains no Wine, Proton, shell, update, or download logic. On Linux, Steam starts this Windows launcher inside the game's existing Proton environment.

`.downgrader/launcher.json` is a versioned ownership record. It stores the two fixed executable names, the SHA-256 digest of the installed launcher, and the location and digest of an entrypoint backup when one was required. Filename alone never proves launcher ownership. A digest mismatch is reported as a replaced launcher, which is the expected result after Steam verifies game files.

Installation requires an existing `gta_sa.exe`. An existing `gta-sa.exe` is preserved once under `.downgrader/launcher-backup/`; reinstall never overwrites that first backup. Removal restores the verified backup when present and leaves `gta_sa.exe` untouched. Removal refuses to overwrite an entrypoint whose digest no longer matches the managed launcher.

While metadata is valid, the scanner and patch engine resolve either manifest executable spelling to `gta_sa.exe` and never hash or patch `gta-sa.exe`. Without metadata, the previous two-name alias behavior is retained for older installations.

## Client layouts and launch behavior

The assumptions below were checked against upstream pages and source in September 2026.

### SA-MP

The supported layout is game-local: `samp.exe` and `samp.dll` must both exist beside `gta_sa.exe`. The launcher runs `samp.exe` in that directory and waits for it. The [official successor download page](https://www.sa-mp.mp/downloads/) identifies 0.3.7-R5-2-MP as the current release and requires GTA SA 1.0, but it offers community-hosted mirrors rather than a stable first-party artifact URL. The downgrader therefore opens that page and asks the user to select the downloaded installer. It does not scrape the page or silently trust a mirror.

### open.mp

The current launcher executable is `omp-launcher.exe`. Detection supports a portable copy in the GTA directory and the current installer locations under Program Files and Local AppData. Native Windows detection also checks the installer's registry location. The [official launcher page](https://open.mp/downloads/launcher) and [official GitHub releases](https://github.com/openmultiplayer/launcher/releases) identify v1.6.1 as current, document support for custom game executable names, and state that the launcher can automatically select a compatible client version. The automatic installer download uses GitHub's official `releases/latest/download/omp-launcher-setup.exe` redirect.

open.mp can use an existing SA-MP client or its own managed client, so both launchers can coexist. The downgrader does not copy or replace either client's DLLs. The open.mp UI uses WebView2. Its [official Linux instructions](https://open.mp/docs/client/ClientOnLinux) say the GUI is not supported under Wine and describe command-line launch arguments instead. The Steam launcher still exposes a detected installation, but the GUI may not render under some Proton versions. No server address or nickname is stored by this project, so it does not invent CLI arguments.

### Multi Theft Auto

MTA is a separate application and is not installed into the GTA directory. Its standard executable is `Multi Theft Auto.exe`. Detection checks current 1.7, 1.6, and 1.5 Program Files layouts plus the `Multi Theft Auto: San Andreas All` registry keys used by the [upstream NSIS installer source](https://github.com/multitheftauto/mtasa-blue/blob/master/Shared/installer/nightly.nsi). The official [client manual](https://wiki.multitheftauto.com/wiki/Client_Manual) says the installer requests a GTA SA 1.0 installation and component choices. The downloader uses the stable 1.6 link published by the [official MTA site](https://multitheftauto.com/). Installation remains interactive so the user can choose those components and the intended separate MTA location.

## Installation backends

On native Windows, installers run directly with an argument array and the installer's own working directory. On Linux, the UI calls `steam.DetectProtonContext(12120)` and runs the same installer through that detected context. This reuses Steam's exact library, compatibility data directory, and selected Proton build. No system Wine, protontricks, arbitrary Proton build, or second prefix is used.

Before launching an installer on Linux, the backend gives the parent of the GTA directory a dedicated unused drive letter such as `G:` inside the game's Proton prefix. The game is therefore presented as a regular folder such as `G:\Grand Theft Auto San Andreas`; SA-MP rejects installations located directly at a drive root. The mapping is stored with that prefix and never replaces an unrelated drive. SA-MP receives the visible folder through NSIS's standard `/D=` argument, so its directory page and Browse tree do not need to enter the hidden `.steam` hierarchy through `Z:`. The backend also writes the mapped `gta_sa.exe` and GTA directory into the prefix's normal Rockstar and MTA Registry keys using `reg.exe`; MTA can therefore preselect the game without browsing into a hidden Linux directory.

Automatic downloads require HTTPS, propagate HTTP failures through the existing job UI, use staged file writes, and reject files without a Windows `MZ` signature. Neither open.mp nor MTA publishes a checksum beside the stable programmatic URL, so there is no upstream checksum available to pin. TLS plus the executable header check protects against transport failures and HTML/error pages; it is not a publisher signature verification.

## Process lifetime and limitations

The Steam launcher stays open and waits for the directly selected game or client process. SA-MP, open.mp, and MTA normally retain a launcher/client process while the game is active. Their bootstrap behavior can change independently, and this project does not inject into or take ownership of unrelated processes. If a future client release starts a fully detached process and immediately exits, Steam tracking may end early until a client-specific supervisor is added.

Steam verification can restore its `gta-sa.exe`. The next scan reports the launcher as replaced and offers reinstall. Removal intentionally refuses to replace that newly restored file; reinstall keeps the first preserved backup and writes the launcher again.

The optional `.downgrader/multiplayer.json` schema accepts only supported client IDs and game-relative executable paths. Absolute paths and traversal are rejected. It is intended for portable layouts that discovery cannot cover. External MTA paths are rediscovered from known locations or the Windows Registry rather than executed from configuration.
