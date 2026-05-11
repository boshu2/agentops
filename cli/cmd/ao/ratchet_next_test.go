// practices: [dora-metrics, continuous-delivery]
package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/ratchet"
)

func TestComputeNextStep(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		chain    *ratchet.Chain
		wantNext string
		wantDone bool
	}{
		{
			name: "empty chain returns research",
			chain: &ratchet.Chain{
				ID:      "test-1",
				Started: now,
				Entries: []ratchet.ChainEntry{},
			},
			wantNext: "research",
			wantDone: false,
		},
		{
			name: "research locked returns pre-mortem",
			chain: &ratchet.Chain{
				ID:      "test-2",
				Started: now,
				Entries: []ratchet.ChainEntry{
					{
						Step:      ratchet.StepResearch,
						Timestamp: now,
						Output:    ".agents/research/2024-01-01-findings.md",
						Locked:    true,
					},
				},
			},
			wantNext: "pre-mortem",
			wantDone: false,
		},
		{
			name: "pre-mortem locked returns plan",
			chain: &ratchet.Chain{
				ID:      "test-3",
				Started: now,
				Entries: []ratchet.ChainEntry{
					{
						Step:      ratchet.StepResearch,
						Timestamp: now,
						Output:    ".agents/research/findings.md",
						Locked:    true,
					},
					{
						Step:      ratchet.StepPreMortem,
						Timestamp: now.Add(time.Hour),
						Output:    ".agents/council/pre-mortem.md",
						Locked:    true,
					},
				},
			},
			wantNext: "plan",
			wantDone: false,
		},
		{
			name: "plan locked returns implement",
			chain: &ratchet.Chain{
				ID:      "test-4",
				Started: now,
				Entries: []ratchet.ChainEntry{
					{
						Step:      ratchet.StepResearch,
						Timestamp: now,
						Output:    ".agents/research/findings.md",
						Locked:    true,
					},
					{
						Step:      ratchet.StepPreMortem,
						Timestamp: now.Add(time.Hour),
						Output:    ".agents/council/pre-mortem.md",
						Locked:    true,
					},
					{
						Step:      ratchet.StepPlan,
						Timestamp: now.Add(2 * time.Hour),
						Output:    ".agents/plans/epic-plan.md",
						Locked:    true,
					},
				},
			},
			wantNext: "implement",
			wantDone: false,
		},
		{
			name: "implement locked returns vibe (skip crank)",
			chain: &ratchet.Chain{
				ID:      "test-5",
				Started: now,
				Entries: []ratchet.ChainEntry{
					{
						Step:      ratchet.StepResearch,
						Timestamp: now,
						Output:    ".agents/research/findings.md",
						Locked:    true,
					},
					{
						Step:      ratchet.StepPreMortem,
						Timestamp: now.Add(time.Hour),
						Output:    ".agents/council/pre-mortem.md",
						Locked:    true,
					},
					{
						Step:      ratchet.StepPlan,
						Timestamp: now.Add(2 * time.Hour),
						Output:    ".agents/plans/epic-plan.md",
						Locked:    true,
					},
					{
						Step:      ratchet.StepImplement,
						Timestamp: now.Add(3 * time.Hour),
						Output:    "feature implemented",
						Locked:    true,
					},
				},
			},
			wantNext: "vibe",
			wantDone: false,
		},
		{
			name: "crank locked returns vibe",
			chain: &ratchet.Chain{
				ID:      "test-6",
				Started: now,
				Entries: []ratchet.ChainEntry{
					{
						Step:      ratchet.StepResearch,
						Timestamp: now,
						Output:    ".agents/research/findings.md",
						Locked:    true,
					},
					{
						Step:      ratchet.StepPreMortem,
						Timestamp: now.Add(time.Hour),
						Output:    ".agents/council/pre-mortem.md",
						Locked:    true,
					},
					{
						Step:      ratchet.StepPlan,
						Timestamp: now.Add(2 * time.Hour),
						Output:    ".agents/plans/epic-plan.md",
						Locked:    true,
					},
					{
						Step:      ratchet.StepCrank,
						Timestamp: now.Add(3 * time.Hour),
						Output:    "epic completed via crank",
						Locked:    true,
					},
				},
			},
			wantNext: "vibe",
			wantDone: false,
		},
		{
			name: "vibe locked returns post-mortem",
			chain: &ratchet.Chain{
				ID:      "test-7",
				Started: now,
				Entries: []ratchet.ChainEntry{
					{
						Step:      ratchet.StepResearch,
						Timestamp: now,
						Output:    ".agents/research/findings.md",
						Locked:    true,
					},
					{
						Step:      ratchet.StepPreMortem,
						Timestamp: now.Add(time.Hour),
						Output:    ".agents/council/pre-mortem.md",
						Locked:    true,
					},
					{
						Step:      ratchet.StepPlan,
						Timestamp: now.Add(2 * time.Hour),
						Output:    ".agents/plans/epic-plan.md",
						Locked:    true,
					},
					{
						Step:      ratchet.StepImplement,
						Timestamp: now.Add(3 * time.Hour),
						Output:    "feature implemented",
						Locked:    true,
					},
					{
						Step:      ratchet.StepVibe,
						Timestamp: now.Add(4 * time.Hour),
						Output:    ".agents/council/vibe.md",
						Locked:    true,
					},
				},
			},
			wantNext: "post-mortem",
			wantDone: false,
		},
		{
			name: "post-mortem locked returns complete",
			chain: &ratchet.Chain{
				ID:      "test-8",
				Started: now,
				Entries: []ratchet.ChainEntry{
					{
						Step:      ratchet.StepResearch,
						Timestamp: now,
						Output:    ".agents/research/findings.md",
						Locked:    true,
					},
					{
						Step:      ratchet.StepPreMortem,
						Timestamp: now.Add(time.Hour),
						Output:    ".agents/council/pre-mortem.md",
						Locked:    true,
					},
					{
						Step:      ratchet.StepPlan,
						Timestamp: now.Add(2 * time.Hour),
						Output:    ".agents/plans/epic-plan.md",
						Locked:    true,
					},
					{
						Step:      ratchet.StepImplement,
						Timestamp: now.Add(3 * time.Hour),
						Output:    "feature implemented",
						Locked:    true,
					},
					{
						Step:      ratchet.StepVibe,
						Timestamp: now.Add(4 * time.Hour),
						Output:    ".agents/council/vibe.md",
						Locked:    true,
					},
					{
						Step:      ratchet.StepPostMortem,
						Timestamp: now.Add(5 * time.Hour),
						Output:    ".agents/council/post-mortem.md",
						Locked:    true,
					},
				},
			},
			wantNext: "",
			wantDone: true,
		},
		{
			name: "unlocked entries are ignored",
			chain: &ratchet.Chain{
				ID:      "test-9",
				Started: now,
				Entries: []ratchet.ChainEntry{
					{
						Step:      ratchet.StepResearch,
						Timestamp: now,
						Output:    ".agents/research/findings.md",
						Locked:    true,
					},
					{
						Step:      ratchet.StepPreMortem,
						Timestamp: now.Add(time.Hour),
						Output:    ".agents/council/pre-mortem.md",
						Locked:    false, // not locked
					},
				},
			},
			wantNext: "pre-mortem",
			wantDone: false,
		},
		{
			name: "skipped steps are treated like locked",
			chain: &ratchet.Chain{
				ID:      "test-10",
				Started: now,
				Entries: []ratchet.ChainEntry{
					{
						Step:      ratchet.StepResearch,
						Timestamp: now,
						Output:    ".agents/research/findings.md",
						Locked:    true,
					},
					{
						Step:      ratchet.StepPreMortem,
						Timestamp: now.Add(time.Hour),
						Skipped:   true,
						Reason:    "simple change, no pre-mortem needed",
					},
				},
			},
			wantNext: "plan",
			wantDone: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := computeNextStep(tt.chain)

			if result.Complete != tt.wantDone {
				t.Errorf("Complete = %v, want %v", result.Complete, tt.wantDone)
			}

			if result.Next != tt.wantNext {
				t.Errorf("Next = %q, want %q", result.Next, tt.wantNext)
			}

			// Verify skill mapping is present when not complete
			if !result.Complete && result.Next != "" {
				if result.Skill == "" {
					t.Errorf("Skill mapping missing for next step %q", result.Next)
				}
			}
		})
	}
}

