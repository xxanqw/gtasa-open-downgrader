package ui

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/xxanqw/gtasa-open-downgrader/internal/patch"
	appwidget "github.com/xxanqw/gtasa-open-downgrader/internal/uiwidget"
)

type gameWelcome struct {
	root                                                           fyne.CanvasObject
	title, description, folderValue, folderState, patchState, hint *widget.Label
	folderIcon, patchIcon                                          *widget.Icon
	action, alternative, changeFolder                              *appwidget.Button
	mode                                                           setupStep
}

func (w *Window) chooseGameFolder() {
	w.chooseFolder("Select GTA San Andreas folder", w.path.Text, func(path string) { w.path.SetText(path); w.scan() })
}
func (w *Window) enterGameFolder() {
	w.prompt(promptRequest{kind: promptEntry, title: "Game folder", text: "Enter the directory containing gta_sa.exe or gta-sa.exe.", initial: w.path.Text, accept: "Use folder", done: func(path string, ok bool) {
		if ok && strings.TrimSpace(path) != "" {
			w.path.SetText(strings.TrimSpace(path))
			w.scan()
		}
	}})
}
func (w *Window) importSetupPatches() {
	w.chooseArchive("Import Patches_v2.1.zip", []string{".zip"}, func(path string) {
		w.importPatchArchive(path)
	})
}

