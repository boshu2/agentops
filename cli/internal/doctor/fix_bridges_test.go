package doctor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/openclaw"
)

// newBridgesTestCtx builds a real MutateContext rooted at a temp repo, with a
// real run artifact (run dir, backups, reports, actions.jsonl).
func newBridgesTestCtx(t *testing.T, repoRoot string) (*MutateContext, *RunArtifact) {
	t.Helper()
	ra, err := NewRunArtifact(repoRoot, "deadbeef", time.Now())
	if err != nil {
		t.Fatalf("NewRunArtifact: %v", err)
	}
	actionsFile, err := ra.OpenActionsFile()
	if err != nil {
		t.Fatalf("OpenActionsFile: %v", err)
	}
	t.Cleanup(func() { _ = actionsFile.Close() })
	caps := NewCapabilities("test")
	locks := NewLockManager(filepath.Join(repoRoot, ".doctor", "locks"))
	ctx := NewMutateContext(ra, caps, repoRoot, locks, actionsFile, false)
	return ctx.WithFixer("test"), ra
}

// validSnapshotBytes returns byte-valid, schema-v1 ConsumerSnapshot JSON.
func validSnapshotBytes(t *testing.T, id string, generatedAt time.Time) []byte {
	t.Helper()
	snap := openclaw.ConsumerSnapshot{
		SchemaVersion: openclaw.ConsumerSnapshotSchemaVersion,
		SnapshotID:    id,
		GeneratedAt:   generatedAt.UTC().Format(time.RFC3339Nano),
		Source:        openclaw.SnapshotSource{Ledger: "test-ledger"},
		Status:        openclaw.SnapshotStatusCurrent,
		Resources: openclaw.SnapshotResources{
			Runs: []openclaw.ResourceSummary{},
			Jobs: []openclaw.ResourceSummary{},
			Wiki: []openclaw.ResourceSummary{},
		},
	}
	if err := openclaw.ValidateConsumerSnapshot(snap); err != nil {
		t.Fatalf("fixture snapshot invalid: %v", err)
	}
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		t.Fatalf("marshal fixture snapshot: %v", err)
	}
	return append(data, '\n')
}

// TestBridgesRegistration verifies all 6 detectors and 6 fixers are registered.
func TestBridgesRegistration(t *testing.T) {
	ids := []string{
		fmGCBinaryMissing, fmGCVersionIncompatible, fmGCControllerDown,
		fmGCStatusParseError, fmOpenClawHealthUnreachable, fmOpenClawSnapshotStale,
	}
	for _, id := range ids {
		var found bool
		for _, d := range Detectors() {
			if d.ID() == id {
				found = true
				if d.Subsystem() != subsystemBridges {
					t.Errorf("detector %s subsystem = %q, want %q", id, d.Subsystem(), subsystemBridges)
				}
			}
		}
		if !found {
			t.Errorf("detector %s not registered", id)
		}
		fx := FixerByID(id)
		if fx == nil {
			t.Fatalf("fixer %s not registered", id)
		}
	}
}

// TestBridgesAutoFixableFlags verifies exactly one fixer is auto-fixable.
func TestBridgesAutoFixableFlags(t *testing.T) {
	cases := map[string]bool{
		fmGCBinaryMissing:           false,
		fmGCVersionIncompatible:     false,
		fmGCControllerDown:          false,
		fmGCStatusParseError:        false,
		fmOpenClawHealthUnreachable: false,
		fmOpenClawSnapshotStale:     true,
	}
	autoCount := 0
	for id, want := range cases {
		fx := FixerByID(id)
		if fx == nil {
			t.Fatalf("fixer %s missing", id)
		}
		if got := fx.AutoFixable(); got != want {
			t.Errorf("fixer %s AutoFixable() = %v, want %v", id, got, want)
		}
		if fx.AutoFixable() {
			autoCount++
		}
	}
	if autoCount != 1 {
		t.Errorf("auto-fixable fixer count = %d, want 1", autoCount)
	}
}

// TestBridgesGCBinaryMissingDetector verifies the detector classifies a missing
// `gc` binary by sanitizing PATH to a directory with no `gc`.
func TestBridgesGCBinaryMissingDetector(t *testing.T) {
	repo := t.TempDir()
	emptyBin := t.TempDir()
	t.Setenv("PATH", emptyBin)

	env := &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: t.TempDir()}
	fs, err := gcBinaryMissingDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("findings = %d, want 1", len(fs))
	}
	if fs[0].ID != fmGCBinaryMissing {
		t.Errorf("finding ID = %q, want %q", fs[0].ID, fmGCBinaryMissing)
	}
	if fs[0].Severity != "P2" {
		t.Errorf("severity = %q, want P2", fs[0].Severity)
	}
	if fs[0].Remediation.AutoFixable {
		t.Error("binary-missing finding must not be auto-fixable")
	}
}

