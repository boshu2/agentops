package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/openclaw"
)

// The bridges subsystem detects failures in the OpenClaw consumer surface. Per
// the Phase 2 analysis, the health probe is detect-only and the snapshot fixer
// is partial — the doctor must never start or kill processes or rewrite
// activation files. The single partial fixer is fm-bridges-openclaw-snapshot-stale,
// whose torn-`latest.json` sub-case is safely reconstructible from an intact
// versioned snapshot.
//
// The GasCity (`gc`) bridge detectors/fixers were removed in ag-gdns: AgentOps
// no longer references Gas City (ag-124p), so the doctor no longer probes the
// `gc` binary, version, controller, or status schema.

// init registers every bridges detector and fixer with the package registry.
func init() {
	RegisterDetector(openclawHealthUnreachableDetector{})
	RegisterDetector(openclawSnapshotStaleDetector{})

	RegisterFixer(openclawHealthUnreachableFixer{})
	RegisterFixer(openclawSnapshotStaleFixer{})
}

const (
	subsystemBridges = "bridges"

	fmOpenClawHealthUnreachable = "fm-bridges-openclaw-health-unreachable"
	fmOpenClawSnapshotStale     = "fm-bridges-openclaw-snapshot-stale"
)

// ---------------------------------------------------------------------------
// Shared helpers (all pure / read-only).
// ---------------------------------------------------------------------------

