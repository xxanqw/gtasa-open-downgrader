package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
	"github.com/xxanqw/gtasa-open-downgrader/internal/fsutil"
	"github.com/xxanqw/gtasa-open-downgrader/internal/game"
	"github.com/xxanqw/gtasa-open-downgrader/internal/mods"
	"github.com/xxanqw/gtasa-open-downgrader/internal/patch"
	"github.com/xxanqw/gtasa-open-downgrader/internal/platform"
	"github.com/xxanqw/gtasa-open-downgrader/internal/updater"
)

func (w *Window) chooseArchive(title string, extensions []string, chosen func(string)) {
	d := dialog.NewFileOpen(func(r fyne.URIReadCloser, e error) {
		if e != nil {
			w.error(e)
			return
		}
		if r == nil {
			return
		}
		p := r.URI().Path()
		r.Close()
		chosen(p)
	}, w.window)
	d.SetConfirmText("Select")
	d.SetFilter(storage.NewExtensionFileFilter(extensions))
	d.Show()
}
func (w *Window) launchOptions() {
	text := widget.NewEntry()
	text.SetText(platform.LaunchOptions)
	dialog.ShowCustom("Steam / Proton launch options", "Close", container.NewVBox(widget.NewLabel("Paste this into the game's Steam Properties → Launch Options:"), text, widget.NewButton("Copy to clipboard", func() { w.window.Clipboard().SetContent(platform.LaunchOptions) })), w.window)
}
func (w *Window) tools() {
	root := w.scanPath
	var d dialog.Dialog
	action := func(label string, fn func()) fyne.CanvasObject {
		return widget.NewButton(label, func() { d.Hide(); fn() })
	}
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
	objects := []fyne.CanvasObject{
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
			dialog.ShowFolderOpen(func(uri fyne.ListableURI, e error) {
				if e != nil {
					w.error(e)
					return
				}
				if uri == nil {
					return
				}
				dir := uri.Path()
				w.run("Validating patches", func(context.Context) error { _, e := patch.ValidateAssets(dir); return e }, func(e error) {
					if e == nil {
						w.patchDir = dir
						if w.path.Text != "" {
							w.scan()
						}
					}
				})
			}, w.window)
		}),
		action("Apply 4GB / Large Address Aware flag", func() {
			runGame("Applying 4GB flag", func(ctx context.Context) error {
				for _, name := range []string{"gta_sa.exe", "gta-sa.exe"} {
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
				dialog.ShowConfirm("Restore originals", "Overwrite current game files with their original backups? Mods remain installed.", func(ok bool) {
					if ok {
						w.run("Restoring backups", func(ctx context.Context) error { return patch.Revert(ctx, root, nil) }, func(error) { w.scan() })
					}
				}, w.window)
			})
		}),
		action("Delete downgrade backups", func() {
			requireGame(func() {
				dialog.ShowConfirm("Delete backups", "Permanently delete all downgrade backups? Revert will no longer be available.", func(ok bool) {
					if ok {
						w.run("Deleting backups", func(ctx context.Context) error {
							if e := ctx.Err(); e != nil {
								return e
							}
							return patch.Cleanup(root)
						}, nil)
					}
				}, w.window)
			})
		}),
		action("Clear saves, settings and gallery", func() {
			requireGame(func() {
				p := platform.UserData(root)
				dialog.ShowConfirm("Delete user data", "Permanently delete saves, settings and gallery from:\n"+p+"?", func(ok bool) {
					if ok {
						w.run("Clearing user data", func(ctx context.Context) error {
							if e := ctx.Err(); e != nil {
								return e
							}
							return os.RemoveAll(p)
						}, nil)
					}
				}, w.window)
			})
		}),
		action("Import mod archive (ZIP / 7z)", func() {
			requireGame(func() {
				names := []string{}
				for _, s := range mods.Catalog {
					names = append(names, s.Name)
				}
				selectMod := widget.NewSelect(names, nil)
				selectMod.SetSelected(names[0])
				dialog.ShowCustomConfirm("Import official mod package", "Choose archive", "Cancel", container.NewVBox(widget.NewLabel("Select the matching mod. Install ASI Loader and ModLoader first.\nFiles and customized settings may be overwritten.\nManual imports have unknown upstream versions."), selectMod), func(ok bool) {
					if !ok {
						return
					}
					name := selectMod.Selected
					w.chooseArchive("Import "+name, []string{".zip", ".7z"}, func(p string) {
						w.run("Importing "+name, func(ctx context.Context) error { return mods.Import(ctx, root, name, p, updater.Version) }, func(e error) {
							if e == nil {
								w.launchAdvice([]string{name})
							}
							w.scan()
						})
					})
				}, w.window)
			})
		}),
	}
	if runtime.GOOS == "windows" {
		objects = append(objects,
			action("Repair installation path in Registry", func() {
				runGame("Repairing Registry", func(ctx context.Context) error { return platform.FixRegistry(ctx, root) })
			}),
			action("Create desktop shortcut", func() {
				runGame("Creating shortcut", func(ctx context.Context) error { return platform.Shortcut(ctx, root) })
			}),
		)
	} else {
		objects = append(objects, action("Steam / Proton launch options", w.launchOptions), action("List Protontricks games", func() {
			var output string
			w.run("Listing Proton games", func(ctx context.Context) error { var e error; output, e = platform.ProtonApps(ctx); return e }, func(e error) {
				if e == nil {
					dialog.ShowInformation("Protontricks games", output, w.window)
				}
			})
		}), action("Run installer through Protontricks", func() {
			id := widget.NewEntry()
			id.SetText("12120")
			dialog.ShowCustomConfirm("Steam app ID", "Choose installer", "Cancel", id, func(ok bool) {
				if !ok {
					return
				}
				appID := id.Text
				w.chooseArchive("Windows installer", []string{".exe"}, func(p string) {
					w.run("Running Proton installer", func(ctx context.Context) error { return platform.ProtonInstall(ctx, appID, p) }, nil)
				})
			}, w.window)
		}))
	}
	d = dialog.NewCustom("Downgrader tools", "Close", container.NewVBox(objects...), w.window)
	d.Resize(fyne.NewSize(540, 600))
	d.Show()
}
