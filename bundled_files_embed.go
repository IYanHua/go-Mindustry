//go:build !mdtserver_devbundle

package mdtserver

import (
	"embed"
	"io/fs"
)

// BundledFiles is the production embedded filesystem used for self-release.
//
//go:embed configs data/vanilla assets/worlds
var bundledFiles embed.FS

var BundledFiles fs.FS = bundledFiles
