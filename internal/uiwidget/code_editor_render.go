package uiwidget

import (
	"image/color"
	"math"
	"sort"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/go-text/typesetting/segmenter"
)

func codeNumber(n int) string { return strconv.Itoa(n) }

type codeMetrics struct {
	size, height, cell, pad, gutter float32
	style                           fyne.TextStyle
}

func (e *CodeEditor) metrics() codeMetrics {
	size := e.FontSize
	if size <= 0 {
		size = e.Theme().Size(theme.SizeNameText)
	}
	style := fyne.TextStyle{Monospace: true}
	font := e.Theme().Font(style)
	if e.metricCache.size != size || e.metricFont != font {
		measured := fyne.MeasureText("M", size, style)
		e.metricCache = codeMetrics{size: size, height: measured.Height + 4, cell: measured.Width, pad: 12, style: style}
		e.metricFont = font
	}
	m := e.metricCache
	m.gutter = max(float32(48), float32(len(strconv.Itoa(len(e.starts)))+2)*m.cell+8)
	return m
}

type codeCluster struct {
	start, end int
	x, width   float32
	text       string
}
type codeLine struct {
	text     string
	clusters []codeCluster
	width    float32
	tokens   []CodeToken
}

func (e *CodeEditor) buildLines() {
	m := e.metrics()
	if e.layoutRevision == e.revision && e.layoutSize == m.size && e.layoutTabWidth == e.TabWidth {
		return
	}
	if e.measurements == nil || e.layoutSize != m.size {
		e.measurements = make(map[string]float32)
	}
	// Retain geometry and tokens for unchanged lines, including shifted lines
	// after a newline insertion. Only edited text needs grapheme segmentation.
	previous := make(map[string]codeLine, len(e.layouts))
	if e.layoutSize == m.size && e.layoutTabWidth == e.TabWidth {
		for _, line := range e.layouts {
			previous[line.text] = line
		}
	}
	e.layouts = make([]codeLine, len(e.starts))
	e.widest = 0
	var sg segmenter.Segmenter
	for row, start := range e.starts {
		runes := e.buffer[start:e.lineEnd(row)]
		line := codeLine{text: string(runes)}
		if cached, ok := previous[line.text]; ok {
			e.layouts[row] = cached
			e.widest = max(e.widest, cached.width)
			continue
		}
		if e.Highlight != nil {
			line.tokens = e.Highlight(line.text)
		}
		sg.Init(runes)
		it := sg.GraphemeIterator()
		x := float32(0)
		for it.Next() {
			g := it.Grapheme()
			text := string(g.Text)
			width := float32(0)
			if text == "\t" {
				stop := float32(max(1, e.TabWidth)) * m.cell
				count := max(1, int(math.Round(float64((stop-float32(math.Mod(float64(x), float64(stop))))/m.cell))))
				text = strings.Repeat(" ", count)
				width = float32(count) * m.cell
			} else {
				var cached bool
				width, cached = e.measurements[text]
				if !cached {
					width = fyne.MeasureText(text, m.size, m.style).Width
					if len(e.measurements) < 1024 && len(text) <= 24 {
						e.measurements[text] = width
					}
				}
			}
			line.clusters = append(line.clusters, codeCluster{start: g.Offset, end: g.Offset + len(g.Text), x: x, width: width, text: text})
			x += width
		}
		line.width = x
		e.layouts[row] = line
		e.widest = max(e.widest, x)
	}
	e.layoutRevision = e.revision
	e.layoutSize = m.size
	e.layoutTabWidth = e.TabWidth
}
func (e *CodeEditor) columnX(row, column int) float32 {
	e.buildLines()
	row = max(0, min(row, len(e.layouts)-1))
	line := e.layouts[row]
	index := sort.Search(len(line.clusters), func(i int) bool { return line.clusters[i].end > column })
	if index < len(line.clusters) {
		return line.clusters[index].x
	}
	return line.width
}
func (e *CodeEditor) columnAt(row int, x float32) int {
	e.buildLines()
	row = max(0, min(row, len(e.layouts)-1))
	line := e.layouts[row]
	index := sort.Search(len(line.clusters), func(i int) bool { return line.clusters[i].x+line.clusters[i].width/2 >= x })
	if index < len(line.clusters) {
		return line.clusters[index].start
	}
	return e.lineEnd(row) - e.starts[row]
}
func (e *CodeEditor) pointOffset(p fyne.Position) int {
	m := e.metrics()
	row := max(0, min(int(math.Floor(float64((p.Y-m.pad)/m.height))), len(e.starts)-1))
	return e.starts[row] + e.columnAt(row, max(0, p.X-m.pad))
}
func (e *CodeEditor) ensureCaret() {
	if e.scroll == nil {
		return
	}
	e.buildLines()
	// Caret movement is not a document/layout change. Refresh scroll bars only
	// when content geometry actually changes, rather than once per arrow key.
	minimum, viewport := e.body.MinSize(), e.scroll.Size()
	content := fyne.NewSize(max(minimum.Width, viewport.Width), max(minimum.Height, viewport.Height))
	if e.body.Size() != content || e.scrollMinimum != minimum {
		e.body.Resize(content)
		e.scrollMinimum = minimum
		e.scroll.Refresh()
	}
	m := e.metrics()
	e.CursorRow = max(0, min(e.CursorRow, len(e.starts)-1))
	e.CursorColumn = max(0, min(e.CursorColumn, e.lineEnd(e.CursorRow)-e.starts[e.CursorRow]))
	e.anchor = max(0, min(e.anchor, len(e.buffer)))
	x, y := m.pad+e.columnX(e.CursorRow, e.CursorColumn), m.pad+float32(e.CursorRow)*m.height
	offset := e.scroll.Offset
	size := e.scroll.Size()
	if x < offset.X+m.pad {
		offset.X = max(0, x-m.pad)
	} else if x+m.cell > offset.X+size.Width-m.pad {
		offset.X = max(0, x+m.cell-size.Width+m.pad)
	}
	if y < offset.Y {
		offset.Y = y
	} else if y+m.height > offset.Y+size.Height {
		offset.Y = max(0, y+m.height-size.Height)
	}
	e.scroll.ScrollToOffset(offset)
}
func (e *CodeEditor) CreateRenderer() fyne.WidgetRenderer {
	e.body = &codeEditorBody{editor: e}
	e.body.ExtendBaseWidget(e.body)
	e.scroll = container.NewScroll(e.body)
	e.gutter = &codeEditorGutter{editor: e}
	e.gutter.ExtendBaseWidget(e.gutter)
	e.scroll.OnScrolled = func(fyne.Position) { e.body.Refresh(); e.gutter.Refresh() }
	gutterClip := container.NewClip(e.gutter)
	// A direct editor layout can resize Clip before any MinSize call initializes
	// its BaseWidget. Initialize explicitly so the gutter receives that layout.
	gutterClip.ExtendBaseWidget(gutterClip)
	return &codeEditorRenderer{editor: e, background: canvas.NewRectangle(color.Transparent), border: canvas.NewRectangle(color.Transparent), gutter: gutterClip}
}

