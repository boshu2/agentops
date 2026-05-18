package verdictledger

import (
	"encoding/json"
	"fmt"
	"os"
)

// Load reads and validates the verdict ledger under projectRoot.
//
//   - Missing ledger: returns an empty Ledger and no error (no iterations yet).
//   - Malformed ledger: returns a path-specific error.
//   - Valid ledger: returns the parsed Ledger with records in append order.
func Load(projectRoot string) (*Ledger, error) {
	return LoadPath(ledgerPath(projectRoot))
}

// LoadPath reads and validates a verdict ledger at an explicit path. It is the
// shared core used by Load and by tests that point at tracked fixtures.
func LoadPath(path string) (*Ledger, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Ledger{SchemaVersion: SchemaVersion}, nil
		}
		return nil, fmt.Errorf("read verdict ledger %s: %w", path, err)
	}

	var ledger Ledger
	if err := json.Unmarshal(data, &ledger); err != nil {
		return nil, fmt.Errorf("parse verdict ledger %s: %w", path, err)
	}
	if ledger.SchemaVersion != SchemaVersion {
		return nil, fmt.Errorf(
			"verdict ledger %s: schema_version %q != %q",
			path, ledger.SchemaVersion, SchemaVersion)
	}
	for i, r := range ledger.Records {
		if defect := validateRecord(r); defect != "" {
			return nil, fmt.Errorf("verdict ledger %s: records[%d] %s", path, i, defect)
		}
	}
	return &ledger, nil
}

// IterationsFor returns the iteration records for one directive in append
// (oldest-first) order. Cooldown records are excluded.
func (l *Ledger) IterationsFor(directiveID string) []Record {
	var out []Record
	for _, r := range l.Records {
		if r.RecordType == RecordIteration && r.DirectiveID == directiveID {
			out = append(out, r)
		}
	}
	return out
}

// FailureStreak returns the directive's current failure streak: the length of
// the longest unbroken run of consecutive `fail` iteration verdicts ending at
// the most recent iteration. Any `pass`, `skip`, or `unknown` verdict resets
// the streak to 0 (ADR-0006 §FAILURE STREAK).
func (l *Ledger) FailureStreak(directiveID string) int {
	iterations := l.IterationsFor(directiveID)
	streak := 0
	for i := len(iterations) - 1; i >= 0; i-- {
		if iterations[i].ScenarioVerdict != VerdictFail {
			break
		}
		streak++
	}
	return streak
}

// IterationCount returns the number of iteration records for the directive —
// the evidence count used by the F5.2 minimum-evidence gate.
func (l *Ledger) IterationCount(directiveID string) int {
	return len(l.IterationsFor(directiveID))
}

// InCooldown reports whether the directive has a cooldown record within the
// most recent cooldownIterations iteration records (ADR-0006 §COOLDOWN). The
// window is measured in iteration records for the directive, not wall time.
func (l *Ledger) InCooldown(directiveID string, cooldownIterations int) bool {
	if cooldownIterations < 1 {
		return false
	}
	iterations := l.IterationsFor(directiveID)
	windowStart := ""
	if len(iterations) >= cooldownIterations {
		windowStart = iterations[len(iterations)-cooldownIterations].RunTime
	}
	for _, r := range l.Records {
		if r.RecordType != RecordCooldown || r.DirectiveID != directiveID {
			continue
		}
		if windowStart == "" || r.RunTime >= windowStart {
			return true
		}
	}
	return false
}