func (w *Window) newGameWelcome() *gameWelcome {
	view := &gameWelcome{}
	view.title = widget.NewLabelWithStyle("Prepare your game", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	view.title.SizeName = theme.SizeNameHeadingText
	view.description = hubText("Choose your GTA San Andreas folder. You can review every change before downgrading.")
	view.folderIcon = widget.NewIcon(theme.FolderOpenIcon())
	view.patchIcon = widget.NewIcon(theme.DownloadIcon())
	folderHeading := widget.NewLabelWithStyle("Game folder", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	view.folderValue = hubText("")
	view.folderValue.Selectable = true
	view.folderState = hubText("")
	view.folderState.Importance = widget.LowImportance
	view.patchState = hubText("")
	view.patchState.Importance = widget.LowImportance
	view.changeFolder = appwidget.NewButton("Change", w.chooseGameFolder)
	view.changeFolder.Importance = widget.LowImportance
	folderTitle := container.NewBorder(nil, nil, nil, container.NewHBox(view.changeFolder), container.NewHBox(folderHeading, view.folderState))
	folder := container.NewVBox(folderTitle, container.NewBorder(nil, nil, container.NewCenter(view.folderIcon), nil, view.folderValue))
	patches := container.NewBorder(nil, nil, container.NewCenter(view.patchIcon), nil, view.patchState)
	requirements := newFramedPanel(container.New(layout.NewCustomPaddedLayout(8, 8, 8, 8), folder))
	view.hint = hubText("")
	view.action = appwidget.NewButton("Choose game folder", func() {
		switch w.nextSetupStep() {
		case setupChooseFolder, setupReadOnly:
			w.chooseGameFolder()
		case setupNeedPatches:
			w.downloadPatches()
		case setupCheckFolder, setupReadyToScan, setupFolderProblem:
			w.scan()
		}
	})
	view.action.Importance = widget.HighImportance
	view.alternative = appwidget.NewButton("Enter a path…", func() {
		switch w.nextSetupStep() {
		case setupNeedPatches, setupReadyToScan:
			w.importSetupPatches()
		default:
			w.enterGameFolder()
		}
	})
	view.alternative.Importance = widget.LowImportance
	actions := container.NewHBox(container.NewThemeOverride(view.action, appwidget.PrimaryActionTheme{Theme: w.app.Settings().Theme()}), view.alternative)
	column := container.New(setupColumnLayout{}, view.title, view.description, requirements, patches, view.hint)
	scroll := container.NewVScroll(column)
	bindPageFocus(scroll, column)
	view.root = container.New(setupSurfaceLayout{body: column}, scroll, actions)
	w.controls = append(w.controls, view.action, view.alternative, view.changeFolder)
	return view
}

func (view *gameWelcome) refresh(w *Window) {
	view.mode = w.nextSetupStep()
	candidate := strings.TrimSpace(w.path.Text)
	identified := candidate != "" && w.scanPath == w.path.Text
	if candidate == "" {
		view.folderValue.SetText("No game folder selected")
		view.folderState.Hide()
		view.changeFolder.Hide()
	} else {
		view.folderValue.SetText(candidate)
		view.folderState.Show()
		view.changeFolder.Show()
		if identified {
			view.folderState.SetText(strings.TrimSpace(strings.SplitN(w.version.Text, "|", 2)[0]))
		} else {
			view.folderState.SetText("Not checked yet")
		}
	}
	view.folderState.Importance = widget.LowImportance
	view.folderIcon.SetResource(theme.FolderOpenIcon())
	if identified {
		view.folderIcon.SetResource(theme.NewColoredResource(theme.ConfirmIcon(), theme.ColorNameSuccess))
	}
	view.patchIcon.SetResource(theme.DownloadIcon())
	if w.patchDir != "" {
		view.patchState.SetText("Patches v2.1 ready")
		view.patchIcon.SetResource(theme.NewColoredResource(theme.ConfirmIcon(), theme.ColorNameSuccess))
	} else if w.patchSetupError != nil {
		view.patchState.SetText("Installation stopped. Retry the download or import Patches_v2.1.zip.")
	} else {
		view.patchState.SetText("Patches v2.1 not installed")
	}
	view.action.Show()
	view.alternative.Show()
	view.hint.Importance = widget.MediumImportance
	view.title.SetText("Choose your game")
	view.description.SetText("Select your GTA San Andreas installation to get started.")
	switch view.mode {
	case setupChooseFolder:
		view.hint.SetText("Choose the folder containing gta_sa.exe or gta-sa.exe.")
		view.action.SetText("Choose game folder")
		view.alternative.SetText("Enter a path…")
	case setupCheckFolder:
		view.title.SetText("Check your installation")
		view.description.SetText("Identify this game folder before preparing the downgrade.")
		view.hint.SetText("Check this folder first so the app can identify your installation.")
		view.action.SetText("Check game folder")
		view.alternative.SetText("Enter a different path…")
	case setupNeedPatches:
		view.title.SetText("Install patch files")
		view.description.SetText("Install the patches to check which game files need changing.")
		view.hint.SetText("Your game stays unchanged until you review and confirm the downgrade.")
		view.action.SetText("Install patches")
		view.alternative.SetText("Import patch archive…")
	case setupReadyToScan:
		view.title.SetText("Ready to check your game")
		view.description.SetText("Compare your game files with the patches to prepare the downgrade.")
		view.hint.SetText("Both requirements are ready. Check the game files to see the downgrade plan.")
		view.action.SetText("Check game files")
		view.alternative.SetText("Import patch archive…")
	case setupFolderProblem:
		view.title.SetText("Check the game folder")
		view.description.SetText("This folder needs attention before you can continue.")
		view.folderIcon.SetResource(theme.NewColoredResource(theme.WarningIcon(), theme.ColorNameWarning))
		view.folderState.SetText("Folder check did not complete")
		view.folderState.Importance = widget.WarningImportance
		if errors.Is(w.gameSetupError, context.Canceled) {
			view.hint.SetText("The check was cancelled. Run it again when you're ready.")
		} else {
			view.hint.SetText("Couldn't check this installation. Retry, or use Change to choose the folder containing gta_sa.exe or gta-sa.exe. Details are in Log.")
		}
		view.action.SetText("Retry folder check")
		view.alternative.SetText("Enter a different path…")
	case setupReadOnly:
		view.title.SetText("Check the game folder")
		view.description.SetText("This folder needs attention before you can continue.")
		view.folderState.SetText("Read-only installation")
		view.folderState.Importance = widget.WarningImportance
		view.hint.SetText("This installation isn't writable. Choose a writable copy before downgrading or installing mods.")
		view.action.SetText("Choose another folder")
		view.alternative.SetText("Enter a path…")
	case setupWorking:
		view.title.SetText("Preparing your game")
		view.description.SetText("You can follow the current operation below.")
		view.action.Hide()
		view.alternative.Hide()
		switch w.setupActivity {
		case setupDiscovering:
			view.patchState.SetText("Checking for existing patch files…")
			view.folderState.Show()
			if candidate == "" {
				view.folderState.SetText("Looking for a Steam installation…")
			} else {
				view.folderState.SetText("Saved folder — waiting for identification")
			}
			view.hint.SetText("Looking for your Steam installation and existing patch files.")
		case setupIdentifying:
			view.folderState.SetText("Identifying the game executable…")
			view.hint.SetText("Checking the selected folder. This does not downgrade the game.")
		case setupInstallingPatches, setupImportingPatches:
			view.patchState.SetText("Preparing patch files…")
			view.hint.SetText("The game file check starts automatically after the patches are ready.")
		case setupScanning:
			view.folderState.SetText("Checking executable and game files…")
			view.folderIcon.SetResource(theme.SearchIcon())
			view.hint.SetText("Comparing the game files with the patch manifest. The results will appear here when the check finishes.")
		}
	case setupResults:
		view.hint.SetText("")
	}

	disabled := w.busy || w.promptActive
	for _, button := range []*appwidget.Button{view.action, view.alternative, view.changeFolder} {
		if disabled {
			button.Disable()
		} else {
			button.Enable()
		}
	}
	view.root.Refresh()
}

// Keep the next action visible even when a long path or small window makes
// the setup details scroll. At normal sizes, the form stays together.
type setupSurfaceLayout struct{ body *fyne.Container }

func (setupSurfaceLayout) MinSize([]fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(320, 200)
}
func (l setupSurfaceLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	width := max(float32(1), min(float32(620), size.Width-32))
	l.body.Resize(fyne.NewSize(width, l.body.MinSize().Height))
	appwidget.Relayout(l.body)
	bodyHeight := l.body.MinSize().Height
	actionHeight := objects[1].MinSize().Height
	if !objects[1].Visible() {
		actionHeight = 0
	}
	height := min(bodyHeight+actionHeight+12, max(float32(1), size.Height-32))
	x, y := (size.Width-width)/2, max(float32(16), (size.Height-height)/2)
	objects[0].Move(fyne.NewPos(x, y))
	objects[0].Resize(fyne.NewSize(width, max(float32(1), height-actionHeight-12)))
	objects[1].Move(fyne.NewPos(x, y+height-actionHeight))
	objects[1].Resize(fyne.NewSize(width, actionHeight))
}

type setupColumnLayout struct{}

var setupGaps = []float32{0, 4, 16, 12, 4}

func (setupColumnLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	height := float32(0)
	for index, object := range objects {
		if object.Visible() {
			height += object.MinSize().Height + setupGaps[index]
		}
	}
	return fyne.NewSize(320, height)
}
func (setupColumnLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	y := float32(0)
	for index, object := range objects {
		if object.Visible() {
			object.Resize(fyne.NewSize(size.Width, object.MinSize().Height))
			height := object.MinSize().Height
			y += setupGaps[index]
			object.Move(fyne.NewPos(0, y))
			object.Resize(fyne.NewSize(size.Width, height))
			y += height
		}
	}
}

// Shared by the setup action and Tools; importing prepares assets, then scans.
func (w *Window) importPatchArchive(path string) {
	if w.busy || w.closing {
		return
	}
	w.patchSetupError = nil
	target := filepath.Join(w.cache, "Patches")
	w.runSetup(setupImportingPatches, "Importing patch assets", func(ctx context.Context) error { return patch.ImportReleaseAssets(ctx, path, target) }, func(err error) {
		w.patchSetupError = err
		if err == nil {
			w.patchDir = target
			if w.path.Text != "" {
				w.scan()
			}
		}
	})
}
