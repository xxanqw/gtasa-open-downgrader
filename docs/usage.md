# Using the downgrader

[Back to the project](../README.md)

## Downgrade your game

Close GTA and back up your saves and customized mod files before starting.

1. Choose your game folder. The app checks the executable to identify the installation; detected Steam installations are checked automatically.
2. If patch files are missing, choose **Install patches**, or import [Patches_v2.1.zip](https://files.xxanqw.me/s/ARC9NX3rm5) with **Import patch archive** or through **Tools**. Offline builds already include the patches. The game file check starts automatically once the patches are ready.
3. Choose **Review downgrade**, check the target folder, edition, affected files, and backup location, then confirm.
4. Let the verification scan finish. **Mods** and **Multiplayer** unlock once the target files are verified.

Original files are saved in `backups/` inside the game folder. Use **Recovery** to restore them. Cancelling stops further work but does not undo completed changes; an interruption warning remains visible after verification. Routine success is reported in the status bar, without a persistent banner. Restoring downgrade backups does not uninstall mods. Permanent user-data deletion in Recovery also deletes the gallery—copy your user-data folder somewhere safe first.

## Mods

The app offers ASI Loader, ModLoader, SilentPatch, Widescreen Fixes, Windowed Mode, Open Limit Adjuster, LOD Vegetation, Framerate Vigilante, GInput, SkyGFX, Project 2DFX, and optional interface, font, and translation packages.

Select rows in **Mods**, then use **Install** or **Update selected**. Selecting mods reveals **Reinstall**, **Remove**, and **Clear** directly in the header. Required dependencies are selected automatically and stay locked while needed. Expand a row with its chevron to view versions, requirements, download details, caveats, and the official source without leaving the list. The settings gear edits supported INI files. Keyboard users can select with Space or Enter and expand or collapse with the right and left arrows.

Mod removals are backed up under `backups/mods/`. Reinstallation can overwrite customized settings.

ShareMods downloads use one queue for the selected mods, including required helpers. Choose **Open remaining pages**, finish the ShareMods download steps in your browser, and save to the watched folder. Archives can finish in any order; shared packages are downloaded once. The app detects completed matching archives, checks reviewed package checksums where available, and starts installation automatically when all required archives are ready. Verified packages are cached for reuse, and your browser downloads are kept. Each row offers **Open page** and **Use existing archive…** for retrying or selecting an archive you already have. You can change the watched folder from the queue; start new downloads after changing folders. Cancel stops waiting, not the browser downloads. **Tools → Import mod archive** remains available for individual imports.

## Multiplayer and Steam Deck

Use **Multiplayer** to manage SA-MP, open.mp, MTA, and the optional Steam launcher. The launcher adds a mode chooser to Steam while keeping the real game executable intact. Use **Check updates** to download a verified launcher release, or **Remove & restore** to undo its installation. Close GTA and the launcher before updating. The launcher's **Settings** menu also offers an update check; installation is handled by the downgrader. **Launch this mode automatically next time** saves your mode. Future launches show a five-second countdown with **Choose another mode** as the safe default; choose it for a one-time bypass, or uncheck automatic launch to disable it.

On native Windows, installers that require administrator access open a UAC approval prompt; the downgrader itself stays unelevated. Finish or close an elevated installer before cancellation can complete.

On Linux and Steam Deck, clients use the game's detected Steam Proton installation and prefix. Follow the launch options and setup instructions shown in the app. open.mp uses a built-in server browser under Proton.

**Linux multiplayer support is experimental.** Server and anti-cheat compatibility is not guaranteed. MTA 1.6 is the default; MTA 1.7 is an opt-in development build. Keep their installation folders separate.

