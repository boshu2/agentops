// practices: [design-by-contract, in-toto-provenance]
package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/parser"
)

// MineEventSchemaVersion is the schema version for mined per-inference events.
const MineEventSchemaVersion = "agentops-provenance-mine-event.v1"

// toolResultMessageType is the parser's message Type for a tool OUTPUT row (both
// Claude top-level tool_result and Codex function_call_output). It is the
// family-agnostic discriminator that an emitted row is a result, not a call.
const toolResultMessageType = "tool_result"

// MineEvent is one deterministic per-inference provenance event mined from a
// session transcript (E6, ADR-0010). Only events the transcript DETERMINISTICALLY
// evidences are emitted; today that is tool_call (a tool_use block). The Kind
// enum is extensible to context_entered / context_missed once their deterministic
// evidence is defined (those are NOT inferred speculatively).
type MineEvent struct {
	SchemaVersion string `json:"schema_version"`
	Kind          string `json:"kind"` // tool_call (extensible: context_entered, context_missed)
	ID            string `json:"id"`   // stable: sha256(session:line:kind:tool)[:16] — idempotent
	SessionID     string `json:"session_id"`
	SourceLine    int    `json:"source_line"`
	Tool          string `json:"tool,omitempty"`
}

// mineState is the incremental watermark for one session file. last_line is the
// highest source line already mined; prefix_checksum is the checksum of the
// parsed content up to and including last_line, so a truncated/rewritten prefix
// (rollback) is detected and forces a clean re-mine.
type mineState struct {
	File           string `json:"file"`
	LastLine       int    `json:"last_line"`
	PrefixChecksum string `json:"prefix_checksum"`
	MinedCount     int    `json:"mined_count"`
}

var (
	mineFile  string
	mineState_ string
	mineJSON  bool
)

var provenanceMineSessionCmd = &cobra.Command{
	Use:   "mine-session --file <session.jsonl>",
	Short: "Mine deterministic per-inference provenance events from a session transcript",
	Long: `Parse a Claude Code or Codex session transcript and emit the per-inference
provenance events it DETERMINISTICALLY evidences (E6, ADR-0010: build-native, own
the PROV-O graph). Today that is one tool_call event per tool use, with a stable
idempotent id; the Kind enum is extensible to context_entered/context_missed once
their deterministic evidence is defined (never inferred speculatively).

Incremental (--state): re-running mines only NEW lines (skip-consumed), keyed by
a watermark + a prefix checksum. If the transcript's already-mined prefix changed
(truncated/rewritten), the watermark is invalid and the whole file is re-mined
(rollback) — borrowed from cass's incremental-index discipline (stale-is-usable,
recover loudly, never rebuild expensive state unnecessarily).

Output (--json, default): one JSON event per line on stdout. The events feed the
PROV-O graph via a downstream step (e.g. wired as an ASSAY --mine-cmd); this
command does not itself write the committed ledger.`,
	RunE: runProvenanceMineSession,
}

func init() {
	provenanceMineSessionCmd.Flags().StringVar(&mineFile, "file", "", "Path to the session transcript (.jsonl) to mine (required)")
	provenanceMineSessionCmd.Flags().StringVar(&mineState_, "state", "", "Path to the incremental watermark state JSON (created/updated; omit for a full one-shot mine)")
	provenanceMineSessionCmd.Flags().BoolVar(&mineJSON, "json", true, "Emit events as JSONL on stdout")
	provenanceCmd.AddCommand(provenanceMineSessionCmd)
}

func runProvenanceMineSession(cmd *cobra.Command, _ []string) error {
	if mineFile == "" {
		return fmt.Errorf("mine-session: --file is required")
	}
	if _, err := os.Stat(mineFile); err != nil {
		return fmt.Errorf("mine-session: cannot read --file %s: %w", mineFile, err)
	}

	p := parser.NewParser()
	result, err := p.ParseFile(mineFile)
	if err != nil {
		return fmt.Errorf("mine-session: parse %s: %w", mineFile, err)
	}

	sessionID := sessionIDFromPath(mineFile)

	// --- incremental watermark + rollback detection ---------------------------
	startAfter := 0 // mine messages with SourceLine > startAfter
	var prior *mineState
	if mineState_ != "" {
		if b, rerr := os.ReadFile(mineState_); rerr == nil {
			var st mineState
			if json.Unmarshal(b, &st) == nil {
				prior = &st
			}
		}
	}
	// A state watermark is bound to ONE transcript. If the prior state was recorded
	// for a different file, its line watermark says nothing about THIS file — trusting
	// it silently drops events (a state pointed at a renamed/different transcript whose
	// prefix happens to line up). The recorded File is the binding; enforce it.
	if prior != nil && prior.File != "" && !sameFile(prior.File, mineFile) {
		fmt.Fprintf(os.Stderr, "mine-session: state was recorded for a different file (%s) — re-mining this file from start\n", prior.File)
		prior = nil
	}
	if prior != nil {
		// Re-mine cleanly (rollback) unless the already-mined prefix is byte-stable.
		if prefixChecksum(result, prior.LastLine) == prior.PrefixChecksum {
			startAfter = prior.LastLine
		} else {
			fmt.Fprintln(os.Stderr, "mine-session: prefix changed since last mine (truncated/rewritten) — re-mining from start (rollback)")
		}
	}

	// --- emit deterministically-evidenced events ------------------------------
	events := mineToolCallEvents(result, sessionID, startAfter)

	out := cmd.OutOrStdout()
	if mineJSON {
		enc := json.NewEncoder(out)
		for _, ev := range events {
			if err := enc.Encode(ev); err != nil {
				return fmt.Errorf("mine-session: encode event: %w", err)
			}
		}
	}

	// --- persist the watermark ------------------------------------------------
	if mineState_ != "" {
		highest := startAfter
		for _, m := range result.Messages {
			if m.MessageIndex > highest {
				highest = m.MessageIndex
			}
		}
		minedCount := len(events)
		if prior != nil && startAfter == prior.LastLine {
			minedCount += prior.MinedCount // incremental: accumulate
		}
		// Store the ABSOLUTE path: the File binding must be cwd-independent. Storing
		// the raw (possibly relative) --file value lets the same relative name from a
		// different cwd compare equal to a physically different transcript, defeating
		// the binding and dropping that file's events.
		storedFile := mineFile
		if abs, aerr := filepath.Abs(mineFile); aerr == nil {
			storedFile = abs
		}
		next := mineState{
			File:           storedFile,
			LastLine:       highest,
			PrefixChecksum: prefixChecksum(result, highest),
			MinedCount:     minedCount,
		}
		if err := writeMineState(mineState_, next); err != nil {
			return fmt.Errorf("mine-session: write state %s: %w", mineState_, err)
		}
	}

	fmt.Fprintf(os.Stderr, "mine-session: %d new event(s) from %s\n", len(events), mineFile)
	return nil
}

