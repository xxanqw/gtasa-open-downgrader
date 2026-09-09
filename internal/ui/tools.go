package ui

import (
	"context"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"os"
	"path/filepath"
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
func (w *Window) tools() {
	w.prompt(promptRequest{present: func(done func(string, bool)) func() {
		var close func()
		groups := []struct {
			title   string
			buttons []fyne.CanvasObject
		}{{title: "Patch assets"}, {title: "Game files & mods"}, {title: "System integration"}}
		for _, action := range w.toolActions() {
			action := action
			group, label := toolPresentation(action.label)
			button := appwidget.NewButton(label, func() { close(); action.run() })
			groups[group].buttons = append(groups[group].buttons, button)
		}
		var sections []fyne.CanvasObject
		for _, group := range groups {
			sections = append(sections, widget.NewLabelWithStyle(group.title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), container.NewGridWithColumns(2, group.buttons...))
		}
		close = w.panel("Downgrader tools", container.NewVScroll(container.NewVBox(sections...)), fyne.NewSize(560, 440), "", "Close", nil, func(bool) { done("", false) })
		return close
	}})
}
func toolPresentation(label string) (int, string) {
	switch label {
	case "Download / refresh patch assets":
		return 0, "Download / update patches"
	case "Import Patches.zip":
		return 0, "Import Patches.zip"
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
		action("Import Patches.zip", func() {
			w.chooseArchive("Import patch archive", []string{".zip"}, func(p string) {
				target := filepath.Join(w.cache, "Patches")
				w.run("Importing patch assets", func(ctx context.Context) error { return patch.ImportAssets(ctx, p, target) }, func(e error) {
					if e == nil {
						w.patchDir = target
						if w.path.Text != "" {
							w.scan()
						}
					}
				})
			})
		}),
		action("Select an existing Patches folder", func() {
			w.chooseFolder("Select Patches folder", w.patchDir, func(dir string) {
				w.run("Validating patches", func(context.Context) error { _, e := patch.ValidateAssets(dir); return e }, func(e error) {
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
				w.confirm("Restore originals", "Overwrite current game files with their original backups? Mods remain installed.", func(ok bool) {
					if ok {
						w.run("Restoring backups", func(ctx context.Context) error { return patch.Revert(ctx, root, nil) }, func(error) { w.scan() })
					}
				})
			})
		}),
		action("Delete downgrade backups", func() {
			requireGame(func() {
				w.confirm("Delete backups", "Permanently delete all downgrade backups? Revert will no longer be available.", func(ok bool) {
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
				w.confirm("Delete user data", "Permanently delete saves, settings and gallery from:\n"+p+"?", func(ok bool) {
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
				w.prompt(promptRequest{kind: promptChoice, title: "Import official mod package", text: "Select the matching mod. Install ASI Loader and ModLoader first.\nFiles and customized settings may be overwritten.\nManual imports have unknown upstream versions.", choices: names, initial: names[0], done: func(name string, ok bool) {
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
