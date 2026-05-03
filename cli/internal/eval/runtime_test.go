package eval

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestRunLiveRuntimeSkipsWhenExecutableUnavailable(t *testing.T) {
	for _, tc := range []struct {
		name       string
		runtime    Runtime
		executable string
	}{
		{name: "claude", runtime: RuntimeClaude, executable: "claude"},
		{name: "codex", runtime: RuntimeCodex, executable: "codex"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			suite := liveRuntimeSuite(tc.runtime)

			run, err := RunLiveRuntime(context.Background(), LiveRuntimeOptions{
				Suite:   suite,
				RunID:   "live-skip",
				Runtime: tc.runtime,
				Enabled: true,
				LookPath: func(name string) (string, error) {
					if name != tc.executable {
						t.Fatalf("lookPath name = %q, want %s", name, tc.executable)
					}
					return "", exec.ErrNotFound
				},
				Now: fixedEvalTime,
			})
			if err != nil {
				t.Fatalf("RunLiveRuntime returned error: %v", err)
			}

			wantReason := tc.executable + " executable not found: executable file not found in $PATH"
			if run.Status != StatusSkipped {
				t.Fatalf("status = %s, want skipped", run.Status)
			}
			if run.Verdict != VerdictInconclusive {
				t.Fatalf("verdict = %s, want inconclusive", run.Verdict)
			}
			if run.Runtime.Name != tc.runtime || !run.Runtime.Live {
				t.Fatalf("runtime = %+v, want live %s", run.Runtime, tc.runtime)
			}
			if run.Runtime.Attempts != 1 {
				t.Fatalf("attempts = %d, want 1", run.Runtime.Attempts)
			}
			if run.Runtime.TimeoutSeconds != 45 {
				t.Fatalf("timeout = %d, want 45", run.Runtime.TimeoutSeconds)
			}
			if run.Runtime.SkippedReason != wantReason {
				t.Fatalf("skipped_reason = %q, want %q", run.Runtime.SkippedReason, wantReason)
			}
			if len(run.CaseResults) != 1 || run.CaseResults[0].Status != StatusSkipped {
				t.Fatalf("case results = %+v, want one skipped case", run.CaseResults)
			}
			if run.CaseResults[0].FailureMessage != wantReason {
				t.Fatalf("case failure = %q, want skip reason", run.CaseResults[0].FailureMessage)
			}
		})
	}
}

func TestRunLiveRuntimeDisabledSkipsBeforeExternalProbe(t *testing.T) {
	run, err := RunLiveRuntime(context.Background(), LiveRuntimeOptions{
		Suite:   liveRuntimeSuite(RuntimeCodex),
		RunID:   "live-disabled",
		Runtime: RuntimeCodex,
		LookPath: func(name string) (string, error) {
			t.Fatalf("lookPath should not be called when live runtime is disabled")
			return "", nil
		},
		Runner: func(ctx context.Context, cmd RuntimeCommand) (RuntimeExecutionResult, error) {
			t.Fatalf("runner should not be called when live runtime is disabled")
			return RuntimeExecutionResult{}, nil
		},
		Now: fixedEvalTime,
	})
	if err != nil {
		t.Fatalf("RunLiveRuntime returned error: %v", err)
	}
	wantReason := "live runtime disabled; set LiveRuntimeOptions.Enabled to true"
	if run.Status != StatusSkipped {
		t.Fatalf("status = %s, want skipped", run.Status)
	}
	if run.Runtime.SkippedReason != wantReason {
		t.Fatalf("skipped_reason = %q, want %q", run.Runtime.SkippedReason, wantReason)
	}
	if !run.Runtime.Live {
		t.Fatalf("runtime live = false, want true for attempted live tier")
	}
}

