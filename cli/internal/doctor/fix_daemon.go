package doctor

// Daemon subsystem detectors and fixers.
//
// This file implements the seven daemon failure modes from the Phase 2
// analysis. Six are auto-fixable, one is detect-only:
//
//	fm-daemon-corrupt-ledger-line       (auto)        — quarantine non-JSON / schema-invalid ledger lines
//	fm-daemon-truncated-trailing-line   (auto)        — quarantine a torn trailing ledger fragment
//	fm-daemon-snapshot-schema-mismatch  (auto)        — retire stale-schema projection snapshots
//	fm-daemon-orphan-tmp-files          (auto)        — retire crash-leftover *.tmp / *.gz.tmp files
//	fm-daemon-corrupt-gzip-archive      (auto)        — quarantine + salvage bad rotated archives
//	fm-daemon-archive-unbounded-growth  (auto, LAST)  — retire oldest archives/snapshots past the cap
//	fm-daemon-unreachable               (detect-only) — runtime condition, no on-disk fix
//
// Detectors are PURE: they only stat, read, and (for the gzip-archive FM)
// stream-decompress into a discard sink. Every fixer disk write flows through
// Mutate — there is no os.WriteFile/os.Remove/os.Rename/os.Create in this
// file. "Deletion" is always a Rename into the run's quarantine directory.
//
// Quarantine files are written with the WriteFile op (never AppendFile): the
// quarantine/ directory may not exist on a fresh store, and only WriteFile's
// atomicWrite path MkdirAll's the parent — AppendFile assumes the parent is
// already present. Each quarantine file is newly created per run, so a single
// WriteFile of the full payload is both correct and idempotent.
//
// Every auto-fixable daemon fixer refuses (exit 5 / concurrency_lost semantics)
// if an `ao daemon run` process is live: a concurrent ledger append, snapshot
// write, or rotation would race the rewrite. Process detection routes through
// daemonProcessRunning, a package var so tests can pin it deterministically.
//
// fm-daemon-archive-unbounded-growth must run LAST: retiring an archive by
// count before the corrupt-gzip / snapshot-schema FMs inspect it would hide a
// broken file from its own detector. The engine topo-sorts fixer order via
// analysis/dependency_graph.json; the fixers here only implement correct,
// idempotent behaviour.

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// daemonLedgerSchemaVersion is the only ledger schema version the daemon's
// ValidateLedgerEvent accepts (cli/internal/daemon/store.go LedgerSchemaVersion).
const daemonLedgerSchemaVersion = 1

// daemonProjectionSchemaVersion is the current projection-snapshot schema the
// daemon's LoadLatestProjectionSnapshot accepts (cli/internal/daemon/
// projections.go ProjectionSchemaVersion). Snapshots with any other value are
// rejected and masked as snapshot=none.
const daemonProjectionSchemaVersion = 1

// daemonArchiveRetention is the doctor's retain cap for rotated ledger
// archives. It is deliberately above the daemon's ArchiveCountWarn (20) so the
// doctor only retires once growth is clearly unbounded, not at the first warn.
const daemonArchiveRetention = 30

// daemonSnapshotRetention is the doctor's retain cap for projection snapshots.
const daemonSnapshotRetention = 10

var daemonMaxGzipArchiveDecompressedBytes int64 = 64 << 20

// daemonTmpGraceWindow is the age below which a temp file may still belong to a
// live in-flight write; orphan-tmp detection and repair both ignore temps
// younger than this.
const daemonTmpGraceWindow = 5 * time.Minute

// daemonNow returns the current time. It is a package var so orphan-tmp tests
// can pin a deterministic clock.
var daemonNow = time.Now

// daemonProcessRunning reports whether an `ao daemon run` process is live. It
// is a package var so tests can pin it without spawning a real daemon. The
// default shells out to pgrep; a missing pgrep (or no match) yields false.
var daemonProcessRunning = func() bool {
	out, err := exec.Command("pgrep", "-af", "ao daemon run").Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != ""
}

// daemonStoreDir returns the daemon store base, <repo>/.agents/daemon.
func daemonStoreDir(env *DetectEnv) string {
	return filepath.Join(env.RepoRoot, ".agents", "daemon")
}

// daemonQuarantineRoot returns the run-scoped quarantine directory,
// .doctor/runs/<run-id>/quarantine. Daemon fixers move or copy offending data
// here — not into the user's daemon store — so `ao doctor undo` leaves the
// repo byte-identical to its pre-fix state. Quarantine output travels with the
// run directory and is reclaimed by `ao doctor gc`.
func daemonQuarantineRoot(ctx *MutateContext) string {
	return filepath.Join(ctx.RunDir, "quarantine")
}

// daemonLedgerPath returns the active ledger path under the daemon store.
func daemonLedgerPath(env *DetectEnv) string {
	return filepath.Join(daemonStoreDir(env), "ledger.jsonl")
}

// daemonProjectionsDir returns the projection-snapshot directory.
func daemonProjectionsDir(env *DetectEnv) string {
	return filepath.Join(daemonStoreDir(env), "projections")
}

// daemonHandoffsDir returns the artifact-temp store the daemon writes into.
func daemonHandoffsDir(env *DetectEnv) string {
	return filepath.Join(env.RepoRoot, ".agents", "handoffs", "sha256")
}

// daemonConcurrencyRefusal builds the standard refusal error a daemon fixer
// returns when a live `ao daemon run` process is found. The wording carries
// the concurrency_lost (exit 5) semantics defined in the repair specs.
func daemonConcurrencyRefusal(fixerID string) error {
	return fmt.Errorf("doctor: %s: refused — `ao daemon run` is live; stop the daemon before --fix (concurrency_lost)", fixerID)
}

// init registers all seven daemon detectors and seven daemon fixers (six
// auto-fixable, one detect-only refuser).
func init() {
	RegisterDetector(corruptLedgerLineDetector{})
	RegisterDetector(truncatedTrailingLineDetector{})
	RegisterDetector(snapshotSchemaMismatchDetector{})
	RegisterDetector(daemonUnreachableDetector{})
	RegisterDetector(orphanTmpFilesDetector{})
	RegisterDetector(corruptGzipArchiveDetector{})
	RegisterDetector(archiveUnboundedGrowthDetector{})

	RegisterFixer(corruptLedgerLineFixer{})
	RegisterFixer(truncatedTrailingLineFixer{})
	RegisterFixer(snapshotSchemaMismatchFixer{})
	RegisterFixer(daemonUnreachableFixer{})
	RegisterFixer(orphanTmpFilesFixer{})
	RegisterFixer(corruptGzipArchiveFixer{})
	RegisterFixer(archiveUnboundedGrowthFixer{})
}

