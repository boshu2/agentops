package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestBuildRetryPrompt_ContextDiscipline_RetryInstructions verifies that the retry prompt
// contains the retry-specific context discipline and phase summary instructions
// (retryContextDisciplineInstruction and retryPhaseSummaryInstruction).
func TestBuildRetryPrompt_ContextDiscipline_RetryInstructions(t *testing.T) {
	state := &phasedState{
		Goal:   "implement feature X",
		EpicID: "ep-001",
		Opts:   phasedEngineOptions{MaxRetries: 3},
	}
	retryCtx := &retryContext{
		Attempt: 1,
		Verdict: "FAIL",
		Findings: []finding{
			{Description: "test failed", Fix: "fix the test", Ref: "ref-1"},
		},
	}

	// Phase 3 has a retry template, so buildRetryPrompt will use it.
	got, err := buildRetryPrompt("", 3, state, retryCtx)
	if err != nil {
		t.Fatalf("buildRetryPrompt returned error: %v", err)
	}

	// Verify the retry context discipline instruction is present.
	if !strings.Contains(got, retryContextDisciplineInstruction) {
		t.Errorf("retry prompt does not contain retryContextDisciplineInstruction\ngot:\n%s", got)
	}

	// Verify the retry phase summary instruction is present.
	if !strings.Contains(got, retryPhaseSummaryInstruction) {
		t.Errorf("retry prompt does not contain retryPhaseSummaryInstruction\ngot:\n%s", got)
	}
}

// TestBuildRetryPrompt_ContextDiscipline_KeyPhrases verifies specific key phrases
// from the retry context discipline and phase summary instructions appear in the prompt.
func TestBuildRetryPrompt_ContextDiscipline_KeyPhrases(t *testing.T) {
	state := &phasedState{
		Goal:   "add context discipline to retry prompts",
		EpicID: "ep-002",
		Opts:   phasedEngineOptions{MaxRetries: 3},
	}
	retryCtx := &retryContext{
		Attempt:  2,
		Verdict:  "FAIL",
		Findings: []finding{},
	}

	got, err := buildRetryPrompt("", 3, state, retryCtx)
	if err != nil {
		t.Fatalf("buildRetryPrompt returned error: %v", err)
	}

	keyPhrases := []string{
		"summarize what was accomplished in prior phases",
		"Do not repeat work that already succeeded",
		"Include a brief summary of prior phase outcomes",
		"focus on the specific failure",
	}

	for _, phrase := range keyPhrases {
		if !strings.Contains(got, phrase) {
			t.Errorf("retry prompt missing key phrase %q\ngot:\n%s", phrase, got)
		}
	}
}

// TestRetryInstructionConstants verifies the retry instruction constants
// have non-empty, meaningful content.
func TestRetryInstructionConstants(t *testing.T) {
	if retryContextDisciplineInstruction == "" {
		t.Error("retryContextDisciplineInstruction must not be empty")
	}
	if retryPhaseSummaryInstruction == "" {
		t.Error("retryPhaseSummaryInstruction must not be empty")
	}

	// Discipline instruction should reference "prior phases" and "retry"
	if !strings.Contains(retryContextDisciplineInstruction, "prior phases") {
		t.Error("retryContextDisciplineInstruction should reference 'prior phases'")
	}
	if !strings.Contains(retryContextDisciplineInstruction, "retry") {
		t.Error("retryContextDisciplineInstruction should reference 'retry'")
	}

	// Phase summary instruction should reference "prior phase outcomes"
	if !strings.Contains(retryPhaseSummaryInstruction, "prior phase outcomes") {
		t.Error("retryPhaseSummaryInstruction should reference 'prior phase outcomes'")
	}
}

// ---------- P2.2: Context assembly tests ----------

