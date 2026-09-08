//go:build !launcher

package assets

import _ "embed"

//go:embed xdelta3_linux
var xdelta []byte