func TestRunLiveRuntimeIsolatesAndScrubsCodexEnvironment(t *testing.T) {
	suite := liveRuntimeSuite(RuntimeCodex)
	suite.Environment.ScrubEnvPrefixes = []string{"SECRET_"}
	suite.Environment.IsolateHome = true
	suite.Environment.IsolateCodexHome = true
	suite.Environment.Network = "allowed"
	suite.Environment.MaxAttempts = 2
	isolationRoot := t.TempDir()
	var got RuntimeCommand

	run, err := RunLiveRuntime(context.Background(), LiveRuntimeOptions{
		Suite:          suite,
		RunID:          "live-codex",
		Runtime:        RuntimeCodex,
		RuntimeCommand: "codex --profile ci",
		Enabled:        true,
		Env: []string{
			"PATH=/bin",
			"SECRET_TOKEN=redacted",
			"AGENTOPS_RPI_RUNTIME=direct",
			"KEEP=1",
		},
		IsolationRoot: isolationRoot,
		LookPath: func(name string) (string, error) {
			if name != "codex" {
				t.Fatalf("lookPath name = %q, want codex", name)
			}
			return "/fake/bin/codex", nil
		},
		VersionRunner: func(ctx context.Context, cmd RuntimeCommand) (string, error) {
			if cmd.Executable != "/fake/bin/codex" {
				t.Fatalf("version executable = %q, want /fake/bin/codex", cmd.Executable)
			}
			return "codex 0.115.0", nil
		},
		Runner: func(ctx context.Context, cmd RuntimeCommand) (RuntimeExecutionResult, error) {
			got = cmd
			return RuntimeExecutionResult{
				Status:         StatusInconclusive,
				Verdict:        VerdictInconclusive,
				TranscriptPath: filepath.Join(isolationRoot, "transcript.jsonl"),
				ScorecardPath:  filepath.Join(isolationRoot, "scorecard.json"),
			}, nil
		},
		Now: fixedEvalTime,
	})
	if err != nil {
		t.Fatalf("RunLiveRuntime returned error: %v", err)
	}

	if got.Executable != "/fake/bin/codex" {
		t.Fatalf("executable = %q, want /fake/bin/codex", got.Executable)
	}
	if want := []string{"--profile", "ci", "exec", "Respond with ok."}; !slices.Equal(got.Args, want) {
		t.Fatalf("args = %v, want %v", got.Args, want)
	}
	if got.TimeoutSeconds != 45 {
		t.Fatalf("command timeout = %d, want 45", got.TimeoutSeconds)
	}
	assertEnvMissingPrefix(t, got.Env, "SECRET_")
	assertEnvMissingPrefix(t, got.Env, "AGENTOPS_RPI_RUNTIME")
	assertEnvContains(t, got.Env, "KEEP=1")
	assertEnvContainsPrefix(t, got.Env, "HOME="+filepath.Join(isolationRoot, "home"))
	assertEnvContainsPrefix(t, got.Env, "CODEX_HOME="+filepath.Join(isolationRoot, "codex-home"))

	if run.Environment.NetworkAccess != NetworkEnabled {
		t.Fatalf("network = %s, want enabled", run.Environment.NetworkAccess)
	}
	if !run.Environment.IsolatedHome || !run.Environment.IsolatedCodexHome {
		t.Fatalf("environment isolation = %+v, want home and codex home isolated", run.Environment)
	}
	wantPrefixes := []string{"AGENTOPS_RPI_RUNTIME", "CLAUDECODE", "CLAUDE_CODE_", "SECRET_"}
	if !slices.Equal(run.Environment.ScrubbedEnvPrefixes, wantPrefixes) {
		t.Fatalf("scrubbed prefixes = %v, want %v", run.Environment.ScrubbedEnvPrefixes, wantPrefixes)
	}
	if run.Runtime.Version != "codex 0.115.0" {
		t.Fatalf("version = %q, want codex 0.115.0", run.Runtime.Version)
	}
	if run.Runtime.Profile != "ci" {
		t.Fatalf("profile = %q, want ci", run.Runtime.Profile)
	}
	if run.Runtime.Attempts != 1 {
		t.Fatalf("attempts = %d, want 1", run.Runtime.Attempts)
	}
}

func TestRunLiveRuntimeCapturesTranscriptAndScorecardArtifacts(t *testing.T) {
	suite := liveRuntimeSuite(RuntimeClaude)
	transcriptPath := filepath.Join(t.TempDir(), "claude-transcript.jsonl")
	scorecardPath := filepath.Join(t.TempDir(), "scorecard.json")

	run, err := RunLiveRuntime(context.Background(), LiveRuntimeOptions{
		Suite:          suite,
		RunID:          "live-artifacts",
		Runtime:        RuntimeClaude,
		RuntimeCommand: "claude --model sonnet",
		Enabled:        true,
		LookPath: func(name string) (string, error) {
			if name != "claude" {
				t.Fatalf("lookPath name = %q, want claude", name)
			}
			return "/fake/bin/claude", nil
		},
		VersionRunner: func(ctx context.Context, cmd RuntimeCommand) (string, error) {
			return "claude 2.0.0", nil
		},
		Runner: func(ctx context.Context, cmd RuntimeCommand) (RuntimeExecutionResult, error) {
			return RuntimeExecutionResult{
				Status:         StatusInconclusive,
				Verdict:        VerdictInconclusive,
				TranscriptPath: transcriptPath,
				ScorecardPath:  scorecardPath,
				Artifacts: []Artifact{
					{Path: filepath.Join(filepath.Dir(scorecardPath), "extra.txt"), Purpose: "runtime note", Kind: "note"},
				},
			}, nil
		},
		Now: fixedEvalTime,
	})
	if err != nil {
		t.Fatalf("RunLiveRuntime returned error: %v", err)
	}

	if run.Runtime.Version != "claude 2.0.0" {
		t.Fatalf("version = %q, want claude 2.0.0", run.Runtime.Version)
	}
	if run.Runtime.Model != "sonnet" {
		t.Fatalf("model = %q, want sonnet", run.Runtime.Model)
	}
	assertArtifact(t, run.Artifacts, Artifact{Path: transcriptPath, Purpose: "runtime transcript", Kind: "transcript"})
	assertArtifact(t, run.Artifacts, Artifact{Path: scorecardPath, Purpose: "runtime scorecard", Kind: "scorecard"})
	assertArtifact(t, run.Artifacts, Artifact{Path: filepath.Join(filepath.Dir(scorecardPath), "extra.txt"), Purpose: "runtime note", Kind: "note"})
}

