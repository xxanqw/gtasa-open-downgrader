package ui

import (
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/xxanqw/gtasa-open-downgrader/internal/inieditor"
	"github.com/xxanqw/gtasa-open-downgrader/internal/platform"
	appwidget "github.com/xxanqw/gtasa-open-downgrader/internal/uiwidget"
	"strconv"
	"strings"
	"unicode/utf8"
)

type iniEditor struct {
	owner              *Window
	window             fyne.Window
	document           *inieditor.Document
	entry              *iniEntry
	status             *widget.Label
	save, undo, redo   *appwidget.Button
	files              *controlSelect
	find, replace      *widget.Entry
	findBar            *fyne.Container
	confirming         bool
	switching, closing bool
	key                string
	shortcutMenu       *fyne.Menu
}

func (w *Window) showINISettings(mod string) {
	if !w.modTableReady() || !w.installedMods[mod] {
		return
	}
	root := w.scanPath
	paths, err := inieditor.Discover(root, mod)
	if err != nil {
		w.error(err)
		return
	}
	if len(paths) == 0 {
		w.info("No INI configuration", "No supported INI files were found for this installed package.")
		return
	}
	key := root + "\x00" + mod
	if e := w.iniEditors[key]; e != nil {
		e.window.Show()
		e.window.RequestFocus()
		return
	}
	document, err := inieditor.Load(root, mod, paths[0])
	if err != nil {
		w.error(err)
		return
	}
	if w.iniEditors == nil {
		w.iniEditors = map[string]*iniEditor{}
	}
	e := &iniEditor{owner: w, window: w.app.NewWindow(mod + " | INI settings"), document: document, entry: newINIEntry(), status: compactLabel(""), key: key}
	w.iniEditors[key] = e
	e.entry.SetText(document.Text)
	e.save = appwidget.NewButton("Save", e.saveFile)
	reload := appwidget.NewButton("Reload", func() { e.withDiscard(func() { e.load(e.document.Path) }) })
	e.files = newControlSelect(paths, func(path string) {
		if e.switching || path == e.document.Path {
			return
		}
		e.switching = true
		e.files.SetSelected(e.document.Path)
		e.switching = false
		e.withDiscard(func() { e.load(path) })
	})
	e.switching = true
	e.files.SetSelected(document.Path)
	e.switching = false
	e.find = widget.NewEntry()
	e.find.SetPlaceHolder("Find (literal, case-sensitive)")
	e.replace = widget.NewEntry()
	e.replace.SetPlaceHolder("Replace with")
	e.find.OnSubmitted = func(string) { e.findNext(false) }
	e.findBar = container.NewVBox(container.NewBorder(nil, nil, nil, container.NewHBox(appwidget.NewButton("Previous", func() { e.findNext(true) }), appwidget.NewButton("Next", func() { e.findNext(false) })), styleInput(e.find)), container.NewBorder(nil, nil, nil, container.NewHBox(appwidget.NewButton("Replace", e.replaceOne), appwidget.NewButton("Replace all", e.replaceAll)), styleInput(e.replace)))
	e.findBar.Hide()
	e.undo = appwidget.NewButton("Undo", func() { e.entry.Undo(); e.focusEditor() })
	e.undo.Importance = widget.LowImportance
	e.redo = appwidget.NewButton("Redo", func() { e.entry.Redo(); e.focusEditor() })
	e.redo.Importance = widget.LowImportance
	var actions *appwidget.Button
	actions = appwidget.NewButtonWithIcon("", theme.MoreHorizontalIcon(), func() {
		menu := fyne.NewMenu("", fyne.NewMenuItem("Find / replace", e.showFind), fyne.NewMenuItem("Go to line", e.gotoDialog), fyne.NewMenuItem("Toggle line comment", e.toggleComment), fyne.NewMenuItem("Reload", reload.OnTapped), fyne.NewMenuItem("Shortcuts", e.help))
		widget.NewPopUpMenu(menu, e.window.Canvas()).ShowAtRelativePosition(fyne.NewPos(0, actions.Size().Height), actions)
	})
	actions.AccessibleLabel = "Editor actions"
	actions.Importance = widget.LowImportance
	toolbar := container.NewVBox(container.NewBorder(nil, nil, nil, container.NewHBox(e.undo, e.redo, e.save, actions), styleInput(e.files)), e.findBar)
	e.window.SetContent(container.NewPadded(container.NewBorder(toolbar, e.status, nil, nil, e.entry)))
	e.entry.OnChanged = func(string) { e.changed() }
	e.entry.OnCursorChanged = e.updateStatus
	e.entry.OnRejected = func(message string) { e.status.SetText(message) }
	e.shortcutMenu = fyne.NewMenu("Editor")
	e.bind(fyne.KeyZ, fyne.KeyModifierControl|fyne.KeyModifierShift, e.entry.Redo)
	e.bind(fyne.KeyS, fyne.KeyModifierControl, e.saveFile)
	e.bind(fyne.KeyF, fyne.KeyModifierControl, e.showFind)
	e.bind(fyne.KeyH, fyne.KeyModifierControl, e.showFind)
	e.bind(fyne.KeyG, fyne.KeyModifierControl, e.gotoDialog)
	e.bind(fyne.KeyW, fyne.KeyModifierControl, e.close)
	e.bind(fyne.KeyQ, fyne.KeyModifierControl, e.close)
	e.bind(fyne.KeySlash, fyne.KeyModifierControl, e.toggleComment)
	e.bind(fyne.KeyF3, 0, func() { e.findNext(false) })
	e.bind(fyne.KeyF3, fyne.KeyModifierShift, func() { e.findNext(true) })
	e.bind(fyne.KeyF5, 0, func() { e.withDiscard(func() { e.load(e.document.Path) }) })
	e.bind(fyne.KeyF1, 0, e.help)
	e.entry.Keys[fyne.KeyF3] = func() { e.findNext(false) }
	e.entry.Keys[fyne.KeyF5] = func() { e.withDiscard(func() { e.load(e.document.Path) }) }
	e.entry.Keys[fyne.KeyF1] = e.help
	e.entry.Keys[fyne.KeyEscape] = func() {
		if appwidget.ModalEscape(e.entry, &fyne.KeyEvent{Name: fyne.KeyEscape}) {
			return
		}
		e.findBar.Hide()
		e.entry.MoveCursor(e.entry.CursorTextOffset(), false)
		e.window.Canvas().Focus(e.entry)
	}
	e.bind(fyne.KeyEscape, 0, e.entry.Keys[fyne.KeyEscape])
	e.window.SetMainMenu(fyne.NewMainMenu(e.shortcutMenu))
	e.window.SetCloseIntercept(e.close)
	e.window.SetOnClosed(func() { appwidget.DismissModals(e.window); delete(w.iniEditors, key) })
	e.window.Resize(fyne.NewSize(1200, 800))
	e.changed()
	e.window.CenterOnScreen()
	e.window.Show()
	e.window.Canvas().Focus(e.entry)
}
func (e *iniEditor) bind(key fyne.KeyName, mod fyne.KeyModifier, action func()) {
	original := action
	action = func() {
		if !e.confirming && !appwidget.ModalOpen(e.window) {
			original()
		}
	}
	shortcut := &desktop.CustomShortcut{KeyName: key, Modifier: mod}
	e.entry.Commands[shortcut.ShortcutName()] = action
	e.window.Canvas().AddShortcut(shortcut, func(fyne.Shortcut) { action() })
	label := map[fyne.KeyName]string{fyne.KeyS: "Save", fyne.KeyF: "Find", fyne.KeyH: "Replace", fyne.KeyG: "Go to line", fyne.KeyW: "Close editor", fyne.KeyQ: "Close editor", fyne.KeySlash: "Toggle line comment", fyne.KeyF3: "Next match", fyne.KeyF5: "Reload", fyne.KeyF1: "Shortcuts", fyne.KeyZ: "Redo", fyne.KeyEscape: "Hide find / replace"}[key]
	if key == fyne.KeyF3 && mod == fyne.KeyModifierShift {
		label = "Previous match"
	}
	item := fyne.NewMenuItem(label, action)
	item.Shortcut = shortcut
	e.shortcutMenu.Items = append(e.shortcutMenu.Items, item)
}
func (e *iniEditor) dirty() bool { return e.entry.Text != e.document.Text }
func (e *iniEditor) guard() error {
	if !e.owner.modTableReady() || e.owner.scanPath != e.document.Root || !e.owner.installedMods[e.document.Mod] {
		return fmt.Errorf("rescan the same writable game with this mod installed before saving; finish any running operation")
	}
	return platform.RequireGameClosed()
}
func (e *iniEditor) changed() {
	if e.undo != nil {
		if e.entry.CanUndo() {
			e.undo.Enable()
		} else {
			e.undo.Disable()
		}
	}
	if e.redo != nil {
		if e.entry.CanRedo() {
			e.redo.Enable()
		} else {
			e.redo.Disable()
		}
	}
	if e.dirty() {
		e.save.Enable()
	} else {
		e.save.Disable()
	}
	title := e.document.Mod + " | " + e.document.Path
	if e.dirty() {
		title = "* " + title
	}
	e.window.SetTitle(title)
	e.updateStatus()
}
func (e *iniEditor) updateStatus() {
	state := "Saved"
	if e.dirty() {
		state = "Unsaved"
	}
	encoding := "UTF-8"
	if e.document.BOM {
		encoding += " BOM"
	}
	ending := "LF"
	if e.document.LineEnding == "\r\n" {
		ending = "CRLF"
	}
	e.status.SetText(fmt.Sprintf("%s | Ln %d, Col %d | %s / %s", state, e.entry.CursorRow+1, e.entry.CursorColumn+1, encoding, ending))
}
func (e *iniEditor) saveFile() { e.saveThen(nil) }
func (e *iniEditor) saveThen(after func()) {
	if e.confirming || appwidget.ModalOpen(e.window) {
		return
	}
	if !e.dirty() {
		if after != nil {
			after()
		}
		return
	}
	checks := inieditor.Validate(e.entry.Text)
	for _, d := range checks {
		if !d.Warning {
			appwidget.ShowError(e.window, fmt.Errorf("line %d: %s", d.Line, d.Message))
			return
		}
	}
	save := func() {
		backup, err := e.document.Save(e.entry.Text, e.guard)
		if err != nil {
			appwidget.ShowError(e.window, err)
			return
		}
		if backup != "" {
			e.owner.logs.Record("INFO", "Saved INI "+e.document.Path+" | backup: "+backup)
		}
		e.changed()
		if after != nil {
			after()
		}
	}
	warnings := 0
	for _, d := range checks {
		if d.Warning {
			warnings++
		}
	}
	if warnings > 0 {
		e.confirming = true
		appwidget.ShowConfirm(e.window, "Save with structural warnings?", fmt.Sprintf("%d structural warnings found. Duplicate sections/keys can have plugin-specific behavior. Save anyway?", warnings), "Save anyway", func(ok bool) {
			e.confirming = false
			if ok {
				save()
			}
		})
	} else {
		save()
	}
}
func (e *iniEditor) withDiscard(proceed func()) {
	if e.confirming || appwidget.ModalOpen(e.window) {
		return
	}
	if !e.dirty() {
		proceed()
		return
	}
	e.confirming = true
	appwidget.ShowModal(e.window, appwidget.ModalOptions{Title: "Unsaved changes", Body: container.NewVScroll(appwidget.ModalText("Save your INI changes before continuing?")), Actions: []appwidget.ModalAction{{Text: "Cancel"}, {ID: "discard", Text: "Discard", Importance: widget.DangerImportance}, {ID: "save", Text: "Save", Importance: widget.HighImportance}}, OnClosed: func(result string) {
		e.confirming = false
		switch result {
		case "save":
			e.saveThen(proceed)
		case "discard":
			proceed()
		}
	}})
}
func (e *iniEditor) close() {
	if e.closing {
		return
	}
	e.withDiscard(func() { e.closing = true; e.window.SetCloseIntercept(nil); e.window.Close() })
}
func (e *iniEditor) load(path string) {
	d, err := inieditor.Load(e.document.Root, e.document.Mod, path)
	if err != nil {
		appwidget.ShowError(e.window, err)
		return
	}
	e.document = d
	e.entry.SetText(d.Text)
	e.switching = true
	e.files.SetSelected(path)
	e.switching = false
	e.changed()
	e.window.Canvas().Focus(e.entry)
}
func (e *iniEditor) focusEditor() { e.window.Canvas().Focus(e.entry) }
func (e *iniEditor) showFind() {
	e.findBar.Show()
	e.window.Canvas().Focus(e.find)
}
func (e *iniEditor) position(offset int) { e.entry.MoveCursor(offset, false); e.focusEditor() }
func (e *iniEditor) selectRange(start, count int) {
	e.entry.SelectRange(start, start+max(0, count))
	e.focusEditor()
}
func (e *iniEditor) findNext(previous bool) {
	query := e.find.Text
	if query == "" {
		e.showFind()
		return
	}
	text := e.entry.Text
	runes := []rune(text)
	start, end := e.entry.SelectionRange()
	offset := end
	if previous {
		offset = start
	}
	offset = min(offset, len(runes))
	byteOffset := len(string(runes[:offset]))
	index := -1
	if previous {
		prefix := text[:byteOffset]
		index = strings.LastIndex(prefix, query)
		if index < 0 {
			index = strings.LastIndex(text, query)
		}
	} else {
		found := strings.Index(text[byteOffset:], query)
		if found >= 0 {
			index = byteOffset + found
		} else {
			index = strings.Index(text, query)
		}
	}
	if index < 0 {
		e.status.SetText("No matching text")
		return
	}
	e.selectRange(utf8.RuneCountInString(text[:index]), utf8.RuneCountInString(query))
}

