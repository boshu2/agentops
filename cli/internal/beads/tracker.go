package beads

import "context"

const (
	TrackerBR = "br"
	TrackerBD = "bd"
)

// TrackerResolution is the command-family view of a selected tracker. It is
// deliberately independent of trackerresolve so driving adapters depend on a
// port, not the concrete workspace adapter.
type TrackerResolution struct {
	Tracker   string   `json:"tracker"`
	Binary    string   `json:"binary"`
	LedgerDir string   `json:"ledger_dir"`
	Source    string   `json:"source"`
	WorkDir   string   `json:"-"`
	ChildEnv  []string `json:"-"`
}

type LedgerResolution struct {
	Path   string
	Source string
}

type TrackerResolver interface {
	Resolve() (TrackerResolution, error)
	BRLedger() (LedgerResolution, error)
	BeadsDirOverride() bool
}

type TrackerClient interface {
	Available() bool
	Output(context.Context, ...string) ([]byte, error)
}
