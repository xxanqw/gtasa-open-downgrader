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
			if (client.ID == "samp" || client.ID == "mta") && client.Installed {
				if _, err := os.Stat(filepath.Join(gameDir, "modloader.asi")); err == nil {
					multiplayerStatus = "SA-MP and MTA start with Mod Loader safely isolated."
				}
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
