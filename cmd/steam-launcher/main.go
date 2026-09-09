package main

import (
	"context"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/xxanqw/gtasa-open-downgrader/internal/assets"
	"github.com/xxanqw/gtasa-open-downgrader/internal/launcherapp"
	"github.com/xxanqw/gtasa-open-downgrader/internal/multiplayer"
)

type monochrome struct{ fyne.Theme }

func (t monochrome) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if name == theme.ColorNamePrimary || name == theme.ColorNameHyperlink {
		if variant == theme.VariantLight {
			return color.NRGBA{R: 32, G: 32, B: 32, A: 255}
		}
		return color.NRGBA{R: 224, G: 224, B: 224, A: 255}
	}
	base := color.NRGBAModel.Convert(t.Theme.Color(name, variant)).(color.NRGBA)
	value := uint8((uint32(base.R)*299 + uint32(base.G)*587 + uint32(base.B)*114) / 1000)
	return color.NRGBA{R: value, G: value, B: value, A: base.A}
}

func (t monochrome) Size(name fyne.ThemeSizeName) float32 {
	if name == theme.SizeNameText {
		return 13
	}
	if name == theme.SizeNamePadding {
		return 4
	}
	return t.Theme.Size(name)
}

func main() {
	executable, err := os.Executable()
	if err != nil {
		return
	}
	gameDir := filepath.Dir(executable)
	launcher := launcherapp.New(gameDir)
	a := app.NewWithID("pp.ua.xxanqw.gtasa-steam-launcher")
	a.Settings().SetTheme(monochrome{theme.DefaultTheme()})
	w := a.NewWindow("Grand Theft Auto: San Andreas")
	w.SetIcon(fyne.NewStaticResource("icon.png", assets.Icon))
	w.Resize(fyne.NewSize(390, 330))

	title := widget.NewLabelWithStyle("Grand Theft Auto: San Andreas", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	status := widget.NewLabel("Choose how to start the game.")
	status.Alignment = fyne.TextAlignCenter
	var controls []fyne.Disableable
	launch := func(run func(context.Context) error) {
		for _, control := range controls {
			control.Disable()
		}
		status.SetText("Running…")
		go func() {
			err := run(context.Background())
			fyne.Do(func() {
				if err != nil {
					status.SetText("Could not start the selected game.")
					dialog.ShowError(err, w)
					for _, control := range controls {
						control.Enable()
					}
					return
				}
				w.Close()
			})
		}()
	}
	showOpenMP := func() {
		host := widget.NewEntry()
		host.SetPlaceHolder("play.example.com")
		host.SetText(a.Preferences().String("openmp-host"))
		host.Validator = func(value string) error {
			_, err := (multiplayer.OpenMPConnection{Host: value, Port: "7777", Nickname: "Player"}).Arguments(gameDir)
			return err
		}
		port := widget.NewEntry()
		port.SetPlaceHolder("7777")
		port.SetText(a.Preferences().StringWithFallback("openmp-port", "7777"))
		port.Validator = func(value string) error {
			_, err := (multiplayer.OpenMPConnection{Host: "server.example", Port: value, Nickname: "Player"}).Arguments(gameDir)
			return err
		}
		nickname := widget.NewEntry()
		nickname.SetPlaceHolder("Nickname")
		nickname.SetText(a.Preferences().String("openmp-nickname"))
		nickname.Validator = func(value string) error {
			_, err := (multiplayer.OpenMPConnection{Host: "server.example", Port: "7777", Nickname: value}).Arguments(gameDir)
			return err
		}
		password := widget.NewPasswordEntry()
		password.SetPlaceHolder("Optional")
		versionNames := make([]string, 0, len(multiplayer.OpenMPSAMPVersions))
		versionIDs := make(map[string]string, len(multiplayer.OpenMPSAMPVersions))
		selectedVersion := a.Preferences().StringWithFallback("openmp-client-version", multiplayer.DefaultOpenMPSAMPVersion)
		selectedVersionName := ""
		for _, version := range multiplayer.OpenMPSAMPVersions {
			versionNames = append(versionNames, version.Name)
			versionIDs[version.Name] = version.ID
			if version.ID == selectedVersion {
				selectedVersionName = version.Name
			}
		}
		if selectedVersionName == "" {
			selectedVersion = multiplayer.DefaultOpenMPSAMPVersion
			selectedVersionName = versionNames[0]
		}
		clientVersion := widget.NewSelect(versionNames, nil)
		clientVersion.SetSelected(selectedVersionName)
		form := dialog.NewForm("Join an open.mp server", "Connect", "Cancel", []*widget.FormItem{
			widget.NewFormItem("Server", host),
			widget.NewFormItem("Port", port),
			widget.NewFormItem("Nickname", nickname),
			widget.NewFormItem("Password", password),
			widget.NewFormItem("SA-MP client", clientVersion),
		}, func(confirmed bool) {
			if !confirmed {
				return
			}
			selectedVersion = versionIDs[clientVersion.Selected]
			connection := multiplayer.OpenMPConnection{
				Host: host.Text, Port: port.Text, Nickname: nickname.Text,
				Password: password.Text, ClientVersion: selectedVersion,
			}
			if _, err := connection.Arguments(gameDir); err != nil {
				dialog.ShowError(err, w)
				return
			}
			a.Preferences().SetString("openmp-host", host.Text)
			a.Preferences().SetString("openmp-port", port.Text)
			a.Preferences().SetString("openmp-nickname", nickname.Text)
			a.Preferences().SetString("openmp-client-version", selectedVersion)
			launch(func(ctx context.Context) error { return launcher.OpenMP(ctx, connection) })
		}, w)
		form.Resize(fyne.NewSize(380, 300))
		form.Show()
	}
	single := widget.NewButtonWithIcon("Single Player", theme.MediaPlayIcon(), func() {
		launch(func(ctx context.Context) error { return launcher.SinglePlayer(ctx, os.Args[1:]...) })
	})
	single.Importance = widget.HighImportance
	controls = append(controls, single)

	clients := launcher.Clients()
	underProton := launcherapp.RunningUnderProton()
	installed := 0
	var multiplayerButtons []fyne.CanvasObject
	var unsupportedMessage string
	for _, client := range clients {
		client := client
		button := widget.NewButton(client.Name, func() {
			if client.ID == multiplayer.ClientOpenMP && underProton {
				showOpenMP()
				return
			}
			launch(func(ctx context.Context) error { return launcher.Multiplayer(ctx, client.ID) })
		})
		supported, reason := multiplayer.LaunchSupport(client.ID, runtime.GOOS, underProton)
		if !supported {
			button.Disable()
			button.SetText(client.Name + " · Unavailable under Proton")
			unsupportedMessage = reason
		} else if !client.Installed {
			button.Disable()
			button.SetText(client.Name + " · Not installed")
		} else {
			installed++
			controls = append(controls, button)
		}
		multiplayerButtons = append(multiplayerButtons, button)
	}
	multiplayerStatus := "Installed multiplayer clients appear below."
	if installed == 0 {
		multiplayerStatus = "No multiplayer clients detected. Install them with the downgrader."
	} else {
		for _, client := range clients {
			if client.ID == multiplayer.ClientMTA && client.Installed {
				if _, err := os.Stat(filepath.Join(gameDir, "modloader.asi")); err == nil {
					multiplayerStatus = "MTA starts with Mod Loader temporarily paused."
				}
			}
		}
	}
	if underProton {
		for _, client := range clients {
			if client.ID == multiplayer.ClientOpenMP && client.Installed {
				multiplayerStatus = "open.mp uses its managed SA-MP clients through a compact connection dialog."
			}
		}
	}
	if unsupportedMessage != "" {
		multiplayerStatus = unsupportedMessage + "."
	}
	multiplayerLabel := widget.NewLabel(multiplayerStatus)
	multiplayerLabel.Wrapping = fyne.TextWrapWord

	downgrader := widget.NewButtonWithIcon("Open Downgrader", theme.SettingsIcon(), func() {
		if err := launcherapp.OpenDowngrader(gameDir); err != nil {
			dialog.ShowError(err, w)
		}
	})
	if launcherapp.FindDowngrader(gameDir) == "" {
		downgrader.Disable()
	}
	settings := widget.NewButton("Settings", func() {
		dialog.ShowInformation("Launcher settings", fmt.Sprintf("Game folder:\n%s\n\nClient paths are detected automatically. Manage installations with the downgrader, then reopen this launcher.", gameDir), w)
	})
	controls = append(controls, downgrader, settings)

	body := container.NewVBox(
		title,
		widget.NewSeparator(),
		single,
		widget.NewLabelWithStyle("Multiplayer", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		multiplayerLabel,
		container.NewGridWithColumns(1, multiplayerButtons...),
		widget.NewSeparator(),
		container.NewGridWithColumns(2, downgrader, settings),
		status,
	)
	w.SetContent(container.NewPadded(body))
	w.ShowAndRun()
}
