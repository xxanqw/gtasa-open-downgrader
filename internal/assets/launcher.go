//go:build launcher

// Package assets provides launcher artwork without linking the downgrader's
// patch, download, or Steam/Proton dependency graph.
package assets

import _ "embed"

//go:embed icon.png
var Icon []byte
