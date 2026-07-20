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
	if command.Short != "Show the single-pass AgentOps workflow" {
		t.Errorf("Short = %q", command.Short)
	}
}

func TestCommand_RunPrintsWorkflow(t *testing.T) {
	command := NewModule().Command()
	var out bytes.Buffer
	command.SetOut(&out)
	command.Run(command, nil)
	for _, want := range []string{
		"RPI -> Plan -> Implement -> fresh Validate -> durable verdict -> report and stop",
		"Deterministic checks: ao gate check",
		"Semantic judgment: invoke the Validate skill from a fresh context",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("quick-start output missing %q, got:\n%s", want, out.String())
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
