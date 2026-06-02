package background

import "strings"

var eligibleLabels = []string{"background-agent-safe", "background_eligible", "managed_eligible"}

var excludedLabels = []string{
	"holdout",
	"evaluator",
	"eval",
	"pii",
	"secret",
	"human",
	"operator-gated",
	"operator",
}

type Candidate struct {
	ID       string         `json:"id"`
	Title    string         `json:"title,omitempty"`
	Priority int            `json:"priority,omitempty"`
	Type     string         `json:"issue_type,omitempty"`
	Labels   []string       `json:"labels,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type EligibilityDecision struct {
	Candidate Candidate `json:"candidate"`
	Eligible  bool      `json:"eligible"`
	Reasons   []string  `json:"reasons"`
}

func FilterEligible(candidates []Candidate) []EligibilityDecision {
	out := make([]EligibilityDecision, 0, len(candidates))
	for _, c := range candidates {
		out = append(out, Decide(c))
	}
	return out
}

func Decide(c Candidate) EligibilityDecision {
	decision := EligibilityDecision{Candidate: c, Eligible: true}
	if strings.TrimSpace(c.ID) == "" {
		decision.Eligible = false
		decision.Reasons = append(decision.Reasons, "missing id")
	}
	if !hasAnyLabel(c.Labels, eligibleLabels) && !metadataBool(c.Metadata, "background_eligible") {
		decision.Eligible = false
		decision.Reasons = append(decision.Reasons, "missing background eligibility label/metadata")
	}
	for _, label := range c.Labels {
		if matchesExcludedLabel(label) {
			decision.Eligible = false
			decision.Reasons = append(decision.Reasons, "excluded label: "+label)
		}
	}
	for _, key := range []string{"holdout", "evaluator", "pii", "operator_gated"} {
		if metadataBool(c.Metadata, key) {
			decision.Eligible = false
			decision.Reasons = append(decision.Reasons, "excluded metadata: "+key)
		}
	}
	if len(decision.Reasons) == 0 {
		decision.Reasons = append(decision.Reasons, "eligible")
	}
	return decision
}

func hasAnyLabel(labels, needles []string) bool {
	for _, label := range labels {
		norm := normalize(label)
		for _, needle := range needles {
			if norm == normalize(needle) {
				return true
			}
		}
	}
	return false
}

func matchesExcludedLabel(label string) bool {
	norm := normalize(label)
	for _, excluded := range excludedLabels {
		ex := normalize(excluded)
		if norm == ex || strings.Contains(norm, ex) {
			return true
		}
	}
	return false
}

func metadataBool(metadata map[string]any, key string) bool {
	if metadata == nil {
		return false
	}
	for k, v := range metadata {
		if normalize(k) != normalize(key) {
			continue
		}
		switch typed := v.(type) {
		case bool:
			return typed
		case string:
			return normalize(typed) == "true" || normalize(typed) == "yes" || normalize(typed) == "1"
		}
	}
	return false
}

func normalize(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.ReplaceAll(s, "-", "_")
	return s
}