func TestOutputNextResult_JSON(t *testing.T) {
	oldOutput := output
	output = "json"
	defer func() { output = oldOutput }()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	result := &NextResult{
		Next:         "implement",
		Reason:       "plan is locked",
		LastStep:     "plan",
		LastArtifact: ".agents/plans/test.md",
		Skill:        "/implement or /crank",
		Complete:     false,
	}

	err := outputNextResult(result)
	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputNextResult: %v", err)
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	r.Close()
	out := string(buf[:n])

	var parsed NextResult
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("parse JSON output: %v\n%s", err, out)
	}
	if parsed.Next != "implement" {
		t.Errorf("Next = %q, want %q", parsed.Next, "implement")
	}
	if parsed.Skill != "/implement or /crank" {
		t.Errorf("Skill = %q, want %q", parsed.Skill, "/implement or /crank")
	}
}

func TestOutputNextResult_TableComplete(t *testing.T) {
	oldOutput := output
	output = "table"
	defer func() { output = oldOutput }()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	result := &NextResult{
		Complete:     true,
		LastStep:     "post-mortem",
		LastArtifact: ".agents/council/post-mortem.md",
	}

	err := outputNextResult(result)
	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputNextResult: %v", err)
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	r.Close()
	out := string(buf[:n])

	if !strings.Contains(out, "All RPI steps complete") {
		t.Errorf("expected 'All RPI steps complete' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Last step: post-mortem") {
		t.Errorf("expected last step in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Last artifact:") {
		t.Errorf("expected last artifact in output, got:\n%s", out)
	}
}

func TestOutputNextResult_TableIncomplete(t *testing.T) {
	oldOutput := output
	output = "table"
	defer func() { output = oldOutput }()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	result := &NextResult{
		Next:     "vibe",
		Reason:   "implement locked",
		Skill:    "/vibe",
		Complete: false,
	}

	err := outputNextResult(result)
	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputNextResult: %v", err)
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	r.Close()
	out := string(buf[:n])

	if !strings.Contains(out, "Next step: vibe") {
		t.Errorf("expected 'Next step: vibe' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Suggested skill: /vibe") {
		t.Errorf("expected 'Suggested skill: /vibe' in output, got:\n%s", out)
	}
}