// truncatePayload bounds an observed-payload string for evidence storage.
func truncatePayload(s string) string {
	const maxLen = 240
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// reportPath returns the run-dir-relative report file path for a finding id.
func reportPath(ctx *MutateContext, name string) string {
	return filepath.Join(ctx.RunDir, "reports", name)
}

// writeReport routes one advisory-report write through the Mutate chokepoint.
// It returns the number of actions taken (0 or 1).
func writeReport(ctx *MutateContext, name, content string) (int, error) {
	res, err := Mutate(ctx, reportPath(ctx, name), WriteFile{
		Content: []byte(content),
		Mode:    0o644,
	})
	if err != nil {
		return 0, err
	}
	if res.OK {
		return 1, nil
	}
	return 0, nil
}

// detectOnlyRemediation builds the standard detect-only remediation block.
func detectOnlyRemediation(id string) Remediation {
	return Remediation{
		Command:          "ao doctor --fix --only " + id,
		ExplainCommand:   "ao doctor explain " + id,
		AutoFixable:      false,
		EstimatedActions: 1,
	}
}

// ---------------------------------------------------------------------------
// FM 1: fm-bridges-openclaw-health-unreachable — DETECT-ONLY (online probe).
// ---------------------------------------------------------------------------

// daemonActivation is the subset of .agents/daemon/activation.json the bridge
// health probe needs.
type daemonActivation struct {
	BaseURL string `json:"base_url"`
}

// readDaemonActivation reads the per-project daemon activation file. Pure.
func readDaemonActivation(repoRoot string) (daemonActivation, bool) {
	path := filepath.Join(repoRoot, ".agents", "daemon", "activation.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return daemonActivation{}, false
	}
	var act daemonActivation
	if json.Unmarshal(data, &act) != nil {
		return daemonActivation{}, false
	}
	return act, true
}

func openclawSnapshotDirExists(repoRoot string) bool {
	info, err := os.Stat(filepath.Join(repoRoot, openclaw.SnapshotDirRel))
	return err == nil && info.IsDir()
}

// httpGetBounded performs a read-only HTTP GET under a hard wall-clock
// deadline. It returns the response body, HTTP status, and any transport
// error. A wedged endpoint cannot hang `ao doctor` past the deadline.
func httpGetBounded(url string, deadline time.Duration) ([]byte, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	client := &http.Client{Timeout: deadline}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

// openclawHealthUnreachableDetector fires when the OpenClaw consumer health
// endpoint cannot be confirmed healthy.
type openclawHealthUnreachableDetector struct{}

func (openclawHealthUnreachableDetector) ID() string        { return fmOpenClawHealthUnreachable }
func (openclawHealthUnreachableDetector) Subsystem() string { return subsystemBridges }
func (openclawHealthUnreachableDetector) Severity() string  { return "P2" }
func (openclawHealthUnreachableDetector) Describe() string {
	return "OpenClaw consumer health endpoint is unreachable or not ok"
}
func (openclawHealthUnreachableDetector) EstimatedCostMS() int { return 3000 }
func (openclawHealthUnreachableDetector) OnlineRequired() bool { return true }
func (openclawHealthUnreachableDetector) QuickPath() bool      { return false }

// Detect reads the activation file and probes <base_url>/openclaw/v1/health
// under a bounded 3s deadline. The probe is read-only (HTTP GET). PURE.
func (openclawHealthUnreachableDetector) Detect(env *DetectEnv) ([]Finding, error) {
	act, ok := readDaemonActivation(env.RepoRoot)
	if !ok {
		if !openclawSnapshotDirExists(env.RepoRoot) {
			return nil, nil
		}
		return []Finding{healthFinding("activation_missing", "")}, nil
	}
	kind, detail := probeOpenClawHealth(act.BaseURL)
	if kind == "" {
		return nil, nil // healthy
	}
	return []Finding{healthFinding(kind, detail)}, nil
}

// healthFinding builds the openclaw-health finding for a given sub-case.
func healthFinding(kind, detail string) Finding {
	return Finding{
		ID:         fmOpenClawHealthUnreachable,
		Severity:   "P2",
		Subsystem:  subsystemBridges,
		Title:      "OpenClaw consumer health unreachable (" + kind + ")",
		Confidence: 1.0,
		Evidence: Evidence{
			Query: "curl -fsS --max-time 2 \"$DAEMON_URL/openclaw/v1/health\"",
			File:  kind + ":" + detail,
		},
		Remediation: detectOnlyRemediation(fmOpenClawHealthUnreachable),
	}
}

// probeOpenClawHealth performs a bounded read-only GET against the health
// endpoint and classifies the result. Returns ("", "") when healthy.
func probeOpenClawHealth(baseURL string) (kind, detail string) {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		return "unreachable", "empty base_url in activation.json"
	}
	url := base + "/openclaw/v1/health"
	body, status, err := httpGetBounded(url, 3*time.Second)
	switch {
	case err != nil:
		if strings.Contains(strings.ToLower(err.Error()), "timeout") ||
			strings.Contains(strings.ToLower(err.Error()), "deadline") {
			return "slow_or_hung", base
		}
		return "unreachable", base + " (" + truncatePayload(err.Error()) + ")"
	case status == 404:
		return "route_missing", base
	case status/100 != 2:
		return "http_error", fmt.Sprintf("%s HTTP %d", base, status)
	default:
		var h struct {
			Status string `json:"status"`
		}
		if json.Unmarshal(body, &h) == nil && h.Status == "ok" {
			return "", ""
		}
		return "not_ok", base + " " + truncatePayload(string(body))
	}
}

// openclawHealthUnreachableFixer is detect-only: it never starts/restarts the
// OpenClaw daemon and never rewrites the activation file.
type openclawHealthUnreachableFixer struct{}

func (openclawHealthUnreachableFixer) ID() string { return fmOpenClawHealthUnreachable }
func (openclawHealthUnreachableFixer) Preconditions() []string {
	return []string{"online_required", "run_dir_writable"}
}
func (openclawHealthUnreachableFixer) WritesTo() []string {
	return []string{".doctor/runs/<run-id>/reports"}
}
func (openclawHealthUnreachableFixer) Ops() []string     { return []string{"WriteFile"} }
func (openclawHealthUnreachableFixer) Reversible() bool  { return true }
func (openclawHealthUnreachableFixer) Idempotent() bool  { return true }
func (openclawHealthUnreachableFixer) AutoFixable() bool { return false }

// Fix re-runs the detector and reports the remediation guidance. It starts
// nothing and rewrites no activation file. The finding persists.
func (openclawHealthUnreachableFixer) Fix(ctx *MutateContext, env *DetectEnv, _ []Finding) (FixResult, error) {
	fs, err := openclawHealthUnreachableDetector{}.Detect(env)
	if err != nil {
		return FixResult{FixerID: fmOpenClawHealthUnreachable, Err: err}, err
	}
	if len(fs) == 0 {
		return FixResult{FixerID: fmOpenClawHealthUnreachable, Fixed: true}, nil
	}
	parts := strings.SplitN(fs[0].Evidence.File, ":", 2)
	kind := parts[0]
	detail := ""
	if len(parts) == 2 {
		detail = parts[1]
	}
	report := openclawHealthReport(kind, detail)
	actions, err := writeReport(ctx, fmOpenClawHealthUnreachable+".txt", report)
	if err != nil {
		return FixResult{FixerID: fmOpenClawHealthUnreachable, ActionsTaken: actions, Err: err}, err
	}
	return FixResult{
		FixerID:      fmOpenClawHealthUnreachable,
		FindingIDs:   []string{fmOpenClawHealthUnreachable},
		ActionsTaken: actions,
		Fixed:        false,
	}, nil
}

// openclawHealthReport builds the operator instruction for a health sub-case.
func openclawHealthReport(kind, detail string) string {
	switch kind {
	case "activation_missing":
		return "The OpenClaw activation file `.agents/daemon/activation.json` is " +
			"absent. The in-repo daemon that wrote it was removed (ADR-0009), so " +
			"`ao` no longer starts a daemon in-session. Bring up the OpenClaw " +
			"daemon out-of-band, then re-run `ao doctor`. The doctor will not " +
			"start the daemon."
	case "unreachable":
		return fmt.Sprintf("OpenClaw health endpoint %s is unreachable. The daemon "+
			"is down or the activation file points at a dead port. Bring the "+
			"OpenClaw daemon back up out-of-band (the in-repo daemon was removed — "+
			"ADR-0009). The doctor will not restart the daemon or rewrite the "+
			"activation file.", detail)
	case "slow_or_hung":
		return fmt.Sprintf("OpenClaw health endpoint %s did not respond within 3s. "+
			"The daemon is slow or wedged. Inspect and restart the OpenClaw daemon "+
			"out-of-band; the doctor will not manage the daemon.", detail)
	case "route_missing":
		return fmt.Sprintf("The daemon at %s answers but `/openclaw/v1/health` "+
			"returns 404 — this OpenClaw build predates the consumer routes. "+
			"Upgrade and restart the OpenClaw daemon out-of-band.", detail)
	case "http_error":
		return fmt.Sprintf("OpenClaw health endpoint %s returned an HTTP error. "+
			"Inspect the daemon log and restart the OpenClaw daemon out-of-band if "+
			"it crashed.", detail)
	default: // not_ok
		return fmt.Sprintf("OpenClaw health endpoint %s responded but status != ok. "+
			"Inspect the daemon log and restart the OpenClaw daemon out-of-band if "+
			"needed.", detail)
	}
}

// ---------------------------------------------------------------------------
// FM 2: fm-bridges-openclaw-snapshot-stale — PARTIAL (torn-latest auto-fix).
// ---------------------------------------------------------------------------

// snapshotObservation classifies the OpenClaw consumer snapshot state.
type snapshotObservation struct {
	kind            string // "not_configured" | "file_ok" | "latest_missing" | "schema_mismatch" | "torn_latest" | "torn_no_recovery"
	schemaVersion   int
	goodVersionFile string // basename of the recovery snap_*.json (torn_latest only)
}

// openclawSnapshotStaleDetector fires when the OpenClaw consumer snapshot is
// torn, schema-mismatched, or absent.
type openclawSnapshotStaleDetector struct{}

func (openclawSnapshotStaleDetector) ID() string        { return fmOpenClawSnapshotStale }
func (openclawSnapshotStaleDetector) Subsystem() string { return subsystemBridges }
func (openclawSnapshotStaleDetector) Severity() string  { return "P2" }
func (openclawSnapshotStaleDetector) Describe() string {
	return "OpenClaw consumer snapshot is stale, torn, or schema-mismatched"
}
func (openclawSnapshotStaleDetector) EstimatedCostMS() int { return 30 }
func (openclawSnapshotStaleDetector) OnlineRequired() bool { return false }
func (openclawSnapshotStaleDetector) QuickPath() bool      { return false }

// observeSnapshot inspects the on-disk OpenClaw snapshot directory. Pure: it
// only reads files and parses JSON.
func observeSnapshot(repoRoot string) snapshotObservation {
	snapDir := filepath.Join(repoRoot, openclaw.SnapshotDirRel)
	if _, err := os.Stat(snapDir); err != nil {
		if os.IsNotExist(err) {
			if _, activated := readDaemonActivation(repoRoot); !activated {
				return snapshotObservation{kind: "not_configured"}
			}
		}
		return snapshotObservation{kind: "latest_missing"}
	}
	latestPath := filepath.Join(snapDir, "latest.json")
	raw, err := os.ReadFile(latestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return snapshotObservation{kind: "latest_missing"}
		}
		return snapshotObservation{kind: "latest_missing"}
	}
	snap, parseErr := openclaw.ParseConsumerSnapshot(raw)
	if parseErr == nil {
		return snapshotObservation{kind: "file_ok", schemaVersion: snap.SchemaVersion}
	}
	// A parseable-but-wrong-version file: classify as schema mismatch.
	if v, ok := schemaVersionOf(raw); ok && v != openclaw.ConsumerSnapshotSchemaVersion {
		return snapshotObservation{kind: "schema_mismatch", schemaVersion: v}
	}
	// Torn/truncated/empty: look for a recoverable versioned sibling.
	good := newestValidVersionSnapshot(snapDir)
	if good == "" {
		return snapshotObservation{kind: "torn_no_recovery"}
	}
	return snapshotObservation{kind: "torn_latest", goodVersionFile: good}
}

