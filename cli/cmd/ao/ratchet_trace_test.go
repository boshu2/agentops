// practices: [dora-metrics, continuous-delivery]
package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"os"
	"path/filepath"

	"github.com/boshu2/agentops/cli/internal/ratchet"
)

func TestBuildTrace_EmptyChain(t *testing.T) {
	chain := &ratchet.Chain{
		ID:      "test-chain",
		Started: time.Now(),
		Entries: []ratchet.ChainEntry{},
	}

	trace := buildTrace("some-artifact.md", chain)

	if trace.Artifact != "some-artifact.md" {
		t.Errorf("Artifact = %q, want %q", trace.Artifact, "some-artifact.md")
	}
	if len(trace.Chain) != 0 {
		t.Errorf("expected empty trace chain, got %d entries", len(trace.Chain))
	}
}

func TestBuildTrace_SingleEntry(t *testing.T) {
	now := time.Now()
	chain := &ratchet.Chain{
		ID:      "test",
		Started: now,
		Entries: []ratchet.ChainEntry{
			{
				Step:      ratchet.StepResearch,
				Timestamp: now,
				Input:     "",
				Output:    ".agents/research/findings.md",
				Locked:    true,
			},
		},
	}

	trace := buildTrace(".agents/research/findings.md", chain)

	if len(trace.Chain) != 1 {
		t.Fatalf("expected 1 trace entry, got %d", len(trace.Chain))
	}

	entry := trace.Chain[0]
	if entry.Step != ratchet.StepResearch {
		t.Errorf("Step = %q, want %q", entry.Step, ratchet.StepResearch)
	}
	if entry.Output != ".agents/research/findings.md" {
		t.Errorf("Output = %q", entry.Output)
	}
}

func TestBuildTrace_MultiStepProvenance(t *testing.T) {
	now := time.Now()
	chain := &ratchet.Chain{
		ID:      "test",
		Started: now,
		Entries: []ratchet.ChainEntry{
			{
				Step:      ratchet.StepResearch,
				Timestamp: now,
				Input:     "",
				Output:    "research.md",
				Locked:    true,
			},
			{
				Step:      ratchet.StepPreMortem,
				Timestamp: now.Add(time.Hour),
				Input:     "research.md",
				Output:    "spec-v2.md",
				Locked:    true,
			},
			{
				Step:      ratchet.StepPlan,
				Timestamp: now.Add(2 * time.Hour),
				Input:     "spec-v2.md",
				Output:    "epic:ol-0001",
				Locked:    true,
			},
		},
	}

	trace := buildTrace("epic:ol-0001", chain)

	if len(trace.Chain) != 3 {
		t.Fatalf("expected 3 trace entries, got %d", len(trace.Chain))
	}

	// Verify chain order (oldest first)
	if trace.Chain[0].Step != ratchet.StepResearch {
		t.Errorf("first entry step = %q, want research", trace.Chain[0].Step)
	}
	if trace.Chain[1].Step != ratchet.StepPreMortem {
		t.Errorf("second entry step = %q, want pre-mortem", trace.Chain[1].Step)
	}
	if trace.Chain[2].Step != ratchet.StepPlan {
		t.Errorf("third entry step = %q, want plan", trace.Chain[2].Step)
	}
}

func TestBuildTrace_SuffixMatching(t *testing.T) {
	now := time.Now()
	chain := &ratchet.Chain{
		ID:      "test",
		Started: now,
		Entries: []ratchet.ChainEntry{
			{
				Step:      ratchet.StepResearch,
				Timestamp: now,
				Output:    ".agents/research/findings.md",
				Locked:    true,
			},
		},
	}

	// buildTrace uses HasSuffix matching
	trace := buildTrace("findings.md", chain)

	if len(trace.Chain) != 1 {
		t.Errorf("suffix match should find entry, got %d", len(trace.Chain))
	}
}

func TestBuildTrace_NoMatchingArtifact(t *testing.T) {
	now := time.Now()
	chain := &ratchet.Chain{
		ID:      "test",
		Started: now,
		Entries: []ratchet.ChainEntry{
			{
				Step:      ratchet.StepResearch,
				Timestamp: now,
				Output:    "something-else.md",
				Locked:    true,
			},
		},
	}

	trace := buildTrace("nonexistent.md", chain)

	if len(trace.Chain) != 0 {
		t.Errorf("expected empty trace, got %d entries", len(trace.Chain))
	}
}

