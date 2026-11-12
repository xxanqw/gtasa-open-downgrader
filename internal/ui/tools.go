package ui

import (
	"context"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"net/url"
	"os"
	"runtime"

	"github.com/xxanqw/gtasa-open-downgrader/internal/entrypoint"
	"github.com/xxanqw/gtasa-open-downgrader/internal/fsutil"
	"github.com/xxanqw/gtasa-open-downgrader/internal/game"
	"github.com/xxanqw/gtasa-open-downgrader/internal/mods"
	"github.com/xxanqw/gtasa-open-downgrader/internal/patch"
	"github.com/xxanqw/gtasa-open-downgrader/internal/platform"
	appwidget "github.com/xxanqw/gtasa-open-downgrader/internal/uiwidget"
	"github.com/xxanqw/gtasa-open-downgrader/internal/updater"
)

// Only game-changing tools that depend on the downgraded executable and
// loaders are gated. Patch preparation and app/Steam integration stay usable.
func toolNeedsDowngrade(name string) bool {
	return name == "Apply 4GB / Large Address Aware flag" || name == "Import mod archive (ZIP / 7z)"
}

// Each tool retains its existing validation and confirmation flow after selection.
type toolAction struct {
	label string
	run   func()
}

func (w *Window) launchOptions() {
	w.prompt(promptRequest{kind: promptEntry, title: "Steam / Proton launch options", text: "In Steam, open the game's Properties and paste this into Launch Options:", initial: platform.LaunchOptions, accept: "Copy", dismiss: "Close", done: func(_ string, ok bool) {
		if ok {
			w.app.Clipboard().SetContent(platform.LaunchOptions)
			w.setStatus("Steam / Proton launch options copied.")
		}
	}})
}
func (w *Window) buildToolsPage() fyne.CanvasObject {
	groups := []struct {
		title   string
		buttons []fyne.CanvasObject
	}{{title: "Patch assets"}, {title: "Game adjustments & mods"}, {title: "System integration"}}
	for _, action := range w.toolActions() {
		// Recovery owns destructive backup and user-data operations.
		switch action.label {
		case "Restore downgrade backups", "Delete downgrade backups", "Clear saves, settings and gallery":
			continue
		}
		group, label := toolPresentation(action.label)
		name := action.label
		button := appwidget.NewButton(label, func() { w.runHubTool(name) })
		w.controls = append(w.controls, button)
		if toolNeedsDowngrade(name) {
			w.postDowngradeActions = append(w.postDowngradeActions, button)
		}
		groups[group].buttons = append(groups[group].buttons, button)
	}
	sections := make([]fyne.CanvasObject, 0, len(groups)*2)
	for _, group := range groups {
		if len(group.buttons) == 0 {
			continue
		}
		sections = append(sections, hubSection(group.title, container.NewGridWithColumns(2, group.buttons...)))
	}
	openFolder := appwidget.NewButtonWithIcon("Open downgrader folder", theme.FolderOpenIcon(), w.openDowngraderFolder)
	w.controls = append(w.controls, openFolder)
	sections = append(sections, hubSection("Application", container.NewGridWithColumns(2, w.updateButton, openFolder)))
	return w.hubPage(sections...)
}
func toolPresentation(label string) (int, string) {
	switch label {
	case "Download / refresh patch assets":
		return 0, "Download / update patches"
	case "Import Patches_v2.1.zip":
		return 0, "Import Patches_v2.1.zip"
	case "Select an existing Patches folder":
		return 0, "Select patches folder"
	case "Apply 4GB / Large Address Aware flag":
		return 1, "Apply 4GB patch"
	case "Restore downgrade backups":
		return 1, "Restore original files"
	case "Delete downgrade backups":
		return 1, "Delete backups"
	case "Clear saves, settings and gallery":
		return 1, "Clear saves & settings"
	case "Import mod archive (ZIP / 7z)":
		return 1, "Import mod archive"
	case "Repair installation path in Registry":
		return 2, "Repair Registry path"
	case "Create desktop shortcut":
		return 2, "Create desktop shortcut"
	case "Steam / Proton launch options":
		return 2, "Steam launch options"
	default:
		return 2, label
	}
}
func (w *Window) toolActions() []toolAction {
	root := w.scanPath
	action := func(label string, fn func()) toolAction { return toolAction{label: label, run: fn} }
	requireGame := func(fn func()) {
		if root == "" || !game.HasEXE(root) {
			w.error(fmt.Errorf("scan a game directory first"))
			return
		}
		fn()
	}
	runGame := func(title string, fn func(context.Context) error) {
		requireGame(func() { w.run(title, fn, func(error) { w.scan() }) })
	}
	actions := []toolAction{
		action("Download / refresh patch assets", w.downloadPatches),
		action("Import Patches_v2.1.zip", w.importSetupPatches),
		action("Select an existing Patches folder", func() {
			w.chooseFolder("Select Patches folder", w.patchDir, func(dir string) {
				w.run("Validating patches", func(context.Context) error { _, e := patch.ValidateReleaseAssets(dir); return e }, func(e error) {
					if e == nil {
						w.patchDir = dir
						if w.path.Text != "" {
							w.scan()
						}
					}
				})
			})
		}),
		action("Apply 4GB / Large Address Aware flag", func() {
			runGame("Applying 4GB flag", func(ctx context.Context) error {
				names := []string{entrypoint.RealGameExecutable, entrypoint.SteamExecutable}
				if entrypoint.MetadataPresent(root) {
					names = names[:1]
				}
				for _, name := range names {
					p, e := fsutil.Join(root, name)
					if e != nil {
						return e
					}
					if _, e = os.Stat(p); os.IsNotExist(e) {
						continue
					}
					if e = patch.Backup(ctx, root, name); e != nil {
						return e
					}
					if e = platform.ApplyLAA(ctx, p); e != nil {
						return e
					}
				}
				return nil
			})
		}),
		action("Restore downgrade backups", func() {
			requireGame(func() {
				w.confirmAction("Restore original files", "Restore original files", "Overwrite current game files with their original backups in:\n"+root+"\n\nMods remain installed.", func(ok bool) {
					if ok {
						w.run("Restoring backups", func(ctx context.Context) error { return patch.Revert(ctx, root, nil) }, func(error) { w.scan() })
					}
				})
			})
		}),
		action("Delete downgrade backups", func() {
			requireGame(func() {
				w.confirmAction("Delete downgrade backups", "Delete downgrade backups", "Permanently delete downgrade originals in:\n"+root+"\n\nRevert will no longer be available. Mod-uninstall snapshots under backups/mods will be kept.", func(ok bool) {
					if ok {
						w.run("Deleting backups", func(ctx context.Context) error {
							if e := ctx.Err(); e != nil {
								return e
							}
							return patch.Cleanup(root)
						}, nil)
					}
				})
			})
		}),
		action("Clear saves, settings and gallery", func() {
			requireGame(func() {
				p := platform.UserData(root)
				w.confirmAction("Delete saves, settings & gallery", "Delete user data permanently", "Permanently delete saves, settings and gallery from:\n"+p+"?\n\nThis cannot be undone. Copy this folder to a safe location before proceeding.", func(ok bool) {
					if ok {
						w.run("Clearing user data", func(ctx context.Context) error {
							if e := ctx.Err(); e != nil {
								return e
							}
							return os.RemoveAll(p)
						}, nil)
					}
				})
			})
		}),
		action("Import mod archive (ZIP / 7z)", func() {
			requireGame(func() {
				names := []string{}
				for _, s := range mods.Catalog {
					names = append(names, s.Name)
				}
				w.prompt(promptRequest{kind: promptChoice, title: "Import official mod package", text: "Select the matching mod. Install ASI Loader and ModLoader first.\nExtended Fonts requires a complete matched ELL 0.4.2 + fonts bundle, not the font-only ZIP; use online installation to fetch both.\nHD interface imports require reviewed complete files. Atmosphere needs HQ Icons first; nadalao needs its helper in the same archive.\nFiles and customized settings may be overwritten.\nManual imports have unknown upstream versions.", choices: names, initial: names[0], done: func(name string, ok bool) {
					if !ok {
						return
					}
					w.chooseArchive("Import "+name, []string{".zip", ".7z"}, func(p string) {
						w.run("Importing "+name, func(ctx context.Context) error { return mods.Import(ctx, root, name, p, updater.Version) }, func(e error) {
							if e == nil {
								w.launchAdvice([]string{name})
							}
							w.scan()
						})
					})
				}})
			})
		}),
	}
	if runtime.GOOS == "windows" {
		actions = append(actions,
			action("Repair installation path in Registry", func() {
				runGame("Repairing Registry", func(ctx context.Context) error { return platform.FixRegistry(ctx, root) })
			}),
			action("Create desktop shortcut", func() {
				runGame("Creating shortcut", func(ctx context.Context) error { return platform.Shortcut(ctx, root) })
			}),
		)
	} else {
		actions = append(actions, action("Steam / Proton launch options", w.launchOptions))
	}
	return actions
}

func (w *Window) openDowngraderFolder() {
	if err := os.MkdirAll(w.cache, 0700); err != nil {
		w.error(fmt.Errorf("open downgrader folder: %w", err))
		return
	}
	folder, err := url.Parse(storage.NewFileURI(w.cache).String())
	if err == nil {
		err = w.app.OpenURL(folder)
	}
	if err != nil {
		w.error(fmt.Errorf("open downgrader folder: %w", err))
	}
}
