//go:build !darwin && !linux && !windows

package subprocess

import (
	"fmt"
	"os"
	"runtime"
)

func newProcessCompletion(*os.Process) processCompletion {
	return &pollingProcessCompletion{
		probe: func() (bool, error) {
			return false, fmt.Errorf("bounded process completion is unsupported on %s", runtime.GOOS)
		},
	}
}
