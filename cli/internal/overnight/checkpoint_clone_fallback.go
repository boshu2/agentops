//go:build !darwin

package overnight

import "os"

func cloneFileForCheckpoint(src, dst string, mode os.FileMode) (int64, bool, error) {
	return 0, false, nil
}
