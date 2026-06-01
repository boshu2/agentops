package contextpacket

import "fmt"

// ExplainPayloadHealth describes the overall health of a context explain payload.
type ExplainPayloadHealth struct {
	Status        string `json:"status"`
	SelectedCount int    `json:"selected_count"`
	Reason        string `json:"reason"`
}

// ExplainFamilyHealth describes the health of a single artifact family.
type ExplainFamilyHealth struct {
	Family string `json:"family"`
	Count  int    `json:"count"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}

// ExplainSelection describes a single selected context artifact.
type ExplainSelection struct {
	Class      string `json:"class"`
	Title      string `json:"title"`
	Reason     string `json:"reason"`
	SourcePath string `json:"source_path,omitempty"`
}

// ExplainSuppression describes a suppressed artifact class.
type ExplainSuppression struct {
	Class  string `json:"class"`
	Reason string `json:"reason"`
	Count  int    `json:"count,omitempty"`
}

// ExplainPayload evaluates the health of a set of selected context artifacts.
func ExplainPayload(selected []ExplainSelection) ExplainPayloadHealth {
	count := len(selected)
	switch {
	case count == 0:
		return ExplainPayloadHealth{Status: "empty", SelectedCount: 0, Reason: "No ranked artifacts matched the current query and phase."}
	case count < 4:
		return ExplainPayloadHealth{Status: "thin", SelectedCount: count, Reason: "Payload is present but thin; manual review recommended before trusting it as the only runtime context."}
	default:
		return ExplainPayloadHealth{Status: "healthy", SelectedCount: count, Reason: "Payload has enough ranked coverage to explain current startup or briefing context."}
	}
}

// DescribeContextFamily classifies the health of an artifact family by count.
func DescribeContextFamily(name string, count int, experimental bool) ExplainFamilyHealth {
	status := "healthy"
	reason := "Family has enough artifacts to participate without additional warnings."

	switch {
	case count == 0:
		status = "missing"
		reason = "No artifacts are available from this family in the current workspace."
	case count < 3:
		status = "thin"
		reason = "Coverage is thin; manual review is recommended before treating this family as strong runtime context."
	}

	if experimental {
		if count == 0 {
			return ExplainFamilyHealth{Family: name, Count: count, Status: "missing", Reason: "Experimental family has no artifacts in this workspace."}
		}
		if count < 3 {
			return ExplainFamilyHealth{Family: name, Count: count, Status: "manual_review", Reason: "Experimental family is thin and stays suppressed from default startup payloads."}
		}
		return ExplainFamilyHealth{Family: name, Count: count, Status: "experimental", Reason: "Experimental family is available but remains out of default startup injection until health gates harden."}
	}

	return ExplainFamilyHealth{Family: name, Count: count, Status: status, Reason: reason}
}

// ProofBackedNextWorkReason returns a human-readable reason for a proof-backed next-work item.
func ProofBackedNextWorkReason(source, detail string) string {
	sourceLabel := proofSourceLabel(source)
	if detail == "" {
		return fmt.Sprintf("Proof-backed next-work completion via %s proof.", sourceLabel)
	}
	return fmt.Sprintf("Proof-backed next-work completion via %s proof (%s).", sourceLabel, detail)
}

// ProofBackedNextWorkSuppressionReason returns a human-readable reason for suppressed proof-backed items.
func ProofBackedNextWorkSuppressionReason(source string, count int, detail string) string {
	sourceLabel := proofSourceLabel(source)

	plural := "item"
	if count != 1 {
		plural = "items"
	}
	if detail == "" {
		return fmt.Sprintf("Proof-backed next-work completion suppressed %d %s via %s proof.", count, plural, sourceLabel)
	}
	return fmt.Sprintf("Proof-backed next-work completion suppressed %d %s via %s proof (%s).", count, plural, sourceLabel, detail)
}

func proofSourceLabel(source string) string {
	label := map[string]string{
		"completed_run":         "completed-run",
		"evidence_only_closure": "evidence-only-closure",
		"execution_packet":      "execution-packet",
	}[source]
	if label == "" {
		return source
	}
	return label
}