func TestCtx_BuildPromptForPhase_Phase1(t *testing.T) {
	state := &phasedState{
		Goal:       "add caching layer",
		EpicID:     "ep-100",
		FastPath:   false,
		TestFirst:  true,
		SwarmFirst: true,
		Verdicts:   map[string]string{},
		Attempts:   map[string]int{},
		Opts:       defaultPhasedEngineOptions(),
	}

	prompt, err := buildPromptForPhase("", 1, state, nil)
	if err != nil {
		t.Fatalf("buildPromptForPhase(1): %v", err)
	}

	// Phase 1 should include research, plan, and pre-mortem skill invocations
	for _, keyword := range []string{"/research", "/plan", "/pre-mortem"} {
		if !strings.Contains(prompt, keyword) {
			t.Errorf("phase 1 prompt missing %q", keyword)
		}
	}
}

func TestCtx_BuildPromptForPhase_Phase2_WithHandoffs(t *testing.T) {
	tmp := t.TempDir()

	// Write a structured handoff for phase 1
	rpiDir := filepath.Join(tmp, ".agents", "rpi")
	if err := os.MkdirAll(rpiDir, 0750); err != nil {
		t.Fatal(err)
	}

	handoff := fmt.Sprintf(`{
		"schema_version": 1,
		"run_id": "r1",
		"phase": 1,
		"phase_name": "discovery",
		"status": "completed",
		"goal": "handoff goal",
		"epic_id": "ep-200",
		"verdicts": {"pre-mortem": "PASS"},
		"artifacts_produced": ["plan.md"],
		"applied_findings": ["f-2026-03-09-001"],
		"planning_rules": ["f-2026-03-09-001 — Do not skip prevention context"],
		"known_risks": ["f-2026-03-09-001 — Validate before implementation"],
		"decisions_made": ["use redis"],
		"open_risks": ["cache invalidation"],
		"duration_seconds": 120,
		"narrative": "Explored options, chose redis.",
		"completed_at": %q
	}`, time.Now().UTC().Format(time.RFC3339))

	if err := os.WriteFile(filepath.Join(rpiDir, "phase-1-handoff.json"), []byte(handoff), 0600); err != nil {
		t.Fatal(err)
	}

	state := &phasedState{
		Goal:     "handoff goal",
		EpicID:   "ep-200",
		Phase:    2,
		Verdicts: map[string]string{"pre-mortem": "PASS"},
		Attempts: map[string]int{},
		Opts:     defaultPhasedEngineOptions(),
	}

	prompt, err := buildPromptForPhase(tmp, 2, state, nil)
	if err != nil {
		t.Fatalf("buildPromptForPhase(2): %v", err)
	}

	// Phase 2 should include handoff context
	if !strings.Contains(prompt, "structured handoffs") {
		t.Errorf("phase 2 prompt missing structured handoff context header")
	}
	if !strings.Contains(prompt, "handoff goal") {
		t.Errorf("phase 2 prompt missing goal from handoff")
	}
	if !strings.Contains(prompt, "Do not skip prevention context") {
		t.Errorf("phase 2 prompt missing planning rule context")
	}
	if !strings.Contains(prompt, "Validate before implementation") {
		t.Errorf("phase 2 prompt missing known risk context")
	}
	// Phase 2 should invoke crank
	if !strings.Contains(prompt, "/crank") {
		t.Errorf("phase 2 prompt missing /crank")
	}
}

