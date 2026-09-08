# Steam and Proton integration

`internal/steam` provides the Linux Steam/Proton discovery layer for future
launcher and installer features. It only reads Steam metadata. Detection does
not create a prefix, change the selected compatibility tool, edit launch
options, or modify the game installation.

## Detection and resolution

`steam.DetectProtonContext(12120)` performs these steps:

1. Detect native and Flatpak Steam roots below the current user's home.
2. Parse each root's `steamapps/libraryfolders.vdf` and de-duplicate symlinked
   libraries.
3. Find `appmanifest_12120.acf`, read its `installdir`, and require both the
   game directory and `steamapps/compatdata/12120` in that same library.
4. Enumerate compatibility tools in every library's `steamapps/common` and in
   the Steam root's `compatibilitytools.d` directory.
5. Parse `compatibilitytool.vdf` to retain each tool's internal name, display
   name, install path, metadata priority, and actual `proton` executable.
6. Parse `config/config.vdf` and resolve `CompatToolMapping` in this order:
   the exact AppID mapping, Steam's `0` default mapping, then an installed-tool
   fallback sorted by metadata priority and natural internal-name order.

An explicit per-game or default mapping that names a missing tool is an error;
the resolver does not silently use a different Proton build. The final
installed-tool fallback is only used when Steam has recorded neither mapping.
That fallback is deterministic, but Steam's automatic recommendation is not
fully represented in `config.vdf`, so callers can inspect
`ProtonContext.Selection` and explain when this assumption was necessary.

The Valve-supported layout with `compatibilitytool.vdf` directly inside
`compatibilitytools.d` is supported, including relative and absolute
`install_path` redirects. Directories containing a `proton` file but no
metadata are also recognized by directory name. Metadata remains the
authoritative way to map a Steam internal tool name to a differently named
directory.

## Running an executable

Future code should detect the context immediately before launching and call:

```go
proton, err := steam.DetectProtonContext(12120)
if err != nil {
    return err
}
return proton.Run(ctx, installerPath, installerArgs...)
```

`Run` invokes the resolved Proton executable in `run` mode. It rechecks the
Proton executable, the Windows executable, and the existing compatdata path,
then supplies `STEAM_COMPAT_DATA_PATH`,
`STEAM_COMPAT_CLIENT_INSTALL_PATH`, `STEAM_COMPAT_APP_ID`, `SteamAppId`, and
`SteamGameId`. It does not set `WINEPREFIX` or invoke Wine or protontricks.

The Windows executable must be available as a host filesystem path. Running a
Flatpak Steam Proton build directly from an application outside Steam is
supported at the filesystem-discovery level, but a particular build can still
depend on libraries or container runtime setup supplied by its Steam/Flatpak
environment. Such launch failures are returned to the caller with the selected
tool name.

For a read-only developer diagnostic on Linux:

```bash
go run ./cmd/proton-context
```

The command prints the detected Steam root, library, game directory,
compatdata path, compatibility-tool names and Proton executable. It does not
launch Proton.
