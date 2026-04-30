package quest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestJSONMarshal_NoSwallowedErrors is the regression guard for the 5
// swallowed json.Marshal errors found in olympus's Feb 24 P0 review.
// Pattern (anti-pattern olympus had): `data, _ := json.Marshal(x)` — the
// blank identifier silently discards encoding errors and writes the empty
// `null` body to disk. Production code must capture the error.
//
// This test walks every non-test .go file under cli/internal/ and parses the
// AST looking for json.Marshal callsites where the LHS uses `_` for the
// error. Each such hit is a regression. The expected count is ZERO.
//
// Per vyp scope: this is a constraint-injection test (CLAUDE.md principle
// #10) — failure surfaces the bug itself, not just documentation.
func TestJSONMarshal_NoSwallowedErrors(t *testing.T) {
	root := filepath.Join("..", "..")
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("internal root %q not reachable: %v", root, err)
	}

	type swallowSite struct {
		path string
		line int
		text string
	}
	var sites []swallowSite

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Skip vendored or generated dirs that have legitimate _ patterns.
			if d.Name() == "testdata" || d.Name() == "fixture" || d.Name() == "vendor" {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Logf("parse skip %s: %v", path, err)
			return nil
		}
		ast.Inspect(file, func(n ast.Node) bool {
			assign, ok := n.(*ast.AssignStmt)
			if !ok {
				return true
			}
			// Looking for: a, _ := json.Marshal(...) OR a, _ := json.MarshalIndent(...)
			if len(assign.Rhs) != 1 {
				return true
			}
			call, ok := assign.Rhs[0].(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkgIdent, ok := sel.X.(*ast.Ident)
			if !ok || pkgIdent.Name != "json" {
				return true
			}
			if sel.Sel.Name != "Marshal" && sel.Sel.Name != "MarshalIndent" {
				return true
			}
			// Two LHS positions are required (data, err); error should not be `_`.
			if len(assign.Lhs) < 2 {
				return true
			}
			errIdent, ok := assign.Lhs[1].(*ast.Ident)
			if !ok {
				return true
			}
			if errIdent.Name == "_" {
				pos := fset.Position(assign.Pos())
				sites = append(sites, swallowSite{
					path: path,
					line: pos.Line,
					text: "json." + sel.Sel.Name,
				})
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}

	// Allowlist: known-safe patterns that pre-date this guard. Empty = strict.
	// If a legitimate test-only or schema-stub case needs to be excluded,
	// add it here with a justifying comment.
	var allowlist []string

	filtered := sites[:0]
	for _, s := range sites {
		skip := false
		for _, allow := range allowlist {
			if strings.HasSuffix(s.path, allow) {
				skip = true
				break
			}
		}
		if !skip {
			filtered = append(filtered, s)
		}
	}

	if len(filtered) != 0 {
		for _, s := range filtered {
			t.Errorf("swallowed %s error at %s:%d", s.text, s.path, s.line)
		}
		t.Fatalf("swallowed json.Marshal/MarshalIndent error count: got %d, want 0", len(filtered))
	}
}
