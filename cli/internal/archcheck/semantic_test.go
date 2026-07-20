package archcheck

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The tests in this file are behavioral coverage for the semantic production
// gate paths in semantic.go. Each check is driven with a crafted-BAD input
// (asserting the SPECIFIC rejection message) plus a happy-path GOOD input, so
// the tests prove the checks DISCRIMINATE rather than merely fail. They pin
// current behavior; they do not change enforcement semantics.

// wantOneViolation asserts exactly one violation carrying rule, whose message
// contains msgContains. Returns the violation for any extra assertions.
func wantOneViolation(t *testing.T, violations []Violation, rule Rule, msgContains string) Violation {
	t.Helper()
	if len(violations) != 1 {
		t.Fatalf("want exactly 1 violation, got %d: %v", len(violations), violations)
	}
	got := violations[0]
	if got.Rule != rule {
		t.Fatalf("want rule %s, got %s: %v", rule, got.Rule, got)
	}
	if !strings.Contains(got.Message, msgContains) {
		t.Fatalf("violation message %q does not contain %q", got.Message, msgContains)
	}
	return got
}

// wantNoViolations asserts a clean (nil-error, zero-violation) result.
func wantNoViolations(t *testing.T, violations []Violation, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("want no violations, got %d: %v", len(violations), violations)
	}
}

func TestSemanticProductionGate_CandidateAndCanaries(t *testing.T) {
	// BAD: a candidate SHA that is not a complete 40-char hex digest is refused
	// before any canary runs, with the specific "externally resolved candidate
	// SHA is required" rejection.
	const wantErrContains = "externally resolved candidate SHA is required"
	for _, tc := range []struct {
		name      string
		candidate string
	}{
		{"empty", ""},
		{"non_hex", "not-hex"},
		{"too_short_39_hex", strings.Repeat("a", 39)},
		{"non_hex_chars_40_len", strings.Repeat("g", 40)},
	} {
		t.Run("bad_candidate_"+tc.name, func(t *testing.T) {
			_, err := SemanticProductionGate(tc.candidate)
			if err == nil {
				t.Fatalf("candidate %q escaped the hex-digest requirement", tc.candidate)
			}
			if !strings.Contains(err.Error(), wantErrContains) {
				t.Fatalf("candidate %q error %q does not contain %q", tc.candidate, err.Error(), wantErrContains)
			}
		})
	}

	// GOOD: a valid externally-resolved candidate lets every embedded canary
	// run and be caught, so the gate returns the full proven rule set.
	rules, err := SemanticProductionGate(strings.Repeat("a", 40))
	if err != nil {
		t.Fatalf("valid candidate failed the gate: %v", err)
	}
	want := []Rule{
		RuleContext, RuleTrackerExecution, RuleEffects, RuleOutput,
		RuleRecursiveContracts, RuleGeneratedEvidence, RuleEvidenceBinding,
	}
	if len(rules) != len(want) {
		t.Fatalf("gate returned %d rules, want %d: %v", len(rules), len(want), rules)
	}
	for i, rule := range want {
		if rules[i] != rule {
			t.Fatalf("gate rule[%d] = %s, want %s", i, rules[i], rule)
		}
	}
}

