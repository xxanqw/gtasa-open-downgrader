package ui

import (
	"image/color"
	"strings"
	"unicode"
	"unicode/utf8"

	appwidget "github.com/xxanqw/gtasa-open-downgrader/internal/uiwidget"
)

func findLog(e *logEntry, query string, reverse bool) bool {
	text := []rune(e.Text)
	start, end := e.SelectionRange()
	var offset int
	if reverse {
		prefix := string(text[:start])
		index := strings.LastIndex(prefix, query)
		if index < 0 {
			prefix = e.Text
			index = strings.LastIndex(prefix, query)
		}
		if index < 0 {
			return false
		}
		offset = utf8.RuneCountInString(prefix[:index])
	} else {
		suffix := string(text[end:])
		index := strings.Index(suffix, query)
		if index < 0 {
			end = 0
			suffix = e.Text
			index = strings.Index(suffix, query)
		}
		if index < 0 {
			return false
		}
		offset = end + utf8.RuneCountInString(suffix[:index])
	}
	e.SelectRange(offset, offset+utf8.RuneCountInString(query))
	return true
}

func logTokens(line string) []appwidget.CodeToken {
	r := []rune(line)
	muted := color.NRGBA{R: 164, G: 164, B: 164, A: 255}
	key := color.NRGBA{R: 116, G: 183, B: 204, A: 255}
	value := color.NRGBA{R: 190, G: 203, B: 169, A: 255}
	number := color.NRGBA{R: 229, G: 187, B: 108, A: 255}
	failure := color.NRGBA{R: 240, G: 139, B: 133, A: 255}
	var tokens []appwidget.CodeToken
	add := func(a, b int, c color.Color) {
		if a < b {
			tokens = append(tokens, appwidget.CodeToken{Start: a, End: b, Color: c})
		}
	}
	i := 0
	// Accept both our timestamped records and captured standard-library logs.
	if position := strings.IndexByte(line, '['); position >= 0 {
		for _, level := range []string{"DEBUG", "INFO", "WARN", "ERROR"} {
			marker := "[" + level + "]"
			if !strings.HasPrefix(line[position:], marker) {
				continue
			}
			i = utf8.RuneCountInString(line[:position])
			add(0, i, muted)
			c := key
			switch level {
			case "DEBUG":
				c = muted
			case "WARN":
				c = number
			case "ERROR":
				c = failure
			}
			add(i, i+len(marker), c)
			i += len(marker)
			break
		}
	}
	for i < len(r) {
		if unicode.IsSpace(r[i]) {
			i++
			continue
		}
		start := i
		for i < len(r) && !unicode.IsSpace(r[i]) && r[i] != '=' {
			i++
		}
		if i == len(r) || r[i] != '=' {
			continue
		}
		field := string(r[start:i])
		add(start, i, key)
		i++
		start = i
		i = logValueEnd(r, i)
		c := number
		if start < len(r) && (r[start] == '"' || r[start] == '[' || r[start] == '{') {
			c = value
		}
		if string(r[start:i]) == "null" {
			c = muted
		} else if field == "error" || field == "errors" || field == "cause" {
			c = failure
		}
		add(start, i, c)
	}
	return tokens
}

// JSON strings/arrays can contain spaces, escaped quotes, and key=value text.
func logValueEnd(r []rune, start int) int {
	depth, i := 0, start
	quoted := false
	for i < len(r) {
		if quoted {
			if r[i] == '\\' {
				i = min(i+2, len(r))
				continue
			}
			if r[i] == '"' {
				quoted = false
			}
		} else {
			if unicode.IsSpace(r[i]) && depth == 0 {
				break
			}
			switch r[i] {
			case '"':
				quoted = true
			case '[', '{':
				depth++
			case ']', '}':
				depth = max(0, depth-1)
			}
		}
		i++
	}
	return i
}
