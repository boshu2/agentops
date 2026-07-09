package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestNoUnguardedCobraWriterLeak is the age-ztf8 mechanical gate for the cmd/ao
// shuffle-isolation class. A test that sets a SHARED cobra command's out/err writer
// (a package-global *Cmd, a rootCmd.Find(...) result, or a plain alias of one) to a
// non-nil value MUST pair it with a reset, or the writer leaks into whatever test
// the -shuffle order runs next: cmd.OutOrStdout() parent-walks to the dead buffer
// and silently swallows that test's output captured via os.Stdout — the
// TestGoals_Integration_MeasureDirectivesJSON empty-output flake (age-2vzb).
//
// A set is matched (not a leak) by EITHER a deferred / t.Cleanup reset of the SAME
// receiver and SAME stream to nil — the only sound baseline (SetOut(<saved>) is
// rejected: a saved OutOrStdout() resolves to a concrete os.Stdout that can still
// swallow a later os.Stdout-swap capture) — OR a call to a known writer-resetting
// helper (writerResetHelpers; resetCommandState is excluded, it nils only at entry).
// Detection is AST-based (not grep): per-receiver/per-stream matching pairs sets
// with resets across defer closures and helper calls; fresh &cobra.Command{} locals
// are excluded so a test-local *Cmd is not a false positive.
func TestNoUnguardedCobraWriterLeak(t *testing.T) {
	files, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var violations []string
	for _, file := range files {
		af, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		for _, decl := range af.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			for _, s := range unguardedCmdWriterSets(fn.Body) {
				violations = append(violations,
					file+": func "+fn.Name.Name+" sets "+s+" without a matching reset")
			}
		}
	}
	if len(violations) > 0 {
		t.Fatalf("unguarded cobra out-writer leak(s) (age-ztf8) — pair each with a "+
			"deferred/t.Cleanup reset of the SAME command+stream "+
			"(e.g. t.Cleanup(func(){ X.SetOut(nil); X.SetErr(nil) })) or a known "+
			"writer-resetting helper (%s):\n  %s",
			strings.Join(writerResetHelperNames(), ", "), strings.Join(violations, "\n  "))
	}
}

// writerResetHelpers are test helpers that register a t.Cleanup which restores
// their command's out/err writers AFTER the test body (so a SetOut later in the
// caller is guarded). Membership requires the helper to (a) register a t.Cleanup
// — NOT merely clear writers at entry — that (b) resets the SAME command its
// callers set. resetCommandState is deliberately NOT here: it nils writers only
// at entry (no cleanup), so a SetOut after it leaks (age-ztf8 review). Keep tight.
var writerResetHelpers = map[string]bool{
	"resetSkillsResolveFlags": true,
	"resetSkillsCheckFlags":   true,
	// setDigestProjectDir registers a t.Cleanup that resets membraneDigestCmd's
	// out-writer (SetOut(nil)) — the SAME command every `ao membrane digest` test
	// sets — so a SetOut after it is guarded (age-xbmf).
	"setDigestProjectDir": true,
	// setYieldReportState registers a t.Cleanup that resets yieldReportCmd's
	// out/err writers — the SAME command every `ao yield report` test sets
	// (age-mv67, mirroring the setDigestProjectDir precedent).
	"setYieldReportState": true,
}

