package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/xxanqw/gtasa-open-downgrader/internal/entrypoint"
	"github.com/xxanqw/gtasa-open-downgrader/internal/game"
	"github.com/xxanqw/gtasa-open-downgrader/internal/multiplayer"
	appwidget "github.com/xxanqw/gtasa-open-downgrader/internal/uiwidget"
)

func (w *Window) multiplayer() {
	root := w.scanPath
	if root == "" || !game.HasEXE(root) {
		w.error(fmt.Errorf("scan a game directory first"))
		return
	}
	var environment multiplayer.Environment
	var runner multiplayer.Runner
	var compatibility string
	var environmentError error
	var clients []multiplayer.Client
	var launcher entrypoint.Status
	w.run("Detecting Steam and multiplayer", func(ctx context.Context) error {
		launcher = entrypoint.Inspect(ctx, root)
		environment, runner, compatibility, environmentError = multiplayer.PlatformEnvironment(root)
		if environment.GameDir == "" {
			environment.GameDir = root
		}
		clients = multiplayer.Detect(environment)
		return ctx.Err()
	}, func(err error) {
		if err == nil {
			w.showMultiplayer(root, launcher, environment, runner, compatibility, environmentError, clients)
		}
	})
}

func (w *Window) showMultiplayer(root string, launcher entrypoint.Status, environment multiplayer.Environment, runner multiplayer.Runner, compatibility string, environmentError error, clients []multiplayer.Client) {
	w.prompt(promptRequest{present: func(done func(string, bool)) func() {
		var close func()
		launcherAction := "Install Steam launcher"
		launcherIcon := theme.DownloadIcon()
		if launcher.State == entrypoint.Installed {
			launcherAction = "Remove & restore"
			launcherIcon = theme.DeleteIcon()
		} else if launcher.State == entrypoint.Replaced {
			launcherAction = "Reinstall Steam launcher"
		}
		launcherButton := appwidget.NewButtonWithIcon(launcherAction, launcherIcon, func() {
			close()
			if launcher.State == entrypoint.Installed {
				w.confirm("Remove Steam launcher", "Remove the launcher and restore the preserved Steam entrypoint? The real gta_sa.exe will remain unchanged.", func(ok bool) {
					if ok {
						w.run("Removing Steam launcher", func(ctx context.Context) error { return entrypoint.Remove(ctx, root) }, func(error) { w.scan() })
					}
				})
				return
			}
			source, err := launcherBinary()
			if err != nil {
				w.error(err)
				return
			}
			w.run("Installing Steam launcher", func(ctx context.Context) error {
				downgrader := ""
				if runtime.GOOS == "windows" {
					downgrader, _ = os.Executable()
				}
				_, err := entrypoint.InstallWithDowngrader(ctx, root, source, downgrader)
				return err
			}, func(error) { w.scan() })
		})

		launcherDetails := compactLabel("Steam entry point · real game remains gta_sa.exe")
		launcherBody := container.NewBorder(nil, nil, nil, container.NewCenter(launcherButton), container.NewVBox(
			widget.NewLabelWithStyle("Steam launcher", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			launcherDetails,
		))
		sections := []fyne.CanvasObject{
			container.New(layout.NewCustomPaddedLayout(2, 5, 4, 4), launcherBody),
			widget.NewSeparator(),
		}
		clientHeading := container.NewHBox(widget.NewLabelWithStyle("Multiplayer clients", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
		if compatibility != "" {
			clientHeading.Add(widget.NewLabel("· " + compatibility))
		}
		sections = append(sections, clientHeading)
		if environmentError != nil {
			message := widget.NewLabel("Client installation is unavailable: " + environmentError.Error())
			message.Wrapping = fyne.TextWrapWord
			message.Importance = widget.WarningImportance
			sections = append(sections, message)
		}
		for index, client := range clients {
			client := client
			detail := "Ready to install"
			if client.Installed {
				switch client.ID {
				case multiplayer.ClientSAMP:
					detail = "Game-folder client"
				case multiplayer.ClientOpenMP:
					detail = "Launcher and managed client versions"
				case multiplayer.ClientMTA:
					detail = "Separate client installation"
				}
			}
			if client.ID == multiplayer.ClientOpenMP && client.Installed && runtime.GOOS == "linux" {
				path, err := multiplayer.OpenMPClientPath(environment)
				if err != nil || multiplayer.ValidateOpenMPClient(path) != nil || multiplayer.ValidateOpenMPSAMPInstallation(environment) != nil {
					detail = "Managed SA-MP client files are incomplete"
				}
			}
			if client.ID == multiplayer.ClientMTA && client.Installed {
				if _, err := os.Stat(filepath.Join(root, "modloader.asi")); err == nil {
					detail += " · Mod Loader is paused while running"
				}
			}
			supported, reason := multiplayer.LaunchSupport(client.ID, runtime.GOOS, false)
			if !supported {
				detail = reason
			}
			detailLabel := compactLabel(detail)
			detailLabel.TextStyle.Italic = true
			var actions *fyne.Container
			if client.Installed {
				repair := appwidget.NewButtonWithIcon("Repair", theme.ViewRefreshIcon(), func() {
					close()
					w.installMultiplayer(environment, runner, client.ID)
				})
				remove := appwidget.NewButtonWithIcon("Uninstall", theme.DeleteIcon(), func() {
					close()
					w.uninstallMultiplayer(environment, runner, client)
				})
				if runner == nil || !supported {
					repair.Disable()
				}
				if runner == nil {
					remove.Disable()
				}
				actions = container.NewHBox(repair, remove)
			} else {
				install := appwidget.NewButtonWithIcon("Install", theme.DownloadIcon(), func() {
					close()
					w.installMultiplayer(environment, runner, client.ID)
				})
				install.Importance = widget.HighImportance
				if runner == nil || !supported {
					install.Disable()
				}
				actions = container.NewHBox(install)
			}
			row := container.NewBorder(nil, nil, nil, container.NewCenter(actions), container.NewVBox(
				widget.NewLabelWithStyle(client.Name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
				detailLabel,
			))
			sections = append(sections, container.New(layout.NewCustomPaddedLayout(3, 5, 4, 4), row))
			if index < len(clients)-1 {
				sections = append(sections, widget.NewSeparator())
			}
		}
		close = w.panel("Multiplayer & Steam", container.NewVScroll(container.NewVBox(sections...)), fyne.NewSize(690, 430), "", "Close", nil, func(bool) { done("", false) })
		return close
	}})
}

func (w *Window) uninstallMultiplayer(environment multiplayer.Environment, runner multiplayer.Runner, client multiplayer.Client) {
	message := "Open the " + client.Name + " uninstaller and remove this client? The game and other multiplayer clients will remain installed."
	if client.ID == multiplayer.ClientOpenMP {
		message = "Open the open.mp uninstaller and remove its managed client files? The game and standalone SA-MP installation will remain installed."
	}
	w.confirm("Uninstall "+client.Name, message, func(ok bool) {
		if !ok {
			return
		}
		var result multiplayer.UninstallResult
		w.run("Uninstalling "+client.Name, func(ctx context.Context) error {
			var err error
			result, err = multiplayer.Uninstall(ctx, client.ID, environment, runner)
			return err
		}, func(err error) {
			if err == nil && result.RestartRequired {
				w.info("Restart required", "SA-MP was removed. Windows still had one or more SA-MP fonts open, so their deletion is scheduled for the next restart.")
			}
			w.scan()
		})
	})
}

func (w *Window) installMultiplayer(environment multiplayer.Environment, runner multiplayer.Runner, id multiplayer.ClientID) {
	if supported, reason := multiplayer.LaunchSupport(id, runtime.GOOS, false); !supported {
		w.error(fmt.Errorf("%s", reason))
		return
	}
	installer, ok := multiplayer.Installers[id]
	if !ok {
		w.error(fmt.Errorf("unsupported multiplayer client %q", id))
		return
	}
	if runner == nil {
		w.error(fmt.Errorf("installer execution is unavailable"))
		return
	}
	run := func(path string) {
		w.run("Installing "+installer.Name, func(ctx context.Context) error {
			if err := multiplayer.ValidateInstaller(path); err != nil {
				return err
			}
			arguments, err := runner.Prepare(ctx, id, environment)
			if err != nil {
				return err
			}
			if err := multiplayer.InstallerResult(id, environment, runner.Run(ctx, path, arguments...)); err != nil {
				return err
			}
			if id == multiplayer.ClientOpenMP && runtime.GOOS == "linux" {
				destination, err := multiplayer.OpenMPClientPath(environment)
				if err != nil {
					return err
				}
				if err := w.client.File(ctx, multiplayer.OpenMPClientURL, destination, func(done, total int64) {
					w.report("Adding open.mp Linux support", done, total)
				}); err != nil {
					return fmt.Errorf("download open.mp command-line client: %w", err)
				}
				if err := multiplayer.ValidateOpenMPClient(destination); err != nil {
					_ = os.Remove(destination)
					return err
				}
				archive := filepath.Join(w.cache, "multiplayer", "samp_clients.7z")
				if err := w.client.File(ctx, multiplayer.OpenMPSAMPArchiveURL, archive, func(done, total int64) {
					w.report("Downloading open.mp SA-MP clients", done, total)
				}); err != nil {
					return fmt.Errorf("download open.mp managed SA-MP clients: %w", err)
				}
				if err := multiplayer.InstallOpenMPSAMPArchive(ctx, archive, environment); err != nil {
					_ = os.Remove(archive)
					return err
				}
			}
			return nil
		}, func(error) {
			w.scan()
		})
	}
	if installer.Manual {
		w.info("Download SA-MP installer", "SA-MP does not publish a stable first-party installer URL. Its official page will open; download the current client from a listed source, then select the installer here. The installer will use the detected Steam environment automatically.")
		_ = w.app.OpenURL(mustURL(installer.OfficialPage))
		w.chooseArchive("Select SA-MP installer", []string{".exe"}, run)
		return
	}
	if err := multiplayer.ValidateDownloadURL(installer.URL); err != nil {
		w.error(err)
		return
	}
	destination := filepath.Join(w.cache, "multiplayer", installer.FileName)
	w.run("Downloading "+installer.Name, func(ctx context.Context) error {
		if err := os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
			return err
		}
		if err := w.client.File(ctx, installer.URL, destination, func(done, total int64) { w.report("Downloading "+installer.Name, done, total) }); err != nil {
			return err
		}
		if err := multiplayer.ValidateInstaller(destination); err != nil {
			_ = os.Remove(destination)
			return err
		}
		return nil
	}, func(err error) {
		if err == nil {
			run(destination)
		}
	})
}

func launcherBinary() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	var candidates []string
	if appDir := os.Getenv("APPDIR"); appDir != "" {
		candidates = append(candidates, filepath.Join(appDir, "usr", "libexec", entrypoint.SteamExecutable))
	}
	dir := filepath.Dir(executable)
	candidates = append(candidates,
		filepath.Join(dir, entrypoint.SteamExecutable),
		filepath.Join(dir, "libexec", entrypoint.SteamExecutable),
		filepath.Join(filepath.Dir(dir), "libexec", entrypoint.SteamExecutable),
	)
	for _, candidate := range candidates {
		info, statErr := os.Stat(candidate)
		if statErr == nil && info.Mode().IsRegular() {
			return candidate, nil
		}
	}
	platformName := strings.Title(runtime.GOOS)
	return "", fmt.Errorf("the Windows Steam launcher was not found beside this %s build; download a release bundle containing gta-sa.exe", platformName)
}
