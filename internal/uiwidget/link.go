package uiwidget

import (
	"net/url"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Link draws its hover underline with the text so it follows the baseline.
type Link struct{ widget.Hyperlink }

func NewLink(text string, address *url.URL) *Link {
	link := &Link{}
	link.Text, link.URL = text, address
	link.ExtendBaseWidget(link)
	return link
}

func (l *Link) MouseIn(event *desktop.MouseEvent) { l.MouseMoved(event) }

func (l *Link) MouseMoved(event *desktop.MouseEvent) {
	th := l.Theme()
	sizeName := l.SizeName
	if sizeName == "" {
		sizeName = theme.SizeNameText
	}
	text := fyne.MeasureText(l.Text, th.Size(sizeName), l.TextStyle)
	innerPadding := th.Size(theme.SizeNameInnerPadding)
	size := l.Size()
	width := min(size.Width-innerPadding, text.Width+2*th.Size(theme.SizeNamePadding))
	left := innerPadding / 2
	switch l.Alignment {
	case fyne.TextAlignCenter:
		left = (size.Width - width) / 2
	case fyne.TextAlignTrailing:
		left = size.Width - width - left
	}
	pos := event.Position
	height := min(size.Height, l.MinSize().Height)
	l.setHoverUnderline(pos.X >= left && pos.X <= left+width && pos.Y >= innerPadding/2 && pos.Y <= height)
}

func (l *Link) MouseOut() { l.setHoverUnderline(false) }

func (l *Link) Cursor() desktop.Cursor {
	if l.TextStyle.Underline {
		return desktop.PointerCursor
	}
	return desktop.DefaultCursor
}

func (l *Link) setHoverUnderline(hovered bool) {
	if l.TextStyle.Underline == hovered {
		return
	}
	l.TextStyle.Underline = hovered
	l.Refresh()
}
