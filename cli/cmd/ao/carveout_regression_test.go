// practices: [tdd, hexagonal-architecture]
package main

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// carvedFamilies are the command groups migrated out of package main into
// internal/commands/<family> modules. Each keeps exactly one thin composition
// file in cmd/ao; every other legacy source file must be gone, and the
// composition file must declare no package-level var (constructor-scoped flag
// state lives inside the module).
var carvedFamilies = map[string]string{
	"goals":      "goals_composition.go",
	"status":     "status_composition.go",
	"provenance": "provenance_composition.go",
	"skills":     "skills_composition.go",
	"session":    "session_composition.go",
	"demo":       "demo_composition.go",
	"init":       "init_composition.go",
	"version":    "version_composition.go",
	"robotdocs":  "robotdocs_composition.go",
	"quickstart": "quickstart_composition.go",
	"redact":     "redact_composition.go",
	"flywheel":   "flywheel_composition.go",
}

// packageGoFiles returns the non-test .go source files in the cmd/ao package
// directory, keyed by base name.
func packageGoFiles(t *testing.T) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(packageDir)
	if err != nil {
		t.Fatalf("read package dir %s: %v", packageDir, err)
	}
	files := map[string]string{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		files[name] = filepath.Join(packageDir, name)
	}
	return files
}

// TestCarvedFamiliesLeaveOnlyComposition proves each migrated family retains no
// legacy source file in cmd/ao beyond its single composition file. A file named
// like the family prefix (e.g. goals_measure.go) reappearing here means the
// carve-out regressed.
func TestCarvedFamiliesLeaveOnlyComposition(t *testing.T) {
	files := packageGoFiles(t)
	for family, composition := range carvedFamilies {
		if _, ok := files[composition]; !ok {
			t.Errorf("family %q lost its composition file %q", family, composition)
		}
		var stray []string
		for name := range files {
			if name == composition {
				continue
			}
			// A non-test source file whose name starts with the family prefix is
			// a legacy owner file that should have moved into the module.
			if strings.HasPrefix(name, family) {
				stray = append(stray, name)
			}
		}
		if len(stray) != 0 {
			sort.Strings(stray)
			t.Errorf("family %q left legacy source files in cmd/ao (must live in internal/commands/%s): %s",
				family, family, strings.Join(stray, ", "))
		}
	}
}

// TestCompositionFilesHaveNoPackageVars proves each composition file is a pure
// wiring seam: it declares no package-level var. Command and flag state must be
// constructor-scoped inside the module, not a package global here.
func TestCompositionFilesHaveNoPackageVars(t *testing.T) {
	files := packageGoFiles(t)
	for family, composition := range carvedFamilies {
		path, ok := files[composition]
		if !ok {
			t.Errorf("family %q missing composition file %q", family, composition)
			continue
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", composition, err)
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			for _, spec := range gen.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				names := make([]string, 0, len(value.Names))
				for _, ident := range value.Names {
					names = append(names, ident.Name)
				}
				t.Errorf("composition file %s declares package-level var(s) %s; flag state must be constructor-scoped in internal/commands/%s",
					composition, strings.Join(names, ", "), family)
			}
		}
	}
}

// TestNoCarvedLegacySymbolsRemain proves none of the frozen legacy symbols from
// either migrated family reappears as a package-level declaration anywhere in
// cmd/ao. This mirrors the family archcheck gate as a fast, always-run Go
// regression test. The authoritative symbol lists are the frozen family
// ownership records, so this test cannot silently drift from the gate.
func TestNoCarvedLegacySymbolsRemain(t *testing.T) {
	denylist := map[string]string{} // symbol -> family
	for family := range carvedFamilies {
		for _, symbol := range loadFrozenLegacySymbols(t, family) {
			denylist[symbol] = family
		}
	}
	if len(denylist) == 0 {
		t.Fatal("no frozen legacy symbols loaded; ownership records missing")
	}

	fset := token.NewFileSet()
	for name, path := range packageGoFiles(t) {
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, declared := range packageLevelNames(file) {
			if family, bad := denylist[declared]; bad {
				t.Errorf("cmd/ao/%s re-declares carved %s legacy symbol %q at package level; it belongs in internal/commands/%s",
					name, family, declared, family)
			}
		}
	}
}

