// practices: [design-by-contract, ai-assisted-dev]
package runtimecmd

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestSplit(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		wantExec string
		wantArgs []string
	}{
		{"simple", "codex", "codex", []string{}},
		{"with prefix args", "env -i claude", "env", []string{"-i", "claude"}},
		{"extra whitespace", "  codex   exec  ", "codex", []string{"exec"}},
		{"empty", "", "", nil},
		{"whitespace only", "   ", "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotExec, gotArgs := Split(tt.command)
			if gotExec != tt.wantExec {
				t.Errorf("Split(%q) exec = %q, want %q", tt.command, gotExec, tt.wantExec)
			}
			if len(gotArgs) != 0 || len(tt.wantArgs) != 0 {
				if !reflect.DeepEqual(gotArgs, tt.wantArgs) {
					t.Errorf("Split(%q) args = %v, want %v", tt.command, gotArgs, tt.wantArgs)
				}
			}
		})
	}
}

func TestBinaryName(t *testing.T) {
	tests := []struct {
		command string
		want    string
	}{
		{"codex", "codex"},
		{"/usr/local/bin/Codex", "codex"},
		{"claude.exe", "claude"},
		{"env -i codex exec", "env"}, // base name of the FIRST token
		{"", ""},
	}
	for _, tt := range tests {
		if got := BinaryName(tt.command); got != tt.want {
			t.Errorf("BinaryName(%q) = %q, want %q", tt.command, got, tt.want)
		}
	}
}