// ---------------------------------------------------------------------------
// Shared ledger helpers (pure).
// ---------------------------------------------------------------------------

// ledgerEventValid reports whether a trimmed ledger line parses as JSON AND
// passes the daemon's schema contract: schema_version == 1 and the six
// required string fields are non-empty. It re-derives the verdict the daemon's
// replayLedgerFile + ValidateLedgerEvent would reach, without touching disk.
func ledgerEventValid(trimmed string) bool {
	if trimmed == "" {
		return false
	}
	var ev struct {
		SchemaVersion int    `json:"schema_version"`
		EventID       string `json:"event_id"`
		RequestID     string `json:"request_id"`
		JobID         string `json:"job_id"`
		EventType     string `json:"event_type"`
		OccurredAt    string `json:"occurred_at"`
		Actor         string `json:"actor"`
	}
	if err := json.Unmarshal([]byte(trimmed), &ev); err != nil {
		return false
	}
	if ev.SchemaVersion != daemonLedgerSchemaVersion {
		return false
	}
	for _, v := range []string{ev.EventID, ev.RequestID, ev.JobID, ev.EventType, ev.OccurredAt, ev.Actor} {
		if strings.TrimSpace(v) == "" {
			return false
		}
	}
	if _, err := time.Parse(time.RFC3339Nano, ev.OccurredAt); err != nil {
		return false
	}
	return true
}

// daemonSplitLines splits raw file bytes on newline, returning each line
// untrimmed plus a parallel slice of trimmed forms. Order is preserved and
// blank lines are kept (callers skip them) so 1-based line numbers stay exact.
func daemonSplitLines(raw []byte) []string {
	return strings.Split(string(raw), "\n")
}

// ---------------------------------------------------------------------------
// FM: fm-daemon-corrupt-ledger-line (auto-fixable)
// ---------------------------------------------------------------------------

// corruptLedgerLineDetector flags ledger.jsonl lines that fail JSON parse or
// schema validation — events the daemon silently drops on replay.
type corruptLedgerLineDetector struct{}

func (corruptLedgerLineDetector) ID() string           { return "fm-daemon-corrupt-ledger-line" }
func (corruptLedgerLineDetector) Subsystem() string    { return "daemon" }
func (corruptLedgerLineDetector) Severity() string     { return "P1" }
func (corruptLedgerLineDetector) EstimatedCostMS() int { return 6 }
func (corruptLedgerLineDetector) OnlineRequired() bool { return false }
func (corruptLedgerLineDetector) QuickPath() bool      { return false }
func (corruptLedgerLineDetector) Describe() string {
	return "ledger.jsonl has lines that fail JSON parse or schema validation"
}

// corruptLedgerLineNos returns the 1-based line numbers of corrupt ledger
// entries. Blank lines are skipped (the daemon's replay tolerates them).
func corruptLedgerLineNos(raw []byte) []int {
	var bad []int
	for i, ln := range daemonSplitLines(raw) {
		t := strings.TrimSpace(ln)
		if t == "" {
			continue
		}
		if !ledgerEventValid(t) {
			bad = append(bad, i+1)
		}
	}
	return bad
}

// daemonQuarantineLineFileCount counts files matching the daemon's own
// quarantine naming (ledger-line-*.json) — corroborating evidence the daemon
// already dropped events on a prior boot. The doctor's own quarantine output
// uses a distinct glob (ledger-corrupt-*.jsonl) so it is not counted here.
func daemonQuarantineLineFileCount(env *DetectEnv) int {
	matches, _ := filepath.Glob(filepath.Join(daemonStoreDir(env), "quarantine", "ledger-line-*.json"))
	return len(matches)
}

func (d corruptLedgerLineDetector) Detect(env *DetectEnv) ([]Finding, error) {
	ledger := daemonLedgerPath(env)
	info, err := os.Stat(ledger)
	if err != nil || info.Size() == 0 {
		// A missing or empty ledger may still have daemon quarantine files.
		if daemonQuarantineLineFileCount(env) == 0 {
			return nil, nil
		}
	}
	var bad []int
	if err == nil && info.Size() > 0 {
		raw, rerr := os.ReadFile(ledger)
		if rerr != nil {
			return nil, fmt.Errorf("doctor: read ledger: %w", rerr)
		}
		bad = corruptLedgerLineNos(raw)
	}
	quarantineCount := daemonQuarantineLineFileCount(env)
	if len(bad) == 0 && quarantineCount == 0 {
		return nil, nil
	}
	return []Finding{{
		ID:         d.ID(),
		Severity:   d.Severity(),
		Subsystem:  d.Subsystem(),
		Title:      fmt.Sprintf("%d corrupt ledger line(s), %d daemon quarantine file(s)", len(bad), quarantineCount),
		Confidence: 1.0,
		Evidence: Evidence{
			File:  ".agents/daemon/ledger.jsonl",
			Lines: bad,
			Query: "python3 -c \"import json;[json.loads(l) for l in open('.agents/daemon/ledger.jsonl') if l.strip()]\"",
		},
		Remediation: Remediation{
			Command:          "ao doctor --fix --only " + d.ID(),
			ExplainCommand:   "ao doctor explain " + d.ID(),
			AutoFixable:      true,
			EstimatedActions: 2,
		},
	}}, nil
}

// corruptLedgerLineFixer quarantines corrupt ledger lines and rewrites
// ledger.jsonl with only clean lines. It never reconstructs dropped events —
// the corrupt bytes are appended verbatim to a per-run quarantine file first.
type corruptLedgerLineFixer struct{}

func (corruptLedgerLineFixer) ID() string { return "fm-daemon-corrupt-ledger-line" }
func (corruptLedgerLineFixer) Preconditions() []string {
	return []string{
		"no `ao daemon run` process is live (concurrency_lost otherwise)",
		".agents/daemon/ledger.jsonl is inside write_scopes",
		"at least one ledger line is clean",
	}
}
func (corruptLedgerLineFixer) WritesTo() []string {
	return []string{".agents/daemon/ledger.jsonl"}
}
func (corruptLedgerLineFixer) Ops() []string     { return []string{"WriteFile"} }
func (corruptLedgerLineFixer) Reversible() bool  { return true }
func (corruptLedgerLineFixer) Idempotent() bool  { return true }
func (corruptLedgerLineFixer) AutoFixable() bool { return true }