func TestCtx_BuildPromptForPhase_Phase3(t *testing.T) {
	tmp := t.TempDir()
	rpiDir := filepath.Join(tmp, ".agents", "rpi")
	if err := os.MkdirAll(rpiDir, 0750); err != nil {
		t.Fatal(err)
	}

	handoff := fmt.Sprintf(`{
		"schema_version": 1,
		"run_id": "r2",
		"phase": 2,
		"phase_name": "implementation",
		"status": "completed",
		"goal": "add caching",
		"epic_id": "ep-300",
		"verdicts": {"crank": "PASS"},
		"artifacts_produced": ["cache.go", "cache_test.go"],
		"applied_findings": ["f-2026-03-09-001"],
		"planning_rules": ["f-2026-03-09-001 — Do not skip prevention context"],
		"known_risks": ["f-2026-03-09-001 — Validate before implementation"],
		"completed_at": %q
	}`, time.Now().UTC().Format(time.RFC3339))
	if err := os.WriteFile(filepath.Join(rpiDir, "phase-2-handoff.json"), []byte(handoff), 0600); err != nil {
		t.Fatal(err)
	}

	state := &phasedState{
		Goal:       "add caching",
		EpicID:     "ep-300",
		FastPath:   true,
		SwarmFirst: false,
		Verdicts:   map[string]string{},
		Attempts:   map[string]int{},
		Opts:       defaultPhasedEngineOptions(),
	}

	prompt, err := buildPromptForPhase(tmp, 3, state, nil)
	if err != nil {
		t.Fatalf("buildPromptForPhase(3): %v", err)
	}

	for _, keyword := range []string{"/vibe", "/post-mortem"} {
		if !strings.Contains(prompt, keyword) {
			t.Errorf("phase 3 prompt missing %q", keyword)
		}
	}
	if !strings.Contains(prompt, "f-2026-03-09-001") {
		t.Errorf("phase 3 prompt missing applied findings context")
	}
	if strings.Contains(prompt, "Do not skip prevention context") || strings.Contains(prompt, "Validate before implementation") {
		t.Errorf("phase 3 prompt should not include raw planning rules or known risks")
	}
}

func TestCtx_BuildPromptForPhase_MixedModeFlag(t *testing.T) {
	opts := defaultPhasedEngineOptions()
	opts.Mixed = true

	tests := []struct {
		name     string
		phaseNum int
		state    *phasedState
		want     []string
	}{
		{
			name:     "discovery",
			phaseNum: 1,
			state: &phasedState{
				Goal:       "mixed discovery",
				TestFirst:  true,
				SwarmFirst: true,
				Opts:       opts,
			},
			want: []string{"/pre-mortem --mixed"},
		},
		{
			name:     "validation",
			phaseNum: 3,
			state: &phasedState{
				Goal:       "mixed validation",
				EpicID:     "ag-mixed",
				TestFirst:  true,
				SwarmFirst: true,
				Opts:       opts,
			},
			want: []string{"/vibe --mixed recent", "/post-mortem --mixed ag-mixed"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt, err := buildPromptForPhase("", tt.phaseNum, tt.state, nil)
			if err != nil {
				t.Fatalf("buildPromptForPhase(%d): %v", tt.phaseNum, err)
			}
			for _, want := range tt.want {
				if !strings.Contains(prompt, want) {
					t.Fatalf("prompt missing %q:\n%s", want, prompt)
				}
			}
		})
	}
}

func TestCtx_BuildPromptForPhase_TasklistModeUsesExecutionPacket(t *testing.T) {
	state := &phasedState{
		Goal:        "run Codex no-beads lifecycle",
		TrackerMode: "tasklist",
		Opts:        defaultPhasedEngineOptions(),
	}

	prompt, err := buildPromptForPhase("", 2, state, nil)
	if err != nil {
		t.Fatalf("buildPromptForPhase(2): %v", err)
	}
	if !strings.Contains(prompt, "TASKLIST MODE") {
		t.Fatalf("tasklist phase 2 prompt missing tasklist marker:\n%s", prompt)
	}
	if !strings.Contains(prompt, "/crank .agents/rpi/execution-packet.json") {
		t.Fatalf("tasklist phase 2 prompt missing execution-packet handoff:\n%s", prompt)
	}
}

