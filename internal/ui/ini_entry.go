package ui

import (
	"image/color"
	"unicode"

	appwidget "github.com/xxanqw/gtasa-open-downgrader/internal/uiwidget"
)

// INI contributes only token colours. All input, geometry and rendering belong
// to the shared editor widget, independent of Fyne Entry and its renderer.
type iniEntry = appwidget.CodeEditor

func newINIEntry() *iniEntry { e := appwidget.NewCodeEditor(); e.Highlight = iniTokens; return e }
func iniTokens(line string) []appwidget.CodeToken {
	runes := []rune(line)
	begin := 0
	for begin < len(runes) && unicode.IsSpace(runes[begin]) {
		begin++
	}
	if begin == len(runes) {
		return nil
	}
	comment := color.NRGBA{R: 149, G: 164, B: 150, A: 255}
	section := color.NRGBA{R: 229, G: 187, B: 108, A: 255}
	key := color.NRGBA{R: 116, G: 183, B: 204, A: 255}
	value := color.NRGBA{R: 190, G: 203, B: 169, A: 255}
	bad := color.NRGBA{R: 218, G: 128, B: 118, A: 255}
	token := func(start, end int, c color.Color) appwidget.CodeToken {
		return appwidget.CodeToken{Start: start, End: end, Color: c}
	}
	if runes[begin] == ';' || runes[begin] == '#' {
		return []appwidget.CodeToken{token(begin, len(runes), comment)}
	}
	if runes[begin] == '[' {
		end := len(runes)
		for i := begin + 1; i < len(runes); i++ {
			if runes[i] == ']' {
				end = i + 1
				break
			}
		}
		c := section
		if end == len(runes) && runes[end-1] != ']' {
			c = bad
		}
		result := []appwidget.CodeToken{token(begin, end, c)}
		if commentStart := iniInlineComment(runes, end); commentStart < len(runes) {
			result = append(result, token(commentStart, len(runes), comment))
		}
		return result
	}
	equal := -1
	for i, r := range runes {
		if r == '=' {
			equal = i
			break
		}
	}
	if equal <= begin {
		return []appwidget.CodeToken{token(begin, len(runes), bad)}
	}
	result := []appwidget.CodeToken{token(begin, equal, key)}
	start := equal + 1
	for start < len(runes) && unicode.IsSpace(runes[start]) {
		start++
	}
	// Inline comment colouring is lexical only; it does not change parsing or
	// claim a plugin-specific meaning for the value. Quoted markers are literal.
	commentStart := iniInlineComment(runes, start)
	if start < commentStart && (runes[start] == '\'' || runes[start] == '"' || unicode.IsDigit(runes[start]) || runes[start] == '-' || runes[start] == '+') {
		result = append(result, token(start, commentStart, value))
	}
	if commentStart < len(runes) {
		result = append(result, token(commentStart, len(runes), comment))
	}
	return result
}

func iniInlineComment(runes []rune, start int) int {
	var quote rune
	for i := start; i < len(runes); i++ {
		r := runes[i]
		if quote != 0 {
			if r == '\\' {
				i++
				continue
			}
			if r == quote {
				quote = 0
			}
			continue
		}
		if (r == '\'' || r == '"') && (i == start || unicode.IsSpace(runes[i-1])) {
			quote = r
			continue
		}
		// Preserve embedded URL fragments, hex colours and unquoted separators.
		if r == ';' && (i == start || unicode.IsSpace(runes[i-1]) || runes[i-1] == '\'' || runes[i-1] == '"') {
			return i
		}
		if r == '#' && i > start && unicode.IsSpace(runes[i-1]) {
			return i
		}
	}
	return len(runes)
}
