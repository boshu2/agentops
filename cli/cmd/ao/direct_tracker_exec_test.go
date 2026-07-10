package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestDirectTrackerExecOwnedByAdapter(t *testing.T) {
	cliRoot := filepath.Clean(filepath.Join("..", ".."))
	var violations []string
	err := filepath.WalkDir(cliRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return walkErr
		}
		slash := filepath.ToSlash(path)
		if strings.Contains(slash, "/internal/adapters/tracker_br/") || strings.Contains(slash, "/internal/adapters/tracker_bd/") {
			return nil
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || !isTrackerProcessCall(call.Fun) {
				return true
			}
			for _, arg := range call.Args {
				lit, ok := arg.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				value, _ := strconv.Unquote(lit.Value)
				if value == "br" || value == "bd" {
					pos := fset.Position(lit.Pos())
					violations = append(violations, pos.String()+" executes "+value+" outside a tracker adapter")
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 0 {
		t.Fatalf("direct tracker execution bypasses selected backend:\n%s", strings.Join(violations, "\n"))
	}
}

func isTrackerProcessCall(expr ast.Expr) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	if ok {
		switch selector.Sel.Name {
		case "Command", "CommandContext", "run":
			return true
		}
	}
	ident, ok := expr.(*ast.Ident)
	return ok && (ident.Name == "run" || ident.Name == "tickPassthrough")
}