func writerResetHelperNames() []string {
	names := make([]string, 0, len(writerResetHelpers))
	for n := range writerResetHelpers {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// unguardedCmdWriterSets returns "<recv>.<Stream>" sites where a SHARED cobra
// command's out/err writer is set to a non-nil value with NO matching reset.
//
// A receiver is shared if it is a package-global command ident (name ends in
// "Cmd") or a local bound from rootCmd.Find(...)/<cmd>.Find(...) (which returns
// the shared registered command pointer, whatever the local is named). Throwaway
// locals (c := &cobra.Command{}) are ignored: they are neither named *Cmd nor
// Find-bound, so they cannot leak shared state.
//
// A set is matched (not a leak) when EITHER the function calls a known
// writer-resetting helper, OR the function defers / t.Cleanup-registers a reset
// of the SAME receiver and the SAME stream (SetOut paired with a SetOut reset,
// SetErr with a SetErr reset). Matching is per-receiver and per-stream so a
// SetOut(nil) cannot mask a SetErr leak, and a reset of one command cannot mask
// a leak of another (age-ztf8 cross-family review). Only deferred / Cleanup
// resets count, so a leading baseline reset before a later non-nil set does not
// falsely match.
func unguardedCmdWriterSets(body *ast.BlockStmt) []string {
	if callsWriterResetHelper(body) {
		return nil
	}
	findBound := findBoundCommandIdents(body)
	throwaway := throwawayCommandIdents(body)
	resets := deferredWriterResets(body)
	var sites []string
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		recv, stream, ok := writerSetCall(call)
		if !ok {
			return true
		}
		if throwaway[recv] {
			return true // local `x := &cobra.Command{}` — no shared state to leak
		}
		if !strings.HasSuffix(recv, "Cmd") && !findBound[recv] {
			return true
		}
		if resets[recv+"|"+stream] {
			return true // matched: same receiver + same stream reset, deferred/Cleanup
		}
		sites = append(sites, recv+"."+stream)
		return true
	})
	return sites
}

// writerSetCall reports whether call is a non-nil <recv>.SetOut/SetErr(...) on an
// ident receiver, returning the receiver name and stream ("SetOut"/"SetErr").
func writerSetCall(call *ast.CallExpr) (recv, stream string, ok bool) {
	sel, isSel := call.Fun.(*ast.SelectorExpr)
	if !isSel || (sel.Sel.Name != "SetOut" && sel.Sel.Name != "SetErr") {
		return "", "", false
	}
	id, isID := sel.X.(*ast.Ident)
	if !isID {
		return "", "", false
	}
	if len(call.Args) == 1 {
		if arg, isID := call.Args[0].(*ast.Ident); isID && arg.Name == "nil" {
			return "", "", false // SetOut(nil) is a reset, not a set
		}
	}
	return id.Name, sel.Sel.Name, true
}

// writerResetCall reports whether call restores a writer to its correct baseline:
// <recv>.SetOut/SetErr(nil). Only nil is accepted — SetOut(<saved>) is NOT a sound
// reset because a saved cmd.OutOrStdout() resolves to a concrete *os.File (real
// os.Stdout, or a parent-walked writer), and re-installing that pins a non-nil
// writer that can still swallow a later test which captures by swapping os.Stdout
// (age-ztf8 cross-family review). nil is always the correct cobra baseline.
func writerResetCall(call *ast.CallExpr) (recv, stream string, ok bool) {
	sel, isSel := call.Fun.(*ast.SelectorExpr)
	if !isSel || (sel.Sel.Name != "SetOut" && sel.Sel.Name != "SetErr") {
		return "", "", false
	}
	id, isID := sel.X.(*ast.Ident)
	if !isID || len(call.Args) != 1 {
		return "", "", false
	}
	if arg, isID := call.Args[0].(*ast.Ident); isID && arg.Name == "nil" {
		return id.Name, sel.Sel.Name, true
	}
	return "", "", false
}

// deferredWriterResets collects "<recv>|<stream>" reset keys that appear inside a
// defer statement or a t.Cleanup closure — the only positions that actually
// restore the writer after the test body uses it.
func deferredWriterResets(body *ast.BlockStmt) map[string]bool {
	resets := map[string]bool{}
	collect := func(n ast.Node) {
		ast.Inspect(n, func(m ast.Node) bool {
			if call, ok := m.(*ast.CallExpr); ok {
				if recv, stream, ok := writerResetCall(call); ok {
					resets[recv+"|"+stream] = true
				}
			}
			return true
		})
	}
	ast.Inspect(body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.DeferStmt:
			collect(x)
		case *ast.CallExpr:
			if sel, ok := x.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Cleanup" {
				collect(x)
			}
		}
		return true
	})
	return resets
}

