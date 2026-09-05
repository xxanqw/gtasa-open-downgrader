//go:build !offline

package assets

import "embed"

var Patches embed.FS

const Offline = false
