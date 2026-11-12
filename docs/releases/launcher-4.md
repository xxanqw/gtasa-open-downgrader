## Changes since launcher v3

- Singleplayer now tracks GTA's actual process exit, allowing the launcher to finish when a Steam/Wine launch command lingers after the game closes.
- Recover identical active and disabled Mod Loader copies after interrupted multiplayer sessions or reinstalls. The redundant copy is backed up; differing copies remain preserved and require reconciliation.
- Saved launch modes now show a cancellable five-second countdown with Choose another mode as the safe default.
- Improved dialog layout and responsiveness through the shared UI components.

## Install or update

Use Multiplayer → Steam launcher → Check updates in the downgrader. Close GTA and the launcher before installing the update.

This is a standalone Windows/Proton launcher release. Its independent update version is 4, and it does not change the latest downgrader release. This is a stable release. Launcher tags use `make_latest: false`, so the repository’s Latest download continues to point to the stable downgrader.
