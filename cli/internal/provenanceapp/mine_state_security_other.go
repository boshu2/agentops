//go:build !darwin && !linux && !windows

package provenanceapp

import (
	"fmt"
	"os"
	"runtime"
)

func checkpointSecurity(_ string, _ os.FileInfo) ([]byte, error) {
	return nil, fmt.Errorf("checkpoint permission inspection unavailable on %s", runtime.GOOS)
}