type codeEditorRenderer struct {
	editor             *CodeEditor
	background, border *canvas.Rectangle
	gutter             *container.Clip
}

func (r *codeEditorRenderer) Destroy() {
	if r.editor.blink != nil {
		r.editor.blink.Stop()
	}
	r.editor.scroll.OnScrolled = nil
	r.editor.Focused = false
	r.editor.body = nil
	r.editor.gutter = nil
	r.editor.scroll = nil
}
func (r *codeEditorRenderer) MinSize() fyne.Size { return fyne.NewSize(320, 160) }
func (r *codeEditorRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.background, r.gutter, r.editor.scroll, r.border}
}
func (r *codeEditorRenderer) Layout(s fyne.Size) {
	m := r.editor.metrics()
	r.background.Resize(s)
	r.border.Resize(s)
	r.gutter.Move(fyne.NewPos(1, 1))
	r.gutter.Resize(fyne.NewSize(m.gutter, max(0, s.Height-2)))
	r.editor.scroll.Move(fyne.NewPos(m.gutter+1, 1))
	r.editor.scroll.Resize(fyne.NewSize(max(0, s.Width-m.gutter-2), max(0, s.Height-2)))
}
func (r *codeEditorRenderer) Refresh() {
	e := r.editor
	th := e.Theme()
	r.background.FillColor = th.Color(theme.ColorNameInputBackground, theme.VariantDark)
	r.border.FillColor = color.Transparent
	r.border.StrokeColor = th.Color(theme.ColorNameInputBorder, theme.VariantDark)
	r.border.StrokeWidth = 1
	if e.Focused && !e.Disabled() {
		r.border.StrokeColor = r.editor.Theme().Color(theme.ColorNameForeground, theme.VariantDark)
	}
	r.background.Refresh()
	r.border.Refresh()
	r.Layout(e.Size())
	e.body.Refresh()
	e.gutter.Refresh()
}

// Pools contain only visible glyphs and selection rectangles. Document size
// contributes scroll geometry, not a canvas object for every character/line.
type codePaintPool struct {
	texts                []*canvas.Text
	rects                []*canvas.Rectangle
	objects              []fyne.CanvasObject
	textCount, rectCount int
}

