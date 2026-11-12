package applog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"
)

type scopeKey struct{}
type scope struct {
	journal *Journal
	id      uint64
}

// Context carries diagnostics through workers without a global logger or UI access.
func (j *Journal) Context(ctx context.Context) context.Context {
	return context.WithValue(ctx, scopeKey{}, scope{journal: j})
}

// Event writes one searchable line. Values are JSON encoded, including newlines.
func (j *Journal) Event(level, event string, fields ...any) {
	var line strings.Builder
	line.WriteString(event)
	for i := 0; i < len(fields); i += 2 {
		key := fmt.Sprint(fields[i])
		var value any = "<missing>"
		if i+1 < len(fields) {
			value = fields[i+1]
		}
		if err, ok := value.(error); ok {
			if err == nil {
				value = nil
			} else {
				value = SafeError(err)
			}
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			encoded, _ = json.Marshal(fmt.Sprint(value))
		}
		fmt.Fprintf(&line, " %s=%s", key, encoded)
	}
	j.Record(level, line.String())
}

func Event(ctx context.Context, level, event string, fields ...any) {
	if s, ok := ctx.Value(scopeKey{}).(scope); ok {
		s.journal.Event(level, event, append([]any{"op", s.id}, fields...)...)
	}
}

// Begin nests a timed operation inside the current operation. The returned end
// function is safe to call more than once; errors include their unwrap tree.
func Begin(ctx context.Context, name string, fields ...any) (context.Context, func(error)) {
	s, ok := ctx.Value(scopeKey{}).(scope)
	if !ok {
		return ctx, func(error) {}
	}
	s.journal.mu.Lock()
	s.journal.nextOperation++
	id := s.journal.nextOperation
	s.journal.mu.Unlock()
	parent := s.id
	ctx = context.WithValue(ctx, scopeKey{}, scope{journal: s.journal, id: id})
	started := time.Now()
	Event(ctx, "DEBUG", name+".start", append([]any{"parent", parent}, fields...)...)
	var once sync.Once
	return ctx, func(err error) {
		once.Do(func() {
			state, level := "ok", "DEBUG"
			if errors.Is(err, context.Canceled) {
				state, level = "cancelled", "INFO"
			} else if err != nil {
				state, level = "failed", "ERROR"
			}
			Event(ctx, level, name+".end", "result", state, "duration_ms", float64(time.Since(started).Microseconds())/1000, "errors", errorTree(err))
		})
	}
}

func errorTree(err error) []string {
	var tree []string
	var visit func(error, int)
	visit = func(err error, depth int) {
		if err == nil || depth > 16 || len(tree) >= 128 {
			return
		}
		tree = append(tree, fmt.Sprintf("%T: %s", err, SafeError(err)))
		switch e := err.(type) {
		case interface{ Unwrap() []error }:
			for _, child := range e.Unwrap() {
				visit(child, depth+1)
			}
		case interface{ Unwrap() error }:
			visit(e.Unwrap(), depth+1)
		}
	}
	visit(err, 0)
	return tree
}

// URL deliberately excludes credentials, signed query parameters and fragments.
func URL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "<invalid URL>"
	}
	u.User, u.RawQuery, u.Fragment, u.RawFragment = nil, "", "", ""
	u.ForceQuery = false
	return u.String()
}

// SafeError removes full request URLs even when net/http wraps them in errors.
func SafeError(err error) string {
	if err == nil {
		return ""
	}
	text := err.Error()
	pending := []error{err}
	for steps := 0; len(pending) > 0 && steps < 128; steps++ {
		current := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if current == nil {
			continue
		}
		if requestErr, ok := current.(*url.Error); ok && requestErr.URL != "" {
			text = strings.ReplaceAll(text, requestErr.URL, URL(requestErr.URL))
		}
		switch e := current.(type) {
		case interface{ Unwrap() []error }:
			pending = append(pending, e.Unwrap()...)
		case interface{ Unwrap() error }:
			pending = append(pending, e.Unwrap())
		}
	}
	return text
}
