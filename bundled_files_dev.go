//go:build mdtserver_devbundle

package mdtserver

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
)

// BundledFiles provides a development-time fallback filesystem for bootstrap.
var BundledFiles fs.FS = devBundleFS()

func devBundleFS() fs.FS {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return os.DirFS(".")
	}
	return os.DirFS(filepath.Dir(file))
}