// TestPackageVarsAreAllowlisted is the durable finish-line invariant for the
// cmd/ao carve-out: every package-level `var` declared in a non-test source file
// must appear on the committed allowlist (testdata/package-var-allowlist.json),
// each carrying a one-line reason (root flag target / ldflags injection /
// immutable wiring / documented test seam). The test fails two ways so the
// allowlist cannot rot: (1) an unlisted package-level var (a new mutable global
// snuck in — it almost always belongs constructor-scoped inside
// internal/commands/<family>), and (2) a stale allowlist entry whose var no
// longer exists. Keyed by name+file so a var moving files is caught too.
func TestPackageVarsAreAllowlisted(t *testing.T) {
	type allowEntry struct {
		Name   string `json:"name"`
		File   string `json:"file"`
		Reason string `json:"reason"`
	}
	allowPath := filepath.Join(packageDir, "testdata", "package-var-allowlist.json")
	data, err := os.ReadFile(allowPath)
	if err != nil {
		t.Fatalf("read package-var allowlist %s: %v", allowPath, err)
	}
	var doc struct {
		Allowed []allowEntry `json:"allowed"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("decode package-var allowlist: %v", err)
	}

	key := func(name, file string) string { return file + "::" + name }
	allowed := make(map[string]allowEntry, len(doc.Allowed))
	for _, entry := range doc.Allowed {
		if entry.Name == "" || entry.File == "" || strings.TrimSpace(entry.Reason) == "" {
			t.Errorf("allowlist entry %+v is incomplete: name, file, and reason are all required", entry)
			continue
		}
		allowed[key(entry.Name, entry.File)] = entry
	}

	// Collect the actual package-level vars from every non-test source file.
	actual := map[string]bool{}
	fset := token.NewFileSet()
	for name, path := range packageGoFiles(t) {
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			for _, spec := range gen.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, ident := range value.Names {
					actual[key(ident.Name, name)] = true
					if _, ok := allowed[key(ident.Name, name)]; !ok {
						t.Errorf("cmd/ao/%s declares un-allowlisted package-level var %q; add it to testdata/package-var-allowlist.json with a reason, or (better) move the state constructor-scoped into internal/commands/<family>",
							name, ident.Name)
					}
				}
			}
		}
	}

	// Fail on stale allowlist entries so the list cannot drift from reality.
	for k, entry := range allowed {
		if !actual[k] {
			t.Errorf("stale allowlist entry: %s (%s) no longer exists as a package-level var; remove it from testdata/package-var-allowlist.json",
				entry.Name, entry.File)
		}
	}
}

// packageLevelNames returns every top-level declared identifier (var, const,
// type, func) in the parsed file.
func packageLevelNames(file *ast.File) []string {
	var names []string
	for _, decl := range file.Decls {
		switch item := decl.(type) {
		case *ast.FuncDecl:
			if item.Recv == nil {
				names = append(names, item.Name.Name)
			}
		case *ast.GenDecl:
			for _, spec := range item.Specs {
				switch value := spec.(type) {
				case *ast.ValueSpec:
					for _, ident := range value.Names {
						names = append(names, ident.Name)
					}
				case *ast.TypeSpec:
					names = append(names, value.Name.Name)
				}
			}
		}
	}
	return names
}

// loadFrozenLegacySymbols reads the frozen legacy_symbols list for a family from
// its committed ownership record.
func loadFrozenLegacySymbols(t *testing.T, family string) []string {
	t.Helper()
	path := filepath.Join(packageDir, "..", "..", "testdata", "compatibility-baseline", "families", family, "ownership.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read ownership record for %q: %v", family, err)
	}
	var record struct {
		LegacySymbols []string `json:"legacy_symbols"`
	}
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatalf("decode ownership record for %q: %v", family, err)
	}
	return record.LegacySymbols
}

// ---------------------------------------------------------------------------
// Seam-uniformity + drift guards (bead age-6j9ee.1 / B1)
//
// The cmd/ao carve-out finished with four incompatible host-seam shapes,
// three re-rolled writeJSON bodies, and two duplicated ExitError types. The
// checks below are the drift lock: they pin the single canonical host seam
// (clicontract.HostOptions), keep command modules free of direct host effects,
// and keep module/service singletons out of package main. All are AST-based and
// cheap, mirroring the existing carve-out invariants above.
// ---------------------------------------------------------------------------

// commandModuleFiles returns every internal/commands/<family>/module.go, keyed
// by family name, resolved against packageDir so concurrent os.Chdir in other
// tests cannot break discovery.
func commandModuleFiles(t *testing.T) map[string]string {
	t.Helper()
	base := filepath.Join(packageDir, "..", "..", "internal", "commands")
	entries, err := os.ReadDir(base)
	if err != nil {
		t.Fatalf("read commands dir %s: %v", base, err)
	}
	modules := map[string]string{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(base, entry.Name(), "module.go")
		if _, statErr := os.Stat(path); statErr == nil {
			modules[entry.Name()] = path
		}
	}
	if len(modules) == 0 {
		t.Fatalf("no command module.go files found under %s", base)
	}
	return modules
}

// importAliasMap maps each import's local alias to its full import path for one
// parsed file. A dot or blank import is keyed by "." / "_".
func importAliasMap(file *ast.File) map[string]string {
	aliases := map[string]string{}
	for _, spec := range file.Imports {
		path := strings.Trim(spec.Path.Value, `"`)
		alias := ""
		if spec.Name != nil {
			alias = spec.Name.Name
		} else {
			segments := strings.Split(path, "/")
			alias = segments[len(segments)-1]
		}
		aliases[alias] = path
	}
	return aliases
}

