//go:build !windows

package utils

import "golang.org/x/sys/unix"

func CanExecute(path string) bool {
	return unix.Access(path, unix.X_OK) == nil
}
