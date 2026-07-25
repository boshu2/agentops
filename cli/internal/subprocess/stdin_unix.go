//go:build !windows

package subprocess

import (
	"errors"
	"syscall"
)

func isIgnorableStdinCopyError(err error) bool {
	return errors.Is(err, syscall.EPIPE)
}
