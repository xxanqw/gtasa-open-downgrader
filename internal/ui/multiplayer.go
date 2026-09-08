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
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/xxanqw/gtasa-open-downgrader/internal/entrypoint"
	"github.com/xxanqw/gtasa-open-downgrader/internal/game"
	"github.com/xxanqw/gtasa-open-downgrader/internal/multiplayer"
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
		statusText := string(launcher.State)
		if launcher.State == entrypoint.Replaced {
			statusText = "Replaced by Steam · reinstall available"
		}
		launcherStatus := widget.NewLabel("Status: " + statusText)
		launcherStatus.Wrapping = fyne.TextWrapWord
		launcherAction := "Install Steam launcher"
		launcherIcon := theme.DownloadIcon()
		if launcher.State == entrypoint.Installed {
			launcherAction = "Remove & restore"
			launcherIcon = theme.DeleteIcon()
		} else if launcher.State == entrypoint.Replaced {
			launcherAction = "Reinstall Steam launcher"
		}
		launcherButton := widget.NewButtonWithIcon(launcherAction, launcherIcon, func() {
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

		launcherBody := container.NewBorder(nil, nil, nil, launcherButton, launcherStatus)
		sections := []fyne.CanvasObject{
			widget.NewLabelWithStyle("Steam launcher", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabel("Installs gta-sa.exe as the Steam menu and keeps the real game at gta_sa.exe."),
			launcherBody,
			widget.NewSeparator(),
			widget.NewLabelWithStyle("Multiplayer clients", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		}
		if compatibility != "" {
			sections = append(sections, widget.NewLabel("Steam compatibility: "+compatibility))
		}
		if environmentError != nil {
			message := widget.NewLabel("Client installation is unavailable: " + environmentError.Error())
			message.Wrapping = fyne.TextWrapWord
			message.Importance = widget.WarningImportance
			sections = append(sections, message)
		}
		for _, client := range clients {
			client := client
			state := "Not installed"
			action := "Install"
			if client.Installed {
				state = "Installed · " + client.Executable
				action = "Reinstall"
			}
			label := widget.NewLabel(state)
			label.Truncation = fyne.TextTruncateEllipsis
			if client.ID == multiplayer.ClientSAMP && client.Installed {
				if _, err := os.Stat(filepath.Join(root, "modloader.asi")); err == nil {
					label.SetText(state + " · launcher uses compatibility mode")
				}
			}
			button := widget.NewButton(action, func() {
				close()
				w.installMultiplayer(environment, runner, client.ID)
			})
			if runner == nil {
				button.Disable()
			}
			row := container.NewBorder(nil, nil, nil, button, container.NewVBox(widget.NewLabelWithStyle(client.Name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), label))
			sections = append(sections, row)
		}
		close = w.panel("Multiplayer & Steam", container.NewVScroll(container.NewVBox(sections...)), fyne.NewSize(650, 440), "", "Close", nil, func(bool) { done("", false) })
		return close
	}})
}

func (w *Window) installMultiplayer(environment multiplayer.Environment, runner multiplayer.Runner, id multiplayer.ClientID) {
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
			return multiplayer.InstallerResult(id, environment, runner.Run(ctx, path, arguments...))
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
