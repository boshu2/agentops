//go:build !windows

package sourceread

import (
	"fmt"
	"os"
	"syscall"
)

func nativeIdentity(_ *os.File, info os.FileInfo) string {
	if native, ok := info.Sys().(*syscall.Stat_t); ok {
		return fmt.Sprintf("%d:%d", native.Dev, native.Ino)
	}
	return "unavailable"
}
