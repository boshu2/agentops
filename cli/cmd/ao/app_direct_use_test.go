package main

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestAppDirectInvocationHasProductionPorts(t *testing.T) {
	app := AppFromContext(context.Background())
	if app == nil || app.ExecCommand == nil || app.LookPath == nil || app.Stdout == nil || app.Stderr == nil {
		t.Fatalf("direct command invocation has no usable App ports: %+v", app)
	}
}

func TestDirectExecRatchetQuickstartUsesApp(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "quickstart.go", nil, 0)
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