func TestDirectArgs(t *testing.T) {
	// codex uses `exec <prompt>`; prefix args are preserved.
	got, err := DirectArgs("codex", "hello")
	if err != nil {
		t.Fatalf("DirectArgs codex returned error: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"exec", "hello"}) {
		t.Errorf("DirectArgs codex = %v, want [exec hello]", got)
	}

	gotPrefix, err := DirectArgs("/usr/local/bin/codex", "hi")
	if err != nil {
		t.Fatalf("DirectArgs codex path returned error: %v", err)
	}
	if !reflect.DeepEqual(gotPrefix, []string{"exec", "hi"}) {
		t.Errorf("DirectArgs codex path = %v, want [exec hi]", gotPrefix)
	}
}

// TestDirectArgs_ClaudeRefused is the LAW 0 fail-closed contract (age-6j9ee.4):
// a headless `claude -p` bills the Anthropic API / burns quota, so DirectArgs
// MUST refuse any command that resolves to the claude binary — replacing the old
// behavior that emitted [-p <prompt>].
func TestDirectArgs_ClaudeRefused(t *testing.T) {
	claudeForms := []string{
		"claude",
		"Claude",
		"claude.exe",
		"/usr/local/bin/claude",
		"env -i claude",             // env-wrapped: binary is `env`, but claude is a token
		"env CLAUDE_CODE=1 claude",  // still refused
		"  claude   ",               // whitespace padding
	}
	for _, form := range claudeForms {
		got, err := DirectArgs(form, "hello")
		if !errors.Is(err, ErrClaudeHeadlessProhibited) {
			t.Errorf("DirectArgs(%q): err = %v, want ErrClaudeHeadlessProhibited", form, err)
		}
		if got != nil {
			t.Errorf("DirectArgs(%q): args = %v, want nil (refused)", form, got)
		}
	}
}

// TestDirectArgs_NeverEmitsDashP is a behavioral guard: over a battery of inputs
// — the supported codex forms, refused claude forms, and unrecognized runtimes —
// DirectArgs must NEVER return an argv containing `-p` or `--print`. This is the
// argv-construction chokepoint the original latent `claude -p` path ran through.
func TestDirectArgs_NeverEmitsDashP(t *testing.T) {
	inputs := []string{
		"codex", "/usr/local/bin/codex", "codex exec",
		"claude", "claude.exe", "/opt/claude", "env -i claude",
		"gemini", "ollama", "some-future-runtime", "", "   ",
		"env -i codex", // binary is `env` (unrecognized) — must refuse, not `-p`
	}
	for _, in := range inputs {
		args, _ := DirectArgs(in, "prompt text -p not-a-flag")
		for _, a := range args {
			if a == "-p" || a == "--print" {
				t.Errorf("DirectArgs(%q) emitted prohibited flag %q in argv %v", in, a, args)
			}
		}
	}
}

// headlessPrintLiteral reports whether the CONTENT of a single Go string literal
// is a headless-print argv smell. It flags two shapes (age-6j9ee.4):
//
//  1. the argv-element form — the literal is exactly `-p` or `--print`, i.e. a
//     single argv token passed straight to a runtime; and
//  2. the smuggled-command form — one whitespace-separated literal that carries a
//     `claude` token AND a standalone `-p` / `--print` flag token, e.g.
//     `var x = "claude -p"`, `"env -i claude --print"`. The old byte-substring
//     tripwire matched only the exact `"-p"` / `"--print"` source token and missed
//     this entirely (validator finding: `var TripwireScratch = "claude -p"`
//     slipped through).
//
// Matching is token-based (not raw substring) so it does NOT trip on identifiers
// that merely embed the characters — e.g. the gate id `"always.door9-no-claude-p"`
// or help text `"...default: claude..."` — which are legitimate and would only
// bloat the allowlist.
func headlessPrintLiteral(value string) bool {
	if value == "-p" || value == "--print" {
		return true
	}
	var hasClaude, hasPrintFlag bool
	for _, field := range strings.Fields(value) {
		if field == "-p" || field == "--print" {
			hasPrintFlag = true
			continue
		}
		base := strings.ToLower(filepath.Base(field))
		base = strings.TrimSuffix(base, ".exe")
		if base == "claude" {
			hasClaude = true
		}
	}
	return hasClaude && hasPrintFlag
}

// TestNoUnreviewedDashPArgvInCLI is a repo-wide tripwire (age-6j9ee.4): it parses
// every non-test .go file in the cli module and asserts that the set of files
// containing a headless-print string LITERAL (see headlessPrintLiteral) matches a
// known, reviewed allowlist. Unlike the original byte-substring scan, it inspects
// the parsed contents of each string literal, so it also catches a `-p`/`--print`
// flag hidden inside a larger claude command literal (e.g. `"claude -p"`), not
// only a bare `"-p"` argv token. It fails loudly if any new file reintroduces such
// a literal — forcing a human to confirm it is not a headless-claude invocation
// before it can be allowlisted.
//
// Known-safe today:
//   - internal/adapters/eval/scenario_ab_sandbox.go — `sandbox-exec -p <profile>`
//     (the macOS sandbox profile flag), wrapping *codex*, never claude.
func TestNoUnreviewedDashPArgvInCLI(t *testing.T) {
	allowed := map[string]bool{
		filepath.FromSlash("internal/adapters/eval/scenario_ab_sandbox.go"): true,
	}

	cliRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve cli root: %v", err)
	}
	// Sanity: confirm we are actually rooted at the cli module.
	if _, statErr := os.Stat(filepath.Join(cliRoot, "go.mod")); statErr != nil {
		t.Fatalf("cli root %q has no go.mod (test cwd assumption broke): %v", cliRoot, statErr)
	}

	fset := token.NewFileSet()
	found := map[string]bool{}
	walkErr := filepath.WalkDir(cliRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "testdata" || d.Name() == "vendor" || d.Name() == "bin" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return parseErr
		}
		rel, relErr := filepath.Rel(cliRoot, path)
		if relErr != nil {
			return relErr
		}
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			value, unquoteErr := strconv.Unquote(lit.Value)
			if unquoteErr != nil {
				return true
			}
			if headlessPrintLiteral(value) {
				found[rel] = true
			}
			return true
		})
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk cli tree: %v", walkErr)
	}

	for rel := range found {
		if !allowed[rel] {
			t.Errorf("unreviewed `-p`/`--print` string literal in %s — if this is NOT a headless "+
				"claude invocation (LAW 0), review it and add it to the allowlist in this test", rel)
		}
	}
	// The allowlist must not rot: every allowlisted file must still contain such a
	// literal, else the entry is stale and should be removed.
	for rel := range allowed {
		if !found[rel] {
			t.Errorf("allowlisted file %s no longer contains a `-p`/`--print` literal; remove the stale allowlist entry", rel)
		}
	}
}