// schemaVersionOf extracts the schema_version field from raw JSON without full
// validation. Pure.
func schemaVersionOf(raw []byte) (int, bool) {
	var probe struct {
		SchemaVersion *int `json:"schema_version"`
	}
	if json.Unmarshal(raw, &probe) != nil || probe.SchemaVersion == nil {
		return 0, false
	}
	return *probe.SchemaVersion, true
}

// newestValidVersionSnapshot scans snapDir for snap_*.json files that parse
// cleanly as schema-v1 snapshots and returns the basename of the newest by
// generated_at. Returns "" when none is recoverable. Pure.
func newestValidVersionSnapshot(snapDir string) string {
	entries, err := os.ReadDir(snapDir)
	if err != nil {
		return ""
	}
	var bestName string
	var bestTime time.Time
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasPrefix(name, "snap_") || !strings.HasSuffix(name, ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(snapDir, name))
		if err != nil {
			continue
		}
		snap, err := openclaw.ParseConsumerSnapshot(raw)
		if err != nil || snap.SchemaVersion != openclaw.ConsumerSnapshotSchemaVersion {
			continue
		}
		gen, err := time.Parse(time.RFC3339Nano, snap.GeneratedAt)
		if err != nil {
			continue
		}
		if bestName == "" || gen.After(bestTime) {
			bestName, bestTime = name, gen
		}
	}
	return bestName
}

