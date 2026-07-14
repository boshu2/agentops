// Package tracker_br is the subprocess adapter for a resolved beads_rust
// backend. Selection and ledger discovery remain owned by trackerresolve.
package tracker_br

import (
	"context"
	"fmt"

	"github.com/boshu2/agentops/cli/internal/trackerexec"
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

func (adapter *Adapter) CommandContext(
	ctx context.Context,
	args []string,
	streams trackerexec.Streams,
) *trackerexec.ResolvedCommand {
	return (trackerexec.Factory{}).Command(ctx, adapter.resolution, args, streams)
}