// mineToolCallEvents emits one tool_call event per tool use in messages whose
// source line is strictly greater than startAfter. Stable, sorted, idempotent.
func mineToolCallEvents(result *parser.ParseResult, sessionID string, startAfter int) []MineEvent {
	var events []MineEvent
	for _, m := range result.Messages {
		if m.MessageIndex <= startAfter {
			continue
		}
		// A tool_result MESSAGE is an output, not a call. The parser marks every
		// result row — Claude top-level (Type=="tool_result", Tools[0].Name = the
		// real tool name, e.g. "Bash") and Codex function_call_output — with
		// Type=="tool_result". This message-level type is the family-agnostic
		// discriminator; a tool-name check alone misses the Claude top-level form.
		if m.Type == toolResultMessageType {
			continue
		}
		for i, tc := range m.Tools {
			// Belt-and-suspenders for tool_result blocks nested inside an assistant
			// message (Name=="tool_result") and empty placeholders.
			if tc.Name == "" || tc.Name == toolResultMessageType {
				continue
			}
			ev := MineEvent{
				SchemaVersion: MineEventSchemaVersion,
				Kind:          "tool_call",
				SessionID:     sessionID,
				SourceLine:    m.MessageIndex,
				Tool:          tc.Name,
			}
			// The ordinal i is the tool's position within this message. One message
			// (one source line) can hold several tool_use blocks of the SAME tool
			// (parallel Bash/Read calls) — without the ordinal both would hash to the
			// same id and a downstream id-dedup would collapse two real calls into one.
			ev.ID = mineEventID(sessionID, m.MessageIndex, "tool_call", tc.Name, i)
			events = append(events, ev)
		}
	}
	// Deterministic order: by source line, then tool, then id.
	sort.Slice(events, func(i, j int) bool {
		if events[i].SourceLine != events[j].SourceLine {
			return events[i].SourceLine < events[j].SourceLine
		}
		if events[i].Tool != events[j].Tool {
			return events[i].Tool < events[j].Tool
		}
		return events[i].ID < events[j].ID
	})
	return events
}

// mineEventID is the stable, idempotent id for an event: a rerun over the same
// (session, line, kind, tool, ordinal) yields the identical id, so downstream
// dedup is trivial and reruns never double-emit. The ordinal (the tool's
// position within its message) disambiguates several same-tool calls on one
// source line, so two real parallel calls never collapse to one id.
func mineEventID(sessionID string, line int, kind, tool string, ordinal int) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%s:%s:%d", sessionID, line, kind, tool, ordinal)))
	return fmt.Sprintf("%x", h)[:16]
}

// prefixChecksum is the checksum of the parsed messages up to and including line
// `upto`. Used as the rollback sentinel: if the previously-mined prefix's
// checksum no longer matches, the transcript was rewritten and we re-mine.
func prefixChecksum(result *parser.ParseResult, upto int) string {
	var b strings.Builder
	for _, m := range result.Messages {
		if m.MessageIndex > upto {
			continue
		}
		// Line + type + each tool's name AND content (input + output). Hashing the
		// names alone is NOT enough: a rewrite that keeps the tool name but changes
		// the arguments (Edit file_path a.txt -> b.txt) would hash identically and
		// be silently trusted as unchanged, defeating rollback detection. Including
		// the JSON-serialized input (Go sorts map keys, so it is stable across
		// re-parses of the same bytes) makes a content rewrite change the checksum.
		fmt.Fprintf(&b, "%d|%s|", m.MessageIndex, m.Type)
		for _, tc := range m.Tools {
			b.WriteString(tc.Name)
			b.WriteByte(0x1f)
			if len(tc.Input) > 0 {
				if ib, err := json.Marshal(tc.Input); err == nil {
					b.Write(ib)
				}
			}
			b.WriteByte(0x1f)
			b.WriteString(tc.Output)
			b.WriteByte(',')
		}
		b.WriteByte('\n')
	}
	h := sha256.Sum256([]byte(b.String()))
	return fmt.Sprintf("%x", h)[:16]
}

func sessionIDFromPath(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// sameFile reports whether two paths refer to the same transcript. Compared by
// absolute path so a state recorded as "./s.jsonl" still matches "s.jsonl" from
// the same cwd; falls back to the raw strings if abs resolution fails.
func sameFile(a, b string) bool {
	aa, aerr := filepath.Abs(a)
	bb, berr := filepath.Abs(b)
	if aerr != nil || berr != nil {
		return a == b
	}
	return aa == bb
}

func writeMineState(path string, st mineState) error {
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0o755)
	}
	return os.WriteFile(path, b, 0o644)
}
