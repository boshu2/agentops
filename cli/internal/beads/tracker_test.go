package beads

import "testing"

func TestLedgerMissingPreservesBRAndBDArtifactPolicies(t *testing.T) {
	for name, test := range map[string]struct {
		tracker  string
		snapshot LedgerSnapshot
		want     string
	}{
		"missing":           {tracker: TrackerBR, want: "resolved path does not exist"},
		"file":              {tracker: TrackerBR, snapshot: LedgerSnapshot{Exists: true}, want: "resolved path is not a directory"},
		"br jsonl":          {tracker: TrackerBR, snapshot: LedgerSnapshot{Exists: true, Directory: true, Readable: true, Entries: []string{"issues.jsonl"}}},
		"br rejects config": {tracker: TrackerBR, snapshot: LedgerSnapshot{Exists: true, Directory: true, Readable: true, Entries: []string{"config.yaml"}}, want: "no ledger artifact (issues.jsonl or beads.db) in resolved directory"},
		"bd config":         {tracker: TrackerBD, snapshot: LedgerSnapshot{Exists: true, Directory: true, Readable: true, Entries: []string{"config.yaml"}}},
		"bd unreadable":     {tracker: TrackerBD, snapshot: LedgerSnapshot{Exists: true, Directory: true}, want: "resolved directory is unreadable"},
	} {
		t.Run(name, func(t *testing.T) {
			if got := LedgerMissing(test.tracker, test.snapshot); got != test.want {
				t.Fatalf("LedgerMissing() = %q, want %q", got, test.want)
			}
		})
	}
}
