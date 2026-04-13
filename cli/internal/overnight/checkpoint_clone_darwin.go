//go:build darwin

package overnight

import (
	"os"

	"golang.org/x/sys/unix"
)

func cloneFileForCheckpoint(src, dst string, mode os.FileMode) (int64, bool, error) {
	if err := unix.Clonefile(src, dst, 0); err != nil {
		_ = os.Remove(dst)
		return 0, false, nil
	}
	if err := os.Chmod(dst, mode.Perm()); err != nil {
		return 0, true, err
	}
	info, err := os.Stat(src)
	if err != nil {
		return 0, true, err
	}
	return info.Size(), true, nil
}