func TestCheckEvidenceBinding(t *testing.T) {
	candidate := strings.Repeat("a", 40)

	// buildValid writes a coherent evidence layout under a fresh temp root and
	// returns the manifest that binds it.
	buildValid := func(t *testing.T) (string, semanticSealManifest) {
		t.Helper()
		root := t.TempDir()
		sourceData := []byte("resolved source\n")
		if err := os.WriteFile(filepath.Join(root, "source.txt"), sourceData, 0o600); err != nil {
			t.Fatal(err)
		}
		document := semanticEvidenceDocument{
			SchemaVersion: 1,
			Class:         "evidence-binding",
			CandidateSHA:  candidate,
			SourceDigests: map[string]string{"source.txt": digestBytes(sourceData)},
		}
		evidenceData, err := json.Marshal(document)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "evidence.json"), evidenceData, 0o600); err != nil {
			t.Fatal(err)
		}
		manifest := semanticSealManifest{
			Class:        "evidence-binding",
			Sources:      []string{"source.txt"},
			CandidateSHA: candidate,
			Evidence:     &semanticEvidence{Path: "evidence.json", SHA256: digestBytes(evidenceData)},
		}
		return root, manifest
	}

	// GOOD: fully-bound evidence citation passes.
	root, manifest := buildValid(t)
	wantNoViolations(t, checkEvidenceBinding(root, manifest, candidate), nil)

	// BAD: the manifest cites a candidate that is not the externally supplied
	// one — an unbound evidence citation. This is the load-bearing rejection.
	root, manifest = buildValid(t)
	manifest.CandidateSHA = strings.Repeat("b", 40)
	wantOneViolation(t,
		checkEvidenceBinding(root, manifest, candidate),
		RuleEvidenceBinding,
		"evidence manifest is not bound to the externally supplied candidate")

	// BAD: an incomplete evidence digest fails the hex-digest floor.
	root, manifest = buildValid(t)
	manifest.Evidence.SHA256 = "short"
	wantOneViolation(t,
		checkEvidenceBinding(root, manifest, candidate),
		RuleEvidenceBinding,
		"must be complete lowercase hex digests")

	// BAD: the bound evidence file's on-disk digest no longer matches its
	// declaration.
	root, manifest = buildValid(t)
	manifest.Evidence.SHA256 = digestBytes([]byte("a different document\n"))
	wantOneViolation(t,
		checkEvidenceBinding(root, manifest, candidate),
		RuleEvidenceBinding,
		"bound evidence digest does not match its declaration")

	// BAD: the candidate source's digest inside the evidence document does not
	// match the actual on-disk source.
	root = t.TempDir()
	sourceData := []byte("resolved source\n")
	if err := os.WriteFile(filepath.Join(root, "source.txt"), sourceData, 0o600); err != nil {
		t.Fatal(err)
	}
	badDocument := semanticEvidenceDocument{
		SchemaVersion: 1,
		Class:         "evidence-binding",
		CandidateSHA:  candidate,
		SourceDigests: map[string]string{"source.txt": digestBytes([]byte("stale source\n"))},
	}
	badEvidence, err := json.Marshal(badDocument)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "evidence.json"), badEvidence, 0o600); err != nil {
		t.Fatal(err)
	}
	manifest = semanticSealManifest{
		Class:        "evidence-binding",
		Sources:      []string{"source.txt"},
		CandidateSHA: candidate,
		Evidence:     &semanticEvidence{Path: "evidence.json", SHA256: digestBytes(badEvidence)},
	}
	wantOneViolation(t,
		checkEvidenceBinding(root, manifest, candidate),
		RuleEvidenceBinding,
		"evidence source digest does not match the candidate source")
}

func TestCheckGeneratedEvidence(t *testing.T) {
	// buildRoot writes a source + generated output and returns the temp root.
	buildRoot := func(t *testing.T, sourceContent, generatedContent string) string {
		t.Helper()
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "source.txt"), []byte(sourceContent), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "generated.txt"), []byte(generatedContent), 0o600); err != nil {
			t.Fatal(err)
		}
		return root
	}

	// GOOD: generated output equals the deterministic generator recomputation
	// and every digest matches.
	root := buildRoot(t, "hello\n", "generated from hello\n")
	manifest := semanticSealManifest{
		Sources: []string{"source.txt"},
		Generated: []semanticGenerated{{
			Source:       "source.txt",
			SourceSHA256: digestBytes([]byte("hello\n")),
			Generator:    "archcheck.prefix-generated-from.v1",
			Path:         "generated.txt",
			SHA256:       digestBytes([]byte("generated from hello\n")),
		}},
	}
	wantNoViolations(t, checkGeneratedEvidence(root, manifest), nil)

	// BAD: the declared output digest does not match the on-disk output — a
	// generated-evidence digest mismatch.
	root = buildRoot(t, "hello\n", "generated from hello\n")
	manifest = semanticSealManifest{
		Sources: []string{"source.txt"},
		Generated: []semanticGenerated{{
			Source:       "source.txt",
			SourceSHA256: digestBytes([]byte("hello\n")),
			Generator:    "archcheck.prefix-generated-from.v1",
			Path:         "generated.txt",
			SHA256:       digestBytes([]byte("some other bytes\n")),
		}},
	}
	wantOneViolation(t,
		checkGeneratedEvidence(root, manifest),
		RuleGeneratedEvidence,
		"generated evidence digest mismatch")

	// BAD: digests all match, but the output is NOT what the deterministic
	// generator would have produced from the source. Proves the check
	// recomputes rather than trusting the declared digest alone.
	root = buildRoot(t, "hello\n", "forged output\n")
	manifest = semanticSealManifest{
		Sources: []string{"source.txt"},
		Generated: []semanticGenerated{{
			Source:       "source.txt",
			SourceSHA256: digestBytes([]byte("hello\n")),
			Generator:    "archcheck.prefix-generated-from.v1",
			Path:         "generated.txt",
			SHA256:       digestBytes([]byte("forged output\n")),
		}},
	}
	wantOneViolation(t,
		checkGeneratedEvidence(root, manifest),
		RuleGeneratedEvidence,
		"does not equal deterministic generator recomputation")

	// BAD: an unsupported generator is refused.
	root = buildRoot(t, "hello\n", "generated from hello\n")
	manifest = semanticSealManifest{
		Sources: []string{"source.txt"},
		Generated: []semanticGenerated{{
			Source:       "source.txt",
			SourceSHA256: digestBytes([]byte("hello\n")),
			Generator:    "archcheck.some-other-generator.v9",
			Path:         "generated.txt",
			SHA256:       digestBytes([]byte("generated from hello\n")),
		}},
	}
	wantOneViolation(t,
		checkGeneratedEvidence(root, manifest),
		RuleGeneratedEvidence,
		"must declare a supported deterministic generator")

	// BAD: no generated declarations at all.
	root = buildRoot(t, "hello\n", "generated from hello\n")
	wantOneViolation(t,
		checkGeneratedEvidence(root, semanticSealManifest{Sources: []string{"source.txt"}}),
		RuleGeneratedEvidence,
		"generated evidence declarations are required")
}