func TestBuildRetryPrompt_TasklistModeUsesExecutionPacket(t *testing.T) {
	state := &phasedState{
		Goal:        "run Codex no-beads lifecycle",
		TrackerMode: "tasklist",
		Opts:        phasedEngineOptions{MaxRetries: 3},
	}
	retryCtx := &retryContext{
		Attempt: 1,
		Verdict: "FAIL",
	}

	prompt, err := buildRetryPrompt("", 3, state, retryCtx)
	if err != nil {
		t.Fatalf("buildRetryPrompt(3): %v", err)
	}
	if !strings.Contains(prompt, "/crank .agents/rpi/execution-packet.json") {
		t.Fatalf("tasklist retry prompt missing execution-packet handoff:\n%s", prompt)
	}
}

func TestCtx_BuildPromptForPhase_PreambleSurvival(t *testing.T) {
	state := &phasedState{
		Goal:     "preamble test",
		EpicID:   "ep-400",
		Verdicts: map[string]string{},
		Attempts: map[string]int{},
		Opts:     defaultPhasedEngineOptions(),
	}

	for _, phaseNum := range []int{1, 2, 3} {
		prompt, err := buildPromptForPhase("", phaseNum, state, nil)
		if err != nil {
			t.Fatalf("buildPromptForPhase(%d): %v", phaseNum, err)
		}

		if !strings.Contains(prompt, "CONTEXT DISCIPLINE") {
			t.Errorf("phase %d prompt missing CONTEXT DISCIPLINE preamble", phaseNum)
		}
		if !strings.Contains(prompt, "PHASE SUMMARY CONTRACT") {
			t.Errorf("phase %d prompt missing PHASE SUMMARY CONTRACT preamble", phaseNum)
		}
	}
}

func TestCtx_BuildPhaseContext_GoalAndVerdicts(t *testing.T) {
	state := &phasedState{
		Goal: "implement widget API",
		Verdicts: map[string]string{
			"pre_mortem": "PASS",
			"vibe":       "WARN",
		},
		Attempts: map[string]int{},
	}

	ctx := buildPhaseContext("", state, 2)

	if !strings.Contains(ctx, "implement widget API") {
		t.Errorf("context missing goal")
	}
	if !strings.Contains(ctx, "pre-mortem") {
		t.Errorf("context missing pre-mortem verdict (underscores should be dashes)")
	}
	if !strings.Contains(ctx, "PASS") {
		t.Errorf("context missing PASS verdict value")
	}
	if !strings.Contains(ctx, "vibe") {
		t.Errorf("context missing vibe verdict")
	}
}

func TestCtx_ReadPhaseSummaries_CapsAt2000(t *testing.T) {
	tmp := t.TempDir()
	rpiDir := filepath.Join(tmp, ".agents", "rpi")
	if err := os.MkdirAll(rpiDir, 0750); err != nil {
		t.Fatal(err)
	}

	// Write a summary longer than 2000 chars for phase 1
	longContent := strings.Repeat("x", 3000)
	if err := os.WriteFile(filepath.Join(rpiDir, "phase-1-summary.md"), []byte(longContent), 0600); err != nil {
		t.Fatal(err)
	}

	result := readPhaseSummaries(tmp, 2) // read summaries prior to phase 2

	// The result should contain the truncated content with "..." suffix
	if !strings.Contains(result, "...") {
		t.Error("expected truncation marker '...' for long summary")
	}
	// The raw 3000-char content should NOT appear in full
	if strings.Contains(result, longContent) {
		t.Error("summary was not truncated — full 3000-char content present")
	}
}

func TestCtx_ParsePhaseBudgetSpec_Valid(t *testing.T) {
	spec := "discovery:300,implementation:600,validation:180"
	budgets, err := parsePhaseBudgetSpec(spec)
	if err != nil {
		t.Fatalf("parsePhaseBudgetSpec: %v", err)
	}

	want := map[int]time.Duration{
		1: 300 * time.Second,
		2: 600 * time.Second,
		3: 180 * time.Second,
	}

	for phase, expected := range want {
		if got := budgets[phase]; got != expected {
			t.Errorf("phase %d budget = %v, want %v", phase, got, expected)
		}
	}
}

