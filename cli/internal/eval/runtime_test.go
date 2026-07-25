package eval

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/runtimecmd"
	"github.com/boshu2/agentops/cli/internal/subprocess"
)

func TestRunLiveRuntimeSkipsWhenExecutableUnavailable(t *testing.T) {
	for _, tc := range []struct {
		name       string
		runtime    Runtime
		executable string
	}{
		// claude is refused at adapter construction (LAW 0) before the
		// executable-availability skip path; see TestRunLiveRuntimeRefusesClaude.
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
	// Uses codex: claude is refused at construction (LAW 0), so the transcript /
	// scorecard artifact plumbing is exercised over the only live-invocable runtime.
	suite := liveRuntimeSuite(RuntimeCodex)
	transcriptPath := filepath.Join(t.TempDir(), "codex-transcript.jsonl")
	scorecardPath := filepath.Join(t.TempDir(), "scorecard.json")

	run, err := RunLiveRuntime(context.Background(), LiveRuntimeOptions{
		Suite:          suite,
		RunID:          "live-artifacts",
		Runtime:        RuntimeCodex,
		RuntimeCommand: "codex --model sonnet",
		Enabled:        true,
		LookPath: func(name string) (string, error) {
			if name != "codex" {
				t.Fatalf("lookPath name = %q, want codex", name)
			}
			return "/fake/bin/codex", nil
		},
		VersionRunner: func(ctx context.Context, cmd RuntimeCommand) (string, error) {
			return "codex 2.0.0", nil
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

	if run.Runtime.Version != "codex 2.0.0" {
		t.Fatalf("version = %q, want codex 2.0.0", run.Runtime.Version)
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
	suite := liveRuntimeSuite(RuntimeCodex)
	run, err := RunLiveRuntime(context.Background(), LiveRuntimeOptions{
		Suite:   suite,
		RunID:   "live-error",
		Runtime: RuntimeCodex,
		Enabled: true,
		LookPath: func(name string) (string, error) {
			return "/fake/bin/codex", nil
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

func TestRunLiveRuntimeCleansOwnedIsolationAfterSuccess(t *testing.T) {
	suite := liveRuntimeSuite(RuntimeCodex)
	suite.Environment.IsolateHome = true
	suite.Environment.IsolateCodexHome = true
	var ownedRoot string
	run, err := RunLiveRuntime(context.Background(), liveIsolationTestOptions(suite, func(_ context.Context, command RuntimeCommand) (RuntimeExecutionResult, error) {
		ownedRoot = filepath.Dir(envValue(t, command.Env, "HOME"))
		if err := os.WriteFile(filepath.Join(envValue(t, command.Env, "CODEX_HOME"), "state.json"), []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
		return RuntimeExecutionResult{Status: StatusInconclusive, Verdict: VerdictInconclusive}, nil
	}))
	if err != nil {
		t.Fatalf("RunLiveRuntime: %v", err)
	}
	if run == nil {
		t.Fatal("RunLiveRuntime returned nil run")
	}
	assertRemovedOwnedIsolation(t, ownedRoot)
}

func TestRunLiveRuntimeCleansOwnedIsolationAfterRunnerError(t *testing.T) {
	suite := liveRuntimeSuite(RuntimeCodex)
	suite.Environment.IsolateHome = true
	var ownedRoot string
	run, err := RunLiveRuntime(context.Background(), liveIsolationTestOptions(suite, func(_ context.Context, command RuntimeCommand) (RuntimeExecutionResult, error) {
		ownedRoot = filepath.Dir(envValue(t, command.Env, "HOME"))
		return RuntimeExecutionResult{}, errors.New("runner failed")
	}))
	if err != nil {
		t.Fatalf("RunLiveRuntime: %v", err)
	}
	if run.Status != StatusError {
		t.Fatalf("status = %s, want error", run.Status)
	}
	assertRemovedOwnedIsolation(t, ownedRoot)
}

func TestRunLiveRuntimeCleansOwnedIsolationAfterTimeout(t *testing.T) {
	suite := liveRuntimeSuite(RuntimeCodex)
	suite.Environment.IsolateHome = true
	var ownedRoot string
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	run, err := RunLiveRuntime(ctx, liveIsolationTestOptions(suite, func(ctx context.Context, command RuntimeCommand) (RuntimeExecutionResult, error) {
		ownedRoot = filepath.Dir(envValue(t, command.Env, "HOME"))
		<-ctx.Done()
		return RuntimeExecutionResult{}, ctx.Err()
	}))
	if err != nil {
		t.Fatalf("RunLiveRuntime: %v", err)
	}
	if run.Status != StatusError || !strings.Contains(run.CaseResults[0].FailureMessage, "timed out") {
		t.Fatalf("run = %#v, want timeout error", run)
	}
	assertRemovedOwnedIsolation(t, ownedRoot)
}

func TestRunLiveRuntimeCleansOwnedIsolationAfterOutputFailure(t *testing.T) {
	suite := liveRuntimeSuite(RuntimeCodex)
	suite.Environment.IsolateHome = true
	var ownedRoot string
	options := liveIsolationTestOptions(suite, func(_ context.Context, command RuntimeCommand) (RuntimeExecutionResult, error) {
		ownedRoot = filepath.Dir(envValue(t, command.Env, "HOME"))
		return RuntimeExecutionResult{Status: StatusInconclusive, Verdict: VerdictInconclusive}, nil
	})
	options.OutputPath = t.TempDir() // A directory cannot be replaced with the run JSON file.
	if _, err := RunLiveRuntime(context.Background(), options); err == nil {
		t.Fatal("RunLiveRuntime unexpectedly wrote output over a directory")
	}
	assertRemovedOwnedIsolation(t, ownedRoot)
}

func TestLiveRuntimeEnvCleansOwnedIsolationAfterPartialSetup(t *testing.T) {
	parent := t.TempDir()
	ownedRoot := filepath.Join(parent, "owned")
	mkdirCalls := 0
	removeCalls := 0
	ops := liveIsolationOps{
		mkdirTemp: func(_, _ string) (string, error) {
			if err := os.Mkdir(ownedRoot, 0o700); err != nil {
				return "", err
			}
			return ownedRoot, nil
		},
		mkdirAll: func(path string, mode os.FileMode) error {
			mkdirCalls++
			if mkdirCalls == 2 {
				return errors.New("codex home setup failed")
			}
			return os.MkdirAll(path, mode)
		},
		removeAll: func(path string) error {
			removeCalls++
			return os.RemoveAll(path)
		},
	}
	suite := *liveRuntimeSuite(RuntimeCodex)
	suite.Environment.IsolateHome = true
	suite.Environment.IsolateCodexHome = true
	if _, err := liveRuntimeEnvWithOps(LiveRuntimeOptions{Env: []string{}}, suite, ops); err == nil {
		t.Fatal("liveRuntimeEnvWithOps unexpectedly succeeded")
	}
	if removeCalls != 1 {
		t.Fatalf("remove calls = %d, want 1", removeCalls)
	}
	assertRemovedOwnedIsolation(t, ownedRoot)
}

func TestRunLiveRuntimeNeverRemovesCallerIsolationRoot(t *testing.T) {
	suite := liveRuntimeSuite(RuntimeCodex)
	suite.Environment.IsolateHome = true
	suite.Environment.IsolateCodexHome = true
	explicitRoot := t.TempDir()
	sentinel := filepath.Join(explicitRoot, "caller-owned.txt")
	if err := os.WriteFile(sentinel, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	options := liveIsolationTestOptions(suite, func(_ context.Context, _ RuntimeCommand) (RuntimeExecutionResult, error) {
		return RuntimeExecutionResult{Status: StatusInconclusive, Verdict: VerdictInconclusive}, nil
	})
	options.IsolationRoot = explicitRoot
	if _, err := RunLiveRuntime(context.Background(), options); err != nil {
		t.Fatalf("RunLiveRuntime: %v", err)
	}
	if got, err := os.ReadFile(sentinel); err != nil || string(got) != "keep" {
		t.Fatalf("caller isolation root changed: content=%q err=%v", got, err)
	}
}

func liveIsolationTestOptions(suite *Suite, runner RuntimeRunner) LiveRuntimeOptions {
	return LiveRuntimeOptions{
		Suite:   suite,
		RunID:   "live-isolation",
		Runtime: RuntimeCodex,
		Enabled: true,
		Env:     []string{},
		LookPath: func(string) (string, error) {
			return "/fake/bin/codex", nil
		},
		VersionRunner: func(context.Context, RuntimeCommand) (string, error) {
			return "codex test", nil
		},
		Runner: runner,
		Now:    fixedEvalTime,
	}
}

func envValue(t *testing.T, env []string, key string) string {
	t.Helper()
	prefix := key + "="
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			return strings.TrimPrefix(entry, prefix)
		}
	}
	t.Fatalf("environment missing %s", key)
	return ""
}

func assertRemovedOwnedIsolation(t *testing.T, root string) {
	t.Helper()
	if root == "" {
		t.Fatal("test did not capture owned isolation root")
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("owned isolation root %q was not removed: %v", root, err)
	}
}

// TestRunLiveRuntimeRefusesClaude is the LAW 0 fail-closed contract at the eval
// layer (age-6j9ee.4): a suite may declare runtime=claude and parse fine, but
// enabling a live claude run must be refused BEFORE any process spawns — no
// version probe, no runner call, no `claude -p`. LookPath / VersionRunner / Runner
// all fail the test if invoked, proving nothing external is touched.
func TestRunLiveRuntimeRefusesClaude(t *testing.T) {
	suite := liveRuntimeSuite(RuntimeClaude)
	_, err := RunLiveRuntime(context.Background(), LiveRuntimeOptions{
		Suite:          suite,
		RunID:          "live-claude-refused",
		Runtime:        RuntimeClaude,
		RuntimeCommand: "claude --model sonnet",
		Enabled:        true,
		LookPath: func(name string) (string, error) {
			t.Fatalf("LookPath must not be called for a refused claude runtime (got %q)", name)
			return "", nil
		},
		VersionRunner: func(ctx context.Context, cmd RuntimeCommand) (string, error) {
			t.Fatalf("VersionRunner must not be called for a refused claude runtime")
			return "", nil
		},
		Runner: func(ctx context.Context, cmd RuntimeCommand) (RuntimeExecutionResult, error) {
			t.Fatalf("Runner must not be called for a refused claude runtime")
			return RuntimeExecutionResult{}, nil
		},
		Now: fixedEvalTime,
	})
	if !errors.Is(err, runtimecmd.ErrClaudeHeadlessProhibited) {
		t.Fatalf("RunLiveRuntime(claude) err = %v, want ErrClaudeHeadlessProhibited", err)
	}
}

// TestRunLiveRuntimeRefusesClaudeCommandBeforeSpawn is the finding-1 contract
// (age-6j9ee.4): even when the runtime ENUM is a live-invocable adapter (codex),
// a RESOLVED command that names the claude binary — as a bare token, an
// env-wrapped form, or an absolute path — must be refused BEFORE any process is
// spawned, including before the `claude --version` probe. LookPath / VersionRunner
// / Runner are fatal-if-called, proving nothing external (not even --version) runs.
func TestRunLiveRuntimeRefusesClaudeCommandBeforeSpawn(t *testing.T) {
	for _, tc := range []struct {
		name    string
		command string
	}{
		{name: "flagged command", command: "claude --model sonnet"},
		{name: "absolute path", command: "/fake/bin/claude"},
		{name: "env wrapped", command: "env -i claude"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := RunLiveRuntime(context.Background(), LiveRuntimeOptions{
				Suite:          liveRuntimeSuite(RuntimeCodex),
				RunID:          "live-codex-claude-cmd",
				Runtime:        RuntimeCodex, // enum is codex; the COMMAND smuggles claude
				RuntimeCommand: tc.command,
				Enabled:        true,
				LookPath: func(name string) (string, error) {
					t.Fatalf("LookPath must not run for a claude-resolving command (got %q)", name)
					return "", nil
				},
				VersionRunner: func(ctx context.Context, cmd RuntimeCommand) (string, error) {
					t.Fatalf("VersionRunner must not run — no `claude --version` may spawn")
					return "", nil
				},
				Runner: func(ctx context.Context, cmd RuntimeCommand) (RuntimeExecutionResult, error) {
					t.Fatalf("Runner must not run for a claude-resolving command")
					return RuntimeExecutionResult{}, nil
				},
				Now: fixedEvalTime,
			})
			if !errors.Is(err, runtimecmd.ErrClaudeHeadlessProhibited) {
				t.Fatalf("RunLiveRuntime(codex, %q) err = %v, want ErrClaudeHeadlessProhibited", tc.command, err)
			}
		})
	}
}

// TestRunLiveRuntimeDisabledSkipsClaudeWithoutRefusal is the finding-3 contract
// (age-6j9ee.4): a disabled live run spawns nothing, so it is the deterministic
// parse/skip path. A suite declaring runtime=claude must parse and SKIP here
// exactly as before — no LAW 0 refusal, no error — because the refusal fires only
// on paths that would spawn a process. LookPath / VersionRunner / Runner are
// fatal-if-called.
func TestRunLiveRuntimeDisabledSkipsClaudeWithoutRefusal(t *testing.T) {
	run, err := RunLiveRuntime(context.Background(), LiveRuntimeOptions{
		Suite:          liveRuntimeSuite(RuntimeClaude),
		RunID:          "live-claude-disabled",
		Runtime:        RuntimeClaude,
		RuntimeCommand: "claude --model sonnet",
		Enabled:        false,
		LookPath: func(name string) (string, error) {
			t.Fatalf("LookPath must not run when live runtime is disabled (got %q)", name)
			return "", nil
		},
		VersionRunner: func(ctx context.Context, cmd RuntimeCommand) (string, error) {
			t.Fatalf("VersionRunner must not run when live runtime is disabled")
			return "", nil
		},
		Runner: func(ctx context.Context, cmd RuntimeCommand) (RuntimeExecutionResult, error) {
			t.Fatalf("Runner must not run when live runtime is disabled")
			return RuntimeExecutionResult{}, nil
		},
		Now: fixedEvalTime,
	})
	if err != nil {
		t.Fatalf("RunLiveRuntime(claude, disabled) returned error: %v", err)
	}
	if run.Status != StatusSkipped {
		t.Fatalf("status = %s, want skipped", run.Status)
	}
	wantReason := "live runtime disabled; set LiveRuntimeOptions.Enabled to true"
	if run.Runtime.SkippedReason != wantReason {
		t.Fatalf("skipped_reason = %q, want %q", run.Runtime.SkippedReason, wantReason)
	}
	if run.Runtime.Name != RuntimeClaude || !run.Runtime.Live {
		t.Fatalf("runtime = %+v, want live claude", run.Runtime)
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
	environment, err := liveRuntimeEnv(opts, suite)
	if err != nil {
		t.Fatalf("liveRuntimeEnv: %v", err)
	}
	defer func() { _ = environment.Close() }()
	assertEnvContains(t, environment.Values, "AGENTOPS_HOOKS_DISABLED=1")
	if !slices.ContainsFunc(environment.Notes, func(n string) bool { return strings.Contains(n, "hooks disabled") }) {
		t.Fatalf("notes missing hooks-disabled entry: %v", environment.Notes)
	}
}

func TestLiveRuntimeEnvOmitsHooksDisabledWhenInactive(t *testing.T) {
	suite := Suite{Environment: SuiteEnvironment{}}
	opts := LiveRuntimeOptions{Env: []string{}}
	environment, err := liveRuntimeEnv(opts, suite)
	if err != nil {
		t.Fatalf("liveRuntimeEnv: %v", err)
	}
	defer func() { _ = environment.Close() }()
	for _, e := range environment.Values {
		if strings.HasPrefix(e, "AGENTOPS_HOOKS_DISABLED=") {
			t.Fatalf("env unexpectedly contains AGENTOPS_HOOKS_DISABLED entry: %q", e)
		}
	}
}

func TestLiveRuntimeEnvHonorsOverrideDisableHooks(t *testing.T) {
	suite := Suite{Environment: SuiteEnvironment{}}
	opts := LiveRuntimeOptions{Env: []string{}, OverrideDisableHooks: true}
	environment, err := liveRuntimeEnv(opts, suite)
	if err != nil {
		t.Fatalf("liveRuntimeEnv: %v", err)
	}
	defer func() { _ = environment.Close() }()
	assertEnvContains(t, environment.Values, "AGENTOPS_HOOKS_DISABLED=1")
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

func TestDefaultRuntimeRunnerPreservesCleanupOutcome(t *testing.T) {
	const helperEnv = "GO_WANT_EVAL_RUNTIME_CLEANUP_HELPER"
	if os.Getenv(helperEnv) == "1" {
		os.Exit(0)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}

	result, err := defaultRuntimeRunner(context.Background(), RuntimeCommand{
		Executable: executable,
		Args:       []string{"-test.run=^TestDefaultRuntimeRunnerPreservesCleanupOutcome$"},
		Env:        append(os.Environ(), helperEnv+"=1"),
	})
	if err != nil {
		t.Fatalf("defaultRuntimeRunner: %v", err)
	}
	if result.Cleanup == nil || result.Cleanup.Status != subprocess.CleanupCompleted || !result.Cleanup.Completed {
		t.Fatalf("cleanup = %#v, want completed", result.Cleanup)
	}
}

func TestRuntimeAttemptsPreserveLastCleanupFailure(t *testing.T) {
	cleanup := &subprocess.CleanupOutcome{
		Status:    subprocess.CleanupFailed,
		Attempted: true,
		Error:     "cleanup sentinel",
	}
	sentinel := errors.New("runtime sentinel")
	result, attempts, err := runLiveRuntimeWithAttempts(
		context.Background(),
		func(context.Context, RuntimeCommand) (RuntimeExecutionResult, error) {
			return RuntimeExecutionResult{Cleanup: cleanup}, sentinel
		},
		RuntimeCommand{},
		1,
	)
	if attempts != 1 || !errors.Is(err, sentinel) {
		t.Fatalf("attempts/error = %d/%v, want 1 and sentinel", attempts, err)
	}
	if result.Cleanup == nil || result.Cleanup.Status != subprocess.CleanupFailed {
		t.Fatalf("cleanup = %#v, want failed last-attempt outcome", result.Cleanup)
	}

	record := &RunRecord{}
	applyRuntimeExecutionResult(record, Suite{}, result)
	if record.Runtime.Cleanup == nil || record.Runtime.Cleanup.Status != subprocess.CleanupFailed {
		t.Fatalf("runtime record cleanup = %#v, want failed", record.Runtime.Cleanup)
	}
}

func TestRuntimeAttemptsStopOnCleanupFailure(t *testing.T) {
	cleanupErr := errors.New("cleanup sentinel")
	calls := 0
	result, attempts, err := runLiveRuntimeWithAttempts(
		context.Background(),
		func(context.Context, RuntimeCommand) (RuntimeExecutionResult, error) {
			calls++
			if calls == 1 {
				return RuntimeExecutionResult{Cleanup: &subprocess.CleanupOutcome{
					Status:    subprocess.CleanupFailed,
					Attempted: true,
					Error:     cleanupErr.Error(),
				}}, cleanupErr
			}
			return RuntimeExecutionResult{Cleanup: &subprocess.CleanupOutcome{
				Status:    subprocess.CleanupCompleted,
				Attempted: true,
				Completed: true,
			}}, nil
		},
		RuntimeCommand{},
		2,
	)
	if calls != 1 || attempts != 1 {
		t.Fatalf("calls/attempts = %d/%d, want cleanup failure to stop after attempt 1", calls, attempts)
	}
	if !errors.Is(err, cleanupErr) {
		t.Fatalf("error = %v, want cleanup sentinel", err)
	}
	if result.Cleanup == nil || result.Cleanup.Status != subprocess.CleanupFailed {
		t.Fatalf("cleanup = %#v, want failed first-attempt outcome", result.Cleanup)
	}
}