func (e *iniEditor) insert(text string) { e.entry.InsertText(text) }
func (e *iniEditor) replaceOne() {
	if e.find.Text == "" {
		e.showFind()
		return
	}
	if e.entry.SelectedText() != e.find.Text {
		e.findNext(false)
		return
	}
	e.insert(e.replace.Text)
	e.findNext(false)
}
func (e *iniEditor) replaceAll() {
	if e.find.Text == "" {
		return
	}
	text := strings.ReplaceAll(e.entry.Text, e.find.Text, e.replace.Text)
	if text == e.entry.Text {
		return
	}
	e.window.Canvas().Focus(e.entry)
	e.entry.TypedShortcut(&fyne.ShortcutSelectAll{})
	e.insert(text)
}
func (e *iniEditor) goLine(line int) {
	lines := strings.Split(e.entry.Text, "\n")
	line = max(1, min(line, len(lines)))
	offset := 0
	for _, s := range lines[:line-1] {
		offset += utf8.RuneCountInString(s) + 1
	}
	e.position(offset)
}
func (e *iniEditor) gotoDialog() {
	if appwidget.ModalOpen(e.window) {
		return
	}
	input := appwidget.NewModalEntry()
	input.SetText(strconv.Itoa(e.entry.CursorRow + 1))
	e.confirming = true
	appwidget.ShowModal(e.window, appwidget.ModalOptions{Title: "Go to line", Body: container.NewVBox(appwidget.ModalText("Enter a positive line number."), styleInput(input)), Actions: []appwidget.ModalAction{{Text: "Cancel"}, {ID: "go", Text: "Go", Importance: widget.HighImportance}}, InitialFocus: input, OnClosed: func(result string) {
		e.confirming = false
		if result != "go" {
			return
		}
		n, err := strconv.Atoi(input.Text)
		if err != nil || n < 1 {
			appwidget.ShowError(e.window, fmt.Errorf("enter a positive line number"))
			return
		}
		e.goLine(n)
	}})
}
func (e *iniEditor) toggleComment() {
	lines := strings.Split(e.entry.Text, "\n")
	start, end := e.entry.SelectionRange()
	first, _ := e.entry.TextPosition(start)
	last, _ := e.entry.TextPosition(end)
	if end > start && last > first && end > 0 && []rune(e.entry.Text)[end-1] == '\n' {
		last--
	}
	begin := 0
	for _, line := range lines[:first] {
		begin += utf8.RuneCountInString(line) + 1
	}
	before := strings.Join(lines[first:last+1], "\n")
	uncomment := true
	for _, line := range lines[first : last+1] {
		trim := strings.TrimSpace(line)
		if trim != "" && !strings.HasPrefix(trim, ";") && !strings.HasPrefix(trim, "#") {
			uncomment = false
		}
	}
	for i := first; i <= last; i++ {
		trim := strings.TrimLeft(lines[i], " \t")
		indent := lines[i][:len(lines[i])-len(trim)]
		if trim == "" {
			continue
		}
		if uncomment {
			trim = trim[1:]
		} else {
			trim = ";" + trim
		}
		lines[i] = indent + trim
	}
	after := strings.Join(lines[first:last+1], "\n")
	if before == after {
		return
	}
	column := e.entry.CursorColumn
	e.selectRange(begin, utf8.RuneCountInString(before))
	e.insert(after)
	if start != end {
		e.selectRange(begin, utf8.RuneCountInString(after))
	} else {
		delta := 1
		if uncomment {
			delta = -1
		}
		e.position(begin + max(0, min(column+delta, utf8.RuneCountInString(after))))
	}
}
func (e *iniEditor) help() {
	text := "Ctrl+S — Save\nCtrl+F / Ctrl+H — Find / replace\nF3 / Shift+F3 — Next / previous match\nCtrl+G — Go to line\nCtrl+/ — Toggle line / selection comments\nCtrl+Z — Undo | Ctrl+Y / Ctrl+Shift+Z — Redo\nCtrl+A — Select all\nCtrl+C / X / V — Copy / cut / paste\nCtrl+Left / Right — Move by word\nShift+arrows / Home / End — Extend selection\nCtrl+Home / End — Document start / end\nTab / Shift+Tab — Indent / unindent\nEnter — New line with indentation\nPage Up / Down — Move by viewport\nInsert — Toggle overwrite\nDouble-click — Select word | Triple-click — Select line\nClick line number — Select line\nF5 — Reload (asks before discarding)\nEsc — Hide find / replace\nCtrl+W / Ctrl+Q — Close editor (asks before discarding)\nF1 — This help\n\nSaving checks INI structure, not plugin-specific values. UTF-8/ASCII and original LF/CRLF/BOM are preserved. Legacy encodings are refused, not converted."
	if appwidget.ModalOpen(e.window) {
		return
	}
	appwidget.ShowModal(e.window, appwidget.ModalOptions{Title: "Editor shortcuts", Body: container.NewVScroll(appwidget.ModalText(text)), Size: fyne.NewSize(640, 440)})
}
