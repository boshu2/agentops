package yieldledger

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
)

// Load reads and validates the yield ledger under projectRoot.
//
//   - Missing ledger: returns an empty Ledger and no error (no events yet).
//   - Malformed ledger: returns a path-specific error (with the line number).
//   - Valid ledger: returns the parsed Ledger with events in append order.
func Load(projectRoot string) (*Ledger, error) {
	return LoadPath(ledgerPath(projectRoot))
}

// LoadPath reads and validates an append-only JSONL yield ledger at an explicit
// path: one event envelope per line, oldest-first. It is the shared core used by
// Load and by tests that point at tracked fixtures. A blank line is skipped; a
// malformed line is a hard error citing the 1-based line number so a writer can
// never silently drop or corrupt prior events.
func LoadPath(path string) (*Ledger, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Ledger{SchemaVersion: SchemaVersion}, nil
		}
		return nil, fmt.Errorf("read yield ledger %s: %w", path, err)
	}
	// Read-only handle: a Close error cannot lose data, so it is safe to drop.
	defer func() { _ = f.Close() }()

	ledger := &Ledger{SchemaVersion: SchemaVersion}
	sc := bufio.NewScanner(f)
	// Allow long event lines (default 64KiB token cap is too small for a fat
	// usage/gate-verdict envelope under load).
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		raw := bytes.TrimSpace(sc.Bytes())
		if len(raw) == 0 {
			continue
		}
		var ev Event
		if err := ev.UnmarshalJSON(raw); err != nil {
			return nil, fmt.Errorf("yield ledger %s: line %d: %w", path, lineNo, err)
		}
		if defect := validateEvent(ev); defect != "" {
			return nil, fmt.Errorf("yield ledger %s: line %d: %s", path, lineNo, defect)
		}
		ledger.Events = append(ledger.Events, ev)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scan yield ledger %s: %w", path, err)
	}
	return ledger, nil
}

// EventsFor returns every event for one bead in append (oldest-first) order.
func (l *Ledger) EventsFor(beadID string) []Event {
	var out []Event
	for _, ev := range l.Events {
		if ev.BeadID == beadID {
			out = append(out, ev)
		}
	}
	return out
}

// AcceptsFor returns the accept events for one bead in append order.
func (l *Ledger) AcceptsFor(beadID string) []Event {
	return l.eventsForOfType(beadID, EventAccept)
}

// GateVerdictsFor returns the gate-verdict events for one bead in append order.
func (l *Ledger) GateVerdictsFor(beadID string) []Event {
	return l.eventsForOfType(beadID, EventGateVerdict)
}

// UsageFor returns the usage events for one bead in append order.
func (l *Ledger) UsageFor(beadID string) []Event {
	return l.eventsForOfType(beadID, EventUsage)
}

// eventsForOfType filters to one bead and one event type, preserving order.
func (l *Ledger) eventsForOfType(beadID, eventType string) []Event {
	var out []Event
	for _, ev := range l.Events {
		if ev.BeadID == beadID && ev.Event == eventType {
			out = append(out, ev)
		}
	}
	return out
}