// TestBridgesGCBinaryMissingFixerRefuses verifies the detect-only fixer writes
// exactly one report, marks Fixed=false, and never installs a binary.
func TestBridgesGCBinaryMissingFixerRefuses(t *testing.T) {
	repo := t.TempDir()
	emptyBin := t.TempDir()
	t.Setenv("PATH", emptyBin)

	ctx, ra := newBridgesTestCtx(t, repo)
	env := &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: t.TempDir()}

	res, err := gcBinaryMissingFixer{}.Fix(ctx, env, nil)
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}
	if res.Fixed {
		t.Error("detect-only fixer must report Fixed=false")
	}
	if res.ActionsTaken != 1 {
		t.Errorf("ActionsTaken = %d, want 1", res.ActionsTaken)
	}
	reportFile := filepath.Join(ra.RunDir, "reports", fmGCBinaryMissing+".txt")
	body, err := os.ReadFile(reportFile)
	if err != nil {
		t.Fatalf("report not written: %v", err)
	}
	if !strings.Contains(string(body), "ao doctor") && !strings.Contains(string(body), "Install GasCity") {
		t.Errorf("report does not name an operator action: %q", body)
	}
	assertActionLine(t, ra)
}

// TestBridgesGCStatusDriftClassification verifies the pure JSON drift classifier.
func TestBridgesGCStatusDriftClassification(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		kind    string
		missing []string
	}{
		{"missing_controller", `{"agents":[],"summary":{}}`, "missing_top_level_fields", []string{"controller"}},
		{"wrapped", `{"data":{"controller":{"running":true}}}`, "wrapped_envelope", []string{"agents", "controller", "summary"}},
		{"not_json", `not json at all`, "not_json", nil},
		{"nested", `{"controller":{"x":1},"agents":[],"summary":{}}`, "nested_shape_drift", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kind, missing := classifyGCStatusDrift([]byte(tc.payload))
			if kind != tc.kind {
				t.Errorf("kind = %q, want %q", kind, tc.kind)
			}
			if strings.Join(missing, ",") != strings.Join(tc.missing, ",") {
				t.Errorf("missing = %v, want %v", missing, tc.missing)
			}
		})
	}
}

// TestBridgesGCVersionParsing verifies the pure semver helpers.
func TestBridgesGCVersionParsing(t *testing.T) {
	if got := gcVersionToken("gc version 0.12.4"); got != "0.12.4" {
		t.Errorf("token = %q, want 0.12.4", got)
	}
	if got := gcVersionToken("GasCity build (dev)"); got != "" {
		t.Errorf("token for non-semver = %q, want empty", got)
	}
	if compareSemverParts(semverParts("0.12.4"), semverParts("0.13.0")) >= 0 {
		t.Error("0.12.4 must compare below 0.13.0")
	}
	if compareSemverParts(semverParts("0.13.2"), semverParts("0.13.0")) < 0 {
		t.Error("0.13.2 must compare at or above 0.13.0")
	}
}

// TestBridgesOpenClawHealthActivationMissing verifies the detector fires the
// activation-missing sub-case when there is no activation file.
func TestBridgesOpenClawHealthActivationMissing(t *testing.T) {
	repo := t.TempDir()
	env := &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: t.TempDir(), Online: true}

	fs, err := openclawHealthUnreachableDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("findings = %d, want 1", len(fs))
	}
	if !strings.HasPrefix(fs[0].Evidence.File, "activation_missing:") {
		t.Errorf("evidence = %q, want activation_missing classification", fs[0].Evidence.File)
	}
	if fs[0].Remediation.AutoFixable {
		t.Error("health-unreachable finding must not be auto-fixable")
	}
}

