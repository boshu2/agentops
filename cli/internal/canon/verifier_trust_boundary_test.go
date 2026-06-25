package canon

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCommandVerifier_CommandSourcedFromOperatorConfig is a structural guard on
// the sh -c trust boundary in CommandVerifier.Judge (verifier.go): the Command
// field is executed via `sh -c`, so every production construction MUST source
// it from operator configuration (os.Getenv("AGENTOPS_CANON_VERIFIER_CMD")),
// never from a less-trusted source such as a corpus entry, a Claim, or remote
// config.
//
// Same regression-surface intent as TestFleetLease_PlainWriteFile_RegressionSurface
// in internal/types/quest: it passes today (the single construction site at
// cmd/ao/canon.go wires Command from the env var) and trips if a future caller
// constructs a CommandVerifier whose Command does not trace to that env var.
//
// Precision: the check is per construction site via the AST (not file-level
// string matching), so a file holding one safe construction cannot smuggle a
// second unsafe one past the guard. It fails CLOSED — an unreadable or
// unparseable production file fails the test rather than being silently skipped
// (which would pass with incomplete coverage).
//
// Bound (made visible, per the repo's fail-open/fail-closed convention): this
// covers composite-literal construction, which is the only construction path in
// the tree today. A future `someVerifier.Command = x` assignment from an
// untrusted source would require go/types to attribute the receiver and is out
// of scope here; the doc-comment TRUST BOUNDARY invariant on the field is the
// backstop for that vector.
//
// Recon 2026-06-24 audit M-1.
func TestCommandVerifier_CommandSourcedFromOperatorConfig(t *testing.T) {
	const operatorEnvVar = "AGENTOPS_CANON_VERIFIER_CMD"

	// Production source roots, relative to this package (cli/internal/canon).
	roots := []string{
		filepath.Join("..", "..", "cmd"),
		filepath.Join("..", "..", "internal"),
	}

	type violation struct {
		file string
		line int
	}
	var violations []violation
	sitesChecked := 0

	// isOperatorGetenv reports whether expr is exactly
	// os.Getenv("AGENTOPS_CANON_VERIFIER_CMD").
	isOperatorGetenv := func(expr ast.Expr) bool {
		call, ok := expr.(*ast.CallExpr)
		if !ok || len(call.Args) != 1 {
			return false
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Getenv" {
			return false
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok || pkg.Name != "os" {
			return false
		}
		lit, ok := call.Args[0].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return false
		}
		return strings.Trim(lit.Value, "`\"") == operatorEnvVar
	}

	scan := func(root string) error {
		return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					return nil
				}
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			data, rerr := os.ReadFile(path)
			if rerr != nil {
				// Fail closed: an unreadable production file means incomplete
				// coverage, which must surface as a failure, not a silent skip.
				return fmt.Errorf("read %s: %w", path, rerr)
			}
			if !strings.Contains(string(data), "CommandVerifier") {
				return nil // no construction or reference here
			}
			fset := token.NewFileSet()
			file, perr := parser.ParseFile(fset, path, data, 0)
			if perr != nil {
				return fmt.Errorf("parse %s: %w", path, perr) // fail closed
			}
			ast.Inspect(file, func(n ast.Node) bool {
				lit, ok := n.(*ast.CompositeLit)
				if !ok {
					return true
				}
				// Match `CommandVerifier{...}` (bare ident inside package canon)
				// and `canon.CommandVerifier{...}` (selector elsewhere).
				typeName := ""
				switch tn := lit.Type.(type) {
				case *ast.Ident:
					typeName = tn.Name
				case *ast.SelectorExpr:
					typeName = tn.Sel.Name
				}
				if typeName != "CommandVerifier" {
					return true
				}
				sitesChecked++
				if len(lit.Elts) == 0 {
					return true // CommandVerifier{} -> Command is "" -> Judge fails loudly. Safe.
				}
				if _, keyed := lit.Elts[0].(*ast.KeyValueExpr); keyed {
					for _, elt := range lit.Elts {
						kv, ok := elt.(*ast.KeyValueExpr)
						if !ok {
							continue
						}
						key, ok := kv.Key.(*ast.Ident)
						if !ok || key.Name != "Command" {
							continue
						}
						if !isOperatorGetenv(kv.Value) {
							violations = append(violations, violation{path, fset.Position(kv.Pos()).Line})
						}
					}
				} else {
					// Positional literal: field order is Command, JudgeID.
					if !isOperatorGetenv(lit.Elts[0]) {
						violations = append(violations, violation{path, fset.Position(lit.Elts[0].Pos()).Line})
					}
				}
				return true
			})
			return nil
		})
	}

	for _, root := range roots {
		if err := scan(root); err != nil {
			t.Fatalf("scanning %s: %v", root, err)
		}
	}

	// Non-vacuous: if zero construction sites are found the guard has been
	// silently defeated (type renamed or moved) — fail rather than pass empty.
	if sitesChecked == 0 {
		t.Fatalf("no production CommandVerifier{ construction site found under %v — "+
			"the trust-boundary guard is no longer covering anything (type renamed or moved?)", roots)
	}

	for _, v := range violations {
		t.Errorf("sh -c trust-boundary violation: %s:%d — Command not sourced from "+
			"os.Getenv(%q). It is run via `sh -c` and MUST come from operator config, "+
			"never corpus/Claim/remote data.", v.file, v.line, operatorEnvVar)
	}
}