func (f corruptLedgerLineFixer) Fix(ctx *MutateContext, env *DetectEnv, _ []Finding) (FixResult, error) {
	res := FixResult{FixerID: f.ID(), FindingIDs: []string{f.ID()}}
	if daemonProcessRunning() {
		res.Err = daemonConcurrencyRefusal(f.ID())
		return res, res.Err
	}
	ledger := daemonLedgerPath(env)
	raw, err := os.ReadFile(ledger)
	if err != nil {
		if os.IsNotExist(err) {
			res.Fixed = true
			return res, nil
		}
		res.Err = fmt.Errorf("doctor: %s: read ledger: %w", f.ID(), err)
		return res, res.Err
	}
	var clean []string
	type corruptLine struct {
		no  int
		raw string
	}
	var corrupt []corruptLine
	for i, ln := range daemonSplitLines(raw) {
		t := strings.TrimSpace(ln)
		if t == "" {
			continue
		}
		if ledgerEventValid(t) {
			clean = append(clean, t)
		} else {
			corrupt = append(corrupt, corruptLine{no: i + 1, raw: ln})
		}
	}
	if len(corrupt) == 0 {
		res.Fixed = true
		return res, nil
	}
	// Build the entire quarantine payload in memory and write it through a
	// single WriteFile op. WriteFile's atomicWrite MkdirAll's the parent, so
	// the quarantine/ directory is created through the chokepoint; AppendFile
	// would not create the parent and would fail on a fresh store.
	quarantine := filepath.Join(daemonQuarantineRoot(ctx), "ledger-corrupt-"+ctx.RunID+".jsonl")
	var qbuf bytes.Buffer
	for _, c := range corrupt {
		payload, merr := json.Marshal(map[string]any{"line_no": c.no, "raw": c.raw, "run_id": ctx.RunID})
		if merr != nil {
			res.Err = fmt.Errorf("doctor: %s: marshal quarantine payload: %w", f.ID(), merr)
			return res, res.Err
		}
		qbuf.Write(payload)
		qbuf.WriteByte('\n')
	}
	rq, qerr := Mutate(ctx, quarantine, WriteFile{Content: qbuf.Bytes(), Mode: 0o600})
	if qerr != nil {
		res.Err = fmt.Errorf("doctor: %s: quarantine corrupt lines: %w", f.ID(), qerr)
		return res, res.Err
	}
	if rq.OK {
		res.ActionsTaken++
	}
	body := ""
	if len(clean) > 0 {
		body = strings.Join(clean, "\n") + "\n"
	}
	r, werr := Mutate(ctx, ledger, WriteFile{Content: []byte(body), Mode: 0o600})
	if werr != nil {
		res.Err = fmt.Errorf("doctor: %s: rewrite ledger: %w", f.ID(), werr)
		return res, res.Err
	}
	if r.OK {
		res.ActionsTaken++
	}
	res.Fixed = true
	return res, nil
}

// ---------------------------------------------------------------------------
// FM: fm-daemon-truncated-trailing-line (auto-fixable)
// ---------------------------------------------------------------------------

// truncatedTrailingLineDetector flags a ledger whose final byte is not a
// newline — a crash mid-append leaves the last event torn and replay drops it.
type truncatedTrailingLineDetector struct{}

func (truncatedTrailingLineDetector) ID() string           { return "fm-daemon-truncated-trailing-line" }
func (truncatedTrailingLineDetector) Subsystem() string    { return "daemon" }
func (truncatedTrailingLineDetector) Severity() string     { return "P1" }
func (truncatedTrailingLineDetector) EstimatedCostMS() int { return 2 }
func (truncatedTrailingLineDetector) OnlineRequired() bool { return false }
func (truncatedTrailingLineDetector) QuickPath() bool      { return true }
func (truncatedTrailingLineDetector) Describe() string {
	return "ledger.jsonl ends without a newline — last event will be dropped on replay"
}

// ledgerTrailingFragment returns the bytes after the last newline in raw — the
// torn trailing fragment. If raw has no newline the whole file is the fragment.
func ledgerTrailingFragment(raw []byte) []byte {
	idx := bytes.LastIndexByte(raw, '\n')
	if idx < 0 {
		return raw
	}
	return raw[idx+1:]
}

func (d truncatedTrailingLineDetector) Detect(env *DetectEnv) ([]Finding, error) {
	ledger := daemonLedgerPath(env)
	info, err := os.Stat(ledger)
	if err != nil || info.Size() == 0 {
		return nil, nil
	}
	raw, err := os.ReadFile(ledger)
	if err != nil {
		return nil, fmt.Errorf("doctor: read ledger: %w", err)
	}
	if len(raw) == 0 || raw[len(raw)-1] == '\n' {
		return nil, nil
	}
	fragment := ledgerTrailingFragment(raw)
	preview := fragment
	if len(preview) > 200 {
		preview = preview[:200]
	}
	return []Finding{{
		ID:         d.ID(),
		Severity:   d.Severity(),
		Subsystem:  d.Subsystem(),
		Title:      fmt.Sprintf("ledger.jsonl ends without newline (torn fragment of %d bytes)", len(fragment)),
		Confidence: 1.0,
		Evidence: Evidence{
			File:  ".agents/daemon/ledger.jsonl",
			Query: fmt.Sprintf("tail -c1 ledger.jsonl is %#x; fragment preview: %q", raw[len(raw)-1], string(preview)),
		},
		Remediation: Remediation{
			Command:          "ao doctor --fix --only " + d.ID(),
			ExplainCommand:   "ao doctor explain " + d.ID(),
			AutoFixable:      true,
			EstimatedActions: 2,
		},
	}}, nil
}

// truncatedTrailingLineFixer quarantines the torn trailing fragment and
// rewrites the ledger as the clean newline-terminated prefix only.
type truncatedTrailingLineFixer struct{}

func (truncatedTrailingLineFixer) ID() string { return "fm-daemon-truncated-trailing-line" }
func (truncatedTrailingLineFixer) Preconditions() []string {
	return []string{
		"no `ao daemon run` process is live (concurrency_lost otherwise)",
		".agents/daemon/ledger.jsonl is inside write_scopes",
	}
}
func (truncatedTrailingLineFixer) WritesTo() []string {
	return []string{".agents/daemon/ledger.jsonl"}
}
func (truncatedTrailingLineFixer) Ops() []string     { return []string{"WriteFile"} }
func (truncatedTrailingLineFixer) Reversible() bool  { return true }
func (truncatedTrailingLineFixer) Idempotent() bool  { return true }
func (truncatedTrailingLineFixer) AutoFixable() bool { return true }