// Detect inspects the on-disk OpenClaw snapshot and emits a finding when it is
// not healthy. auto_fixable is true ONLY for the torn-latest sub-case. PURE.
func (openclawSnapshotStaleDetector) Detect(env *DetectEnv) ([]Finding, error) {
	obs := observeSnapshot(env.RepoRoot)
	if obs.kind == "not_configured" || obs.kind == "file_ok" {
		return nil, nil
	}
	autoFixable := obs.kind == "torn_latest"
	rem := Remediation{
		Command:          "ao doctor --fix --only " + fmOpenClawSnapshotStale,
		ExplainCommand:   "ao doctor explain " + fmOpenClawSnapshotStale,
		AutoFixable:      autoFixable,
		EstimatedActions: 1,
	}
	return []Finding{{
		ID:         fmOpenClawSnapshotStale,
		Severity:   "P2",
		Subsystem:  subsystemBridges,
		Title:      "OpenClaw consumer snapshot not current (" + obs.kind + ")",
		Confidence: 1.0,
		Evidence: Evidence{
			Query: "jq -e '.schema_version==1' .agents/daemon/projections/openclaw/latest.json",
			File:  obs.kind + ":" + obs.goodVersionFile,
		},
		Remediation: rem,
	}}, nil
}

// openclawSnapshotStaleFixer is PARTIAL: only the torn-latest sub-case is
// auto-fixed (by reconstructing latest.json verbatim from the intact versioned
// snapshot). All other sub-cases are detect-only.
type openclawSnapshotStaleFixer struct{}

func (openclawSnapshotStaleFixer) ID() string { return fmOpenClawSnapshotStale }
func (openclawSnapshotStaleFixer) Preconditions() []string {
	return []string{"recoverable_versioned_snapshot", "latest_json_unlocked"}
}
func (openclawSnapshotStaleFixer) WritesTo() []string {
	return []string{openclaw.SnapshotDirRel, ".doctor/runs/<run-id>/reports"}
}
func (openclawSnapshotStaleFixer) Ops() []string     { return []string{"WriteFile"} }
func (openclawSnapshotStaleFixer) Reversible() bool  { return true }
func (openclawSnapshotStaleFixer) Idempotent() bool  { return true }
func (openclawSnapshotStaleFixer) AutoFixable() bool { return true }

