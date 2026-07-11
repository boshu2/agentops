package close

import (
	"os"

	closeapp "github.com/boshu2/agentops/cli/internal/close"
)

type SystemRuntime struct{}

func (SystemRuntime) Snapshot() (closeapp.Snapshot, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return closeapp.Snapshot{}, err
	}
	return closeapp.Snapshot{WorkDir: cwd, Env: os.Environ()}, nil
}

type StaticRuntime struct {
	WorkDir string
	Env     []string
}

func (runtime StaticRuntime) Snapshot() (closeapp.Snapshot, error) {
	return closeapp.Snapshot{WorkDir: runtime.WorkDir, Env: append([]string(nil), runtime.Env...)}, nil
}