func codeSameColor(a, b color.Color) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	return ar == br && ag == bg && ab == bb && aa == ba
}
func (p *codePaintPool) begin() { p.objects = p.objects[:0]; p.textCount = 0; p.rectCount = 0 }
func (p *codePaintPool) rect(c color.Color, pos fyne.Position, size fyne.Size) {
	if size.Width <= 0 || size.Height <= 0 {
		return
	}
	if p.rectCount == len(p.rects) {
		p.rects = append(p.rects, canvas.NewRectangle(c))
	}
	r := p.rects[p.rectCount]
	p.rectCount++
	changed := !codeSameColor(r.FillColor, c)
	r.FillColor = c
	r.Move(pos)
	r.Resize(size)
	if changed {
		r.Refresh()
	}
	p.objects = append(p.objects, r)
}
func (p *codePaintPool) text(text string, c color.Color, pos fyne.Position, size fyne.Size, m codeMetrics, align fyne.TextAlign) {
	if text == "" {
		return
	}
	if p.textCount == len(p.texts) {
		p.texts = append(p.texts, canvas.NewText("", c))
	}
	t := p.texts[p.textCount]
	p.textCount++
	changed := t.Text != text || !codeSameColor(t.Color, c) || t.TextSize != m.size || t.TextStyle != m.style || t.Alignment != align
	t.Text = text
	t.Color = c
	t.TextSize = m.size
	t.TextStyle = m.style
	t.Alignment = align
	t.Move(pos)
	resized := t.Size() != size
	t.Resize(size)
	if changed && !resized {
		t.Refresh()
	}
	p.objects = append(p.objects, t)
}
func (p *codePaintPool) finish() {
	if len(p.texts) > p.textCount+128 {
		clear(p.texts[p.textCount+32:])
		p.texts = p.texts[:p.textCount+32]
	}
	if len(p.rects) > p.rectCount+64 {
		clear(p.rects[p.rectCount+16:])
		p.rects = p.rects[:p.rectCount+16]
	}
}
func (e *CodeEditor) viewport() (int, int) {
	m := e.metrics()
	first := max(0, min(len(e.starts), int((e.scroll.Offset.Y-m.pad)/m.height)-1))
	last := min(len(e.starts), first+int(e.scroll.Size().Height/m.height)+4)
	return first, last
}

type codeEditorBody struct {
	widget.BaseWidget
	editor *CodeEditor
	caret  *canvas.Rectangle
}

func (b *codeEditorBody) CreateRenderer() fyne.WidgetRenderer {
	b.caret = canvas.NewRectangle(color.Transparent)
	return &codeBodyRenderer{body: b}
}

// Blinking dirties one rectangle, not every visible line and glyph texture.
func (b *codeEditorBody) refreshCaret() {
	if b.caret == nil {
		return
	}
	e := b.editor
	m := e.metrics()
	r := b.caret
	hidden := !e.Focused || !e.caretVisible || e.Disabled()
	width := float32(1.5)
	c := e.Theme().Color(theme.ColorNameForeground, theme.VariantDark)
	if e.Overwrite {
		width = m.cell
		c = color.NRGBA{R: 225, G: 225, B: 225, A: 100}
	}
	changed := r.Hidden != hidden || !codeSameColor(r.FillColor, c)
	r.Hidden = hidden
	r.FillColor = c
	r.Move(fyne.NewPos(m.pad+e.columnX(e.CursorRow, e.CursorColumn), m.pad+float32(e.CursorRow)*m.height+2))
	r.Resize(fyne.NewSize(width, m.height-4))
	if changed {
		r.Refresh()
	}
}

type codeBodyRenderer struct {
	body *codeEditorBody
	pool codePaintPool
}

