//go:build offline

package assets

import "embed"

//go:embed Patches/manifest.json Patches
var Patches embed.FS

const Offline = true