func TestCheckRecursiveContractsSource(t *testing.T) {
	cases := []struct {
		name        string
		source      string
		wantRule    bool
		msgContains string
	}{
		{
			name: "good_reachable_runnable_single_contract",
			source: `package probe
import ("github.com/spf13/cobra"; "github.com/boshu2/agentops/cli/internal/clicontract")
func command() *cobra.Command {
	root := &cobra.Command{RunE: run}
	_ = clicontract.Attach(root, clicontract.CommandContract{})
	return root
}
func run(*cobra.Command, []string) error { return nil }`,
			wantRule: false,
		},
		{
			name: "bad_reachable_runnable_child_missing_contract",
			source: `package probe
import ("github.com/spf13/cobra"; "github.com/boshu2/agentops/cli/internal/clicontract")
func command() *cobra.Command {
	root := &cobra.Command{RunE: run}
	child := &cobra.Command{RunE: run}
	root.AddCommand(child)
	_ = clicontract.Attach(root, clicontract.CommandContract{})
	return root
}
func run(*cobra.Command, []string) error { return nil }`,
			wantRule:    true,
			msgContains: "reachable runnable child needs exactly one attached contract: got 0",
		},
		{
			name: "bad_contract_on_unreachable_command",
			source: `package probe
import ("github.com/spf13/cobra"; "github.com/boshu2/agentops/cli/internal/clicontract")
func command() *cobra.Command {
	root := &cobra.Command{RunE: run}
	orphan := &cobra.Command{RunE: run}
	_ = clicontract.Attach(root, clicontract.CommandContract{})
	_ = clicontract.Attach(orphan, clicontract.CommandContract{})
	return root
}
func run(*cobra.Command, []string) error { return nil }`,
			wantRule:    true,
			msgContains: "contract attached to unreachable command orphan",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			violations, err := checkRecursiveContractsSource("module.go", []byte(tc.source))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tc.wantRule {
				if len(violations) != 0 {
					t.Fatalf("want no violations, got: %v", violations)
				}
				return
			}
			wantOneViolation(t, violations, RuleRecursiveContracts, tc.msgContains)
		})
	}
}