func (r *codeBodyRenderer) Destroy() { r.pool = codePaintPool{} }
func (r *codeBodyRenderer) MinSize() fyne.Size {
	e := r.body.editor
	e.buildLines()
	m := e.metrics()
	return fyne.NewSize(e.widest+2*m.pad+2*m.cell, float32(len(e.starts))*m.height+2*m.pad)
}
func (r *codeBodyRenderer) Objects() []fyne.CanvasObject { return r.pool.objects }
func (r *codeBodyRenderer) Layout(fyne.Size)             { r.Refresh() }
func (r *codeBodyRenderer) Refresh() {
	e := r.body.editor
	if e.scroll == nil {
		return
	}
	e.buildLines()
	m := e.metrics()
	first, last := e.viewport()
	r.pool.begin()
	start, end := e.selection()
	foreground := e.Theme().Color(theme.ColorNameForeground, theme.VariantDark)
	if e.Disabled() {
		foreground = e.Theme().Color(theme.ColorNameDisabled, theme.VariantDark)
	}
	selection := color.NRGBA{R: 69, G: 83, B: 105, A: 255}
	if !e.Focused {
		selection = color.NRGBA{R: 48, G: 54, B: 65, A: 255}
	}
	left, right := e.scroll.Offset.X, e.scroll.Offset.X+e.scroll.Size().Width
	for row := first; row < last; row++ {
		y := m.pad + float32(row)*m.height
		line := e.layouts[row]
		if row == e.CursorRow && e.Focused {
			r.pool.rect(color.NRGBA{R: 36, G: 36, B: 36, A: 255}, fyne.NewPos(left, y), fyne.NewSize(e.scroll.Size().Width, m.height))
		}
		lineStart, lineEnd := e.starts[row], e.lineEnd(row)
		if start != end && end > lineStart && start <= lineEnd {
			a, b := max(start, lineStart)-lineStart, min(end, lineEnd)-lineStart
			x1, x2 := e.columnX(row, a), e.columnX(row, b)
			if end > lineEnd {
				x2 += m.cell
			}
			r.pool.rect(selection, fyne.NewPos(m.pad+x1, y), fyne.NewSize(max(0, x2-x1), m.height))
		}
		// Keep grapheme geometry for hit testing, but batch adjacent glyphs of
		// one colour into a canvas run. One object per character caused thousands
		// of texture invalidations and thread checks on every input event.
		firstCluster := sort.Search(len(line.clusters), func(i int) bool { return m.pad+line.clusters[i].x+line.clusters[i].width >= left })
		var text strings.Builder
		var runColor color.Color
		var runX, runWidth float32
		flush := func() {
			if text.Len() > 0 {
				r.pool.text(text.String(), runColor, fyne.NewPos(runX, y+2), fyne.NewSize(runWidth, m.height-4), m, fyne.TextAlignLeading)
				text.Reset()
			}
		}
		for _, cluster := range line.clusters[firstCluster:] {
			x := m.pad + cluster.x
			if x > right {
				break
			}
			c := foreground
			// A selected token uses readable foreground rather than low-contrast
			// syntax colour against the selection surface.
			selected := start != end && lineStart+cluster.start >= start && lineStart+cluster.start < end
			if !e.Disabled() && !selected {
				for _, token := range line.tokens {
					if cluster.start >= token.Start && cluster.start < token.End {
						c = token.Color
						break
					}
				}
			}
			if text.Len() > 0 && !codeSameColor(c, runColor) {
				flush()
			}
			if text.Len() == 0 {
				runX = x
				runColor = c
				runWidth = 0
			}
			text.WriteString(cluster.text)
			runWidth += cluster.width
		}
		flush()
	}
	r.body.refreshCaret()
	r.pool.objects = append(r.pool.objects, r.body.caret)
	r.pool.finish()
	canvas.Refresh(r.body)
}

type codeEditorGutter struct {
	widget.BaseWidget
	editor *CodeEditor
}

func (g *codeEditorGutter) CreateRenderer() fyne.WidgetRenderer {
	return &codeGutterRenderer{gutter: g}
}

type codeGutterRenderer struct {
	gutter *codeEditorGutter
	pool   codePaintPool
}

func (r *codeGutterRenderer) Destroy() { r.pool = codePaintPool{} }
func (r *codeGutterRenderer) MinSize() fyne.Size {
	return fyne.NewSize(r.gutter.editor.metrics().gutter, 1)
}
func (r *codeGutterRenderer) Objects() []fyne.CanvasObject { return r.pool.objects }
func (r *codeGutterRenderer) Layout(fyne.Size)             { r.Refresh() }
func (r *codeGutterRenderer) Refresh() {
	e := r.gutter.editor
	if e.scroll == nil {
		return
	}
	m := e.metrics()
	first, last := e.viewport()
	r.pool.begin()
	width := r.gutter.Size().Width
	r.pool.rect(color.NRGBA{R: 25, G: 25, B: 25, A: 255}, fyne.NewPos(0, 0), r.gutter.Size())
	r.pool.rect(color.NRGBA{R: 48, G: 48, B: 48, A: 255}, fyne.NewPos(width-1, 0), fyne.NewSize(1, r.gutter.Size().Height))
	for row := first; row < last; row++ {
		c := color.NRGBA{R: 148, G: 148, B: 148, A: 255}
		if row == e.CursorRow {
			c = color.NRGBA{R: 225, G: 225, B: 225, A: 255}
		}
		r.pool.text(codeNumber(row+1), c, fyne.NewPos(0, m.pad+float32(row)*m.height-e.scroll.Offset.Y+2), fyne.NewSize(width-10, m.height-4), m, fyne.TextAlignTrailing)
	}
	r.pool.finish()
	canvas.Refresh(r.gutter)
}
