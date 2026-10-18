package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/theme"
	"github.com/xxanqw/gtasa-open-downgrader/internal/assets"
	"github.com/xxanqw/gtasa-open-downgrader/internal/launcherapp"
	"github.com/xxanqw/gtasa-open-downgrader/internal/launcherversion"
	appwidget "github.com/xxanqw/gtasa-open-downgrader/internal/uiwidget"
	"github.com/xxanqw/gtasa-open-downgrader/internal/winedrive"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Println(launcherversion.Version())
		return
	}
	if handled, err := winedrive.HandleHelper(os.Args[1:]); handled {
		if err != nil {
			fmt.Fprintln(os.Stdout, "GTASA-DRIVE-ERROR:", err)
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	executable, err := os.Executable()
	if err != nil {
		return
	}
	gameDir := filepath.Dir(executable)
	launcher := launcherapp.New(gameDir)
	gameDrive, driveError := winedrive.EnsureWineGameDrive(gameDir)
	appwidget.ConfigureDesktopCursors()
	a := app.NewWithID("pp.ua.xxanqw.gtasa-steam-launcher")
	a.Settings().SetTheme(appwidget.CompactTheme{Theme: theme.DefaultTheme()})
	w := a.NewWindow("Grand Theft Auto: San Andreas")
	w.SetIcon(fyne.NewStaticResource("steam-launcher-icon.png", assets.Icon))
	ui := newLauncherWindow(a, w, gameDir, runtime.GOOS, launcherapp.RunningUnderProton(), launcher.Clients(), launcherActions{
		single:      func(ctx context.Context) error { return launcher.SinglePlayer(ctx, os.Args[1:]...) },
		multiplayer: launcher.Multiplayer, openMP: launcher.OpenMP,
		downgrader:    func() error { return launcherapp.OpenDowngrader(gameDir) },
		hasDowngrader: launcherapp.FindDowngrader(gameDir) != "",
	})
	if winedrive.RunningUnderWine() && driveError == nil {
		ui.details += "\n\nGTA-only Wine drive: " + gameDrive + "\nOther Wine drives are unchanged; this view is not a filesystem sandbox."
	}
	if driveError != nil {
		ui.status.SetText("Wine game drive unavailable | see details")
		ui.details += "\n\nWine game drive: " + driveError.Error()
		fyne.Do(func() { appwidget.ShowError(w, driveError) })
	} else {
		fyne.Do(ui.startRemembered)
	}
	w.ShowAndRun()
}
