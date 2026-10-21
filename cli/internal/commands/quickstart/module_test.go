package quickstart

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/clicontract"
)

func TestModule_Contract(t *testing.T) {
	contract := NewModule().Contract()
	if contract.ID != "ao.quick-start" {
		t.Fatalf("contract ID = %q, want ao.quick-start", contract.ID)
	}
	if contract.Output != clicontract.OutputText {
		t.Fatalf("output = %v, want OutputText", contract.Output)
	}
	if contract.Effects != clicontract.EffectPure {
		t.Fatalf("effects = %v, want EffectPure", contract.Effects)
	}
	if contract.Args.Name != "no-args" {
		t.Fatalf("args = %q, want no-args", contract.Args.Name)
	}
}

func TestModule_CommandAttributes(t *testing.T) {
	command := NewModule().Command()
	if command.Use != "quick-start" {
		t.Errorf("Use = %q, want quick-start", command.Use)
	}
	if command.GroupID != "start" {
		t.Errorf("GroupID = %q, want start", command.GroupID)
	}
	if command.Short != "Show the native AgentOps execution brief" {
		t.Errorf("Short = %q", command.Short)
	}
}

// conditionalReviewRule is the AGENTS.md / docs/agent-workflow-reference.md
// review rule (#1189, ADR-0017 2026-10-03 amendment) as the brief states it.
const conditionalReviewRule = "Checks and CI gate an ordinary change; obtain one fresh author-distinct read only when the caller asks, a mistake cannot be cheaply undone, or no deterministic check covers the change."

// requestedVerdictRule scopes NOT_PROVEN to a requested binding verdict.
const requestedVerdictRule = "A requested binding verdict reports missing proof or unchecked acceptance as NOT_PROVEN."

// retiredReviewClaims are the pre-#1189 always-review claims the brief must
// not print.
var retiredReviewClaims = []string{
	"checks alone do not prove completion",
	"checks alone is insufficient",
	"Missing independent judgment",
	"convergence law",
	"repair_rounds",
}

// normalizeSpace collapses line wrapping so a pinned sentence matches however
// the brief wraps it.
func normalizeSpace(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func TestCommand_RunPrintsNativeCompletionBrief(t *testing.T) {
	command := NewModule().Command()
	var out bytes.Buffer
	command.SetOut(&out)
	command.Run(command, nil)
	text := normalizeSpace(out.String())
	for _, want := range []string{
		"native agent and shell",
		"ao gate check supplies deterministic facts",
		conditionalReviewRule,
		"checked and not_checked",
		requestedVerdictRule,
		"From an AgentOps checkout",
		"including the Validate skill, are optional",
		"Artifact persistence: optional",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("quick-start output missing %q, got:\n%s", want, out.String())
		}
	}
	for _, retired := range retiredReviewClaims {
		if strings.Contains(text, retired) {
			t.Errorf("quick-start output keeps retired review claim %q, got:\n%s", retired, out.String())
		}
	}
}

func TestCommand_HelpStatesConditionalReview(t *testing.T) {
	text := normalizeSpace(NewModule().Command().Long)
	if !strings.Contains(text, conditionalReviewRule) {
		t.Errorf("quick-start help missing %q, got:\n%s", conditionalReviewRule, text)
	}
	for _, retired := range append([]string{"required judgment"}, retiredReviewClaims...) {
		if strings.Contains(text, retired) {
			t.Errorf("quick-start help keeps retired review claim %q, got:\n%s", retired, text)
		}
	}
}

// TestQuickStartNamesOnlySurvivingResponsibilities relocates the cathedral-cut
// guard that previously lived in cmd/ao: the quick-start help must not advertise
// responsibilities the CLI no longer owns.
func TestQuickStartNamesOnlySurvivingResponsibilities(t *testing.T) {
	text := NewModule().Command().Long
	for _, forbidden := range []string{"ao land", "ao verify", "ao beads", "ao pawl"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("quick-start advertises removed responsibility %q", forbidden)
		}
	}
}

// TestQuickStartUsesNoDirectExec relocates the ratchet guard that previously
// parsed cmd/ao/quickstart.go: the command must not shell out directly. The
// module is a static text render and must stay that way.
func TestQuickStartUsesNoDirectExec(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "module.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var direct []token.Position
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		ident, identOK := selector.X.(*ast.Ident)
		if identOK && ident.Name == "exec" && (selector.Sel.Name == "Command" || selector.Sel.Name == "CommandContext") {
			direct = append(direct, fset.Position(call.Pos()))
		}
		return true
	})
	if len(direct) != 0 {
		t.Fatalf("quick-start bypasses App runner at %v", direct)
	}
}
