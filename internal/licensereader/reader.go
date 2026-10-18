// Package licensereader presents notices in a virtualized, continuous scroller.
// Only visible lines have Fyne labels; full copy/source exports remain separate.
package licensereader

import (
	"image/color"
	"math"
	"strings"
	"unicode"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const MaxLineBytes = 4096

type line struct{ start, end int }

// wrapLines indexes source byte ranges using cached glyph advances, without
// shaping a Fyne text object for the whole document. Newlines remain indexed;
// only the visible row's trailing newline is omitted when painting it.
func wrapLines(text string, width float32, advance func(rune) float32) []line {
	var rows []line
	start, at, wordBreak := 0, 0, -1
	used := float32(0)
	for at < len(text) {
		r, n := utf8.DecodeRuneInString(text[at:])
		// Even zero-advance/combining-only input must not create a giant row.
		if at > start && at+n-start > MaxLineBytes {
			rows = append(rows, line{start, at})
			start, wordBreak, used = at, -1, 0
			continue
		}
		if r == '\n' {
			rows = append(rows, line{start, at + n})
			start, at, wordBreak, used = at+n, at+n, -1, 0
			continue
		}
		w := advance(r)
		if used+w > width && at > start {
			end := at
			if wordBreak > start {
				end = wordBreak
			}
			rows = append(rows, line{start, end})
			start, at, wordBreak, used = end, end, -1, 0
			continue
		}
		used += w
		at += n
		if unicode.IsSpace(r) {
			wordBreak = at
		}
	}
	if start < len(text) {
		rows = append(rows, line{start, len(text)})
	}
	if len(rows) == 0 {
		rows = append(rows, line{})
	}
	return rows
}

// Reader inherits Fyne's pooled/virtualized List scrolling and keyboard support.
// Row heights are uniform; there are no estimates, scroll jumps, growing VBox,
// asynchronous mutation, or SetItemHeight/Refresh calls inside UpdateItem.
type Reader struct {
	widget.List
	text      string
	rows      []line
	wrapWidth float32
	textSize  float32
	font      fyne.Resource
	advances  map[rune]float32
	scroller  *container.Scroll
	hoverRows map[fyne.CanvasObject]bool
}

// denseTheme removes list-row gaps and label padding, not text size. A virtual
// line should occupy one normal text line, rather than one padded control row.
type denseTheme struct{ fyne.Theme }

func (t denseTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if name == theme.ColorNameHover {
		return color.Transparent
	}
	// Keep the selection color: selecting actual notice text must stay visible.
	return t.Theme.Color(name, variant)
}

func (t denseTheme) Size(name fyne.ThemeSizeName) float32 {
	if name == theme.SizeNamePadding || name == theme.SizeNameInnerPadding {
		return 0
	}
	return t.Theme.Size(name)
}

func New(text string) *container.ThemeOverride {
	r := newReader(text)
	return container.NewThemeOverride(r, denseTheme{fyne.CurrentApp().Settings().Theme()})
}

func newReader(text string) *Reader {
	r := &Reader{text: text}
	r.ExtendBaseWidget(r)
	r.HideSeparators = true
	r.Length = func() int { return len(r.rows) }
	r.CreateItem = func() fyne.CanvasObject {
		label := widget.NewLabel(" ")
		label.Selectable = true
		// List passes its embedded *List as the theme scope; that is not the
		// wrapper Reader's cache key. Explicitly theme pooled row labels.
		return container.NewThemeOverride(label, denseTheme{fyne.CurrentApp().Settings().Theme()})
	}
	r.UpdateItem = func(id widget.ListItemID, object fyne.CanvasObject) {
		label := object.(*container.ThemeOverride).Content.(*widget.Label)
		row := r.rows[id]
		text := strings.TrimRight(r.text[row.start:row.end], "\r\n")
		if label.Text != text {
			label.SetText(text)
		}
	}
	// Clicking text should select text, not leave a whole list row selected.
	r.OnSelected = func(widget.ListItemID) { r.UnselectAll() }
	// Fyne creates pooled list-item backgrounds outside the child label's
	// theme scope. Style visible item widgets before MouseIn paints them.
	r.OnHighlighted = func(widget.ListItemID) { r.removeRowHover() }
	// Defer theme measurement until after the dense theme scope is attached;
	// measuring earlier would cache a padded prototype's height in List.
	r.rows = []line{{}}
	return r
}

func (r *Reader) CreateRenderer() fyne.WidgetRenderer {
	renderer := r.List.CreateRenderer()
	r.scroller = renderer.Objects()[0].(*container.Scroll)
	return renderer
}

func (r *Reader) removeRowHover() {
	if r.scroller == nil {
		return
	}
	visible := make(map[fyne.CanvasObject]bool)
	for _, object := range r.scroller.Content.(*fyne.Container).Objects {
		if _, ok := object.(fyne.Widget); !ok {
			continue
		}
		if !r.hoverRows[object] {
			// Apply to the item itself, not just its wrapped Label. Preserve
			// native text selection and all scroll/keyboard behavior.
			container.NewThemeOverride(object, denseTheme{fyne.CurrentApp().Settings().Theme()})
		}
		visible[object] = true
	}
	// Do not retain items discarded from the pool by a refresh or resize.
	r.hoverRows = visible
}

func (r *Reader) rewrap(width float32) bool {
	if width <= 0 {
		width = max(r.wrapWidth, 320)
	}
	size := r.Theme().Size(theme.SizeNameText)
	font := r.Theme().Font(fyne.TextStyle{})
	if math.Abs(float64(width-r.wrapWidth)) < 0.5 && r.textSize == size && r.font == font {
		return false
	}
	if r.textSize != size || r.font != font || r.advances == nil {
		r.advances = make(map[rune]float32)
	}
	r.wrapWidth, r.textSize, r.font = width, size, font
	// Allow label padding, scrollbar, and a small kerning/shaping margin.
	available := max(float32(1), width-2*r.Theme().Size(theme.SizeNamePadding)-2*size)
	r.rows = wrapLines(r.text, available, func(char rune) float32 {
		if width, ok := r.advances[char]; ok {
			return width
		}
		width := fyne.MeasureText(string(char), size, fyne.TextStyle{}).Width
		r.advances[char] = width
		return width
	})
	return true
}

func (r *Reader) Resize(size fyne.Size) {
	r.rewrap(size.Width)
	r.List.Resize(size)
}

func (r *Reader) Refresh() {
	r.rewrap(r.Size().Width)
	r.List.Refresh()
}