// Fix reconstructs a torn latest.json from the newest intact versioned
// snapshot via a single Mutate WriteFile. For every other sub-case it refuses
// and writes a detect-only advisory report through Mutate.
func (openclawSnapshotStaleFixer) Fix(ctx *MutateContext, env *DetectEnv, _ []Finding) (FixResult, error) {
	obs := observeSnapshot(env.RepoRoot)
	if obs.kind == "not_configured" || obs.kind == "file_ok" {
		return FixResult{FixerID: fmOpenClawSnapshotStale, Fixed: true}, nil
	}
	if obs.kind == "torn_latest" {
		return fixTornLatest(ctx, env, obs)
	}
	// Detect-only sub-cases: write an advisory report, do not bail.
	report := snapshotStaleReport(obs)
	actions, err := writeReport(ctx, fmOpenClawSnapshotStale+".txt", report)
	if err != nil {
		return FixResult{FixerID: fmOpenClawSnapshotStale, ActionsTaken: actions, Err: err}, err
	}
	return FixResult{
		FixerID:      fmOpenClawSnapshotStale,
		FindingIDs:   []string{fmOpenClawSnapshotStale},
		ActionsTaken: actions,
		Fixed:        false,
	}, nil
}

// fixTornLatest reconstructs latest.json from the intact versioned snapshot.
// It re-validates the recovery source NOW and refuses if it no longer parses.
func fixTornLatest(ctx *MutateContext, env *DetectEnv, obs snapshotObservation) (FixResult, error) {
	snapDir := filepath.Join(env.RepoRoot, openclaw.SnapshotDirRel)
	goodPath := filepath.Join(snapDir, obs.goodVersionFile)
	goodBytes, err := os.ReadFile(goodPath)
	if err != nil {
		return FixResult{FixerID: fmOpenClawSnapshotStale, Err: err}, fmt.Errorf("doctor: read recovery snapshot: %w", err)
	}
	snap, err := openclaw.ParseConsumerSnapshot(goodBytes)
	if err != nil || snap.SchemaVersion != openclaw.ConsumerSnapshotSchemaVersion {
		return FixResult{FixerID: fmOpenClawSnapshotStale, Err: err},
			fmt.Errorf("doctor: recovery source %s no longer valid (refused_unsafe)", obs.goodVersionFile)
	}
	latestPath := filepath.Join(snapDir, "latest.json")
	res, err := Mutate(ctx, latestPath, WriteFile{Content: goodBytes, Mode: 0o600})
	if err != nil {
		return FixResult{FixerID: fmOpenClawSnapshotStale, Err: err}, err
	}
	actions := 0
	if res.OK {
		actions = 1
	}
	// Verify the torn-latest finding was eliminated.
	post := observeSnapshot(env.RepoRoot)
	if !ctx.DryRun && post.kind != "file_ok" {
		return FixResult{FixerID: fmOpenClawSnapshotStale, ActionsTaken: actions, Err: fmt.Errorf("doctor: fix did not eliminate torn-latest finding")},
			fmt.Errorf("doctor: fix did not eliminate torn-latest finding")
	}
	return FixResult{
		FixerID:      fmOpenClawSnapshotStale,
		FindingIDs:   []string{fmOpenClawSnapshotStale},
		ActionsTaken: actions,
		Fixed:        true,
	}, nil
}

// snapshotStaleReport builds the operator instruction for a detect-only
// snapshot sub-case.
func snapshotStaleReport(obs snapshotObservation) string {
	switch obs.kind {
	case "schema_mismatch":
		return fmt.Sprintf("`latest.json` schema_version=%d, but the bridge "+
			"requires version %d. A daemon upgrade bumped the snapshot schema. "+
			"The in-repo daemon that emitted this projection was removed "+
			"(ADR-0009); regenerate the OpenClaw projection from your out-of-session "+
			"substrate so it re-emits a v%d snapshot. The doctor will not fabricate "+
			"a schema downgrade.", obs.schemaVersion,
			openclaw.ConsumerSnapshotSchemaVersion,
			openclaw.ConsumerSnapshotSchemaVersion)
	case "latest_missing":
		return "`latest.json` is absent and no versioned snapshot exists to " +
			"recover from. The in-repo daemon that emitted this projection was " +
			"removed (ADR-0009); regenerate the OpenClaw projection from your " +
			"out-of-session substrate."
	case "torn_no_recovery":
		return "`latest.json` is torn/truncated and no byte-valid versioned " +
			"`snap_*.json` exists to recover from. The in-repo daemon that emitted " +
			"this projection was removed (ADR-0009); regenerate the OpenClaw " +
			"projection from your out-of-session substrate."
	default:
		return "OpenClaw snapshot is not current. The in-repo daemon that emitted " +
			"this projection was removed (ADR-0009); regenerate the OpenClaw " +
			"projection from your out-of-session substrate."
	}
}
