package overnight

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestValidateRunLoopOptions exercises the pure validation helper across
// the Cwd/OutputDir/RunID matrix. Error strings are asserted verbatim so
// downstream logs that match them remain stable.
func TestValidateRunLoopOptions(t *testing.T) {
	tests := []struct {
		name    string
		opts    RunLoopOptions
		wantErr string
	}{
		{
			name:    "missing cwd",
			opts:    RunLoopOptions{OutputDir: "out", RunID: "r1"},
			wantErr: "overnight: RunLoopOptions.Cwd is required",
		},
		{
			name:    "missing output dir",
			opts:    RunLoopOptions{Cwd: "/tmp", RunID: "r1"},
			wantErr: "overnight: RunLoopOptions.OutputDir is required",
		},
		{
			name:    "missing run id",
			opts:    RunLoopOptions{Cwd: "/tmp", OutputDir: "out"},
			wantErr: "overnight: RunLoopOptions.RunID is required",
		},
		{
			name:    "all fields present",
			opts:    RunLoopOptions{Cwd: "/tmp", OutputDir: "out", RunID: "r1"},
			wantErr: "",
		},
		{
			name:    "cwd checked before output dir",
			opts:    RunLoopOptions{},
			wantErr: "overnight: RunLoopOptions.Cwd is required",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRunLoopOptions(tt.opts)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validateRunLoopOptions: got error %q, want nil", err.Error())
				}
				return
			}
			if err == nil {
				t.Fatalf("validateRunLoopOptions: got nil, want error %q", tt.wantErr)
			}
			if got := err.Error(); got != tt.wantErr {
				t.Fatalf("validateRunLoopOptions: got %q, want %q", got, tt.wantErr)
			}
		})
	}
}

// TestNewRunLoopResult verifies the constructor copies warn-only ratchet
// fields into the result and preserves the caller-supplied iterations and
// degraded slices.
func TestNewRunLoopResult(t *testing.T) {
	priors := []IterationSummary{
		{Index: 1, Status: StatusDone},
		{Index: 2, Status: StatusDegraded},
	}
	degraded := []string{"note a", "note b"}

	t.Run("no ratchet leaves budget zero", func(t *testing.T) {
		got := newRunLoopResult(priors, degraded, RunLoopOptions{})
		if got.WarnOnlyBudgetInitial != 0 {
			t.Fatalf("WarnOnlyBudgetInitial: got %d, want 0", got.WarnOnlyBudgetInitial)
		}
		if got.WarnOnlyBudgetRemaining != 0 {
			t.Fatalf("WarnOnlyBudgetRemaining: got %d, want 0", got.WarnOnlyBudgetRemaining)
		}
		if len(got.Iterations) != 2 {
			t.Fatalf("Iterations length: got %d, want 2", len(got.Iterations))
		}
		if got.Iterations[0].Index != 1 {
			t.Fatalf("Iterations[0].Index: got %d, want 1", got.Iterations[0].Index)
		}
		if len(got.Degraded) != 2 {
			t.Fatalf("Degraded length: got %d, want 2", len(got.Degraded))
		}
		if got.Degraded[0] != "note a" {
			t.Fatalf("Degraded[0]: got %q, want %q", got.Degraded[0], "note a")
		}
	})

	t.Run("ratchet populates initial and remaining", func(t *testing.T) {
		opts := RunLoopOptions{
			WarnOnlyBudget: &WarnOnlyRatchet{Initial: 7, Remaining: 3},
		}
		got := newRunLoopResult(nil, nil, opts)
		if got.WarnOnlyBudgetInitial != 7 {
			t.Fatalf("WarnOnlyBudgetInitial: got %d, want 7", got.WarnOnlyBudgetInitial)
		}
		if got.WarnOnlyBudgetRemaining != 3 {
			t.Fatalf("WarnOnlyBudgetRemaining: got %d, want 3", got.WarnOnlyBudgetRemaining)
		}
	})
}

