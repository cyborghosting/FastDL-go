//go:build windows

package utils

func TrySetNoFile(n uint64) (uint64, error) {
	// No-op on Windows
	return 0, nil
}
