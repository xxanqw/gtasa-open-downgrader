//go:build launcher

// Package assets provides launcher artwork without linking the downgrader's
// patch, download, or Steam/Proton dependency graph.
package assets

import _ "embed"

//go:embed steam-launcher-icon.png
var Icon []byte

// Share the supplied client artwork without linking the downgrader UI.
//
//go:embed samp.png
var SAMPClientIcon []byte

//go:embed openmp.png
var OpenMPClientIcon []byte

//go:embed mta.png
var MTAClientIcon []byte
