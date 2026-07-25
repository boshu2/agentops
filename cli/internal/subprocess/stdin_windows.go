//go:build windows

package subprocess

import (
	"errors"
	"syscall"
)

func isIgnorableStdinCopyError(err error) bool {
	const errorNoData = syscall.Errno(0xe8)
	return errors.Is(err, syscall.ERROR_BROKEN_PIPE) || errors.Is(err, errorNoData)
}