// TestNewLoopIteration asserts the deterministic ID format and initial
// status.
func TestNewLoopIteration(t *testing.T) {
	start := time.Date(2026, 4, 22, 10, 30, 0, 0, time.UTC)
	got := newLoopIteration("run-abc", 5, start)
	if string(got.ID) != "run-abc-iter-5" {
		t.Fatalf("ID: got %q, want %q", string(got.ID), "run-abc-iter-5")
	}
	if got.Index != 5 {
		t.Fatalf("Index: got %d, want 5", got.Index)
	}
	if !got.StartedAt.Equal(start) {
		t.Fatalf("StartedAt: got %v, want %v", got.StartedAt, start)
	}
	if got.Status != StatusDone {
		t.Fatalf("Status: got %q, want %q", got.Status, StatusDone)
	}
}

// TestFinishIteration asserts FinishedAt is set in the future of iterStart
// and Duration is a parseable non-negative string.
func TestFinishIteration(t *testing.T) {
	iter := &IterationSummary{}
	start := time.Now().Add(-2 * time.Second)
	finishIteration(iter, start)
	if iter.FinishedAt.Before(start) {
		t.Fatalf("FinishedAt: got %v, want >= %v", iter.FinishedAt, start)
	}
	d, err := time.ParseDuration(iter.Duration)
	if err != nil {
		t.Fatalf("Duration parse: got error %q, want nil", err.Error())
	}
	if d < 0 {
		t.Fatalf("Duration: got %v, want >= 0", d)
	}
}

