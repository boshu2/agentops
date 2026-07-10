package provenancegraph

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

// VerifyResult is the structured outcome of verifying a committed ledger file
// in place. It reports whether the whole chain is intact and, on the first
// break, the 1-based FILE LINE number of the offending record (counting blank
// lines, so the number maps directly to what an editor shows) plus a
// descriptive message.
type VerifyResult struct {
	// Pass is true only when every record is field-valid AND the hash chain is
	// fully intact.
	Pass bool
	// RecordCount is the number of non-blank edge records read.
	RecordCount int
	// FirstBrokenLine is the 1-based file line of the first broken record, or 0
	// when Pass is true.
	FirstBrokenLine int
	// Message describes the first break (empty when Pass is true).
	Message string
}

// VerifyFile reads the ledger file at s.Path and verifies it in place: every
// non-blank line must parse as a schema-valid edge AND the per-record hash
// chain must be intact (prev_hash links to the prior record's hash, and
// payload_hash/hash recompute). Unlike export --verify, this does NOT re-chain
// or re-sort — it checks the committed bytes exactly as they sit on disk, so a
// tampered field, a forged hash, or a reordered row is caught and the offending
// FILE LINE is named.
//
// A missing file is treated as an intact empty ledger (Pass=true, count 0) so a
// fresh clone with no events does not fail the gate. A malformed JSON line is a
// hard break naming that line.
func (s *Store) VerifyFile() (VerifyResult, error) {
	f, err := os.Open(s.Path)
	if os.IsNotExist(err) {
		return VerifyResult{Pass: true}, nil
	}
	if err != nil {
		return VerifyResult{}, fmt.Errorf("open ledger: %w", err)
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	prevHash := ""
	lineNo := 0
	count := 0
	for scanner.Scan() {
		lineNo++
		raw := scanner.Bytes()
		if len(trimSpace(raw)) == 0 {
			continue
		}
		var e Edge
		if err := json.Unmarshal(raw, &e); err != nil {
			return VerifyResult{
				Pass:            false,
				RecordCount:     count,
				FirstBrokenLine: lineNo,
				Message:         fmt.Sprintf("invalid JSON: %v", err),
			}, nil
		}
		count++

		if verr := ValidateFields(e); verr != nil {
			return VerifyResult{
				Pass:            false,
				RecordCount:     count,
				FirstBrokenLine: lineNo,
				Message:         verr.Error(),
			}, nil
		}
		if e.PrevHash != prevHash {
			return VerifyResult{
				Pass:            false,
				RecordCount:     count,
				FirstBrokenLine: lineNo,
				Message:         fmt.Sprintf("prev_hash mismatch: got %q want %q", e.PrevHash, prevHash),
			}, nil
		}
		payloadHash, hash, herr := ComputeHashes(e)
		if herr != nil {
			return VerifyResult{
				Pass:            false,
				RecordCount:     count,
				FirstBrokenLine: lineNo,
				Message:         herr.Error(),
			}, nil
		}
		if e.PayloadHash != payloadHash {
			return VerifyResult{
				Pass:            false,
				RecordCount:     count,
				FirstBrokenLine: lineNo,
				Message:         payloadHashSkewHint,
			}, nil
		}
		if e.Hash != hash {
			return VerifyResult{
				Pass:            false,
				RecordCount:     count,
				FirstBrokenLine: lineNo,
				Message:         "hash mismatch (chain anchor was altered)",
			}, nil
		}
		prevHash = e.Hash
	}
	if serr := scanner.Err(); serr != nil {
		return VerifyResult{}, fmt.Errorf("scan ledger: %w", serr)
	}

	return VerifyResult{Pass: true, RecordCount: count}, nil
}
