// practices: [ddd-bounded-context, anti-self-certification]

package canon

import "fmt"

// Gate is the earned-promotion policy. An entry joins the team canon only when
// it clears every requirement. Defaults encode the thesis minimum: at least one
// cross-engineer citation AND at least one independent confirmation.
type Gate struct {
	// MinCitations is the required count of distinct non-author engineers who
	// cited the entry (proves useful).
	MinCitations int
	// MinVerifications is the required count of distinct non-author engineers
	// who confirmed the entry (proves checked-true).
	MinVerifications int
	// RequireReceipt, when set, counts only verifications whose verifier
	// recorded the evidence they independently gathered.
	RequireReceipt bool
}

// DefaultGate is the thesis minimum: one of each independent signal.
func DefaultGate() Gate {
	return Gate{MinCitations: 1, MinVerifications: 1, RequireReceipt: false}
}

// Decision is the gate's verdict for one entry, with the reasons any
// requirement was unmet so the CLI can tell an engineer exactly what is missing.
type Decision struct {
	EntryID       string   `json:"entry_id"`
	Eligible      bool     `json:"eligible"`
	Tier          Tier     `json:"tier,omitempty"`
	Citations     int      `json:"citations"`
	Verifications int      `json:"verifications"`
	Refuted       bool     `json:"refuted"`
	Unmet         []string `json:"unmet,omitempty"`
}

// Evaluate decides whether entryID may be promoted under this gate, reading the
// two ledgers. An independent refutation is a hard block: a refuted entry is
// never eligible, regardless of how many citations or confirmations it carries.
func (g Gate) Evaluate(entryID string, cl *CitationLedger, vl *VerificationLedger) (Decision, error) {
	cites, err := cl.CrossEngineerCitations(entryID)
	if err != nil {
		return Decision{}, fmt.Errorf("count citations: %w", err)
	}
	confs, err := vl.IndependentConfirmations(entryID, g.RequireReceipt)
	if err != nil {
		return Decision{}, fmt.Errorf("count verifications: %w", err)
	}
	refuted, err := vl.Refuted(entryID)
	if err != nil {
		return Decision{}, fmt.Errorf("check refutation: %w", err)
	}

	d := Decision{
		EntryID:       entryID,
		Citations:     cites,
		Verifications: confs,
		Refuted:       refuted,
	}
	if refuted {
		d.Unmet = append(d.Unmet, "independently refuted — cannot be promoted")
	}
	if cites < g.MinCitations {
		d.Unmet = append(d.Unmet, fmt.Sprintf("needs %d cross-engineer citation(s), has %d", g.MinCitations, cites))
	}
	if confs < g.MinVerifications {
		receipt := ""
		if g.RequireReceipt {
			receipt = " with receipt"
		}
		d.Unmet = append(d.Unmet, fmt.Sprintf("needs %d independent confirmation(s)%s, has %d", g.MinVerifications, receipt, confs))
	}
	d.Eligible = len(d.Unmet) == 0
	return d, nil
}