func TestBuildTrace_TimeFormatting(t *testing.T) {
	ts := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	chain := &ratchet.Chain{
		ID:      "test",
		Started: ts,
		Entries: []ratchet.ChainEntry{
			{
				Step:      ratchet.StepResearch,
				Timestamp: ts,
				Output:    "output.md",
				Locked:    true,
			},
		},
	}

	trace := buildTrace("output.md", chain)

	if len(trace.Chain) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(trace.Chain))
	}

	// Time should be RFC3339
	if !strings.Contains(trace.Chain[0].Time, "2025-01-15") {
		t.Errorf("Time = %q, expected RFC3339 with date 2025-01-15", trace.Chain[0].Time)
	}
}

func TestOutputTrace_JSON(t *testing.T) {
	origOutput := output
	output = "json"
	defer func() { output = origOutput }()

	trace := traceResult{
		Artifact: "test.md",
		Chain: []traceEntry{
			{
				Step:   ratchet.StepResearch,
				Input:  "",
				Output: "test.md",
				Time:   "2025-01-15T10:00:00Z",
			},
		},
	}

	// outputTrace writes to os.Stdout, test the JSON encoding logic
	data, err := json.Marshal(trace)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	if !bytes.Contains(data, []byte("test.md")) {
		t.Error("JSON output missing artifact name")
	}
	if !bytes.Contains(data, []byte("research")) {
		t.Error("JSON output missing step name")
	}
}

func TestOutputTraceText_EmptyChain(t *testing.T) {
	// Just verify it doesn't panic
	trace := traceResult{
		Artifact: "missing.md",
		Chain:    []traceEntry{},
	}

	// outputTraceText writes to stdout; capture is not straightforward
	// but we can at least verify it doesn't error
	err := outputTraceText(trace)
	if err != nil {
		t.Errorf("outputTraceText(empty) error = %v", err)
	}
}

func TestTraceEntry_Structure(t *testing.T) {
	entry := traceEntry{
		Step:   ratchet.StepPlan,
		Input:  "spec-v2.md",
		Output: "epic:ol-0001",
		Time:   "2025-01-15T10:00:00Z",
	}

	if entry.Step != ratchet.StepPlan {
		t.Errorf("Step = %q", entry.Step)
	}
	if entry.Input != "spec-v2.md" {
		t.Errorf("Input = %q", entry.Input)
	}
	if entry.Output != "epic:ol-0001" {
		t.Errorf("Output = %q", entry.Output)
	}
}

func TestTraceResult_JSONSerialization(t *testing.T) {
	trace := traceResult{
		Artifact: "retro.md",
		Chain: []traceEntry{
			{Step: ratchet.StepResearch, Output: "findings.md", Time: "2025-01-15T10:00:00Z"},
			{Step: ratchet.StepPlan, Input: "findings.md", Output: "epic:ol-001", Time: "2025-01-15T11:00:00Z"},
		},
	}

	data, err := json.MarshalIndent(trace, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}

	var decoded traceResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.Artifact != trace.Artifact {
		t.Errorf("Artifact = %q, want %q", decoded.Artifact, trace.Artifact)
	}
	if len(decoded.Chain) != len(trace.Chain) {
		t.Errorf("Chain length = %d, want %d", len(decoded.Chain), len(trace.Chain))
	}
}

// ---------------------------------------------------------------------------
// ratchet_trace.go — buildTrace
// ---------------------------------------------------------------------------

func TestRatchetTrace_buildTrace_emptyChain(t *testing.T) {
	chain := &ratchet.Chain{
		ID:      "test-chain-1",
		Started: time.Now(),
		Entries: []ratchet.ChainEntry{},
	}

	trace := buildTrace("artifact.md", chain)

	if trace.Artifact != "artifact.md" {
		t.Errorf("expected artifact 'artifact.md', got %q", trace.Artifact)
	}
	if len(trace.Chain) != 0 {
		t.Errorf("expected empty chain, got %d entries", len(trace.Chain))
	}
}

func TestRatchetTrace_buildTrace_matchesOutput(t *testing.T) {
	now := time.Now()
	chain := &ratchet.Chain{
		ID:      "test-chain-2",
		Started: now,
		Entries: []ratchet.ChainEntry{
			{
				Step:      ratchet.StepResearch,
				Input:     "transcript.jsonl",
				Output:    "research-notes.md",
				Timestamp: now.Add(-2 * time.Hour),
			},
			{
				Step:      ratchet.StepPlan,
				Input:     "research-notes.md",
				Output:    "plan.md",
				Timestamp: now.Add(-1 * time.Hour),
			},
			{
				Step:      ratchet.StepImplement,
				Input:     "plan.md",
				Output:    "result.md",
				Timestamp: now,
			},
		},
	}

	// Trace from result.md backward
	trace := buildTrace("result.md", chain)

	if trace.Artifact != "result.md" {
		t.Errorf("expected artifact 'result.md', got %q", trace.Artifact)
	}

	// Should trace back: result.md <- plan.md <- research-notes.md
	if len(trace.Chain) != 3 {
		t.Errorf("expected 3 chain entries, got %d", len(trace.Chain))
	}

	// Verify chain order is forward (reversed from backward walk)
	if len(trace.Chain) >= 3 {
		if trace.Chain[0].Step != ratchet.StepResearch {
			t.Errorf("first entry step: got %q, want %q", trace.Chain[0].Step, ratchet.StepResearch)
		}
		if trace.Chain[2].Step != ratchet.StepImplement {
			t.Errorf("last entry step: got %q, want %q", trace.Chain[2].Step, ratchet.StepImplement)
		}
	}
}

