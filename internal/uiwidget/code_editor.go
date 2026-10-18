package uiwidget

import (
	"image/color"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
	"github.com/go-text/typesetting/segmenter"
)

// CodeToken uses rune offsets within one logical line.
type CodeToken struct {
	Start, End int
	Color      color.Color
}

// CodeEditor is an unwrapped, left-to-right technical text editor. It owns its
// buffer, history, caret, selection and painter; it does not wrap widget.Entry.
type CodeEditor struct {
	widget.DisableableWidget
	Text                         string
	CursorRow, CursorColumn      int
	OnChanged                    func(string)
	OnCursorChanged              func()
	OnRejected                   func(string)
	Highlight                    func(string) []CodeToken
	MaxBytes, MaxLines, TabWidth int
	FontSize                     float32
	Overwrite                    bool
	Focused                      bool
	caretVisible                 bool
	caretEpoch                   time.Time
	blink                        *fyne.Animation
	Commands                     map[string]func()
	Keys                         map[fyne.KeyName]func()
	buffer                       []rune
	starts                       []int
	anchor                       int
	shift, control               bool
	preferredX                   float32
	keepX                        bool
	scroll                       *container.Scroll
	body                         *codeEditorBody
	gutter                       *codeEditorGutter
	history                      []codeEdit
	redo                         []codeEdit
	historyBytes                 int
	merge                        bool
	lastEdit                     time.Time
	revision                     int
	layoutRevision               int
	layoutSize                   float32
	layoutTabWidth               int
	metricCache                  codeMetrics
	metricFont                   fyne.Resource
	scrollMinimum                fyne.Size
	measurements                 map[string]float32
	layouts                      []codeLine
	widest                       float32
	pointerHandled, dragging     bool
	lastDouble                   time.Time
}
type codeEdit struct {
	start                                                int
	removed, inserted                                    []rune
	beforeCursor, beforeAnchor, afterCursor, afterAnchor int
}