func (f truncatedTrailingLineFixer) Fix(ctx *MutateContext, env *DetectEnv, _ []Finding) (FixResult, error) {
	res := FixResult{FixerID: f.ID(), FindingIDs: []string{f.ID()}}
	if daemonProcessRunning() {
		res.Err = daemonConcurrencyRefusal(f.ID())
		return res, res.Err
	}
	ledger := daemonLedgerPath(env)
	raw, err := os.ReadFile(ledger)
	if err != nil {
		if os.IsNotExist(err) {
			res.Fixed = true
			return res, nil
		}
		res.Err = fmt.Errorf("doctor: %s: read ledger: %w", f.ID(), err)
		return res, res.Err
	}
	if len(raw) == 0 || raw[len(raw)-1] == '\n' {
		// already clean
		res.Fixed = true
		return res, nil
	}
	idx := bytes.LastIndexByte(raw, '\n')
	var cleanPrefix, fragment []byte
	if idx < 0 {
		cleanPrefix, fragment = nil, raw
	} else {
		cleanPrefix, fragment = raw[:idx+1], raw[idx+1:]
	}
	// WriteFile (not AppendFile) so atomicWrite MkdirAll's the quarantine/
	// parent; the fragment file is always newly created.
	quarantine := filepath.Join(daemonQuarantineRoot(ctx), "trailing-fragment-"+ctx.RunID+".bin")
	r1, qerr := Mutate(ctx, quarantine, WriteFile{Content: fragment, Mode: 0o600})
	if qerr != nil {
		res.Err = fmt.Errorf("doctor: %s: quarantine torn fragment: %w", f.ID(), qerr)
		return res, res.Err
	}
	if r1.OK {
		res.ActionsTaken++
	}
	r2, werr := Mutate(ctx, ledger, WriteFile{Content: cleanPrefix, Mode: 0o600})
	if werr != nil {
		res.Err = fmt.Errorf("doctor: %s: rewrite ledger: %w", f.ID(), werr)
		return res, res.Err
	}
	if r2.OK {
		res.ActionsTaken++
	}
	res.Fixed = true
	return res, nil
}

// ---------------------------------------------------------------------------
// FM: fm-daemon-snapshot-schema-mismatch (auto-fixable)
// ---------------------------------------------------------------------------

// snapshotSchemaMismatchDetector flags projection snapshots whose
// schema_version differs from the daemon's current ProjectionSchemaVersion —
// they are unloadable and force a full ledger replay on every probe.
type snapshotSchemaMismatchDetector struct{}

func (snapshotSchemaMismatchDetector) ID() string           { return "fm-daemon-snapshot-schema-mismatch" }
func (snapshotSchemaMismatchDetector) Subsystem() string    { return "daemon" }
func (snapshotSchemaMismatchDetector) Severity() string     { return "P2" }
func (snapshotSchemaMismatchDetector) EstimatedCostMS() int { return 8 }
func (snapshotSchemaMismatchDetector) OnlineRequired() bool { return false }
func (snapshotSchemaMismatchDetector) QuickPath() bool      { return false }
func (snapshotSchemaMismatchDetector) Describe() string {
	return "projection snapshots have a stale schema_version — daemon replays full ledger"
}

