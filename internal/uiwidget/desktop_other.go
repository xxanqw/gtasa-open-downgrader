//go:build !linux

package uiwidget

// Windows uses its own (or Wine's) cursor/backend integration.
func ConfigureDesktopCursors() {}
