// Package probeheadroom classifies behavioral-probe scorecards by the
// HEADROOM the scenario still has, which is a different question from the
// probe's own verdict.
//
// A probe verdict compares the two arms: BEHAVIORAL when treatment beats
// control, INERT when the rates are equal. INERT is ambiguous on its own — it
// covers both "the skill changed nothing" (a real null, worth recording) and
// "the control arm already aced the scenario, so there was nothing left to
// measure" (a void row: the measurement failed, not the skill). Only the
// CONTROL arm's absolute rate separates those two, and re-running a saturated
// scenario at a lower effort produces ledger rows instead of knowledge.
//
// RED-FIRST STATUS: this file is the declared surface only. Classify returns
// ErrNotImplemented until the rule lands; the fixture-separation test in
// probeheadroom_test.go fails against it on purpose.
package probeheadroom

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ErrNotImplemented is returned by Classify until the headroom rule is ported.
var ErrNotImplemented = errors.New("probeheadroom: classification rule not implemented")

// ControlCeiling is the control-arm rate at or above which one effort level
// counts as "aced": the scenario left no room for the treatment arm to show a
// difference at that level.
const ControlCeiling = 0.75

// MinUsableReps is the number of usable control reps an effort level needs
// before its rate is allowed to count toward saturation. One rep is an
// anecdote, not a level.
const MinUsableReps = 2

// SchemaPrefix is the scorecard schema family this package accepts. The
// harness has emitted v1, v2, and v3 scorecards; the arm/producer fields the
// headroom rule reads are stable across all three.
const SchemaPrefix = "agentops-skill-probe."

// Classification is the headroom verdict over one probe's scorecard group.
type Classification string

const (
	// Saturated means the control arm aced the scenario at two or more effort
	// levels: no headroom. Retire the scenario, do not re-run it.
	Saturated Classification = "SATURATED"
	// Floor means the treatment arm never produced the act at any effort
	// level: the defect is below the calibration window or the discriminator
	// does not match the act.
	Floor Classification = "FLOOR"
	// Unmeasured means no usable treatment reps. The run did not happen; this
	// is never INERT.
	Unmeasured Classification = "UNMEASURED"
	// Separated means the control arm left room, so the probe's own verdict
	// reflects the skill rather than the scenario.
	Separated Classification = "SEPARATED"
)

// Arm is one measured arm of a probe (control or treatment). Rate is a
// pointer because the harness writes JSON null when an arm has no usable reps.
type Arm struct {
	Present int      `json:"present"`
	Usable  int      `json:"usable"`
	Rate    *float64 `json:"rate"`
}

// Producer identifies the configuration a scorecard was captured under. Older
// scorecards omit the block entirely.
type Producer struct {
	Model  string `json:"model"`
	Effort string `json:"effort"`
}

// Scorecard is the subset of an agentops-skill-probe.* scorecard the headroom
// rule reads. Unknown fields are ignored on purpose: this package must keep
// classifying v1 scorecards after the capture contract moves on.
type Scorecard struct {
	Schema    string   `json:"schema"`
	Probe     string   `json:"probe"`
	Skill     string   `json:"skill"`
	Producer  Producer `json:"producer"`
	Control   Arm      `json:"control"`
	Treatment Arm      `json:"treatment"`
	Verdict   string   `json:"verdict"`
	Path      string   `json:"-"`
}

// Result is the headroom classification of one probe's scorecard group.
type Result struct {
	Probe       string
	Skill       string
	Class       Classification
	AcedEfforts []string
	Detail      string
}

// HasHeadroom reports whether the scenario still had room to measure in. Only
// SATURATED answers no; FLOOR and UNMEASURED are separate diagnoses that the
// gate reports but does not treat as a saturated ceiling.
func (c Classification) HasHeadroom() bool { return c != Saturated }

// ExitCode maps a classification onto the process exit code the CLI helper
// uses, preserving the codes of the prior-art shell rule this ports:
// 0 SEPARATED, 3 SATURATED, 4 FLOOR, 5 UNMEASURED.
func (c Classification) ExitCode() int {
	switch c {
	case Saturated:
		return 3
	case Floor:
		return 4
	case Unmeasured:
		return 5
	default:
		return 0
	}
}

// EffortLabel returns the effort level a scorecard was captured at, or "?"
// when the scorecard predates the producer block. Two cards that both report
// "?" are one unknown level, never two — an unlabelled pair can never reach
// the two-level saturation bar by accident.
func (s Scorecard) EffortLabel() string {
	if strings.TrimSpace(s.Producer.Effort) == "" {
		return "?"
	}
	return s.Producer.Effort
}

// ParseScorecard decodes one scorecard and rejects anything outside the
// agentops-skill-probe.* family, so a stray JSON file in a scorecard
// directory fails loudly instead of classifying as an empty group.
func ParseScorecard(data []byte, path string) (Scorecard, error) {
	var card Scorecard
	if err := json.Unmarshal(data, &card); err != nil {
		return Scorecard{}, fmt.Errorf("parse scorecard %s: %w", path, err)
	}
	if !strings.HasPrefix(card.Schema, SchemaPrefix) {
		return Scorecard{}, fmt.Errorf("parse scorecard %s: schema %q is not %s*", path, card.Schema, SchemaPrefix)
	}
	if strings.TrimSpace(card.Probe) == "" {
		return Scorecard{}, fmt.Errorf("parse scorecard %s: empty probe id", path)
	}
	card.Path = path
	return card, nil
}

// LoadFiles reads the named scorecard files in argument order.
func LoadFiles(paths []string) ([]Scorecard, error) {
	cards := make([]Scorecard, 0, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(path) // #nosec G304 -- gate-supplied scorecard path, not user input.
		if err != nil {
			return nil, fmt.Errorf("read scorecard %s: %w", path, err)
		}
		card, err := ParseScorecard(data, path)
		if err != nil {
			return nil, err
		}
		cards = append(cards, card)
	}
	return cards, nil
}

// LoadDir reads every *.json scorecard under dir (recursively) and groups them
// by probe id. Group keys come back sorted so output is stable.
func LoadDir(dir string) (map[string][]Scorecard, []string, error) {
	groups := map[string][]Scorecard{}
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			return nil
		}
		data, readErr := os.ReadFile(path) // #nosec G304 -- gate-supplied scorecard path, not user input.
		if readErr != nil {
			return fmt.Errorf("read scorecard %s: %w", path, readErr)
		}
		card, parseErr := ParseScorecard(data, path)
		if parseErr != nil {
			return parseErr
		}
		groups[card.Probe] = append(groups[card.Probe], card)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	names := make([]string, 0, len(groups))
	for name := range groups {
		names = append(names, name)
	}
	sort.Strings(names)
	return groups, names, nil
}

// Classify applies the headroom rule to one probe's scorecard group.
func Classify(cards []Scorecard) (Result, error) {
	_ = cards
	return Result{}, ErrNotImplemented
}
