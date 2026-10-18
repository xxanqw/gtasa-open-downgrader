package assets

import "embed"

// Both executables share the same recolorable interface artwork.
//
//go:embed fontawesome/*.svg
var FontAwesome embed.FS