func NewCodeEditor() *CodeEditor {
	e := &CodeEditor{MaxBytes: 256 * 1024, MaxLines: 6000, TabWidth: 4, preferredX: -1, layoutRevision: -1, Commands: map[string]func(){}, Keys: map[fyne.KeyName]func(){}}
	e.starts = []int{0}
	e.ExtendBaseWidget(e)
	return e
}
func (e *CodeEditor) SetText(text string) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	e.Text = text
	e.buffer = []rune(text)
	e.reindex()
	e.anchor = 0
	e.CursorRow = 0
	e.CursorColumn = 0
	e.history = nil
	e.redo = nil
	e.historyBytes = 0
	e.merge = false
	e.revision++
	e.keepX = false
	if e.scroll != nil {
		e.scroll.ScrollToOffset(fyne.NewPos(0, 0))
	}
	e.changed()
}
func (e *CodeEditor) reindex() {
	e.starts = []int{0}
	for i, r := range e.buffer {
		if r == '\n' {
			e.starts = append(e.starts, i+1)
		}
	}
}
func (e *CodeEditor) lineEnd(row int) int {
	row = max(0, min(row, len(e.starts)-1))
	if row+1 < len(e.starts) {
		return e.starts[row+1] - 1
	}
	return len(e.buffer)
}
func (e *CodeEditor) CursorTextOffset() int {
	row := max(0, min(e.CursorRow, len(e.starts)-1))
	return e.starts[row] + max(0, min(e.CursorColumn, e.lineEnd(row)-e.starts[row]))
}
func (e *CodeEditor) rowCol(offset int) (int, int) {
	offset = max(0, min(offset, len(e.buffer)))
	row := sort.Search(len(e.starts), func(i int) bool { return e.starts[i] > offset }) - 1
	row = max(0, row)
	return row, offset - e.starts[row]
}
func (e *CodeEditor) setCursor(offset int) { e.CursorRow, e.CursorColumn = e.rowCol(offset) }
func (e *CodeEditor) selection() (int, int) {
	cursor := e.CursorTextOffset()
	return min(cursor, e.anchor), max(cursor, e.anchor)
}
func (e *CodeEditor) SelectionRange() (int, int)         { return e.selection() }
func (e *CodeEditor) TextPosition(offset int) (int, int) { return e.rowCol(offset) }
func (e *CodeEditor) SelectedText() string {
	start, end := e.selection()
	return string(e.buffer[start:end])
}
func (e *CodeEditor) SelectRange(start, end int) {
	start = max(0, min(start, len(e.buffer)))
	end = max(0, min(end, len(e.buffer)))
	e.anchor = start
	e.setCursor(end)
	e.merge = false
	e.keepX = false
	e.moved()
}
func (e *CodeEditor) MoveCursor(offset int, extend bool) {
	e.setCursor(offset)
	if !extend {
		e.anchor = e.CursorTextOffset()
	}
	e.merge = false
	e.keepX = false
	e.moved()
}
func (e *CodeEditor) AcceptsTab() bool { return true }
func (e *CodeEditor) FocusGained() {
	e.Focused = true
	e.shift = false
	e.control = false
	e.caretEpoch = time.Now()
	e.caretVisible = true
	if e.blink == nil {
		e.blink = fyne.NewAnimation(1200*time.Millisecond, func(float32) {
			if !e.Focused || e.body == nil {
				return
			}
			visible := time.Since(e.caretEpoch)%(1200*time.Millisecond) < 600*time.Millisecond
			if visible != e.caretVisible {
				e.caretVisible = visible
				e.body.refreshCaret()
			}
		})
		e.blink.RepeatCount = fyne.AnimationRepeatForever
	}
	e.blink.Start()
	e.Refresh()
}
func (e *CodeEditor) FocusLost() {
	e.Focused = false
	if e.blink != nil {
		e.blink.Stop()
	}
	e.caretVisible = false
	e.shift = false
	e.control = false
	e.dragging = false
	e.merge = false
	e.Refresh()
}
func (e *CodeEditor) Cursor() desktop.Cursor {
	if e.Disabled() {
		return desktop.DefaultCursor
	}
	return desktop.TextCursor
}
func (e *CodeEditor) AccessibilityRole() fyne.AccessibleRole { return fyne.AccessibleRoleText }
func (e *CodeEditor) AccessibilityLabel() string {
	return "Code editor. Line " + codeNumber(e.CursorRow+1) + ", column " + codeNumber(e.CursorColumn+1)
}
func (e *CodeEditor) focus() {
	if c := fyne.CurrentApp().Driver().CanvasForObject(e); c != nil {
		c.Focus(e)
	}
}
func (e *CodeEditor) moved() {
	e.caretEpoch = time.Now()
	e.caretVisible = true
	e.ensureCaret()
	e.refreshInput()
	if e.OnCursorChanged != nil {
		e.OnCursorChanged()
	}
}
func (e *CodeEditor) changed() {
	e.caretEpoch = time.Now()
	e.caretVisible = true
	e.ensureCaret()
	e.refreshInput()
	if e.OnChanged != nil {
		e.OnChanged(e.Text)
	}
	if e.OnCursorChanged != nil {
		e.OnCursorChanged()
	}
}
func (e *CodeEditor) refreshInput() {
	if e.body == nil || e.gutter == nil || e.gutter.Size().Width != e.metrics().gutter {
		e.Refresh()
		return
	}
	e.body.Refresh()
	e.gutter.Refresh()
}
func (e *CodeEditor) reject(message string) {
	if e.OnRejected != nil {
		e.OnRejected(message)
	}
}
func (e *CodeEditor) replace(start, end int, text string, typing bool) bool {
	if e.Disabled() {
		return false
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if !utf8.ValidString(text) || strings.ContainsRune(text, 0) || strings.ContainsRune(text, '\r') {
		e.reject("Invalid text or unsupported line endings")
		return false
	}
	if e.MaxBytes > 0 && len(e.Text)-len(string(e.buffer[start:end]))+len(text) > e.MaxBytes {
		e.reject("Editor size limit exceeded")
		return false
	}
	if e.MaxLines > 0 && len(e.starts)-strings.Count(string(e.buffer[start:end]), "\n")+strings.Count(text, "\n") > e.MaxLines {
		e.reject("Editor line limit exceeded")
		return false
	}
	inserted := []rune(text)
	removed := append([]rune(nil), e.buffer[start:end]...)
	if string(removed) == text {
		return false
	}
	action := codeEdit{start: start, removed: removed, inserted: inserted, beforeCursor: e.CursorTextOffset(), beforeAnchor: e.anchor, afterCursor: start + len(inserted), afterAnchor: start + len(inserted)}
	e.apply(start, end, inserted)
	e.setCursor(action.afterCursor)
	e.anchor = action.afterAnchor
	merged := false
	if typing && e.merge && len(e.history) > 0 && len(removed) == 0 && time.Since(e.lastEdit) < 750*time.Millisecond && !strings.ContainsRune(text, '\n') {
		last := &e.history[len(e.history)-1]
		if len(last.removed) == 0 && last.afterCursor == start && last.afterAnchor == start {
			last.inserted = append(last.inserted, inserted...)
			last.afterCursor = action.afterCursor
			last.afterAnchor = action.afterAnchor
			merged = true
		}
	}
	if !merged {
		e.history = append(e.history, action)
	}
	e.historyBytes += 4 * (len(removed) + len(inserted))
	e.redo = nil
	for len(e.history) > 1000 || e.historyBytes > 4*1024*1024 {
		old := e.history[0]
		e.historyBytes -= 4 * (len(old.removed) + len(old.inserted))
		e.history[0] = codeEdit{}
		e.history = e.history[1:]
	}
	e.merge = typing
	e.lastEdit = time.Now()
	e.keepX = false
	e.changed()
	return true
}
func (e *CodeEditor) apply(start, end int, text []rune) {
	next := make([]rune, 0, len(e.buffer)-(end-start)+len(text))
	next = append(next, e.buffer[:start]...)
	next = append(next, text...)
	next = append(next, e.buffer[end:]...)
	e.buffer = next
	e.Text = string(next)
	e.reindex()
	e.revision++
}
func (e *CodeEditor) CanUndo() bool { return !e.Disabled() && len(e.history) > 0 }
func (e *CodeEditor) CanRedo() bool { return !e.Disabled() && len(e.redo) > 0 }
func (e *CodeEditor) Undo() {
	if e.Disabled() || len(e.history) == 0 {
		return
	}
	a := e.history[len(e.history)-1]
	e.history[len(e.history)-1] = codeEdit{}
	e.history = e.history[:len(e.history)-1]
	e.historyBytes -= 4 * (len(a.removed) + len(a.inserted))
	e.apply(a.start, a.start+len(a.inserted), a.removed)
	e.setCursor(a.beforeCursor)
	e.anchor = a.beforeAnchor
	e.redo = append(e.redo, a)
	e.merge = false
	e.changed()
}
func (e *CodeEditor) Redo() {
	if e.Disabled() || len(e.redo) == 0 {
		return
	}
	a := e.redo[len(e.redo)-1]
	e.redo[len(e.redo)-1] = codeEdit{}
	e.redo = e.redo[:len(e.redo)-1]
	e.apply(a.start, a.start+len(a.removed), a.inserted)
	e.setCursor(a.afterCursor)
	e.anchor = a.afterAnchor
	e.history = append(e.history, a)
	e.historyBytes += 4 * (len(a.removed) + len(a.inserted))
	e.merge = false
	e.changed()
}
func (e *CodeEditor) InsertText(text string) {
	start, end := e.selection()
	e.replace(start, end, text, false)
}
func (e *CodeEditor) TypedRune(r rune) {
	if unicode.IsControl(r) || e.Disabled() {
		return
	}
	start, end := e.selection()
	if e.Overwrite && start == end && end < len(e.buffer) && e.buffer[end] != '\n' {
		end = e.nextBoundary(end)
	}
	e.replace(start, end, string(r), true)
}
func (e *CodeEditor) boundary(offset int, next bool) int {
	if next && offset >= len(e.buffer) {
		return len(e.buffer)
	}
	if !next && offset <= 0 {
		return 0
	}
	row, _ := e.rowCol(offset)
	start, end := e.starts[row], e.lineEnd(row)
	if next && offset == end {
		return min(len(e.buffer), end+1)
	}
	if !next && offset == start {
		return max(0, start-1)
	}
	var sg segmenter.Segmenter
	sg.Init(e.buffer[start:end])
	it := sg.GraphemeIterator()
	previous := start
	for it.Next() {
		g := it.Grapheme()
		finish := start + g.Offset + len(g.Text)
		if next && finish > offset {
			return finish
		}
		if !next && finish >= offset {
			return previous
		}
		previous = finish
	}
	return offset
}
func (e *CodeEditor) nextBoundary(offset int) int     { return e.boundary(offset, true) }
func (e *CodeEditor) previousBoundary(offset int) int { return e.boundary(offset, false) }
func wordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsMark(r) || r == '_'
}
func (e *CodeEditor) wordMove(offset int, right bool) int {
	if right {
		for offset < len(e.buffer) && wordRune(e.buffer[offset]) {
			offset++
		}
		for offset < len(e.buffer) && !wordRune(e.buffer[offset]) {
			offset++
		}
		return offset
	}
	for offset > 0 && !wordRune(e.buffer[offset-1]) {
		offset--
	}
	for offset > 0 && wordRune(e.buffer[offset-1]) {
		offset--
	}
	return offset
}
func (e *CodeEditor) KeyDown(k *fyne.KeyEvent) {
	switch k.Name {
	case desktop.KeyShiftLeft, desktop.KeyShiftRight:
		e.shift = true
	case desktop.KeyControlLeft, desktop.KeyControlRight:
		e.control = true
	}
}
func (e *CodeEditor) KeyUp(k *fyne.KeyEvent) {
	switch k.Name {
	case desktop.KeyShiftLeft, desktop.KeyShiftRight:
		e.shift = false
	case desktop.KeyControlLeft, desktop.KeyControlRight:
		e.control = false
	}
}
func (e *CodeEditor) TypedKey(k *fyne.KeyEvent) {
	if e.Disabled() {
		return
	}
	if action := e.Keys[k.Name]; action != nil {
		action()
		return
	}
	cursor := e.CursorTextOffset()
	start, end := e.selection()
	switch k.Name {
	case fyne.KeyLeft, fyne.KeyRight:
		next := cursor
		if !e.shift && start != end {
			if k.Name == fyne.KeyLeft {
				next = start
			} else {
				next = end
			}
		} else if k.Name == fyne.KeyLeft {
			if e.control {
				next = e.wordMove(cursor, false)
			} else {
				next = e.previousBoundary(cursor)
			}
		} else {
			if e.control {
				next = e.wordMove(cursor, true)
			} else {
				next = e.nextBoundary(cursor)
			}
		}
		e.MoveCursor(next, e.shift)
	case fyne.KeyUp, fyne.KeyDown, fyne.KeyPageUp, fyne.KeyPageDown:
		e.buildLines()
		if !e.keepX {
			e.preferredX = e.columnX(e.CursorRow, e.CursorColumn)
		}
		delta := -1
		if k.Name == fyne.KeyDown {
			delta = 1
		}
		if k.Name == fyne.KeyPageUp || k.Name == fyne.KeyPageDown {
			delta = max(1, int(e.Size().Height/e.metrics().height)-2)
			if k.Name == fyne.KeyPageUp {
				delta = -delta
			}
		}
		row := max(0, min(e.CursorRow+delta, len(e.starts)-1))
		column := e.columnAt(row, e.preferredX)
		e.setCursor(e.starts[row] + column)
		if !e.shift {
			e.anchor = e.CursorTextOffset()
		}
		e.merge = false
		e.keepX = true
		e.moved()
	case fyne.KeyHome:
		target := e.starts[e.CursorRow]
		if e.control {
			target = 0
		} else {
			indent := target
			for indent < e.lineEnd(e.CursorRow) && (e.buffer[indent] == ' ' || e.buffer[indent] == '\t') {
				indent++
			}
			if cursor != indent {
				target = indent
			}
		}
		e.MoveCursor(target, e.shift)
	case fyne.KeyEnd:
		target := e.lineEnd(e.CursorRow)
		if e.control {
			target = len(e.buffer)
		}
		e.MoveCursor(target, e.shift)
	case fyne.KeyBackspace:
		if start == end {
			if e.control {
				start = e.wordMove(cursor, false)
			} else {
				start = e.previousBoundary(cursor)
			}
		}
		e.replace(start, end, "", false)
	case fyne.KeyDelete:
		if start == end {
			if e.control {
				end = e.wordMove(cursor, true)
			} else {
				end = e.nextBoundary(cursor)
			}
		}
		e.replace(start, end, "", false)
	case fyne.KeyReturn, fyne.KeyEnter:
		row, _ := e.rowCol(start)
		prefix := e.buffer[e.starts[row]:start]
		indent := 0
		for indent < len(prefix) && (prefix[indent] == ' ' || prefix[indent] == '\t') {
			indent++
		}
		e.replace(start, end, "\n"+string(prefix[:indent]), false)
	case fyne.KeyTab:
		e.indent(e.shift)
	case fyne.KeyEscape:
		e.MoveCursor(cursor, false)
	case fyne.KeyInsert:
		e.Overwrite = !e.Overwrite
		e.Refresh()
		if e.OnCursorChanged != nil {
			e.OnCursorChanged()
		}
	}
}
func (e *CodeEditor) TypedShortcut(s fyne.Shortcut) {
	if action := e.Commands[s.ShortcutName()]; action != nil {
		action()
		return
	}
	switch sh := s.(type) {
	case *fyne.ShortcutSelectAll:
		e.SelectRange(0, len(e.buffer))
	case *fyne.ShortcutUndo:
		e.Undo()
	case *fyne.ShortcutRedo:
		e.Redo()
	case *fyne.ShortcutCopy:
		if sh.Clipboard != nil && e.SelectedText() != "" {
			sh.Clipboard.SetContent(e.SelectedText())
		}
	case *fyne.ShortcutCut:
		if !e.Disabled() && sh.Clipboard != nil && e.SelectedText() != "" {
			sh.Clipboard.SetContent(e.SelectedText())
			e.InsertText("")
		}
	case *fyne.ShortcutPaste:
		if sh.Clipboard != nil {
			e.InsertText(sh.Clipboard.Content())
		}
	default:
		if key, ok := s.(fyne.KeyboardShortcut); ok {
			mod := key.Mod()
			e.shift = mod&fyne.KeyModifierShift != 0
			e.control = mod&fyne.KeyModifierControl != 0
			defer func() { e.shift = false; e.control = false }()
			e.TypedKey(&fyne.KeyEvent{Name: key.Key()})
		}
	}
}
func (e *CodeEditor) indent(unindent bool) {
	start, end := e.selection()
	first, _ := e.rowCol(start)
	last, _ := e.rowCol(end)
	if end > start && last > first && end == e.starts[last] {
		last--
	}
	if !unindent && start == end {
		e.buildLines()
		column := e.columnX(e.CursorRow, e.CursorColumn) / e.metrics().cell
		count := max(1, e.TabWidth) - int(column)%max(1, e.TabWidth)
		e.replace(start, end, strings.Repeat(" ", count), false)
		return
	}
	begin, finish := e.starts[first], e.lineEnd(last)
	lines := strings.Split(string(e.buffer[begin:finish]), "\n")
	for i, line := range lines {
		if unindent {
			if strings.HasPrefix(line, "\t") {
				line = line[1:]
			} else {
				count := 0
				for count < len(line) && count < max(1, e.TabWidth) && line[count] == ' ' {
					count++
				}
				line = line[count:]
			}
		} else {
			line = strings.Repeat(" ", max(1, e.TabWidth)) + line
		}
		lines[i] = line
	}
	text := strings.Join(lines, "\n")
	if e.replace(begin, finish, text, false) {
		e.SelectRange(begin, begin+utf8.RuneCountInString(text))
	}
}
