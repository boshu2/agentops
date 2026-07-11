package archcheck

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// Inventory is the review packet produced before a family fixture is frozen.
// It is deterministic: no timestamps or ambient paths enter the digest.
type Inventory struct {
	SchemaVersion   int      `json:"schema_version"`
	Family          string   `json:"family"`
	HeadSHA         string   `json:"head_sha"`
	OwnerFiles      []string `json:"owner_files"`
	LegacySymbols   []string `json:"legacy_symbols"`
	Effects         []Rule   `json:"effects"`
	OwnerCandidates []string `json:"owner_candidates"`
	AllowedPaths    []string `json:"allowed_paths"`
}

func BuildInventory(root, family string) (Inventory, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return Inventory{}, err
	}
	if strings.TrimSpace(family) == "" {
		return Inventory{}, fmt.Errorf("inventory family is required")
	}
	head, err := gitOutput(root, "rev-parse", "HEAD")
	if err != nil {
		return Inventory{}, err
	}
	needle := strings.ToLower(strings.ReplaceAll(family, "-", "_"))
	cmdRoot := filepath.Join(root, "cli", "cmd", "ao")
	ownerSet := map[string]bool{}
	symbolSet := map[string]bool{}
	effectSet := map[Rule]bool{}
	candidateSet := map[string]bool{}

	entries, err := os.ReadDir(cmdRoot)
	if err != nil {
		return Inventory{}, err
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(cmdRoot, name)
		data, err := os.ReadFile(path)
		if err != nil {
			return Inventory{}, err
		}
		if !strings.Contains(strings.ToLower(name), needle) && !strings.Contains(strings.ToLower(string(data)), needle) {
			continue
		}
		rel := relative(root, path)
		ownerSet[rel] = true
		file, err := parser.ParseFile(token.NewFileSet(), path, data, 0)
		if err != nil {
			return Inventory{}, err
		}
		for _, declaration := range file.Decls {
			switch item := declaration.(type) {
			case *ast.FuncDecl:
				symbolSet[item.Name.Name] = true
			case *ast.GenDecl:
				for _, spec := range item.Specs {
					switch value := spec.(type) {
					case *ast.TypeSpec:
						symbolSet[value.Name.Name] = true
					case *ast.ValueSpec:
						for _, ident := range value.Names {
							symbolSet[ident.Name] = true
						}
					}
				}
			}
		}
		for _, spec := range file.Imports {
			importPath := strings.Trim(spec.Path.Value, `"`)
			const prefix = "github.com/boshu2/agentops/cli/internal/"
			if strings.HasPrefix(importPath, prefix) {
				owner := "cli/internal/" + strings.TrimPrefix(importPath, prefix) + "/**"
				candidateSet[owner] = true
			}
		}
		inducedPath := "cli/internal/commands/" + needle + "/" + name
		found, err := CheckSource(inducedPath, data)
		if err != nil {
			return Inventory{}, err
		}
		for _, violation := range found {
			if strings.HasPrefix(string(violation.Rule), "effect.") {
				effectSet[violation.Rule] = true
			}
		}
	}

	ownerFiles := sortedKeys(ownerSet)
	legacySymbols := sortedKeys(symbolSet)
	ownerCandidates := sortedKeys(candidateSet)
	effects := make([]Rule, 0, len(effectSet))
	for effect := range effectSet {
		effects = append(effects, effect)
	}
	sort.Slice(effects, func(i, j int) bool { return effects[i] < effects[j] })
	allowed := append([]string{"cli/internal/commands/" + needle + "/**"}, ownerFiles...)
	allowed = append(allowed, ownerCandidates...)
	allowed = append(allowed, "cli/testdata/compatibility-baseline/families/"+family+"/lineage.json")
	sort.Strings(allowed)
	allowed = slices.Compact(allowed)
	return Inventory{
		SchemaVersion:   1,
		Family:          family,
		HeadSHA:         head,
		OwnerFiles:      ownerFiles,
		LegacySymbols:   legacySymbols,
		Effects:         effects,
		OwnerCandidates: ownerCandidates,
		AllowedPaths:    allowed,
	}, nil
}

func verifyScopeEvidence(root, family, scopePath string) ([]Violation, error) {
	if !filepath.IsAbs(scopePath) {
		scopePath = filepath.Join(root, scopePath)
	}
	data, err := os.ReadFile(scopePath)
	if err != nil {
		return nil, fmt.Errorf("read scope evidence: %w", err)
	}
	var inventory Inventory
	if err := json.Unmarshal(data, &inventory); err != nil {
		return nil, fmt.Errorf("decode scope evidence: %w", err)
	}
	baseline := filepath.Join(root, "cli", "testdata", "compatibility-baseline", "families", family)
	lineageBytes, err := os.ReadFile(filepath.Join(baseline, "lineage.json"))
	if err != nil {
		return nil, err
	}
	ownershipBytes, err := os.ReadFile(filepath.Join(baseline, "ownership.json"))
	if err != nil {
		return nil, err
	}
	var lineage lineageRecord
	var ownership ownershipRecord
	if err := json.Unmarshal(lineageBytes, &lineage); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(ownershipBytes, &ownership); err != nil {
		return nil, err
	}
	path := relative(root, scopePath)
	var violations []Violation
	if inventory.SchemaVersion != 1 || inventory.Family != family {
		violations = append(violations, Violation{Rule: RuleScope, Path: path, Message: "scope evidence family/schema mismatch"})
	}
	sum := sha256.Sum256(data)
	if len(lineage.ScopeSHA256) != 64 || hex.EncodeToString(sum[:]) != lineage.ScopeSHA256 {
		violations = append(violations, Violation{Rule: RuleScope, Path: path, Message: "scope evidence digest does not match lineage scope_sha256"})
	}
	want := append([]string(nil), ownership.AllowedPaths...)
	got := append([]string(nil), inventory.AllowedPaths...)
	sort.Strings(want)
	sort.Strings(got)
	if !slices.Equal(want, got) {
		violations = append(violations, Violation{Rule: RuleScope, Path: path, Message: "scope evidence allowed_paths differ from frozen ownership"})
	}
	return violations, nil
}

func sortedKeys[V ~bool](set map[string]V) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func gitOutput(root string, args ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("git %v: %w", args, err)
	}
	return strings.TrimSpace(string(output)), nil
}