func liveRuntimeSuite(runtimeName Runtime) *Suite {
	return &Suite{
		SchemaVersion: 1,
		ID:            "live.runtime",
		Name:          "Live runtime",
		Domain:        "runtime",
		Visibility:    VisibilityPublicCanary,
		Tier:          TierLive,
		Allowed:       []Runtime{runtimeName},
		Environment: SuiteEnvironment{
			TimeoutSeconds: 45,
		},
		Scoring: Scoring{
			AggregateThreshold: 1,
			Dimensions: []ScoringDimension{
				{Name: DimensionRuntimeCompatibility, Weight: 1, Threshold: 1},
			},
		},
		BaselinePolicy: BaselinePolicy{Mode: "none"},
		Cases: []Case{
			{
				ID:        "prompt",
				Title:     "runtime prompt",
				Kind:      "runtime_prompt",
				Objective: "Exercise an optional live runtime adapter.",
				Runtime:   runtimeName,
				Inputs: map[string]any{
					"prompt": "Respond with ok.",
				},
				Expectations: []Expectation{{Type: "manual_review"}},
			},
		},
	}
}

func assertEnvMissingPrefix(t *testing.T, env []string, prefix string) {
	t.Helper()
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			t.Fatalf("env contains scrubbed prefix %q in %q", prefix, entry)
		}
	}
}

func assertEnvContains(t *testing.T, env []string, value string) {
	t.Helper()
	for _, entry := range env {
		if entry == value {
			return
		}
	}
	t.Fatalf("env missing %q; got %v", value, env)
}

func assertEnvContainsPrefix(t *testing.T, env []string, prefix string) {
	t.Helper()
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			return
		}
	}
	t.Fatalf("env missing prefix %q; got %v", prefix, env)
}

func assertArtifact(t *testing.T, artifacts []Artifact, want Artifact) {
	t.Helper()
	for _, artifact := range artifacts {
		if artifact == want {
			return
		}
	}
	t.Fatalf("artifacts missing %+v; got %+v", want, artifacts)
}

func TestRunLiveRuntimePropagatesRunnerErrors(t *testing.T) {
	suite := liveRuntimeSuite(RuntimeClaude)
	run, err := RunLiveRuntime(context.Background(), LiveRuntimeOptions{
		Suite:   suite,
		RunID:   "live-error",
		Runtime: RuntimeClaude,
		Enabled: true,
		LookPath: func(name string) (string, error) {
			return "/fake/bin/claude", nil
		},
		VersionRunner: func(ctx context.Context, cmd RuntimeCommand) (string, error) {
			return "", nil
		},
		Runner: func(ctx context.Context, cmd RuntimeCommand) (RuntimeExecutionResult, error) {
			return RuntimeExecutionResult{}, errors.New("runtime failed")
		},
		Now: fixedEvalTime,
	})
	if err != nil {
		t.Fatalf("RunLiveRuntime returned error: %v", err)
	}
	if run.Status != StatusError {
		t.Fatalf("status = %s, want error", run.Status)
	}
	if run.CaseResults[0].FailureMessage != "runtime failed" {
		t.Fatalf("failure = %q, want runtime failed", run.CaseResults[0].FailureMessage)
	}
}

// W1.1 — DisableHooks plumbing. Verifies the hook-suppression toggle propagates
// through liveEnvironmentRecord, liveRuntimeEnv, and liveRuntimePrompt whether
// it comes from the suite or the LiveRuntimeOptions override.

