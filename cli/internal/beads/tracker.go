package beads

import (
	"context"
	"strings"
)

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

type LedgerSnapshot struct {
	Exists    bool
	Directory bool
	Readable  bool
	Entries   []string
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

type LedgerInspector interface {
	InspectLedger(string) LedgerSnapshot
}

// LedgerMissing preserves the deliberately different BR and BD artifact
// policies while keeping filesystem probing behind LedgerInspector.
func LedgerMissing(tracker string, snapshot LedgerSnapshot) string {
	if !snapshot.Exists {
		return "resolved path does not exist"
	}
	if !snapshot.Directory {
		return "resolved path is not a directory"
	}
	if tracker == TrackerBD && !snapshot.Readable {
		return "resolved directory is unreadable"
	}
	for _, name := range snapshot.Entries {
		switch tracker {
		case TrackerBR:
			if name == "issues.jsonl" || name == "beads.db" {
				return ""
			}
		case TrackerBD:
			if strings.HasSuffix(name, ".db") || strings.HasSuffix(name, ".jsonl") ||
				name == "config.yaml" || name == "config.json" || name == "beads.json" {
				return ""
			}
		}
	}
	if tracker == TrackerBD {
		return "no ledger artifact (*.db, *.jsonl, or config) in resolved directory"
	}
	return "no ledger artifact (issues.jsonl or beads.db) in resolved directory"
}
