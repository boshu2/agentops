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
// The rule is the deterministic half of skill measurement: it never judges a
// skill, only whether the scenario left room to judge it in. Prior art is the
// shell rule recovered from the 2026-08-08 clean-room package; the thresholds
// and exit codes are preserved so a scorecard classified there classifies the
// same way here.
package probeheadroom

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

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
	// Root-scoped reads: every open resolves inside dir, so a symlink swapped
	// in mid-walk cannot escape the scorecard tree (gosec G122 / CWE-367).
	root, rootErr := os.OpenRoot(dir)
	if rootErr != nil {
		return nil, nil, fmt.Errorf("open scorecard root %s: %w", dir, rootErr)
	}
	defer func() { _ = root.Close() }() // read-only root; close error carries no data loss
	groups := map[string][]Scorecard{}
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			return nil
		}
		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return fmt.Errorf("scorecard path %s outside %s: %w", path, dir, relErr)
		}
		f, openErr := root.Open(rel)
		if openErr != nil {
			return fmt.Errorf("read scorecard %s: %w", path, openErr)
		}
		data, readErr := io.ReadAll(f)
		if closeErr := f.Close(); readErr == nil && closeErr != nil {
			readErr = closeErr
		}
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

// value returns an arm's rate, treating a JSON null as 0.0. A null rate only
// occurs with zero usable reps, which the UNMEASURED branch catches first.
func (a Arm) value() float64 {
	if a.Rate == nil {
		return 0
	}
	return *a.Rate
}

// Classify applies the headroom rule to one probe's scorecard group. Every
// card must belong to the same probe: two scenarios cannot share one headroom
// verdict, and silently merging them would let a saturated scenario hide
// behind an unsaturated one.
//
// The branches are ordered by what each one invalidates:
//
//  1. UNMEASURED — no usable treatment reps anywhere. The run did not happen,
//     so no statement about headroom is available. This outranks a
//     saturated-looking control arm, because a control arm measured against
//     nothing measured nothing.
//  2. SATURATED — the control arm reached ControlCeiling at two or more
//     distinct effort levels, each with at least MinUsableReps usable control
//     reps. There was no room for the treatment arm to differ. Retire the
//     scenario (promote it to a seeded-defect probe, or record the honest
//     ceiling finding); re-running it at a lower effort adds ledger rows, not
//     knowledge.
//  3. FLOOR — every card has usable treatment reps and the treatment arm never
//     produced the act at any level. Either the defect is below the
//     calibration window or the discriminator does not match the act.
//  4. SEPARATED — the control arm left room, so the probe's own verdict
//     reflects the skill rather than the scenario.
func Classify(cards []Scorecard) (Result, error) {
	if len(cards) == 0 {
		return Result{}, errors.New("probeheadroom: no scorecards to classify")
	}

	probe := cards[0].Probe
	skill := cards[0].Skill
	for _, card := range cards[1:] {
		if card.Probe != probe {
			return Result{}, fmt.Errorf("probeheadroom: mixed probe ids in one group: %q and %q", probe, card.Probe)
		}
	}

	res := Result{Probe: probe, Skill: skill}

	acedSet := map[string]struct{}{}
	anyUsableTreatment := false
	allTreatmentUsable := true
	allTreatmentSilent := true
	for _, card := range cards {
		if card.Treatment.Usable > 0 {
			anyUsableTreatment = true
		} else {
			allTreatmentUsable = false
		}
		if card.Treatment.value() != 0 {
			allTreatmentSilent = false
		}
		if card.Control.Usable >= MinUsableReps && card.Control.value() >= ControlCeiling {
			acedSet[card.EffortLabel()] = struct{}{}
		}
	}
	for effort := range acedSet {
		res.AcedEfforts = append(res.AcedEfforts, effort)
	}
	sort.Strings(res.AcedEfforts)

	switch {
	case !anyUsableTreatment:
		res.Class = Unmeasured
		res.Detail = "no usable treatment reps; the run did not happen, so this is not INERT"
	case len(res.AcedEfforts) >= 2:
		res.Class = Saturated
		res.Detail = fmt.Sprintf("control arm >= %.2f at %d effort levels (%s); retire the scenario rather than re-running it",
			ControlCeiling, len(res.AcedEfforts), strings.Join(res.AcedEfforts, ", "))
	case allTreatmentUsable && allTreatmentSilent:
		res.Class = Floor
		res.Detail = "the treatment arm never produced the act at any effort level; check the discriminator against a hand-written passing transcript before re-seeding"
	case len(res.AcedEfforts) == 1 && distinctEffortLevels(cards) < 2:
		// L2 finding (2026-08-26): one measured level whose control aced it is
		// not headroom — the SEPARATED label would be an artifact of the
		// missing second level. Tightening only: fewer groups pass pre-screen.
		res.Class = Unmeasured
		res.Detail = "single effort level with an aced control arm; capture a second level before any verdict row"
	default:
		res.Class = Separated
		res.Detail = fmt.Sprintf("control aced %d effort level(s); the verdict reflects the skill, not the scenario", len(res.AcedEfforts))
	}
	return res, nil
}

func distinctEffortLevels(cards []Scorecard) int {
	levels := map[string]struct{}{}
	for _, card := range cards {
		levels[card.EffortLabel()] = struct{}{}
	}
	return len(levels)
}
