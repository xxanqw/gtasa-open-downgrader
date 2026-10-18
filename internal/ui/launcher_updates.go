package ui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/xxanqw/gtasa-open-downgrader/internal/entrypoint"
	"github.com/xxanqw/gtasa-open-downgrader/internal/fsutil"
	"github.com/xxanqw/gtasa-open-downgrader/internal/launcherupdate"
	"github.com/xxanqw/gtasa-open-downgrader/internal/platform"
)

func (w *Window) checkSteamLauncher(root string) {
	if !w.modTableReady() || root != w.scanPath {
		return
	}
	var release launcherupdate.Manifest
	var installed entrypoint.Status
	var replacedDigest string
	w.run("Checking Steam launcher updates", func(ctx context.Context) error {
		installed = entrypoint.Inspect(ctx, root)
		if installed.State != entrypoint.Installed && installed.State != entrypoint.NotInstalled && installed.State != entrypoint.Replaced {
			return fmt.Errorf("repair the Steam launcher metadata before updating: %s", installed.Detail)
		}
		var err error
		if installed.State == entrypoint.Replaced {
			replacedDigest, err = launcherEntrypointDigest(ctx, root)
			if err != nil {
				return err
			}
		}
		release, err = launcherupdate.Check(ctx, w.client.HTTP)
		return err
	}, func(err error) {
		if err != nil || root != w.scanPath {
			return
		}
		if installed.State == entrypoint.Installed {
			if release.Digest == "sha256:"+installed.Config.LauncherSHA256 || installed.Version != "" && !launcherupdate.Newer(release.Version, installed.Version) {
				w.info("Steam launcher", "No newer launcher is available. Latest published version: "+release.Version)
				return
			}
		}
		message := "Download and install Steam launcher " + release.Version + "? Close GTA and the launcher first. The real gta_sa.exe, original Steam-entrypoint backup, and remembered choice will be kept."
		if installed.State == entrypoint.Installed && installed.Version == "" {
			message = "The installed launcher has no release version. Replace it with the verified " + release.Version + " release?\n\n" + message
		} else if installed.State == entrypoint.Replaced {
			message = "Steam or another file replaced the launcher. Reinstall the verified " + release.Version + " release at gta-sa.exe?\n\n" + message
		}
		w.confirm("Update Steam launcher", message, func(ok bool) {
			if !ok || root != w.scanPath || !w.modTableReady() {
				return
			}
			w.run("Updating Steam launcher", func(ctx context.Context) error {
				if err := platform.RequireGameClosed(); err != nil {
					return err
				}
				staged, err := launcherupdate.Download(ctx, w.client.HTTP, filepath.Join(w.cache, "launcher-updates"), release, func(done, total int64) { w.report("Downloading Steam launcher", done, total) })
				if err != nil {
					return err
				}
				defer os.Remove(staged)
				if installed.State == entrypoint.Installed {
					_, err = entrypoint.Update(ctx, root, staged, installed.Config.LauncherSHA256, platform.RequireGameClosed)
					return err
				}
				if err := platform.RequireGameClosed(); err != nil {
					return err
				}
				current := entrypoint.Inspect(ctx, root)
				if current.State != installed.State || current.Config != installed.Config {
					return fmt.Errorf("launcher installation changed during download; rescan before installing")
				}
				if installed.State == entrypoint.Replaced {
					digest, err := launcherEntrypointDigest(ctx, root)
					if err != nil || digest != replacedDigest {
						return fmt.Errorf("Steam entrypoint changed during download; rescan before reinstalling")
					}
				}
				_, err = entrypoint.InstallWithDowngrader(ctx, root, staged, nativeDowngraderPath())
				return err
			}, func(error) { w.scan() })
		})
	})
}

func launcherEntrypointDigest(ctx context.Context, root string) (string, error) {
	path, err := fsutil.Join(root, entrypoint.SteamExecutable)
	if err != nil {
		return "", err
	}
	digest, err := entrypoint.Hash(ctx, path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	return digest, err
}

func nativeDowngraderPath() string {
	if runtime.GOOS == "windows" {
		path, _ := os.Executable()
		return path
	}
	return ""
}
