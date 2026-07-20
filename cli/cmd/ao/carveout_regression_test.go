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