// TestBridgesOpenClawHealthUnreachableFixerRefuses verifies the detect-only
// fixer reports without rewriting the activation file.
func TestBridgesOpenClawHealthUnreachableFixerRefuses(t *testing.T) {
	repo := t.TempDir()
	// Activation pointing at a port that deterministically refuses connections.
	actDir := filepath.Join(repo, ".agents", "daemon")
	if err := os.MkdirAll(actDir, 0o755); err != nil {
		t.Fatal(err)
	}
	actPath := filepath.Join(actDir, "activation.json")
	actBytes := []byte(`{"base_url":"http://127.0.0.1:1"}`)
	if err := os.WriteFile(actPath, actBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, ra := newBridgesTestCtx(t, repo)
	env := &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: t.TempDir(), Online: true}

	res, err := openclawHealthUnreachableFixer{}.Fix(ctx, env, nil)
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}
	if res.Fixed {
		t.Error("detect-only fixer must report Fixed=false")
	}
	if res.ActionsTaken != 1 {
		t.Errorf("ActionsTaken = %d, want 1", res.ActionsTaken)
	}
	// The fixer must NOT have rewritten the activation file.
	after, err := os.ReadFile(actPath)
	if err != nil {
		t.Fatalf("activation.json gone: %v", err)
	}
	if string(after) != string(actBytes) {
		t.Errorf("activation.json was rewritten: got %q", after)
	}
	reportFile := filepath.Join(ra.RunDir, "reports", fmOpenClawHealthUnreachable+".txt")
	body, err := os.ReadFile(reportFile)
	if err != nil {
		t.Fatalf("report not written: %v", err)
	}
	if !strings.Contains(string(body), "ao agentopsd") {
		t.Errorf("report does not name ao agentopsd: %q", body)
	}
}

// TestBridgesSnapshotTornLatestDetect verifies the detector classifies a torn
// latest.json with an intact versioned sibling as the auto-fixable sub-case.
func TestBridgesSnapshotTornLatestDetect(t *testing.T) {
	repo := t.TempDir()
	snapDir := filepath.Join(repo, openclaw.SnapshotDirRel)
	if err := os.MkdirAll(snapDir, 0o700); err != nil {
		t.Fatal(err)
	}
	good := validSnapshotBytes(t, "snap_evt0001", time.Now())
	if err := os.WriteFile(filepath.Join(snapDir, "snap_evt0001.json"), good, 0o600); err != nil {
		t.Fatal(err)
	}
	// latest.json: torn — first 40 bytes of valid JSON (decode error).
	if err := os.WriteFile(filepath.Join(snapDir, "latest.json"), good[:40], 0o600); err != nil {
		t.Fatal(err)
	}

	env := &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: t.TempDir()}
	fs, err := openclawSnapshotStaleDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("findings = %d, want 1", len(fs))
	}
	if !fs[0].Remediation.AutoFixable {
		t.Error("torn_latest finding must be auto-fixable")
	}
	if !strings.HasPrefix(fs[0].Evidence.File, "torn_latest:snap_evt0001.json") {
		t.Errorf("evidence = %q, want torn_latest:snap_evt0001.json", fs[0].Evidence.File)
	}
}

// TestBridgesSnapshotTornLatestFix verifies the partial fixer reconstructs
// latest.json verbatim from the intact versioned snapshot, leaves snap_*.json
// untouched, and records an action line + backup.
func TestBridgesSnapshotTornLatestFix(t *testing.T) {
	repo := t.TempDir()
	snapDir := filepath.Join(repo, openclaw.SnapshotDirRel)
	if err := os.MkdirAll(snapDir, 0o700); err != nil {
		t.Fatal(err)
	}
	good := validSnapshotBytes(t, "snap_evt0001", time.Now())
	goodPath := filepath.Join(snapDir, "snap_evt0001.json")
	if err := os.WriteFile(goodPath, good, 0o600); err != nil {
		t.Fatal(err)
	}
	tornBytes := good[:40]
	latestPath := filepath.Join(snapDir, "latest.json")
	if err := os.WriteFile(latestPath, tornBytes, 0o600); err != nil {
		t.Fatal(err)
	}

	ctx, ra := newBridgesTestCtx(t, repo)
	env := &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: t.TempDir()}

	res, err := openclawSnapshotStaleFixer{}.Fix(ctx, env, nil)
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}
	if !res.Fixed {
		t.Error("torn-latest fix must report Fixed=true")
	}
	if res.ActionsTaken != 1 {
		t.Errorf("ActionsTaken = %d, want 1", res.ActionsTaken)
	}

	// latest.json reconstructed byte-identical to the versioned snapshot.
	rebuilt, err := os.ReadFile(latestPath)
	if err != nil {
		t.Fatalf("read rebuilt latest.json: %v", err)
	}
	if string(rebuilt) != string(good) {
		t.Errorf("latest.json not byte-identical to snap_evt0001.json")
	}

	// Versioned snapshot untouched.
	srcAfter, err := os.ReadFile(goodPath)
	if err != nil {
		t.Fatalf("read snap_evt0001.json: %v", err)
	}
	if string(srcAfter) != string(good) {
		t.Error("snap_evt0001.json was modified — recovery source must be read-only")
	}

	// Backup of the torn latest.json exists and matches the pre-fix torn bytes.
	backup := filepath.Join(ra.RunDir, "backups", openclaw.SnapshotDirRel, "latest.json")
	backupBytes, err := os.ReadFile(backup)
	if err != nil {
		t.Fatalf("torn-latest backup missing: %v", err)
	}
	if string(backupBytes) != string(tornBytes) {
		t.Errorf("backup is not the torn pre-fix bytes: got %q want %q", backupBytes, tornBytes)
	}

	// actions.jsonl recorded the WriteFile.
	rec := assertActionLine(t, ra)
	if rec.Op != "WriteFile" {
		t.Errorf("action op = %q, want WriteFile", rec.Op)
	}
	if !strings.HasSuffix(rec.Path, "latest.json") {
		t.Errorf("action path = %q, want suffix latest.json", rec.Path)
	}

	// Idempotence: a second fix is a no-op (file already healthy).
	res2, err := openclawSnapshotStaleFixer{}.Fix(ctx.WithFixer("test2"), env, nil)
	if err != nil {
		t.Fatalf("second Fix: %v", err)
	}
	if res2.ActionsTaken != 0 {
		t.Errorf("idempotent second fix ActionsTaken = %d, want 0", res2.ActionsTaken)
	}
}