func TestRatchetTrace_buildTrace_suffixMatch(t *testing.T) {
	now := time.Now()
	chain := &ratchet.Chain{
		ID:      "test-chain-3",
		Started: now,
		Entries: []ratchet.ChainEntry{
			{
				Step:      ratchet.StepVibe,
				Input:     "input.md",
				Output:    "/full/path/to/artifact.md",
				Timestamp: now,
			},
		},
	}

	// Should match by suffix
	trace := buildTrace("artifact.md", chain)

	if len(trace.Chain) != 1 {
		t.Errorf("expected 1 chain entry via suffix match, got %d", len(trace.Chain))
	}
}

// ---------------------------------------------------------------------------
// ratchet_trace.go — outputTrace (JSON mode)
// ---------------------------------------------------------------------------

func TestRatchetTrace_outputTrace_json(t *testing.T) {
	oldOutput := output
	output = "json"
	defer func() { output = oldOutput }()

	trace := traceResult{
		Artifact: "test.md",
		Chain: []traceEntry{
			{
				Step:   ratchet.StepResearch,
				Input:  "transcript.jsonl",
				Output: "test.md",
				Time:   time.Now().Format(time.RFC3339),
			},
		},
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputTrace(trace)

	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputTrace: %v", err)
	}

	buf := make([]byte, 8192)
	n, _ := r.Read(buf)
	_ = r.Close()
	out := string(buf[:n])

	if !strings.Contains(out, `"artifact"`) {
		t.Errorf("expected JSON with 'artifact' field, got: %s", out)
	}
}

// ---------------------------------------------------------------------------
// ratchet_trace.go — outputTraceText
// ---------------------------------------------------------------------------

func TestRatchetTrace_outputTraceText_emptyChain(t *testing.T) {
	trace := traceResult{
		Artifact: "missing.md",
		Chain:    nil,
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputTraceText(trace)

	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputTraceText: %v", err)
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	_ = r.Close()
	out := string(buf[:n])

	if !strings.Contains(out, "No provenance chain found") {
		t.Errorf("expected 'No provenance chain found', got: %s", out)
	}
}

func TestRatchetTrace_outputTraceText_withEntries(t *testing.T) {
	trace := traceResult{
		Artifact: "output.md",
		Chain: []traceEntry{
			{
				Step:   ratchet.StepResearch,
				Input:  "transcript.jsonl",
				Output: "research.md",
				Time:   "2026-01-15T10:00:00Z",
			},
			{
				Step:   ratchet.StepPlan,
				Input:  "research.md",
				Output: "output.md",
				Time:   "2026-01-15T11:00:00Z",
			},
		},
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputTraceText(trace)

	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputTraceText: %v", err)
	}

	buf := make([]byte, 8192)
	n, _ := r.Read(buf)
	_ = r.Close()
	out := string(buf[:n])

	if !strings.Contains(out, "Provenance Trace: output.md") {
		t.Errorf("expected header, got: %s", out)
	}
	if !strings.Contains(out, "1. research") {
		t.Errorf("expected step 1 research, got: %s", out)
	}
	if !strings.Contains(out, "2. plan") {
		t.Errorf("expected step 2 plan, got: %s", out)
	}
}

// ---------------------------------------------------------------------------
// ratchet_trace.go — runRatchetTrace error path
// ---------------------------------------------------------------------------

func TestRatchetTrace_runRatchetTrace_noAgentsDir(t *testing.T) {
	tmpDir := t.TempDir()
	prev, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(prev) }()

	// Create .agents dir so LoadChain doesn't fail but chain is empty
	if err := os.MkdirAll(filepath.Join(tmpDir, ".agents", "ao"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	oldOutput := output
	output = "table"
	defer func() { output = oldOutput }()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runRatchetTrace(nil, []string{"nonexistent.md"})

	_ = w.Close()
	os.Stdout = oldStdout

	// Should succeed with empty trace
	if err != nil {
		t.Fatalf("runRatchetTrace: %v", err)
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	_ = r.Close()
	out := string(buf[:n])

	if !strings.Contains(out, "No provenance chain found") {
		t.Errorf("expected no provenance message, got: %s", out)
	}
}
