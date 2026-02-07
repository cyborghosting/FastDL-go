//go:build windows

package utils

import (
	"os"
	"path/filepath"
	"strings"
)

func CanExecute(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	if info.IsDir() {
		// "search" permission: try opening the dir
		f, err := os.Open(path)
		if err != nil {
			return false
		}
		f.Close()
		return true
	}

	// Best-effort: check executable extensions
	ext := strings.ToLower(filepath.Ext(path))
	pathext := strings.ToLower(os.Getenv("PATHEXT"))
	for _, e := range strings.Split(pathext, ";") {
		if ext == e {
			return true
		}
	}
	return false
}
