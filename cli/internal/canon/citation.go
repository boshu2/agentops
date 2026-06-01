// practices: [ddd-bounded-context, knowledge-flywheel]

package canon

import (
	"time"
)

// Citation records that an engineer used a learning — surfaced it during work
// and applied it. It is the "useful" signal: someone other than the author got
// value from the knowledge.
//
// Adapted from the warmind prototype (Doug Dan, PR #671): the load-bearing
// field is Self, which marks citations by the entry's own author so the
// promotion gate can exclude them.
type Citation struct {
	EntryID   string    `json:"entry_id"`
	Path      string    `json:"path"`
	By        Identity  `json:"by"`
	At        time.Time `json:"at"`
	Query     string    `json:"query,omitempty"`
	SessionID string    `json:"session_id,omitempty"`
	// Self is true when the citing engineer authored the entry. Self-citations
	// are recorded for provenance but never count toward promotion.
	Self bool `json:"self"`
}

// CitationLedger is the append-only JSONL record of citation events.
type CitationLedger struct{ *ledger[Citation] }

// NewCitationLedger opens (without creating) the citation ledger at path.
func NewCitationLedger(path string) *CitationLedger {
	return &CitationLedger{newLedger[Citation](path)}
}

// Record appends a citation for entryID at path by the current engineer,
// stamping Self by comparing the citer against the entry's author. The clock
// is injected so callers (and tests) control timestamps.
func (l *CitationLedger) Record(entryID, path, query, sessionID string, by Identity, now time.Time) (Citation, error) {
	c := Citation{
		EntryID:   entryID,
		Path:      path,
		By:        by,
		At:        now,
		Query:     query,
		SessionID: sessionID,
		Self:      by.SameAs(AuthorOf(path)),
	}
	return c, l.append(c)
}

// ForEntry returns every citation recorded against entryID.
func (l *CitationLedger) ForEntry(entryID string) ([]Citation, error) {
	all, err := l.load()
	if err != nil {
		return nil, err
	}
	var out []Citation
	for _, c := range all {
		if c.EntryID == entryID {
			out = append(out, c)
		}
	}
	return out, nil
}

// CrossEngineerCitations counts distinct non-author engineers who cited
// entryID. Distinctness uses Identity.SameAs so the same person citing twice
// (or from two workspaces) cannot manufacture promotion eligibility.
func (l *CitationLedger) CrossEngineerCitations(entryID string) (int, error) {
	cites, err := l.ForEntry(entryID)
	if err != nil {
		return 0, err
	}
	var distinct []Identity
	for _, c := range cites {
		if c.Self {
			continue
		}
		if !containsIdentity(distinct, c.By) {
			distinct = append(distinct, c.By)
		}
	}
	return len(distinct), nil
}

// AllEntryIDs returns the distinct entry IDs that appear in the citation ledger.
func (l *CitationLedger) AllEntryIDs() ([]string, error) {
	all, err := l.load()
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(all))
	for _, c := range all {
		ids = append(ids, c.EntryID)
	}
	return distinctStrings(ids), nil
}

func containsIdentity(ids []Identity, target Identity) bool {
	for _, id := range ids {
		if id.SameAs(target) {
			return true
		}
	}
	return false
}
