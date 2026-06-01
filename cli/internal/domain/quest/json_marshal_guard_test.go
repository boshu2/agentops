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

type swallowSite struct {
	path string
	line int
	text string
}

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

	sites, err := findSwallowedJSONMarshalErrors(root, t.Logf)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}

	// Allowlist: known-safe patterns that pre-date this guard. Empty = strict.
	// If a legitimate test-only or schema-stub case needs to be excluded,
	// add it here with a justifying comment.
	var allowlist []string

	filtered := filterAllowedSwallowedJSONMarshalSites(sites, allowlist)
	assertNoSwallowedJSONMarshalErrors(t, filtered)
}

func findSwallowedJSONMarshalErrors(root string, logf func(string, ...any)) ([]swallowSite, error) {
	var sites []swallowSite
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if shouldSkipJSONMarshalGuardDir(d.Name()) {
				return fs.SkipDir
			}
			return nil
		}
		if !isProductionGoFile(path) {
			return nil
		}
		fileSites := swallowedJSONMarshalSitesInFile(path, logf)
		sites = append(sites, fileSites...)
		return nil
	})
	return sites, err
}

func shouldSkipJSONMarshalGuardDir(name string) bool {
	// Skip vendored or generated dirs that have legitimate _ patterns.
	switch name {
	case "testdata", "fixture", "vendor":
		return true
	default:
		return false
	}
}

func isProductionGoFile(path string) bool {
	return strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go")
}

func swallowedJSONMarshalSitesInFile(path string, logf func(string, ...any)) []swallowSite {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		if logf != nil {
			logf("parse skip %s: %v", path, err)
		}
		return nil
	}
	var sites []swallowSite
	ast.Inspect(file, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		callName, ok := swallowedJSONMarshalCall(assign)
		if !ok {
			return true
		}
		pos := fset.Position(assign.Pos())
		sites = append(sites, swallowSite{
			path: path,
			line: pos.Line,
			text: callName,
		})
		return true
	})
	return sites
}

func swallowedJSONMarshalCall(assign *ast.AssignStmt) (string, bool) {
	// Looking for: a, _ := json.Marshal(...) OR a, _ := json.MarshalIndent(...).
	if len(assign.Rhs) != 1 || len(assign.Lhs) < 2 {
		return "", false
	}
	call, ok := assign.Rhs[0].(*ast.CallExpr)
	if !ok {
		return "", false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || !isJSONMarshalSelector(sel) {
		return "", false
	}
	errIdent, ok := assign.Lhs[1].(*ast.Ident)
	if !ok || errIdent.Name != "_" {
		return "", false
	}
	return "json." + sel.Sel.Name, true
}

func isJSONMarshalSelector(sel *ast.SelectorExpr) bool {
	pkgIdent, ok := sel.X.(*ast.Ident)
	if !ok || pkgIdent.Name != "json" {
		return false
	}
	return sel.Sel.Name == "Marshal" || sel.Sel.Name == "MarshalIndent"
}

func filterAllowedSwallowedJSONMarshalSites(sites []swallowSite, allowlist []string) []swallowSite {
	filtered := sites[:0]
	for _, site := range sites {
		if allowedSwallowedJSONMarshalSite(site, allowlist) {
			continue
		}
		filtered = append(filtered, site)
	}
	return filtered
}

func allowedSwallowedJSONMarshalSite(site swallowSite, allowlist []string) bool {
	for _, allow := range allowlist {
		if strings.HasSuffix(site.path, allow) {
			return true
		}
	}
	return false
}

func assertNoSwallowedJSONMarshalErrors(t *testing.T, sites []swallowSite) {
	t.Helper()
	if len(sites) == 0 {
		return
	}
	for _, site := range sites {
		t.Errorf("swallowed %s error at %s:%d", site.text, site.path, site.line)
	}
	t.Fatalf("swallowed json.Marshal/MarshalIndent error count: got %d, want 0", len(sites))
}