// throwawayCommandIdents collects local idents initialized from a cobra.Command
// composite literal (`x := &cobra.Command{...}` or `x := cobra.Command{...}`).
// These are fresh, test-local commands — setting their writer cannot leak into
// another test — so they are excluded even when named with a "Cmd" suffix
// (age-ztf8 cross-family review: avoid false-positives that would block a push).
func throwawayCommandIdents(body *ast.BlockStmt) map[string]bool {
	tw := map[string]bool{}
	ast.Inspect(body, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok || len(as.Lhs) == 0 || len(as.Rhs) != 1 {
			return true
		}
		lhs, ok := as.Lhs[0].(*ast.Ident)
		if !ok || lhs.Name == "_" {
			return true
		}
		if isCobraCommandComposite(as.Rhs[0]) {
			tw[lhs.Name] = true
		}
		return true
	})
	return tw
}

// isCobraCommandComposite reports whether e is `&cobra.Command{...}` or
// `cobra.Command{...}`.
func isCobraCommandComposite(e ast.Expr) bool {
	if u, ok := e.(*ast.UnaryExpr); ok && u.Op == token.AND {
		e = u.X
	}
	cl, ok := e.(*ast.CompositeLit)
	if !ok {
		return false
	}
	sel, ok := cl.Type.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "cobra" && sel.Sel.Name == "Command"
}

// callsWriterResetHelper reports whether the body calls a known writer-resetting
// helper (see writerResetHelpers), which restores all streams for the test.
func callsWriterResetHelper(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if id, ok := call.Fun.(*ast.Ident); ok && writerResetHelpers[id.Name] {
				found = true
			}
		}
		return !found
	})
	return found
}

// findBoundCommandIdents collects local idents that alias a shared cobra command,
// so a SetOut on them leaks just like a package-global *Cmd. Two binding forms,
// resolved to a fixpoint so chains (b := rootCmd; c := b) are all caught:
//   - `cmd, _, _ := rootCmd.Find(...)` — .Find on a command receiver returns the
//     shared registered command (the .Find receiver must be rootCmd or a *Cmd so
//     an unrelated locator.Find / ledger.Find result is not mistaken for one).
//   - `cmd := rootCmd` / `cmd := someCmd` — a plain alias of a shared command.
// A receiver is "shared" if it is rootCmd, ends in "Cmd", or is already bound.
func findBoundCommandIdents(body *ast.BlockStmt) map[string]bool {
	bound := map[string]bool{}
	isShared := func(name string) bool {
		return name == "rootCmd" || strings.HasSuffix(name, "Cmd") || bound[name]
	}
	for changed := true; changed; {
		changed = false
		ast.Inspect(body, func(n ast.Node) bool {
			as, ok := n.(*ast.AssignStmt)
			if !ok || len(as.Lhs) == 0 || len(as.Rhs) != 1 {
				return true
			}
			lhs, ok := as.Lhs[0].(*ast.Ident)
			if !ok || lhs.Name == "_" || bound[lhs.Name] {
				return true
			}
			switch rhs := as.Rhs[0].(type) {
			case *ast.CallExpr: // cmd, _, _ := <shared>.Find(...)
				sel, ok := rhs.Fun.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "Find" {
					return true
				}
				if recvID, ok := sel.X.(*ast.Ident); ok && isShared(recvID.Name) {
					bound[lhs.Name] = true
					changed = true
				}
			case *ast.Ident: // cmd := rootCmd / cmd := someCmd
				if isShared(rhs.Name) {
					bound[lhs.Name] = true
					changed = true
				}
			}
			return true
		})
	}
	return bound
}
