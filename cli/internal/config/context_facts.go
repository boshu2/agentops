package config

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// ContextFact is the versioned JSON text of one native maintenance comment.
// Fact identity survives investigation closure/deletion and knowledge Git rollback.
// Resolutions cite the withdrawal, its original digest, an exact fresh review,
// and an explicitly covered successor digest. Parsing does not judge the review.
type ContextFact struct {
	Type            string         `json:"type"`
	FactID          string         `json:"fact_id"`
	BundleID        string         `json:"bundle_id,omitempty"`
	PageID          string         `json:"page_id,omitempty"`
	PageDigest      string         `json:"page_digest,omitempty"`
	Counterevidence []string       `json:"counterevidence,omitempty"`
	ResolvesFactID  string         `json:"resolves_fact_id,omitempty"`
	ReviewRef       string         `json:"review_ref,omitempty"`
	ReviewDigest    string         `json:"review_digest,omitempty"`
	SuccessorDigest string         `json:"successor_digest,omitempty"`
	Route           *ContextConfig `json:"route,omitempty"`
}

func ParseContextFacts(comments []AnchorComment, anchor string) ([]ContextFact, error) {
	facts := []ContextFact{}
	seen := map[string]bool{}
	commentIDs := map[string]bool{}
	for _, comment := range comments {
		if comment.ID == "" || comment.IssueID != anchor || commentIDs[comment.ID] {
			return nil, fmt.Errorf("incomplete, duplicate or wrong-anchor comment response")
		}
		commentIDs[comment.ID] = true
		text := strings.TrimSpace(comment.Text)
		if !strings.HasPrefix(text, "{") {
			continue
		}
		var header struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal([]byte(text), &header); err != nil {
			return nil, fmt.Errorf("malformed structured anchor comment")
		}
		if !strings.HasPrefix(header.Type, "context.") {
			continue
		}
		var fact ContextFact
		if err := decodeContextObject([]byte(text), &fact, []string{"type", "fact_id"}, []string{"bundle_id", "page_id", "page_digest", "counterevidence", "resolves_fact_id", "review_ref", "review_digest", "successor_digest", "route"}); err != nil {
			return nil, fmt.Errorf("invalid context fact: %w", err)
		}
		if fact.FactID == "" || seen[fact.FactID] {
			return nil, fmt.Errorf("missing or duplicate context fact ID")
		}
		seen[fact.FactID] = true
		if err := validateContextFact(fact, anchor); err != nil {
			return nil, err
		}

		facts = append(facts, fact)
	}
	return facts, nil
}
func contextDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil && value == strings.ToLower(value)
}

func validateContextFact(fact ContextFact, anchor string) error {
	switch fact.Type {
	case "context.route.v1":
		if fact.Route == nil || fact.Route.MaintenanceWorkRef != anchor {
			return fmt.Errorf("route fact must preserve this maintenance anchor")
		}
	case "context.withdrawal.v1":
		if fact.BundleID == "" || fact.PageID == "" || !contextDigest(fact.PageDigest) || len(fact.Counterevidence) == 0 {
			return fmt.Errorf("incomplete withdrawal fact")
		}
		for _, ref := range fact.Counterevidence {
			if strings.TrimSpace(ref) == "" {
				return fmt.Errorf("empty withdrawal counterevidence")
			}
		}
	case "context.resolution.v1":
		if fact.BundleID == "" || fact.PageID == "" || !contextDigest(fact.PageDigest) || fact.ResolvesFactID == "" || fact.ReviewRef == "" || !contextDigest(fact.ReviewDigest) || !contextDigest(fact.SuccessorDigest) {
			return fmt.Errorf("incomplete resolution fact")
		}
	default:
		return fmt.Errorf("unsupported context fact type")
	}
	return nil
}
