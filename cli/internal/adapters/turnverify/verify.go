// practices: [design-by-contract, in-toto-provenance]
package turnverify

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/boshu2/agentops/cli/internal/evidencedturn"
	"github.com/boshu2/agentops/cli/internal/provenancegraph"
	"github.com/boshu2/agentops/cli/internal/turnstate"
)

// Options configures the Evidenced-Turn verification adapter.
type Options struct {
	BeadID     string
	InputPath  string
	LedgerPath string
	GraphPath  string
	JSON       bool
	AllowSelf  bool
	Stdout     io.Writer
}

// InputFile is the on-disk shape consumed by `ao turn verify --input`. It
// carries the facts that are NOT in the committed provenance ledger: the bead's
// state_transition log and its Closes-scenario coverage. Provenance edges and
// orphans are read from the ledger (or --ledger) so they stay the audit
// authority, not a hand-supplied claim.
type InputFile struct {
	BeadID      string                   `json:"bead_id"`
	Transitions []turnstate.Transition   `json:"transitions"`
	Scenarios   []evidencedturn.Scenario `json:"scenarios"`
	// AuthorID and JudgeID carry the no-self-grading invariant (ag-lmdx.4):
	// the identity of the context that AUTHORED the artifact vs. the context
	// that PRODUCED the acceptance verdict. The author_neq_validator predicate
	// fails when they are equal (a self-graded, autocorrelated verdict) unless
	// AllowSelf waives it. Empty judge_id also fails: independence that was
	// never recorded cannot be asserted.
	AuthorID string `json:"author_id,omitempty"`
	JudgeID  string `json:"judge_id,omitempty"`
}

// Run evaluates one bead's Evidenced-Turn Definition-of-Done and writes either
// a legible checklist or the JSON verdict to Stdout.
func Run(opts Options) error {
	beadID := strings.TrimSpace(opts.BeadID)
	if beadID == "" {
		return fmt.Errorf("a non-empty <bead> is required")
	}
	if opts.InputPath == "" {
		return fmt.Errorf("ao turn verify requires --input <turn-input.json>")
	}

	tf, err := ReadInputFile(opts.InputPath)
	if err != nil {
		return err
	}
	// The positional bead is authoritative; an input file declaring a different
	// bead is a mistake worth surfacing rather than silently overriding.
	if tf.BeadID != "" && tf.BeadID != beadID {
		return fmt.Errorf("input file bead_id %q does not match <bead> %q", tf.BeadID, beadID)
	}

	edges, err := ReadLedgerEdges(opts.LedgerPath)
	if err != nil {
		return err
	}

	var orphans []provenancegraph.OrphanFinding
	orphanChecked := false
	if opts.GraphPath != "" {
		graph, gerr := provenancegraph.ReadGraphRecords(opts.GraphPath)
		if gerr != nil {
			return fmt.Errorf("read provenance trace-graph: %w", gerr)
		}
		// Any orphan in the turn's trace-graph means its provenance is
		// incomplete: a turn with a dangling artifact is not provably done.
		orphans = provenancegraph.FindOrphans(graph)
		orphanChecked = true
	}

	v, err := evidencedturn.Evaluate(evidencedturn.Input{
		BeadID:          beadID,
		Transitions:     tf.Transitions,
		Scenarios:       tf.Scenarios,
		ProvenanceEdges: edges,
		OrphanFindings:  orphans,
		OrphanChecked:   orphanChecked,
		AuthorID:        tf.AuthorID,
		JudgeID:         tf.JudgeID,
		AllowSelf:       opts.AllowSelf,
	})
	if err != nil {
		return err
	}

	if err := RenderVerdict(opts.Stdout, v, opts.JSON); err != nil {
		return err
	}
	if !v.Done {
		// Non-zero exit makes this usable as a validated->closed transition
		// guard.
		return fmt.Errorf("turn %s is NOT done: %d gap(s)", v.BeadID, len(v.Gaps))
	}
	return nil
}

// ReadInputFile loads and decodes the turn-input JSON file, rejecting unknown
// fields so a malformed contract fails loudly.
func ReadInputFile(path string) (InputFile, error) {
	b, err := os.ReadFile(path) // #nosec G304 -- operator-supplied input path, same trust model as --graph
	if err != nil {
		return InputFile{}, fmt.Errorf("read turn-input file %q: %w", path, err)
	}
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	var tf InputFile
	if err := dec.Decode(&tf); err != nil {
		return InputFile{}, fmt.Errorf("parse turn-input file %q: %w", path, err)
	}
	return tf, nil
}

// ReadLedgerEdges reads the provenance ledger edges. A missing ledger is not a
// hard error here: the evidencedturn evaluator will simply fail the
// provenance_event predicate with a legible reason rather than crash.
func ReadLedgerEdges(path string) ([]provenancegraph.Edge, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("stat provenance ledger %q: %w", path, err)
	}
	store := provenancegraph.NewStore(path)
	edges, err := store.Read()
	if err != nil {
		return nil, fmt.Errorf("read provenance ledger: %w", err)
	}
	return edges, nil
}

// RenderVerdict prints the legible checklist or JSON. The text form is the
// agent-facing default: a status glyph, the predicate, and its reason.
func RenderVerdict(out io.Writer, v evidencedturn.Verdict, asJSON bool) error {
	if out == nil {
		out = io.Discard
	}
	if asJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	}

	fmt.Fprintf(out, "Evidenced-Turn DoD for %s\n", v.BeadID)
	for _, p := range v.Predicates {
		glyph := "FAIL"
		if p.Passed {
			glyph = "PASS"
		}
		fmt.Fprintf(out, "  [%s] %-18s %s\n", glyph, p.Name, p.Reason)
	}
	fmt.Fprintln(out)
	if v.Done {
		fmt.Fprintf(out, "VERDICT: DONE — validated->closed is legal for %s\n", v.BeadID)
	} else {
		fmt.Fprintf(out, "VERDICT: NOT DONE — %d gap(s):\n", len(v.Gaps))
		for _, g := range v.Gaps {
			fmt.Fprintf(out, "  - %s\n", g)
		}
	}
	return nil
}
