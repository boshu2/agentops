// practices: [ddd-bounded-context, mechanical-verify-before-judgment]

package canon

import (
	"time"
)

// Verdict is the outcome of an independent check of a learning.
type Verdict string

const (
	// VerdictConfirmed means the verifier independently checked the learning
	// and it holds.
	VerdictConfirmed Verdict = "confirmed"
	// VerdictRefuted means the verifier independently checked the learning and
	// it does NOT hold. A refuted entry must never be promoted.
	VerdictRefuted Verdict = "refuted"
)

// Verification records that an engineer independently checked a learning. It is
// the "true" signal — the part the warmind prototype lacked. Where a citation
// proves a learning was *useful*, a verification proves it was *checked*.
//
// The keystone lesson (acfs council refutation, 2026-05-31): a verification is
// only independent if the verifier GATHERED ITS OWN EVIDENCE — ran/read the
// thing and can cite what it actually saw — rather than grading a summary the
// author wrote. Receipt records that evidence (a path, hash, or gate line the
// verifier observed). A verification without a receipt is weak by construction;
// the promotion gate can require one.
type Verification struct {
	EntryID string    `json:"entry_id"`
	Path    string    `json:"path"`
	By      Identity  `json:"by"`
	At      time.Time `json:"at"`
	Verdict Verdict   `json:"verdict"`
	// Method names how the check was done: "manual", "ao-verify", "council",
	// "cross-model". Free-form; informational.
	Method string `json:"method,omitempty"`
	// Receipt is the evidence the verifier independently gathered (e.g. a gate
	// log path, an evidence hash, a file:line they read). Empty = unreceipted.
	Receipt string `json:"receipt,omitempty"`
	// Self is true when the verifying engineer authored the entry.
	// Self-verifications are recorded for provenance but never count toward
	// promotion — the anti-self-certification rule applied to verification.
	Self bool `json:"self"`
}

// VerificationLedger is the append-only JSONL record of verification events.
type VerificationLedger struct{ *ledger[Verification] }

// NewVerificationLedger opens (without creating) the verification ledger at path.
func NewVerificationLedger(path string) *VerificationLedger {
	return &VerificationLedger{newLedger[Verification](path)}
}

// Record appends a verification for entryID at path by the current engineer,
// stamping Self by comparing the verifier against the entry's author.
func (l *VerificationLedger) Record(entryID, path, method, receipt string, verdict Verdict, by Identity, now time.Time) (Verification, error) {
	v := Verification{
		EntryID: entryID,
		Path:    path,
		By:      by,
		At:      now,
		Verdict: verdict,
		Method:  method,
		Receipt: receipt,
		Self:    by.SameAs(AuthorOf(path)),
	}
	return v, l.append(v)
}

// ForEntry returns every verification recorded against entryID.
func (l *VerificationLedger) ForEntry(entryID string) ([]Verification, error) {
	all, err := l.load()
	if err != nil {
		return nil, err
	}
	var out []Verification
	for _, v := range all {
		if v.EntryID == entryID {
			out = append(out, v)
		}
	}
	return out, nil
}

// Refuted reports whether any independent (non-self) verifier refuted entryID.
// A single independent refutation blocks promotion regardless of confirmations.
func (l *VerificationLedger) Refuted(entryID string) (bool, error) {
	vers, err := l.ForEntry(entryID)
	if err != nil {
		return false, err
	}
	for _, v := range vers {
		if !v.Self && v.Verdict == VerdictRefuted {
			return true, nil
		}
	}
	return false, nil
}

// AllEntryIDs returns the distinct entry IDs that appear in the verification ledger.
func (l *VerificationLedger) AllEntryIDs() ([]string, error) {
	all, err := l.load()
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(all))
	for _, v := range all {
		ids = append(ids, v.EntryID)
	}
	return distinctStrings(ids), nil
}

// IndependentConfirmations counts distinct non-author engineers who confirmed
// entryID. When requireReceipt is set, only verifications carrying a receipt
// (evidence the verifier independently gathered) are counted.
func (l *VerificationLedger) IndependentConfirmations(entryID string, requireReceipt bool) (int, error) {
	vers, err := l.ForEntry(entryID)
	if err != nil {
		return 0, err
	}
	var distinct []Identity
	for _, v := range vers {
		if v.Self || v.Verdict != VerdictConfirmed {
			continue
		}
		if requireReceipt && v.Receipt == "" {
			continue
		}
		if !containsIdentity(distinct, v.By) {
			distinct = append(distinct, v.By)
		}
	}
	return len(distinct), nil
}