// TestResolveWorktreeModeFromConfig_NoConfig verifies that when no config file exists,
// the function returns the flagDefault value unchanged.
func TestResolveWorktreeModeFromConfig_NoConfig(t *testing.T) {
	// Point AGENTOPS_CONFIG to a nonexistent file so Load returns defaults (WorktreeMode="auto").
	// "auto" hits the default switch case, which returns flagDefault.
	t.Setenv("AGENTOPS_CONFIG", filepath.Join(t.TempDir(), "nonexistent.yaml"))

	// With flagDefault=false, should return false (auto mode preserves flag default).
	got := resolveWorktreeModeFromConfig(false)
	if got != false {
		t.Errorf("resolveWorktreeModeFromConfig(false) with no config = %v, want false", got)
	}

	// With flagDefault=true, should return true.
	got = resolveWorktreeModeFromConfig(true)
	if got != true {
		t.Errorf("resolveWorktreeModeFromConfig(true) with no config = %v, want true", got)
	}
}

// TestResolveWorktreeModeFromConfig_FlagTrue verifies that when config has worktree_mode="always",
// the function returns false (NoWorktree=false means worktrees ARE enabled).
func TestResolveWorktreeModeFromConfig_FlagTrue(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")
	// "always" means always use worktrees → NoWorktree should be false.
	if err := os.WriteFile(cfgPath, []byte("rpi:\n  worktree_mode: always\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENTOPS_CONFIG", cfgPath)

	// Regardless of flagDefault, "always" should return false (NoWorktree=false → worktrees on).
	got := resolveWorktreeModeFromConfig(true)
	if got != false {
		t.Errorf("resolveWorktreeModeFromConfig(true) with always = %v, want false", got)
	}
}

// TestResolveWorktreeModeFromConfig_FlagFalse verifies that when config has worktree_mode="never",
// the function returns true (NoWorktree=true means worktrees are disabled).
func TestResolveWorktreeModeFromConfig_FlagFalse(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")
	// "never" means never use worktrees → NoWorktree should be true.
	if err := os.WriteFile(cfgPath, []byte("rpi:\n  worktree_mode: never\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENTOPS_CONFIG", cfgPath)

	// Regardless of flagDefault, "never" should return true (NoWorktree=true → worktrees off).
	got := resolveWorktreeModeFromConfig(false)
	if got != true {
		t.Errorf("resolveWorktreeModeFromConfig(false) with never = %v, want true", got)
	}
}

func TestCtx_ResolvePhaseBudget_ComplexityDefault(t *testing.T) {
	tests := []struct {
		name       string
		complexity ComplexityLevel
		fastPath   bool
		phase      int
		wantBudget time.Duration
		wantHas    bool
	}{
		{"fast-phase1", ComplexityFast, true, 1, 6 * time.Minute, true},
		{"standard-phase1", ComplexityStandard, false, 1, 13 * time.Minute, true},
		{"full-phase1", ComplexityFull, false, 1, 25 * time.Minute, true},
		{"standard-phase2-unbounded", ComplexityStandard, false, 2, 0, false},
		{"standard-phase3", ComplexityStandard, false, 3, 5 * time.Minute, true},
		{"full-phase3", ComplexityFull, false, 3, 10 * time.Minute, true},
		{"fast-phase3-zero", ComplexityFast, true, 3, 0, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state := &phasedState{
				Complexity: tc.complexity,
				FastPath:   tc.fastPath,
				Verdicts:   map[string]string{},
				Attempts:   map[string]int{},
				Opts:       phasedEngineOptions{},
			}

			budget, hasBudget, err := resolvePhaseBudget(state, tc.phase)
			if err != nil {
				t.Fatalf("resolvePhaseBudget: %v", err)
			}
			if hasBudget != tc.wantHas {
				t.Errorf("hasBudget = %v, want %v", hasBudget, tc.wantHas)
			}
			if budget != tc.wantBudget {
				t.Errorf("budget = %v, want %v", budget, tc.wantBudget)
			}
		})
	}
}