func TestCheckOutputSource(t *testing.T) {
	cases := []struct {
		name        string
		source      string
		wantRule    bool
		msgContains string
	}{
		{
			name: "good_structured_contract_attached_once",
			source: `package probe
import ("encoding/json"; "github.com/spf13/cobra"; "github.com/boshu2/agentops/cli/internal/clicontract")
func contract() clicontract.CommandContract { return clicontract.CommandContract{Output: clicontract.OutputStructured} }
func build() *cobra.Command {
	root := &cobra.Command{RunE: run}
	_ = clicontract.Attach(root, contract())
	return root
}
func run(command *cobra.Command, _ []string) error { return json.NewEncoder(command.OutOrStdout()).Encode(true) }`,
			wantRule: false,
		},
		{
			name: "good_no_structured_contract_is_skipped",
			source: `package probe
import ("fmt"; "github.com/spf13/cobra")
func build() *cobra.Command { return &cobra.Command{RunE: run} }
func run(command *cobra.Command, _ []string) error { fmt.Fprintln(command.OutOrStdout(), "human text"); return nil }`,
			wantRule: false,
		},
		{
			name: "bad_structured_runnable_missing_attachment",
			source: `package probe
import ("encoding/json"; "github.com/spf13/cobra"; "github.com/boshu2/agentops/cli/internal/clicontract")
func contract() clicontract.CommandContract { return clicontract.CommandContract{Output: clicontract.OutputStructured} }
func build() *cobra.Command {
	root := &cobra.Command{RunE: run}
	return root
}
func run(command *cobra.Command, _ []string) error { return json.NewEncoder(command.OutOrStdout()).Encode(true) }`,
			wantRule:    true,
			msgContains: "structured output contract must be attached exactly once to runnable root",
		},
		{
			name: "bad_structured_uses_human_text_formatting",
			source: `package probe
import ("fmt"; "github.com/spf13/cobra"; "github.com/boshu2/agentops/cli/internal/clicontract")
func contract() clicontract.CommandContract { return clicontract.CommandContract{Output: clicontract.OutputStructured} }
func build() *cobra.Command {
	root := &cobra.Command{RunE: run}
	_ = clicontract.Attach(root, contract())
	return root
}
func run(command *cobra.Command, _ []string) error { fmt.Fprintln(command.OutOrStdout(), "human text"); return nil }`,
			wantRule:    true,
			msgContains: "structured output contract must use a structured encoder, not human text formatting",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			violations, err := checkOutputSource("module.go", []byte(tc.source))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tc.wantRule {
				if len(violations) != 0 {
					t.Fatalf("want no violations, got: %v", violations)
				}
				return
			}
			wantOneViolation(t, violations, RuleOutput, tc.msgContains)
		})
	}
}

func TestCheckUniversalEffectsSource(t *testing.T) {
	// BAD: a universal PersistentPreRunE performs an os filesystem effect.
	// os.Remove is in osRule's RuleFilesystem set; it is used instead of
	// os.Setenv so this fixture STRING is not miscounted by the raw-grep
	// test-isolation ratchet.
	bad := `package probe
import ("os"; "github.com/spf13/cobra")
var root = &cobra.Command{PersistentPreRunE: func(*cobra.Command, []string) error { return os.Remove("x") }}`
	violations, err := checkUniversalEffectsSource("root.go", []byte(bad))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantOneViolation(t, violations, RuleEffects,
		"universal pre-run lifecycle may not perform command-specific environment, filesystem, or process effects")

	// BAD variant: a named pre-run func performing a filesystem effect is caught
	// through the identifier indirection.
	badNamed := `package probe
import ("os"; "github.com/spf13/cobra")
func prepare(*cobra.Command, []string) error { _, err := os.Getwd(); return err }
var root = &cobra.Command{PersistentPreRunE: prepare}`
	violations, err = checkUniversalEffectsSource("root.go", []byte(badNamed))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantOneViolation(t, violations, RuleEffects,
		"universal pre-run lifecycle may not perform command-specific environment, filesystem, or process effects")

	// GOOD: an effect-free universal pre-run passes.
	good := `package probe
import "github.com/spf13/cobra"
var root = &cobra.Command{PersistentPreRunE: func(cmd *cobra.Command, _ []string) error { return cmd.Help() }}`
	violations, err = checkUniversalEffectsSource("root.go", []byte(good))
	wantNoViolations(t, violations, err)
}

