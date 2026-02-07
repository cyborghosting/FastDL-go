//go:build !windows

package utils

import (
	"syscall"
)

func TrySetNoFile(n uint64) (uint64, error) {
	// Query current limits
	var rLimit syscall.Rlimit
	err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		return 0, err
	}

	// If current limit is already sufficient, do nothing
	if rLimit.Cur >= n {
		return rLimit.Cur, nil
	}

	// Attempt to set new limit
	rLimit.Cur = n
	if rLimit.Cur > rLimit.Max {
		rLimit.Cur = rLimit.Max
	}
	err = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		return rLimit.Cur, err
	}

	// Verify new limit
	err = syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		return 0, err
	}
	return rLimit.Cur, nil
}