// findNewModule returns the NewModule func declaration in a parsed file, if any.
func findNewModule(file *ast.File) *ast.FuncDecl {
	for _, decl := range file.Decls {
		if function, ok := decl.(*ast.FuncDecl); ok && function.Recv == nil && function.Name.Name == "NewModule" {
			return function
		}
	}
	return nil
}

// TestModuleHostSeamIsSharedContract proves every command module receives its
// host seam through the single shared clicontract.HostOptions type: no module
// declares its own HostOptions struct, and no NewModule passes host wiring as a
// bare positional func. Together these kill the four drifted seam shapes
// (positional funcs, bespoke HostOptions structs, and the GlobalOptions bundle).
func TestModuleHostSeamIsSharedContract(t *testing.T) {
	for family, path := range commandModuleFiles(t) {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		// (a) No bespoke seam struct type declared in the module package.
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if typeSpec.Name.Name == "HostOptions" || typeSpec.Name.Name == "GlobalOptions" {
					t.Errorf("module %s declares bespoke seam type %q; the host seam must be the shared clicontract.HostOptions",
						family, typeSpec.Name.Name)
				}
			}
		}
		// (b) NewModule passes no host wiring as a bare positional func.
		newModule := findNewModule(file)
		if newModule == nil || newModule.Type.Params == nil {
			continue
		}
		for _, param := range newModule.Type.Params.List {
			if _, isFunc := param.Type.(*ast.FuncType); isFunc {
				names := make([]string, 0, len(param.Names))
				for _, ident := range param.Names {
					names = append(names, ident.Name)
				}
				t.Errorf("module %s NewModule takes bare positional func param %s; host seams must arrive via clicontract.HostOptions",
					family, strings.Join(names, ", "))
			}
		}
	}
}

// TestModuleImportsNoDirectHostEffects proves each module.go reaches for no
// direct host effect: it imports neither os nor os/exec (filesystem/process
// effects belong to injected adapters), and it never calls time.Now (the clock
// arrives through clicontract.HostOptions.Now). Types and constants from time
// (time.Duration, time.RFC3339) stay allowed, matching current legitimate use.
func TestModuleImportsNoDirectHostEffects(t *testing.T) {
	deniedImports := map[string]bool{"os": true, "os/exec": true}
	for family, path := range commandModuleFiles(t) {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, spec := range file.Imports {
			imported := strings.Trim(spec.Path.Value, `"`)
			if deniedImports[imported] {
				t.Errorf("module %s imports direct-effect package %q; delegate the effect to an adapter or a host seam",
					family, imported)
			}
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := selector.X.(*ast.Ident)
			if ok && pkg.Name == "time" && selector.Sel.Name == "Now" {
				t.Errorf("module %s calls time.Now; inject the clock through clicontract.HostOptions.Now", family)
			}
			return true
		})
	}
}

// TestNoModuleOrServiceSingletonPackageVars proves package main declares no
// package-level var whose value is a command module or an app service singleton
// — the pattern that let config and doctor drift into package globals while gate
// and eval used constructor funcs. A var initialized by a call into an
// internal/commands/* package, or by any New*Service constructor, must instead
// live inside a newXxxCommand() constructor.
func TestNoModuleOrServiceSingletonPackageVars(t *testing.T) {
	for name, path := range packageGoFiles(t) {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		aliases := importAliasMap(file)
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			for _, spec := range gen.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, expr := range value.Values {
					call, ok := expr.(*ast.CallExpr)
					if !ok {
						continue
					}
					selector, ok := call.Fun.(*ast.SelectorExpr)
					if !ok {
						continue
					}
					pkg, ok := selector.X.(*ast.Ident)
					if !ok {
						continue
					}
					importPath := aliases[pkg.Name]
					if strings.Contains(importPath, "/internal/commands/") {
						t.Errorf("cmd/ao/%s declares package-level var initialized by module constructor %s.%s; move it into a newXxxCommand() constructor",
							name, pkg.Name, selector.Sel.Name)
					}
					if strings.HasPrefix(selector.Sel.Name, "New") && strings.HasSuffix(selector.Sel.Name, "Service") {
						t.Errorf("cmd/ao/%s declares package-level service singleton var via %s.%s; construct it inside a newXxxCommand() constructor",
							name, pkg.Name, selector.Sel.Name)
					}
				}
			}
		}
	}
}