// TestLastCompoundedSnapshot walks backwards through prior iterations and
// returns the most recent corpus-compounded iteration's FitnessAfter as a
// FitnessSnapshot.
func TestLastCompoundedSnapshot(t *testing.T) {
	tests := []struct {
		name       string
		priors     []IterationSummary
		wantNil    bool
		wantMetric string
		wantValue  float64
	}{
		{
			name:    "empty slice",
			priors:  nil,
			wantNil: true,
		},
		{
			name: "no compounded entries returns nil",
			priors: []IterationSummary{
				{Status: StatusDegraded, FitnessAfter: map[string]any{"m": 0.5}},
				{Status: StatusRolledBackPreCommit, FitnessAfter: map[string]any{"m": 0.6}},
			},
			wantNil: true,
		},
		{
			name: "picks most recent StatusDone",
			priors: []IterationSummary{
				{Status: StatusDone, FitnessAfter: map[string]any{"m": 0.1}},
				{Status: StatusDone, FitnessAfter: map[string]any{"m": 0.7}},
				{Status: StatusDegraded},
			},
			wantNil:    false,
			wantMetric: "m",
			wantValue:  0.7,
		},
		{
			name: "post-commit halt counts as compounded",
			priors: []IterationSummary{
				{Status: StatusDone, FitnessAfter: map[string]any{"m": 0.2}},
				{Status: StatusHaltedOnRegressionPostCommit, FitnessAfter: map[string]any{"m": 0.9}},
			},
			wantNil:    false,
			wantMetric: "m",
			wantValue:  0.9,
		},
		{
			name: "stops at first compounded even when its map is unusable",
			priors: []IterationSummary{
				{Status: StatusDone, FitnessAfter: map[string]any{"m": 0.3}},
				{Status: StatusDone, FitnessAfter: map[string]any{}},
			},
			wantNil: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lastCompoundedSnapshot(tt.priors)
			if tt.wantNil {
				if got != nil {
					t.Fatalf("lastCompoundedSnapshot: got %+v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("lastCompoundedSnapshot: got nil, want metric %q=%v", tt.wantMetric, tt.wantValue)
			}
			if v := got.Metrics[tt.wantMetric]; v != tt.wantValue {
				t.Fatalf("Metrics[%q]: got %v, want %v", tt.wantMetric, v, tt.wantValue)
			}
		})
	}
}

// TestIngestSummary asserts the exact field-to-key mapping of the summary
// marshal.
func TestIngestSummary(t *testing.T) {
	r := &IngestResult{
		HarvestPreviewCount: 4,
		ForgeArtifactsMined: 5,
		ProvenanceAudited:   6,
		MineFindingsNew:     7,
	}
	got := ingestSummary(r)
	if got["harvest_preview_count"] != 4 {
		t.Fatalf("harvest_preview_count: got %v, want 4", got["harvest_preview_count"])
	}
	if got["forge_artifacts_mined"] != 5 {
		t.Fatalf("forge_artifacts_mined: got %v, want 5", got["forge_artifacts_mined"])
	}
	if got["provenance_audited"] != 6 {
		t.Fatalf("provenance_audited: got %v, want 6", got["provenance_audited"])
	}
	if got["mine_findings_new"] != 7 {
		t.Fatalf("mine_findings_new: got %v, want 7", got["mine_findings_new"])
	}
	if len(got) != 4 {
		t.Fatalf("summary key count: got %d, want 4", len(got))
	}
}

// TestReduceSummary asserts every field of ReduceResult is projected to
// the expected key in the summary map.
func TestReduceSummary(t *testing.T) {
	r := &ReduceResult{
		HarvestPromoted:   1,
		DedupMerged:       2,
		MaturityTempered:  3,
		DefragPruned:      4,
		CloseLoopPromoted: 5,
		FindingsRouted:    6,
		InjectRefreshed:   true,
		RolledBack:        true,
	}
	got := reduceSummary(r)
	wantInts := map[string]int{
		"harvest_promoted":    1,
		"dedup_merged":        2,
		"maturity_tempered":   3,
		"defrag_pruned":       4,
		"close_loop_promoted": 5,
		"findings_routed":     6,
	}
	for k, v := range wantInts {
		if got[k] != v {
			t.Fatalf("%s: got %v, want %d", k, got[k], v)
		}
	}
	if got["inject_refreshed"] != true {
		t.Fatalf("inject_refreshed: got %v, want true", got["inject_refreshed"])
	}
	if got["rolled_back"] != true {
		t.Fatalf("rolled_back: got %v, want true", got["rolled_back"])
	}
	if len(got) != 8 {
		t.Fatalf("summary key count: got %d, want 8", len(got))
	}
}

// TestMeasureSummary asserts the summary contains findings/inject keys
// always, and fitness only when Fitness is non-nil.
func TestMeasureSummary(t *testing.T) {
	t.Run("nil fitness omits fitness key", func(t *testing.T) {
		r := &MeasureResult{FindingsResolved: 2, InjectVisibility: 0.75}
		got := measureSummary(r)
		if got["findings_resolved"] != 2 {
			t.Fatalf("findings_resolved: got %v, want 2", got["findings_resolved"])
		}
		if got["inject_visibility"] != 0.75 {
			t.Fatalf("inject_visibility: got %v, want 0.75", got["inject_visibility"])
		}
		if _, ok := got["fitness"]; ok {
			t.Fatalf("fitness key: got present, want absent")
		}
		if len(got) != 2 {
			t.Fatalf("summary key count: got %d, want 2", len(got))
		}
	})
}

// TestSnapshotToMap asserts every metric key flows into the output map
// unchanged.
func TestSnapshotToMap(t *testing.T) {
	snap := FitnessSnapshot{
		Metrics: map[string]float64{"a": 1.5, "b": -2.0, "c": 0.0},
	}
	got := snapshotToMap(snap)
	if len(got) != 3 {
		t.Fatalf("map len: got %d, want 3", len(got))
	}
	if got["a"] != 1.5 {
		t.Fatalf("a: got %v, want 1.5", got["a"])
	}
	if got["b"] != -2.0 {
		t.Fatalf("b: got %v, want -2.0", got["b"])
	}
	if got["c"] != 0.0 {
		t.Fatalf("c: got %v, want 0.0", got["c"])
	}
}

// TestSnapshotToMap_Empty exercises the zero-length case.
func TestSnapshotToMap_Empty(t *testing.T) {
	got := snapshotToMap(FitnessSnapshot{})
	if len(got) != 0 {
		t.Fatalf("empty snapshot map: got len %d, want 0", len(got))
	}
}

// TestMapToSnapshot verifies JSON-round-trip numeric decoding and
// silent-drop semantics on non-numeric entries. Nil input and
// all-non-numeric input both return nil.
func TestMapToSnapshot(t *testing.T) {
	tests := []struct {
		name    string
		in      map[string]any
		wantNil bool
		want    map[string]float64
	}{
		{
			name:    "nil map",
			in:      nil,
			wantNil: true,
		},
		{
			name:    "empty map",
			in:      map[string]any{},
			wantNil: true,
		},
		{
			name:    "all non-numeric dropped yields nil",
			in:      map[string]any{"s": "hello", "b": true},
			wantNil: true,
		},
		{
			name: "float64 preserved",
			in:   map[string]any{"a": 1.25, "b": -3.5},
			want: map[string]float64{"a": 1.25, "b": -3.5},
		},
		{
			name: "int widened to float64",
			in:   map[string]any{"a": 2, "b": -5},
			want: map[string]float64{"a": 2.0, "b": -5.0},
		},
		{
			name: "int64 widened to float64",
			in:   map[string]any{"a": int64(9)},
			want: map[string]float64{"a": 9.0},
		},
		{
			name: "json.Number parsed to float64",
			in:   map[string]any{"a": json.Number("3.14")},
			want: map[string]float64{"a": 3.14},
		},
		{
			name: "json.Number with parse error is silently dropped (alongside valid)",
			in:   map[string]any{"a": json.Number("not-a-number"), "b": 1.0},
			want: map[string]float64{"b": 1.0},
		},
		{
			name: "non-numeric values silently dropped alongside numeric",
			in:   map[string]any{"good": 1.0, "bad": "x"},
			want: map[string]float64{"good": 1.0},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapToSnapshot(tt.in)
			if tt.wantNil {
				if got != nil {
					t.Fatalf("mapToSnapshot: got %+v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("mapToSnapshot: got nil, want %v", tt.want)
			}
			if len(got.Metrics) != len(tt.want) {
				t.Fatalf("metrics len: got %d, want %d (got=%v want=%v)",
					len(got.Metrics), len(tt.want), got.Metrics, tt.want)
			}
			for k, v := range tt.want {
				if got.Metrics[k] != v {
					t.Fatalf("metric[%q]: got %v, want %v", k, got.Metrics[k], v)
				}
			}
		})
	}
}

// TestSnapshotMapRoundTrip verifies snapshotToMap and mapToSnapshot are
// inverse for purely-numeric inputs.
func TestSnapshotMapRoundTrip(t *testing.T) {
	orig := FitnessSnapshot{Metrics: map[string]float64{"p": 0.42, "q": -1.125}}
	m := snapshotToMap(orig)
	back := mapToSnapshot(m)
	if back == nil {
		t.Fatalf("mapToSnapshot: got nil, want non-nil round-trip")
	}
	if len(back.Metrics) != 2 {
		t.Fatalf("round-trip len: got %d, want 2", len(back.Metrics))
	}
	if back.Metrics["p"] != 0.42 {
		t.Fatalf("p round-trip: got %v, want 0.42", back.Metrics["p"])
	}
	if back.Metrics["q"] != -1.125 {
		t.Fatalf("q round-trip: got %v, want -1.125", back.Metrics["q"])
	}
}

// TestRegressionNames asserts extraction is order-preserving and handles
// the empty slice.
func TestRegressionNames(t *testing.T) {
	tests := []struct {
		name string
		in   []MetricRegression
		want []string
	}{
		{
			name: "empty slice yields zero-length result",
			in:   nil,
			want: []string{},
		},
		{
			name: "single entry",
			in:   []MetricRegression{{Name: "retrieval_precision"}},
			want: []string{"retrieval_precision"},
		},
		{
			name: "preserves input order",
			in: []MetricRegression{
				{Name: "a"}, {Name: "b"}, {Name: "c"},
			},
			want: []string{"a", "b", "c"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := regressionNames(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("len: got %d, want %d", len(got), len(tt.want))
			}
			for i, want := range tt.want {
				if got[i] != want {
					t.Fatalf("[%d]: got %q, want %q", i, got[i], want)
				}
			}
		})
	}
}

// TestRunTestPostCommitFaultInjector_NoInjector returns empty when no hook
// is registered, which is the only path exercised when tests reset the
// hook between runs.
func TestRunTestPostCommitFaultInjector_NoInjector(t *testing.T) {
	SetTestPostCommitFaultInjector(nil)
	got := runTestPostCommitFaultInjector(3, "/tmp/doesnt-matter")
	if got != "" {
		t.Fatalf("no injector: got %q, want empty string", got)
	}
}

// TestRunTestPostCommitFaultInjector_InjectorError exercises the error
// wrap path. The message embeds both iteration index and the underlying
// error text.
func TestRunTestPostCommitFaultInjector_InjectorError(t *testing.T) {
	t.Cleanup(func() { SetTestPostCommitFaultInjector(nil) })
	var sawIter int
	var sawCwd string
	SetTestPostCommitFaultInjector(func(iterIndex int, cwd string) error {
		sawIter = iterIndex
		sawCwd = cwd
		return &testErr{msg: "synthetic fault"}
	})
	got := runTestPostCommitFaultInjector(7, "/tmp/repo")
	want := "iter-7 post-commit fault injection: synthetic fault"
	if got != want {
		t.Fatalf("msg: got %q, want %q", got, want)
	}
	if sawIter != 7 {
		t.Fatalf("injector received iter: got %d, want 7", sawIter)
	}
	if sawCwd != "/tmp/repo" {
		t.Fatalf("injector received cwd: got %q, want %q", sawCwd, "/tmp/repo")
	}
}

// TestRunTestPostCommitFaultInjector_InjectorReturnsNil produces empty
// output when the registered hook does not return an error.
func TestRunTestPostCommitFaultInjector_InjectorReturnsNil(t *testing.T) {
	t.Cleanup(func() { SetTestPostCommitFaultInjector(nil) })
	SetTestPostCommitFaultInjector(func(iterIndex int, cwd string) error {
		return nil
	})
	got := runTestPostCommitFaultInjector(1, "/tmp")
	if got != "" {
		t.Fatalf("nil-error injector: got %q, want empty string", got)
	}
}

// TestIterationStatusCompoundedEnum asserts the compounded/non-compounded
// partition the loop depends on. lastCompoundedSnapshot relies on this
// partition to skip rolled-back and degraded iterations.
func TestIterationStatusCompoundedEnum(t *testing.T) {
	compounded := []IterationStatus{
		StatusDone,
		StatusHaltedOnRegressionPostCommit,
	}
	notCompounded := []IterationStatus{
		StatusDegraded,
		StatusRolledBackPreCommit,
		StatusHaltedOnRegressionPreCommit,
		StatusFailed,
	}
	for _, s := range compounded {
		if !s.IsCorpusCompounded() {
			t.Fatalf("IsCorpusCompounded(%q): got false, want true", s)
		}
	}
	for _, s := range notCompounded {
		if s.IsCorpusCompounded() {
			t.Fatalf("IsCorpusCompounded(%q): got true, want false", s)
		}
	}
}

// TestErrNotImplementedText pins the sentinel error's public message so
// callers that match on it stay stable.
func TestErrNotImplementedText(t *testing.T) {
	got := ErrNotImplemented.Error()
	want := "overnight: stage not implemented yet (skeleton wave)"
	if got != want {
		t.Fatalf("ErrNotImplemented.Error(): got %q, want %q", got, want)
	}
	if !strings.Contains(got, "skeleton wave") {
		t.Fatalf("ErrNotImplemented.Error(): got %q, want substring %q", got, "skeleton wave")
	}
}

// testErr is a tiny error type used to assert the post-commit fault
// injector wraps the error message intact.
type testErr struct{ msg string }

func (e *testErr) Error() string { return e.msg }