// TestBridgesSnapshotSchemaMismatchRefuses verifies a schema-bumped latest.json
// is reported (not fixed) — the doctor never fabricates a downgrade.
func TestBridgesSnapshotSchemaMismatchRefuses(t *testing.T) {
	repo := t.TempDir()
	snapDir := filepath.Join(repo, openclaw.SnapshotDirRel)
	if err := os.MkdirAll(snapDir, 0o700); err != nil {
		t.Fatal(err)
	}
	// A parseable JSON object with schema_version=2.
	v2 := []byte(`{"schema_version":2,"snapshot_id":"snap_x","generated_at":"2026-01-01T00:00:00Z","source":{"ledger":"l"},"status":"current","resources":{"runs":[],"jobs":[],"wiki":[]}}` + "\n")
	latestPath := filepath.Join(snapDir, "latest.json")
	if err := os.WriteFile(latestPath, v2, 0o600); err != nil {
		t.Fatal(err)
	}

	ctx, ra := newBridgesTestCtx(t, repo)
	env := &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: t.TempDir()}

	// Detector classifies as schema_mismatch, not auto-fixable.
	fs, err := openclawSnapshotStaleDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("findings = %d, want 1", len(fs))
	}
	if fs[0].Remediation.AutoFixable {
		t.Error("schema_mismatch finding must NOT be auto-fixable")
	}
	if !strings.HasPrefix(fs[0].Evidence.File, "schema_mismatch:") {
		t.Errorf("evidence = %q, want schema_mismatch", fs[0].Evidence.File)
	}

	// Fixer reports without rewriting latest.json.
	res, err := openclawSnapshotStaleFixer{}.Fix(ctx, env, nil)
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}
	if res.Fixed {
		t.Error("schema-mismatch sub-case must report Fixed=false")
	}
	after, err := os.ReadFile(latestPath)
	if err != nil {
		t.Fatalf("latest.json gone: %v", err)
	}
	if string(after) != string(v2) {
		t.Error("latest.json was rewritten — schema downgrade must be refused")
	}
	reportFile := filepath.Join(ra.RunDir, "reports", fmOpenClawSnapshotStale+".txt")
	body, err := os.ReadFile(reportFile)
	if err != nil {
		t.Fatalf("report not written: %v", err)
	}
	if !strings.Contains(string(body), "projection rebuild") {
		t.Errorf("report does not name the rebuild command: %q", body)
	}
}

// assertActionLine asserts that the run's actions.jsonl has at least one line
// and returns the last record.
func assertActionLine(t *testing.T, ra *RunArtifact) ActionRecord {
	t.Helper()
	recs, err := readActions(ra.ActionsPath())
	if err != nil {
		t.Fatalf("readActions: %v", err)
	}
	if len(recs) == 0 {
		t.Fatal("actions.jsonl has no records")
	}
	return recs[len(recs)-1]
}