// snapshotSchemaVersion decodes just the schema_version field of a snapshot
// file. A read error or unparseable JSON yields (-1, false).
func snapshotSchemaVersion(path string) (int, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return -1, false
	}
	var obj struct {
		SchemaVersion int `json:"schema_version"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return -1, false
	}
	return obj.SchemaVersion, true
}

// snapshotFiles returns the non-recursive snapshot-*.json files in dir, sorted.
// The glob is non-recursive, so files under projections/retired/ are invisible.
func snapshotFiles(dir string) []string {
	matches, _ := filepath.Glob(filepath.Join(dir, "snapshot-*.json"))
	sort.Strings(matches)
	return matches
}

// staleSnapshots returns the snapshot files whose schema_version != the
// daemon's current version (an unparseable snapshot counts as stale).
func staleSnapshots(dir string) []string {
	var stale []string
	for _, f := range snapshotFiles(dir) {
		v, _ := snapshotSchemaVersion(f)
		if v != daemonProjectionSchemaVersion {
			stale = append(stale, f)
		}
	}
	return stale
}

func (d snapshotSchemaMismatchDetector) Detect(env *DetectEnv) ([]Finding, error) {
	dir := daemonProjectionsDir(env)
	if _, err := os.Stat(dir); err != nil {
		return nil, nil
	}
	stale := staleSnapshots(dir)
	if len(stale) == 0 {
		return nil, nil
	}
	usable := len(snapshotFiles(dir)) > len(stale)
	rel := make([]string, len(stale))
	for i, s := range stale {
		rel[i] = filepath.Base(s)
	}
	return []Finding{{
		ID:         d.ID(),
		Severity:   d.Severity(),
		Subsystem:  d.Subsystem(),
		Title:      fmt.Sprintf("%d stale-schema snapshot(s); usable current snapshot exists: %t", len(stale), usable),
		Confidence: 1.0,
		Evidence: Evidence{
			File:  ".agents/daemon/projections",
			Query: "for f in projections/snapshot-*.json; do jq .schema_version $f; done — want " + fmt.Sprint(daemonProjectionSchemaVersion),
		},
		Remediation: Remediation{
			Command:          "ao doctor --fix --only " + d.ID(),
			ExplainCommand:   "ao doctor explain " + d.ID(),
			AutoFixable:      true,
			EstimatedActions: len(stale),
		},
	}}, nil
}

// snapshotSchemaMismatchFixer retires stale-schema snapshots into
// projections/retired/ via Mutate Rename. The daemon's filename-sort picker no
// longer selects them; it re-derives a fresh current-schema snapshot on replay.
type snapshotSchemaMismatchFixer struct{}

func (snapshotSchemaMismatchFixer) ID() string { return "fm-daemon-snapshot-schema-mismatch" }
func (snapshotSchemaMismatchFixer) Preconditions() []string {
	return []string{
		"no `ao daemon run` process is live (concurrency_lost otherwise)",
		".agents/daemon/projections and projections/retired are inside write_scopes",
	}
}
func (snapshotSchemaMismatchFixer) WritesTo() []string {
	return []string{".agents/daemon/projections"}
}
func (snapshotSchemaMismatchFixer) Ops() []string     { return []string{"Rename"} }
func (snapshotSchemaMismatchFixer) Reversible() bool  { return true }
func (snapshotSchemaMismatchFixer) Idempotent() bool  { return true }
func (snapshotSchemaMismatchFixer) AutoFixable() bool { return true }

func (f snapshotSchemaMismatchFixer) Fix(ctx *MutateContext, env *DetectEnv, _ []Finding) (FixResult, error) {
	res := FixResult{FixerID: f.ID(), FindingIDs: []string{f.ID()}}
	if daemonProcessRunning() {
		res.Err = daemonConcurrencyRefusal(f.ID())
		return res, res.Err
	}
	dir := daemonProjectionsDir(env)
	stale := staleSnapshots(dir)
	if len(stale) == 0 {
		res.Fixed = true
		return res, nil
	}
	retired := filepath.Join(dir, "retired")
	for _, src := range stale {
		dest := filepath.Join(retired, filepath.Base(src))
		r, err := Mutate(ctx, src, Rename{To: dest})
		if err != nil {
			res.Err = fmt.Errorf("doctor: %s: retire %s: %w", f.ID(), filepath.Base(src), err)
			return res, res.Err
		}
		if r.OK {
			res.ActionsTaken++
		}
	}
	if !ctx.DryRun && len(staleSnapshots(dir)) != 0 {
		res.Err = fmt.Errorf("doctor: %s: fix did not eliminate the finding", f.ID())
		return res, res.Err
	}
	res.Fixed = true
	return res, nil
}

// ---------------------------------------------------------------------------
// FM: fm-daemon-unreachable (detect-only)
// ---------------------------------------------------------------------------

// daemonUnreachableDetector flags a daemon that is down, wedged, or bound to a
// stale address. This is a runtime/process condition, not on-disk corruption,
// so the matching fixer refuses. The detector promotes the generic warn to a
// precise fail finding naming the right recovery command.
type daemonUnreachableDetector struct{}

func (daemonUnreachableDetector) ID() string           { return "fm-daemon-unreachable" }
func (daemonUnreachableDetector) Subsystem() string    { return "daemon" }
func (daemonUnreachableDetector) Severity() string     { return "P2" }
func (daemonUnreachableDetector) EstimatedCostMS() int { return 10 }
func (daemonUnreachableDetector) OnlineRequired() bool { return false }
func (daemonUnreachableDetector) QuickPath() bool      { return false }
func (daemonUnreachableDetector) Describe() string {
	return "daemon is down/wedged/stale-bound — readiness and telemetry unavailable"
}

func (d daemonUnreachableDetector) Detect(env *DetectEnv) ([]Finding, error) {
	// Pure, read-only: the daemon store must exist (a never-initialized
	// workspace has no daemon to be unreachable). If the store exists but no
	// `ao daemon run` process is live, the daemon is DOWN.
	if _, err := os.Stat(daemonStoreDir(env)); err != nil {
		return nil, nil
	}
	if daemonProcessRunning() {
		return nil, nil
	}
	return []Finding{{
		ID:         d.ID(),
		Severity:   d.Severity(),
		Subsystem:  d.Subsystem(),
		Title:      "daemon is DOWN — no `ao daemon run` process; readiness/telemetry unavailable",
		Confidence: 0.9,
		Evidence: Evidence{
			File:  ".agents/daemon",
			Query: "pgrep -af 'ao daemon run' — no match",
		},
		Remediation: Remediation{
			Command:        "ao daemon start",
			ExplainCommand: "ao doctor explain " + d.ID(),
			AutoFixable:    false,
		},
	}}, nil
}

// daemonUnreachableFixer is a detect-only refuser. There is no safe on-disk
// mutation that makes a dead/wedged daemon reachable: starting/killing a
// process is a lifecycle side effect, not a scoped file mutation, and a wedged
// daemon mid-replay must not be killed. It never calls Mutate, by design.
type daemonUnreachableFixer struct{}

func (daemonUnreachableFixer) ID() string { return "fm-daemon-unreachable" }
func (daemonUnreachableFixer) Preconditions() []string {
	return []string{"detect-only: no safe scoped mutation; run the command named in the finding"}
}
func (daemonUnreachableFixer) WritesTo() []string { return nil }
func (daemonUnreachableFixer) Ops() []string      { return nil }
func (daemonUnreachableFixer) Reversible() bool   { return true }
func (daemonUnreachableFixer) Idempotent() bool   { return true }
func (daemonUnreachableFixer) AutoFixable() bool  { return false }

func (f daemonUnreachableFixer) Fix(_ *MutateContext, _ *DetectEnv, _ []Finding) (FixResult, error) {
	err := fmt.Errorf("doctor: %s: detect-only — daemon reachability is a runtime condition; "+
		"run `ao daemon start` (or `ao daemon restart`) manually (refused_unsafe)", f.ID())
	return FixResult{FixerID: f.ID(), FindingIDs: []string{f.ID()}, Fixed: false, Err: err}, err
}

// ---------------------------------------------------------------------------
// FM: fm-daemon-orphan-tmp-files (auto-fixable)
// ---------------------------------------------------------------------------

// orphanTmpFilesDetector flags crash-leftover *.tmp / *.gz.tmp files in the
// daemon store older than the grace window — temps no sweeper reliably collects.
type orphanTmpFilesDetector struct{}

func (orphanTmpFilesDetector) ID() string           { return "fm-daemon-orphan-tmp-files" }
func (orphanTmpFilesDetector) Subsystem() string    { return "daemon" }
func (orphanTmpFilesDetector) Severity() string     { return "P3" }
func (orphanTmpFilesDetector) EstimatedCostMS() int { return 5 }
func (orphanTmpFilesDetector) OnlineRequired() bool { return false }
func (orphanTmpFilesDetector) QuickPath() bool      { return false }
func (orphanTmpFilesDetector) Describe() string {
	return "stale temp files left in the daemon store by a crashed write"
}

// orphanTmpPatterns returns the absolute globs for the three daemon temp-file
// classes the orphan-tmp FM scans.
func orphanTmpPatterns(env *DetectEnv) []string {
	store := daemonStoreDir(env)
	return []string{
		filepath.Join(store, "snapshot-*.json.tmp"),
		filepath.Join(store, "projections", "snapshot-*.json.tmp"),
		filepath.Join(store, "ledger.*.jsonl.gz.tmp"),
		filepath.Join(daemonHandoffsDir(env), ".*.tmp"),
	}
}

// orphanTmpFiles returns the temp files matching the orphan globs that are
// older than the grace window — fresher temps may belong to a live write.
func orphanTmpFiles(env *DetectEnv) []string {
	cutoff := daemonNow().Add(-daemonTmpGraceWindow)
	var orphans []string
	for _, pat := range orphanTmpPatterns(env) {
		matches, _ := filepath.Glob(pat)
		for _, m := range matches {
			info, err := os.Stat(m)
			if err != nil || info.IsDir() {
				continue
			}
			if info.ModTime().Before(cutoff) {
				orphans = append(orphans, m)
			}
		}
	}
	sort.Strings(orphans)
	return orphans
}

func (d orphanTmpFilesDetector) Detect(env *DetectEnv) ([]Finding, error) {
	if _, err := os.Stat(daemonStoreDir(env)); err != nil {
		return nil, nil
	}
	orphans := orphanTmpFiles(env)
	if len(orphans) == 0 {
		return nil, nil
	}
	var total int64
	for _, o := range orphans {
		if info, err := os.Stat(o); err == nil {
			total += info.Size()
		}
	}
	return []Finding{{
		ID:         d.ID(),
		Severity:   d.Severity(),
		Subsystem:  d.Subsystem(),
		Title:      fmt.Sprintf("%d orphan temp file(s), %d bytes left by crashed writes", len(orphans), total),
		Confidence: 1.0,
		Evidence: Evidence{
			File:  ".agents/daemon",
			Query: "find .agents/daemon -name '*.tmp' -o -name '*.gz.tmp' (grace window 300s)",
		},
		Remediation: Remediation{
			Command:          "ao doctor --fix --only " + d.ID(),
			ExplainCommand:   "ao doctor explain " + d.ID(),
			AutoFixable:      true,
			EstimatedActions: len(orphans),
		},
	}}, nil
}

// orphanTmpFilesFixer retires grace-aged orphan temps into
// quarantine/orphan-tmp/<run-id>/, preserving their relative path so the
// operator can see which write path produced each. It never deletes.
type orphanTmpFilesFixer struct{}

func (orphanTmpFilesFixer) ID() string { return "fm-daemon-orphan-tmp-files" }
func (orphanTmpFilesFixer) Preconditions() []string {
	return []string{
		"no `ao daemon run` process is live (concurrency_lost otherwise)",
		"only temps older than the 5-minute grace window are retired",
	}
}
func (orphanTmpFilesFixer) WritesTo() []string {
	return []string{".agents/daemon", ".agents/handoffs/sha256"}
}
func (orphanTmpFilesFixer) Ops() []string     { return []string{"Rename"} }
func (orphanTmpFilesFixer) Reversible() bool  { return true }
func (orphanTmpFilesFixer) Idempotent() bool  { return true }
func (orphanTmpFilesFixer) AutoFixable() bool { return true }

func (f orphanTmpFilesFixer) Fix(ctx *MutateContext, env *DetectEnv, _ []Finding) (FixResult, error) {
	res := FixResult{FixerID: f.ID(), FindingIDs: []string{f.ID()}}
	if daemonProcessRunning() {
		res.Err = daemonConcurrencyRefusal(f.ID())
		return res, res.Err
	}
	orphans := orphanTmpFiles(env)
	if len(orphans) == 0 {
		res.Fixed = true
		return res, nil
	}
	qroot := filepath.Join(daemonQuarantineRoot(ctx), "orphan-tmp", ctx.RunID)
	for _, src := range orphans {
		rel, err := filepath.Rel(env.RepoRoot, src)
		if err != nil {
			rel = filepath.Base(src)
		}
		dest := filepath.Join(qroot, rel)
		r, merr := Mutate(ctx, src, Rename{To: dest})
		if merr != nil {
			res.Err = fmt.Errorf("doctor: %s: retire %s: %w", f.ID(), filepath.Base(src), merr)
			return res, res.Err
		}
		if r.OK {
			res.ActionsTaken++
		}
	}
	if !ctx.DryRun && len(orphanTmpFiles(env)) != 0 {
		res.Err = fmt.Errorf("doctor: %s: fix did not eliminate the finding", f.ID())
		return res, res.Err
	}
	res.Fixed = true
	return res, nil
}

// ---------------------------------------------------------------------------
// FM: fm-daemon-corrupt-gzip-archive (auto-fixable)
// ---------------------------------------------------------------------------

// corruptGzipArchiveDetector flags rotated ledger archives that are not valid
// gzip streams — a truncated or non-gzip archive hard-aborts the whole replay.
type corruptGzipArchiveDetector struct{}

func (corruptGzipArchiveDetector) ID() string           { return "fm-daemon-corrupt-gzip-archive" }
func (corruptGzipArchiveDetector) Subsystem() string    { return "daemon" }
func (corruptGzipArchiveDetector) Severity() string     { return "P1" }
func (corruptGzipArchiveDetector) EstimatedCostMS() int { return 15 }
func (corruptGzipArchiveDetector) OnlineRequired() bool { return false }
func (corruptGzipArchiveDetector) QuickPath() bool      { return false }
func (corruptGzipArchiveDetector) Describe() string {
	return "rotated ledger archive is not a valid gzip stream — daemon replay aborts"
}

// gzipArchiveFiles returns the rotated .gz archive files in the daemon store,
// sorted. The active ledger.jsonl carries no embedded timestamp and is not
// matched by the ledger.*.jsonl.gz glob.
func gzipArchiveFiles(env *DetectEnv) []string {
	matches, _ := filepath.Glob(filepath.Join(daemonStoreDir(env), "ledger.*.jsonl.gz"))
	sort.Strings(matches)
	return matches
}

// gzipArchiveCheck streams a .gz archive through the decompressor to EOF and
// returns a non-empty classification ("invalid_header" | "truncated" | "other")
// if the stream is not sound, or "" if the archive decompresses cleanly.
func gzipArchiveCheck(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "other"
	}
	gr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return "invalid_header"
	}
	defer func() { _ = gr.Close() }()
	n, err := io.CopyN(io.Discard, gr, daemonMaxGzipArchiveDecompressedBytes+1)
	if err == nil || n > daemonMaxGzipArchiveDecompressedBytes {
		return "oversized"
	}
	if err != io.EOF {
		if err == io.ErrUnexpectedEOF || err == io.EOF {
			return "truncated"
		}
		return "truncated"
	}
	return ""
}

// badGzipArchives returns the archive paths that fail gzip validation, each
// paired with its corruption classification.
func badGzipArchives(env *DetectEnv) map[string]string {
	bad := make(map[string]string)
	for _, f := range gzipArchiveFiles(env) {
		if kind := gzipArchiveCheck(f); kind != "" {
			bad[f] = kind
		}
	}
	return bad
}

func (d corruptGzipArchiveDetector) Detect(env *DetectEnv) ([]Finding, error) {
	if _, err := os.Stat(daemonStoreDir(env)); err != nil {
		return nil, nil
	}
	bad := badGzipArchives(env)
	if len(bad) == 0 {
		return nil, nil
	}
	names := make([]string, 0, len(bad))
	for f, kind := range bad {
		names = append(names, filepath.Base(f)+" ("+kind+")")
	}
	sort.Strings(names)
	return []Finding{{
		ID:         d.ID(),
		Severity:   d.Severity(),
		Subsystem:  d.Subsystem(),
		Title:      fmt.Sprintf("%d corrupt rotated archive(s): %s", len(bad), strings.Join(names, ", ")),
		Confidence: 1.0,
		Evidence: Evidence{
			File:  ".agents/daemon",
			Query: "for f in .agents/daemon/ledger.*.jsonl.gz; do gzip -t \"$f\" || echo BAD; done",
		},
		Remediation: Remediation{
			Command:          "ao doctor --fix --only " + d.ID(),
			ExplainCommand:   "ao doctor explain " + d.ID(),
			AutoFixable:      true,
			EstimatedActions: len(bad) * 2,
		},
	}}, nil
}

// salvageGzipArchive stream-decompresses a corrupt archive as far as the gzip
// reader allows, returning only the complete, newline-terminated, schema-valid
// ledger lines recovered from the prefix. It is a pure in-memory read.
func salvageGzipArchive(path string) []string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	gr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil
	}
	defer func() { _ = gr.Close() }()
	var buf bytes.Buffer
	n, err := io.CopyN(&buf, gr, daemonMaxGzipArchiveDecompressedBytes+1)
	if err == nil || n > daemonMaxGzipArchiveDecompressedBytes {
		return nil
	}
	// A partial copy on a truncated stream is expected; keep the valid prefix.
	decompressed := buf.Bytes()
	// Only complete newline-terminated lines are trustworthy; drop any torn
	// trailing fragment by ignoring bytes after the last newline.
	if idx := bytes.LastIndexByte(decompressed, '\n'); idx >= 0 {
		decompressed = decompressed[:idx+1]
	} else {
		decompressed = nil
	}
	var kept []string
	for _, ln := range daemonSplitLines(decompressed) {
		t := strings.TrimSpace(ln)
		if t != "" && ledgerEventValid(t) {
			kept = append(kept, t)
		}
	}
	return kept
}

// gzipCompress returns the gzip-compressed form of content.
func gzipCompress(content []byte) ([]byte, error) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	if _, err := gw.Write(content); err != nil {
		_ = gw.Close()
		return nil, err
	}
	if err := gw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// corruptGzipArchiveFixer retires bad archives into quarantine/bad-archives/
// and, when the truncated stream still yields a usable prefix, writes a
// freshly-compressed salvaged replacement at the original path/timestamp.
type corruptGzipArchiveFixer struct{}

func (corruptGzipArchiveFixer) ID() string { return "fm-daemon-corrupt-gzip-archive" }
func (corruptGzipArchiveFixer) Preconditions() []string {
	return []string{
		"no `ao daemon run` process is live (concurrency_lost otherwise)",
		".agents/daemon and the run quarantine dir (.doctor) are inside write_scopes",
	}
}
func (corruptGzipArchiveFixer) WritesTo() []string {
	return []string{".agents/daemon"}
}
func (corruptGzipArchiveFixer) Ops() []string     { return []string{"Rename", "WriteFile"} }
func (corruptGzipArchiveFixer) Reversible() bool  { return true }
func (corruptGzipArchiveFixer) Idempotent() bool  { return true }
func (corruptGzipArchiveFixer) AutoFixable() bool { return true }

func (f corruptGzipArchiveFixer) Fix(ctx *MutateContext, env *DetectEnv, _ []Finding) (FixResult, error) {
	res := FixResult{FixerID: f.ID(), FindingIDs: []string{f.ID()}}
	if daemonProcessRunning() {
		res.Err = daemonConcurrencyRefusal(f.ID())
		return res, res.Err
	}
	bad := badGzipArchives(env)
	if len(bad) == 0 {
		res.Fixed = true
		return res, nil
	}
	quarantine := filepath.Join(daemonQuarantineRoot(ctx), "bad-archives", ctx.RunID)
	// Deterministic order over the bad set.
	paths := make([]string, 0, len(bad))
	for p := range bad {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, src := range paths {
		// Salvage BEFORE the rename — the source is still at its original path.
		salvaged := salvageGzipArchive(src)
		dest := filepath.Join(quarantine, filepath.Base(src))
		r1, err := Mutate(ctx, src, Rename{To: dest})
		if err != nil {
			res.Err = fmt.Errorf("doctor: %s: retire %s: %w", f.ID(), filepath.Base(src), err)
			return res, res.Err
		}
		if r1.OK {
			res.ActionsTaken++
		}
		if len(salvaged) == 0 {
			continue
		}
		gz, gerr := gzipCompress([]byte(strings.Join(salvaged, "\n") + "\n"))
		if gerr != nil {
			res.Err = fmt.Errorf("doctor: %s: compress salvage of %s: %w", f.ID(), filepath.Base(src), gerr)
			return res, res.Err
		}
		r2, werr := Mutate(ctx, src, WriteFile{Content: gz, Mode: 0o600})
		if werr != nil {
			res.Err = fmt.Errorf("doctor: %s: write salvaged %s: %w", f.ID(), filepath.Base(src), werr)
			return res, res.Err
		}
		if r2.OK {
			res.ActionsTaken++
		}
	}
	if !ctx.DryRun && len(badGzipArchives(env)) != 0 {
		res.Err = fmt.Errorf("doctor: %s: fix did not eliminate the finding", f.ID())
		return res, res.Err
	}
	res.Fixed = true
	return res, nil
}

// ---------------------------------------------------------------------------
// FM: fm-daemon-archive-unbounded-growth (auto-fixable — runs LAST)
// ---------------------------------------------------------------------------

// archiveUnboundedGrowthDetector flags rotated archives / snapshots that exceed
// the retention cap — ledger rotation has no retention policy, so replay walks
// an ever-growing chain on every probe.
type archiveUnboundedGrowthDetector struct{}

func (archiveUnboundedGrowthDetector) ID() string           { return "fm-daemon-archive-unbounded-growth" }
func (archiveUnboundedGrowthDetector) Subsystem() string    { return "daemon" }
func (archiveUnboundedGrowthDetector) Severity() string     { return "P3" }
func (archiveUnboundedGrowthDetector) EstimatedCostMS() int { return 4 }
func (archiveUnboundedGrowthDetector) OnlineRequired() bool { return false }
func (archiveUnboundedGrowthDetector) QuickPath() bool      { return false }
func (archiveUnboundedGrowthDetector) Describe() string {
	return "rotated archives / snapshots exceed the retention cap"
}

// allArchiveFiles returns every rotated ledger archive (.jsonl and .jsonl.gz),
// sorted oldest-first. The active ledger.jsonl carries no embedded timestamp
// and is not matched by the ledger.*.jsonl* glob, so it is never returned.
func allArchiveFiles(env *DetectEnv) []string {
	matches, _ := filepath.Glob(filepath.Join(daemonStoreDir(env), "ledger.*.jsonl*"))
	sort.Strings(matches)
	return matches
}

// excessCount returns how many items exceed the cap (0 if within cap).
func excessCount(have, cap int) int {
	if have > cap {
		return have - cap
	}
	return 0
}

func (d archiveUnboundedGrowthDetector) Detect(env *DetectEnv) ([]Finding, error) {
	if _, err := os.Stat(daemonStoreDir(env)); err != nil {
		return nil, nil
	}
	archives := allArchiveFiles(env)
	snapshots := snapshotFiles(daemonProjectionsDir(env))
	excessArch := excessCount(len(archives), daemonArchiveRetention)
	excessSnap := excessCount(len(snapshots), daemonSnapshotRetention)
	if excessArch == 0 && excessSnap == 0 {
		return nil, nil
	}
	return []Finding{{
		ID:        d.ID(),
		Severity:  d.Severity(),
		Subsystem: d.Subsystem(),
		Title: fmt.Sprintf("%d archive(s) over cap %d, %d snapshot(s) over cap %d",
			excessArch, daemonArchiveRetention, excessSnap, daemonSnapshotRetention),
		Confidence: 1.0,
		Evidence: Evidence{
			File:  ".agents/daemon",
			Query: "ls .agents/daemon/ledger.*.jsonl* | wc -l",
		},
		Remediation: Remediation{
			Command:          "ao doctor --fix --only " + d.ID(),
			ExplainCommand:   "ao doctor explain " + d.ID(),
			AutoFixable:      true,
			EstimatedActions: excessArch + excessSnap,
		},
	}}, nil
}

// archiveUnboundedGrowthFixer retires the oldest excess archives and snapshots
// into quarantine/retired-archives|retired-snapshots/<run-id>/. It must run
// AFTER the corrupt-gzip and snapshot-schema FMs (declared in the engine's
// dependency graph) so it never hides a broken file from its own detector.
type archiveUnboundedGrowthFixer struct{}

func (archiveUnboundedGrowthFixer) ID() string { return "fm-daemon-archive-unbounded-growth" }
func (archiveUnboundedGrowthFixer) Preconditions() []string {
	return []string{
		"no `ao daemon run` process is live (concurrency_lost otherwise)",
		"runs after fm-daemon-corrupt-gzip-archive and fm-daemon-snapshot-schema-mismatch",
		"the run quarantine dir (.doctor) is inside write_scopes",
	}
}
func (archiveUnboundedGrowthFixer) WritesTo() []string {
	return []string{".agents/daemon", ".agents/daemon/projections"}
}
func (archiveUnboundedGrowthFixer) Ops() []string     { return []string{"Rename"} }
func (archiveUnboundedGrowthFixer) Reversible() bool  { return true }
func (archiveUnboundedGrowthFixer) Idempotent() bool  { return true }
func (archiveUnboundedGrowthFixer) AutoFixable() bool { return true }

func (f archiveUnboundedGrowthFixer) Fix(ctx *MutateContext, env *DetectEnv, _ []Finding) (FixResult, error) {
	res := FixResult{FixerID: f.ID(), FindingIDs: []string{f.ID()}}
	if daemonProcessRunning() {
		res.Err = daemonConcurrencyRefusal(f.ID())
		return res, res.Err
	}
	archives := allArchiveFiles(env)
	snapshots := snapshotFiles(daemonProjectionsDir(env))
	var archRetire, snapRetire []string
	if n := excessCount(len(archives), daemonArchiveRetention); n > 0 {
		archRetire = archives[:n]
	}
	if n := excessCount(len(snapshots), daemonSnapshotRetention); n > 0 {
		snapRetire = snapshots[:n]
	}
	if len(archRetire) == 0 && len(snapRetire) == 0 {
		res.Fixed = true
		return res, nil
	}
	qArch := filepath.Join(daemonQuarantineRoot(ctx), "retired-archives", ctx.RunID)
	qSnap := filepath.Join(daemonQuarantineRoot(ctx), "retired-snapshots", ctx.RunID)
	for _, src := range archRetire {
		r, err := Mutate(ctx, src, Rename{To: filepath.Join(qArch, filepath.Base(src))})
		if err != nil {
			res.Err = fmt.Errorf("doctor: %s: retire archive %s: %w", f.ID(), filepath.Base(src), err)
			return res, res.Err
		}
		if r.OK {
			res.ActionsTaken++
		}
	}
	for _, src := range snapRetire {
		r, err := Mutate(ctx, src, Rename{To: filepath.Join(qSnap, filepath.Base(src))})
		if err != nil {
			res.Err = fmt.Errorf("doctor: %s: retire snapshot %s: %w", f.ID(), filepath.Base(src), err)
			return res, res.Err
		}
		if r.OK {
			res.ActionsTaken++
		}
	}
	if !ctx.DryRun {
		archives = allArchiveFiles(env)
		snapshots = snapshotFiles(daemonProjectionsDir(env))
		if excessCount(len(archives), daemonArchiveRetention) != 0 ||
			excessCount(len(snapshots), daemonSnapshotRetention) != 0 {
			res.Err = fmt.Errorf("doctor: %s: fix did not eliminate the finding", f.ID())
			return res, res.Err
		}
	}
	res.Fixed = true
	return res, nil
}
