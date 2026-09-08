package steam

import (
	"fmt"
	"strings"
	"unicode"
)

// vdfObject is the small subset of Valve's KeyValues format used by Steam's
// library, app manifest, compatibility mapping, and compatibility tool files.
// A slice preserves duplicate keys instead of silently overwriting them.
type vdfObject []vdfEntry

type vdfEntry struct {
	key      string
	value    string
	children vdfObject
}

func (o vdfObject) entry(name string) (vdfEntry, bool) {
	for _, entry := range o {
		if strings.EqualFold(entry.key, name) {
			return entry, true
		}
	}
	return vdfEntry{}, false
}

func (o vdfObject) object(name string) (vdfObject, bool) {
	entry, ok := o.entry(name)
	return entry.children, ok && entry.children != nil
}

func (o vdfObject) text(name string) (string, bool) {
	entry, ok := o.entry(name)
	return entry.value, ok && entry.children == nil
}

func (o vdfObject) findObject(name string) (vdfObject, bool) {
	if child, ok := o.object(name); ok {
		return child, true
	}
	for _, entry := range o {
		if entry.children == nil {
			continue
		}
		if child, ok := entry.children.findObject(name); ok {
			return child, true
		}
	}
	return nil, false
}

type vdfTokenKind uint8

const (
	vdfString vdfTokenKind = iota
	vdfOpen
	vdfClose
)

type vdfToken struct {
	kind         vdfTokenKind
	text         string
	line, column int
}

func parseVDF(data []byte) (vdfObject, error) {
	tokens, err := lexVDF(string(data))
	if err != nil {
		return nil, err
	}
	position := 0
	result, err := parseVDFObject(tokens, &position, false)
	if err != nil {
		return nil, err
	}
	if position != len(tokens) {
		token := tokens[position]
		return nil, fmt.Errorf("VDF line %d, column %d: unexpected token %q", token.line, token.column, token.text)
	}
	return result, nil
}

func parseVDFObject(tokens []vdfToken, position *int, nested bool) (vdfObject, error) {
	var object vdfObject
	for *position < len(tokens) {
		key := tokens[*position]
		if key.kind == vdfClose {
			if !nested {
				return nil, fmt.Errorf("VDF line %d, column %d: unexpected closing brace", key.line, key.column)
			}
			(*position)++
			return object, nil
		}
		if key.kind != vdfString {
			return nil, fmt.Errorf("VDF line %d, column %d: expected a key", key.line, key.column)
		}
		(*position)++
		if *position >= len(tokens) {
			return nil, fmt.Errorf("VDF line %d, column %d: key %q has no value", key.line, key.column, key.text)
		}
		value := tokens[*position]
		(*position)++
		entry := vdfEntry{key: key.text}
		switch value.kind {
		case vdfString:
			entry.value = value.text
		case vdfOpen:
			children, err := parseVDFObject(tokens, position, true)
			if err != nil {
				return nil, err
			}
			entry.children = children
		case vdfClose:
			return nil, fmt.Errorf("VDF line %d, column %d: key %q has no value", value.line, value.column, key.text)
		}
		object = append(object, entry)
		// KeyValues permits platform conditions after values and objects.
		if *position < len(tokens) && tokens[*position].kind == vdfString && strings.HasPrefix(tokens[*position].text, "[") && strings.HasSuffix(tokens[*position].text, "]") {
			(*position)++
		}
	}
	if nested {
		return nil, fmt.Errorf("VDF: object is missing a closing brace")
	}
	return object, nil
}

func lexVDF(source string) ([]vdfToken, error) {
	var tokens []vdfToken
	line, column := 1, 1
	advance := func() byte {
		character := source[0]
		source = source[1:]
		if character == '\n' {
			line++
			column = 1
		} else {
			column++
		}
		return character
	}
	for len(source) > 0 {
		if strings.HasPrefix(source, "\ufeff") {
			source = strings.TrimPrefix(source, "\ufeff")
			continue
		}
		if unicode.IsSpace(rune(source[0])) {
			advance()
			continue
		}
		if strings.HasPrefix(source, "//") {
			for len(source) > 0 && source[0] != '\n' {
				advance()
			}
			continue
		}
		startLine, startColumn := line, column
		switch source[0] {
		case '{':
			advance()
			tokens = append(tokens, vdfToken{kind: vdfOpen, text: "{", line: startLine, column: startColumn})
		case '}':
			advance()
			tokens = append(tokens, vdfToken{kind: vdfClose, text: "}", line: startLine, column: startColumn})
		case '"':
			advance()
			var value strings.Builder
			closed := false
			for len(source) > 0 {
				character := advance()
				if character == '"' {
					closed = true
					break
				}
				if character != '\\' || len(source) == 0 {
					value.WriteByte(character)
					continue
				}
				escaped := advance()
				switch escaped {
				case '\\', '"':
					value.WriteByte(escaped)
				case 'n':
					value.WriteByte('\n')
				case 'r':
					value.WriteByte('\r')
				case 't':
					value.WriteByte('\t')
				default:
					value.WriteByte('\\')
					value.WriteByte(escaped)
				}
			}
			if !closed {
				return nil, fmt.Errorf("VDF line %d, column %d: unterminated quoted string", startLine, startColumn)
			}
			tokens = append(tokens, vdfToken{kind: vdfString, text: value.String(), line: startLine, column: startColumn})
		default:
			var value strings.Builder
			for len(source) > 0 && !unicode.IsSpace(rune(source[0])) && source[0] != '{' && source[0] != '}' {
				if strings.HasPrefix(source, "//") {
					break
				}
				value.WriteByte(advance())
			}
			if value.Len() == 0 {
				return nil, fmt.Errorf("VDF line %d, column %d: unexpected character", startLine, startColumn)
			}
			tokens = append(tokens, vdfToken{kind: vdfString, text: value.String(), line: startLine, column: startColumn})
		}
	}
	return tokens, nil
}
