package ui

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	appwidget "github.com/xxanqw/gtasa-open-downgrader/internal/uiwidget"
)

// A display projection over the original journal. INI editing keeps its original
// geometry and text; logs get short timestamps, wrapped fields and no gutter.
type logEntry struct {
	*appwidget.CodeEditor
	view       *fyne.Container
	raw, level string
	columns    int
	formatted  map[string]string
}

var logHeader = regexp.MustCompile(`^\d{4}-\d{2}-\d{2} (\d{2}:\d{2}:\d{2}\.\d{3}) [+-]\d{4} \[(DEBUG|INFO|WARN|ERROR)\] (.*)$`)
var logField = regexp.MustCompile(` [A-Za-z_][A-Za-z_0-9]*=`)

func newLogEntry(text string) *logEntry {
	e := &logEntry{CodeEditor: appwidget.NewCodeEditor(), columns: 90}
	e.ReadOnly, e.HideLineNumbers = true, true
	e.FontSize, e.LineSpacing = 14, 4
	e.Highlight = logTokens
	e.view = container.New(logEntryLayout{e}, e.CodeEditor)
	e.UpdateText(text)
	return e
}

func (e *logEntry) Resize(size fyne.Size) {
	cell := fyne.MeasureText("M", e.FontSize, fyne.TextStyle{Monospace: true}).Width
	columns := max(20, int((size.Width-60)/cell))
	if columns != e.columns {
		e.columns, e.formatted = columns, nil
		e.project()
	}
	e.CodeEditor.Resize(size)
}

func (e *logEntry) UpdateText(raw string) { e.raw = raw; e.project() }
func (e *logEntry) SetLevel(level string) { e.level = level; e.project() }

func (e *logEntry) project() {
	cache := make(map[string]string)
	var output strings.Builder
	for _, line := range strings.Split(strings.TrimSuffix(e.raw, "\n"), "\n") {
		match := logHeader.FindStringSubmatch(line)
		if len(match) > 0 && !logLevelVisible(match[2], e.level) {
			continue
		}
		formatted, ok := e.formatted[line]
		if !ok {
			formatted = formatLogLine(line, match, e.columns)
		}
		cache[line] = formatted
		output.WriteString(formatted)
		output.WriteByte('\n')
	}
	e.formatted = cache
	text := strings.TrimSuffix(output.String(), "\n")
	if e.raw == "" {
		text = "No log entries yet."
	} else if text == "" {
		text = "No matching events."
	}
	e.CodeEditor.UpdateText(text)
}

func formatLogLine(line string, match []string, columns int) string {
	if len(match) == 0 {
		return wrapLogLine(line, columns, "  ")
	}
	message := match[3]
	fields := ""
	if index := logField.FindStringIndex(message); index != nil {
		message, fields = message[:index[0]], message[index[0]+1:]
	}
	header := wrapLogLine(match[1]+" ["+match[2]+"] "+message, columns, "  ")
	if fields != "" {
		header += "\n" + wrapLogLine("  "+fields, columns, "  ")
	}
	return header + "\n"
}

func logLevelVisible(level, filter string) bool {
	switch filter {
	case "Activity":
		return level != "DEBUG"
	case "Warnings & errors":
		return level == "WARN" || level == "ERROR"
	default:
		return true
	}
}

func wrapLogLine(line string, columns int, indent string) string {
	runes := []rune(line)
	indentWidth := utf8.RuneCountInString(indent)
	columns = max(columns, indentWidth+1)
	var result strings.Builder
	result.Grow(len(line))
	prefix := ""
	for {
		available := columns - utf8.RuneCountInString(prefix)
		result.WriteString(prefix)
		if len(runes) <= available {
			result.WriteString(string(runes))
			return result.String()
		}
		cut := available
		// Keep the original runes once: large diagnostic arrays must not cause
		// repeated full-line conversions for every wrapped display row.
		for i := available; i > available/2; i-- {
			if runes[i] == ' ' {
				cut = i
				break
			}
		}
		result.WriteString(string(runes[:cut]))
		result.WriteByte('\n')
		runes = runes[cut:]
		for len(runes) > 0 && runes[0] == ' ' {
			runes = runes[1:]
		}
		prefix = indent
	}
}

// Keep one widget identity for Fyne's renderer cache. The surrounding layout
// owns width-dependent projection; the editor itself remains the painted widget.
type logEntryLayout struct{ entry *logEntry }

func (l logEntryLayout) MinSize([]fyne.CanvasObject) fyne.Size        { return fyne.NewSize(320, 160) }
func (l logEntryLayout) Layout(_ []fyne.CanvasObject, size fyne.Size) { l.entry.Resize(size) }