func TestEffectiveDisableHooksOrsSuiteAndOverride(t *testing.T) {
	tests := []struct {
		name         string
		suiteFlag    bool
		overrideFlag bool
		want         bool
	}{
		{name: "neither", suiteFlag: false, overrideFlag: false, want: false},
		{name: "suite only", suiteFlag: true, overrideFlag: false, want: true},
		{name: "override only", suiteFlag: false, overrideFlag: true, want: true},
		{name: "both", suiteFlag: true, overrideFlag: true, want: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			suite := Suite{Environment: SuiteEnvironment{DisableHooks: tc.suiteFlag}}
			opts := LiveRuntimeOptions{OverrideDisableHooks: tc.overrideFlag}
			if got := effectiveDisableHooks(opts, suite); got != tc.want {
				t.Fatalf("effectiveDisableHooks: got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestLiveRuntimeEnvEmitsAgentopsHooksDisabled(t *testing.T) {
	suite := Suite{Environment: SuiteEnvironment{DisableHooks: true}}
	opts := LiveRuntimeOptions{Env: []string{}}
	env, notes, err := liveRuntimeEnv(opts, suite)
	if err != nil {
		t.Fatalf("liveRuntimeEnv: %v", err)
	}
	assertEnvContains(t, env, "AGENTOPS_HOOKS_DISABLED=1")
	if !slices.ContainsFunc(notes, func(n string) bool { return strings.Contains(n, "hooks disabled") }) {
		t.Fatalf("notes missing hooks-disabled entry: %v", notes)
	}
}

func TestLiveRuntimeEnvOmitsHooksDisabledWhenInactive(t *testing.T) {
	suite := Suite{Environment: SuiteEnvironment{}}
	opts := LiveRuntimeOptions{Env: []string{}}
	env, _, err := liveRuntimeEnv(opts, suite)
	if err != nil {
		t.Fatalf("liveRuntimeEnv: %v", err)
	}
	for _, e := range env {
		if strings.HasPrefix(e, "AGENTOPS_HOOKS_DISABLED=") {
			t.Fatalf("env unexpectedly contains AGENTOPS_HOOKS_DISABLED entry: %q", e)
		}
	}
}

func TestLiveRuntimeEnvHonorsOverrideDisableHooks(t *testing.T) {
	suite := Suite{Environment: SuiteEnvironment{}}
	opts := LiveRuntimeOptions{Env: []string{}, OverrideDisableHooks: true}
	env, _, err := liveRuntimeEnv(opts, suite)
	if err != nil {
		t.Fatalf("liveRuntimeEnv: %v", err)
	}
	assertEnvContains(t, env, "AGENTOPS_HOOKS_DISABLED=1")
}

func TestLiveRuntimePromptAppendsNegationWhenDisabled(t *testing.T) {
	suite := Suite{
		Name:        "demo",
		Description: "Run something.",
		Cases:       []Case{{Inputs: map[string]any{"prompt": "Do X."}}},
		Environment: SuiteEnvironment{DisableHooks: true},
	}
	prompt := liveRuntimePrompt(LiveRuntimeOptions{}, suite)
	if !strings.Contains(prompt, "Do X.") {
		t.Fatalf("prompt missing case prompt: %q", prompt)
	}
	if !strings.Contains(prompt, "Do NOT load additional skills or plugins") {
		t.Fatalf("prompt missing negation constraint: %q", prompt)
	}
}

func TestLiveRuntimePromptOmitsNegationWhenEnabled(t *testing.T) {
	suite := Suite{
		Name:  "demo",
		Cases: []Case{{Inputs: map[string]any{"prompt": "Do X."}}},
	}
	prompt := liveRuntimePrompt(LiveRuntimeOptions{}, suite)
	if strings.Contains(prompt, "Do NOT load additional skills") {
		t.Fatalf("prompt unexpectedly contains negation constraint: %q", prompt)
	}
}

func TestLiveEnvironmentRecordReflectsEffectiveDisableHooks(t *testing.T) {
	tests := []struct {
		name         string
		suiteFlag    bool
		overrideFlag bool
		want         bool
	}{
		{name: "default false", suiteFlag: false, overrideFlag: false, want: false},
		{name: "suite true", suiteFlag: true, overrideFlag: false, want: true},
		{name: "override true", suiteFlag: false, overrideFlag: true, want: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			suite := Suite{Environment: SuiteEnvironment{DisableHooks: tc.suiteFlag}}
			opts := LiveRuntimeOptions{OverrideDisableHooks: tc.overrideFlag}
			rec := liveEnvironmentRecord(opts, suite)
			if rec.HooksDisabled != tc.want {
				t.Fatalf("HooksDisabled: got %v, want %v", rec.HooksDisabled, tc.want)
			}
		})
	}
}