func TestCheckTrackerExecutionSource(t *testing.T) {
	cases := []struct {
		name        string
		source      string
		wantRule    bool
		msgContains string
	}{
		{
			name: "good_context_aware_launch_with_workdir_and_env",
			source: `package probe
import ("context"; "os/exec")
type resolution struct { Binary, WorkDir string; ChildEnv []string }
func launch(ctx context.Context, resolved resolution) error {
	command := exec.CommandContext(ctx, resolved.Binary)
	command.Dir = resolved.WorkDir
	command.Env = resolved.ChildEnv
	return command.Run()
}`,
			wantRule: false,
		},
		{
			name: "bad_launch_missing_workdir",
			source: `package probe
import ("context"; "os/exec")
type resolution struct { Binary, WorkDir string; ChildEnv []string }
func launch(ctx context.Context, resolved resolution) error {
	command := exec.CommandContext(ctx, resolved.Binary)
	command.Env = resolved.ChildEnv
	return command.Run()
}`,
			wantRule:    true,
			msgContains: "tracker launch must use caller context plus resolved WorkDir and ChildEnv",
		},
		{
			name: "bad_no_context_aware_launch_at_all",
			source: `package probe
import "context"
func launch(ctx context.Context) error { _ = ctx; return nil }`,
			wantRule:    true,
			msgContains: "tracker adapter has no context-aware process launch",
		},
		{
			name: "bad_launch_uses_background_instead_of_caller_context",
			source: `package probe
import ("context"; "os/exec")
type resolution struct { Binary, WorkDir string; ChildEnv []string }
func launch(ctx context.Context, resolved resolution) error {
	command := exec.CommandContext(context.Background(), resolved.Binary)
	command.Dir = resolved.WorkDir
	command.Env = resolved.ChildEnv
	return command.Run()
}`,
			wantRule:    true,
			msgContains: "tracker launch must use caller context plus resolved WorkDir and ChildEnv",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			violations, err := checkTrackerExecutionSource("tracker.go", []byte(tc.source))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tc.wantRule {
				if len(violations) != 0 {
					t.Fatalf("want no violations, got: %v", violations)
				}
				return
			}
			wantOneViolation(t, violations, RuleTrackerExecution, tc.msgContains)
		})
	}
}

// TestCheckSemanticSeal_DispatchesByClass drives the top-level manifest
// dispatcher: a missing manifest is a no-op, a malformed schema is rejected,
// and each recognized class routes to its source checker.
func TestCheckSemanticSeal_DispatchesByClass(t *testing.T) {
	candidate := strings.Repeat("a", 40)

	// Missing manifest: nothing to check.
	root := t.TempDir()
	violations, err := checkSemanticSeal(root, candidate)
	wantNoViolations(t, violations, err)

	writeSeal := func(t *testing.T, root string, manifest semanticSealManifest) {
		t.Helper()
		data, err := json.Marshal(manifest)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, semanticSealFilename), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	// Malformed schema/class/sources: rejected up front (violation, not error).
	root = t.TempDir()
	writeSeal(t, root, semanticSealManifest{SchemaVersion: 2, Class: "output", Sources: []string{"module.go"}})
	violations, err = checkSemanticSeal(root, candidate)
	if err != nil {
		t.Fatalf("malformed schema returned an error: %v", err)
	}
	wantOneViolation(t, violations, RuleEvidenceBinding, "schema, class, and sources are required")

	// Unknown class routes to the default arm (violation, not error).
	root = t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "module.go"), []byte("package probe\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	writeSeal(t, root, semanticSealManifest{SchemaVersion: 1, Class: "does-not-exist", Sources: []string{"module.go"}})
	violations, err = checkSemanticSeal(root, candidate)
	if err != nil {
		t.Fatalf("unknown class returned an error: %v", err)
	}
	wantOneViolation(t, violations, RuleEvidenceBinding, `unknown semantic seal class "does-not-exist"`)

	// An absolute/non-local source path is refused before any read (violation,
	// not error).
	root = t.TempDir()
	writeSeal(t, root, semanticSealManifest{SchemaVersion: 1, Class: "output", Sources: []string{"../escape.go"}})
	violations, err = checkSemanticSeal(root, candidate)
	if err != nil {
		t.Fatalf("non-local source path returned an error: %v", err)
	}
	wantOneViolation(t, violations, RuleEvidenceBinding, "source path must be repository-relative")

	// The "effects" class routes to the universal-effects source checker, which
	// flags an os effect in a universal pre-run. os.Remove is in archcheck's
	// osRule set (RuleFilesystem) so it trips the RuleEffects violation
	// identically; it is used here rather than os.Setenv so this fixture STRING
	// is not miscounted by the raw-grep test-isolation ratchet.
	root = t.TempDir()
	badEffects := `package probe
import ("os"; "github.com/spf13/cobra")
var root = &cobra.Command{PersistentPreRunE: func(*cobra.Command, []string) error { return os.Remove("x") }}`
	if err := os.WriteFile(filepath.Join(root, "root.go"), []byte(badEffects), 0o600); err != nil {
		t.Fatal(err)
	}
	writeSeal(t, root, semanticSealManifest{SchemaVersion: 1, Class: "effects", Sources: []string{"root.go"}})
	violations, err = checkSemanticSeal(root, candidate)
	if err != nil {
		t.Fatalf("effects class returned an error: %v", err)
	}
	wantOneViolation(t, violations, RuleEffects,
		"universal pre-run lifecycle may not perform command-specific environment, filesystem, or process effects")
}
