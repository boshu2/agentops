// Package tracker_br is the subprocess adapter for a resolved beads_rust
// backend. Selection and ledger discovery remain owned by trackerresolve.
package tracker_br

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/boshu2/agentops/cli/internal/trackerresolve"
)

type Adapter struct {
	resolution trackerresolve.Resolution
}

func New(resolution trackerresolve.Resolution) (*Adapter, error) {
	if resolution.Tracker != trackerresolve.BR {
		return nil, fmt.Errorf("tracker_br: resolution backend is %q, want %q", resolution.Tracker, trackerresolve.BR)
	}
	if resolution.Binary == "" || resolution.WorkDir == "" {
		return nil, fmt.Errorf("tracker_br: incomplete resolution")
	}
	return &Adapter{resolution: resolution}, nil
}

func (adapter *Adapter) CommandContext(ctx context.Context, args ...string) *exec.Cmd {
	command := exec.CommandContext(ctx, adapter.resolution.Binary, args...) // #nosec G204 -- binary is constrained by trackerresolve to br.
	command.Dir = adapter.resolution.WorkDir
	command.Env = append([]string(nil), adapter.resolution.ChildEnv...)
	return command
}
