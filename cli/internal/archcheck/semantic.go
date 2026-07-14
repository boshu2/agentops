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
	"path/filepath"
)

const semanticSealFilename = "semantic-seal.json"

const (
	beadsContextDebtPath   = "cli/internal/commands/beads/module.go"
	beadsContextDebtSHA256 = "b12905e003a327ccff1b31ff6f077c0e69bffe4956c00435cf29dcefd1ac6dae"
)

type semanticSealManifest struct {
	SchemaVersion int                 `json:"schema_version"`
	Class         string              `json:"class"`
	Sources       []string            `json:"sources"`
	Generated     []semanticGenerated `json:"generated"`
	CandidateSHA  string              `json:"candidate_sha"`
	Evidence      *semanticEvidence   `json:"evidence"`
}

type semanticGenerated struct {
	Source string `json:"source"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type semanticEvidence struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type semanticEvidenceDocument struct {
	SchemaVersion int               `json:"schema_version"`
	Class         string            `json:"class"`
	CandidateSHA  string            `json:"candidate_sha"`
	SourceDigests map[string]string `json:"source_digests"`
}

func filterAcceptedSemanticDebt(root string, violations []Violation) []Violation {
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		return violations
	}
	source, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(beadsContextDebtPath)))
	if err != nil || digestBytes(source) != beadsContextDebtSHA256 {
		return violations
	}
	filtered := violations[:0]
	for _, violation := range violations {
		if violation.Rule == RuleContext && violation.Path == beadsContextDebtPath {
			continue
		}
		filtered = append(filtered, violation)
	}
	return filtered
}

func checkSemanticSeal(root string) ([]Violation, error) {
	manifestPath := filepath.Join(root, semanticSealFilename)
	data, err := os.ReadFile(manifestPath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read semantic seal: %w", err)
	}
	var manifest semanticSealManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("decode semantic seal: %w", err)
	}
	if manifest.SchemaVersion != 1 || manifest.Class == "" || len(manifest.Sources) == 0 {
		return []Violation{{Rule: RuleEvidenceBinding, Path: semanticSealFilename, Message: "semantic seal schema, class, and sources are required"}}, nil
	}
	if manifest.Class == "generated-evidence" {
		return checkGeneratedEvidence(root, manifest), nil
	}
	if manifest.Class == "evidence-binding" {
		return checkEvidenceBinding(root, manifest), nil
	}

	var violations []Violation
	for _, sourcePath := range manifest.Sources {
		if filepath.IsAbs(sourcePath) || !filepath.IsLocal(sourcePath) {
			violations = append(violations, Violation{Rule: RuleEvidenceBinding, Path: semanticSealFilename, Message: "semantic seal source path must be repository-relative"})
			continue
		}
		source, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(sourcePath)))
		if err != nil {
			return nil, fmt.Errorf("read semantic source %s: %w", sourcePath, err)
		}
		switch manifest.Class {
		case "tracker-execution":
			found, err := checkTrackerExecutionSource(sourcePath, source)
			if err != nil {
				return nil, err
			}
			violations = append(violations, found...)
		case "effects":
			found, err := checkUniversalEffectsSource(sourcePath, source)
			if err != nil {
				return nil, err
			}
			violations = append(violations, found...)
		case "output":
			found, err := checkOutputSource(sourcePath, source)
			if err != nil {
				return nil, err
			}
			violations = append(violations, found...)
		case "recursive-contracts":
			found, err := checkRecursiveContractsSource(sourcePath, source)
			if err != nil {
				return nil, err
			}
			violations = append(violations, found...)
		default:
			violations = append(violations, Violation{Rule: RuleEvidenceBinding, Path: semanticSealFilename, Message: fmt.Sprintf("unknown semantic seal class %q", manifest.Class)})
		}
	}
	return violations, nil
}

func checkEvidenceBinding(root string, manifest semanticSealManifest) []Violation {
	invalid := func(path, message string) []Violation {
		return []Violation{{Rule: RuleEvidenceBinding, Path: path, Message: message}}
	}
	if !validHexDigest(manifest.CandidateSHA, 40) || manifest.Evidence == nil || !validHexDigest(manifest.Evidence.SHA256, 64) {
		return invalid(semanticSealFilename, "candidate SHA and evidence digest must be complete lowercase hex digests")
	}
	if filepath.IsAbs(manifest.Evidence.Path) || !filepath.IsLocal(manifest.Evidence.Path) {
		return invalid(semanticSealFilename, "evidence path must be repository-relative")
	}
	evidenceData, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(manifest.Evidence.Path)))
	if err != nil {
		return invalid(manifest.Evidence.Path, "bound evidence is missing")
	}
	if digestBytes(evidenceData) != manifest.Evidence.SHA256 {
		return invalid(manifest.Evidence.Path, "bound evidence digest does not match its declaration")
	}
	var document semanticEvidenceDocument
	if err := json.Unmarshal(evidenceData, &document); err != nil {
		return invalid(manifest.Evidence.Path, "bound evidence is not valid JSON")
	}
	if document.SchemaVersion != 1 || document.Class != manifest.Class || document.CandidateSHA != manifest.CandidateSHA {
		return invalid(manifest.Evidence.Path, "evidence is not bound to the declared candidate")
	}
	for _, sourcePath := range manifest.Sources {
		if filepath.IsAbs(sourcePath) || !filepath.IsLocal(sourcePath) {
			return invalid(semanticSealFilename, "semantic seal source path must be repository-relative")
		}
		source, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(sourcePath)))
		if err != nil {
			return invalid(sourcePath, "bound source is missing")
		}
		if document.SourceDigests[sourcePath] != digestBytes(source) {
			return invalid(sourcePath, "evidence source digest does not match the candidate source")
		}
	}
	if len(document.SourceDigests) != len(manifest.Sources) {
		return invalid(manifest.Evidence.Path, "evidence source set does not match the manifest")
	}
	return nil
}

func validHexDigest(value string, length int) bool {
	if len(value) != length {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && hex.EncodeToString(decoded) == value
}

func digestBytes(data []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func checkGeneratedEvidence(root string, manifest semanticSealManifest) []Violation {
	declaredSources := make(map[string]bool, len(manifest.Sources))
	for _, source := range manifest.Sources {
		declaredSources[source] = true
	}
	if len(manifest.Generated) == 0 {
		return []Violation{{Rule: RuleGeneratedEvidence, Path: semanticSealFilename, Message: "generated evidence declarations are required"}}
	}
	var violations []Violation
	for _, generated := range manifest.Generated {
		if !declaredSources[generated.Source] || filepath.IsAbs(generated.Source) || !filepath.IsLocal(generated.Source) || filepath.IsAbs(generated.Path) || !filepath.IsLocal(generated.Path) {
			violations = append(violations, Violation{Rule: RuleGeneratedEvidence, Path: semanticSealFilename, Message: "generated evidence must name a declared local source and local output"})
			continue
		}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(generated.Source))); err != nil {
			violations = append(violations, Violation{Rule: RuleGeneratedEvidence, Path: generated.Source, Message: "generated evidence source is missing"})
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(generated.Path)))
		if err != nil {
			violations = append(violations, Violation{Rule: RuleGeneratedEvidence, Path: generated.Path, Message: "generated evidence output is missing"})
			continue
		}
		actual := digestBytes(data)
		if actual != generated.SHA256 {
			violations = append(violations, Violation{Rule: RuleGeneratedEvidence, Path: generated.Path, Message: fmt.Sprintf("generated evidence digest mismatch: want %s got %s", generated.SHA256, actual)})
		}
	}
	return violations
}

func checkRecursiveContractsSource(path string, source []byte) ([]Violation, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, source, 0)
	if err != nil {
		return nil, fmt.Errorf("parse semantic source %s: %w", path, err)
	}
	aliases := sourceImportAliases(file)
	runnable := 0
	attached := 0
	ast.Inspect(file, func(node ast.Node) bool {
		literal, ok := node.(*ast.CompositeLit)
		if ok {
			selector, selectorOK := literal.Type.(*ast.SelectorExpr)
			if !selectorOK {
				return true
			}
			ident, identOK := selector.X.(*ast.Ident)
			if identOK && aliases[ident.Name] == "github.com/spf13/cobra" && selector.Sel.Name == "Command" {
				for _, element := range literal.Elts {
					field, fieldOK := element.(*ast.KeyValueExpr)
					if !fieldOK {
						continue
					}
					key, keyOK := field.Key.(*ast.Ident)
					if keyOK && (key.Name == "Run" || key.Name == "RunE") {
						runnable++
						break
					}
				}
			}
		}
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "Attach" {
			return true
		}
		ident, ok := selector.X.(*ast.Ident)
		if ok && aliases[ident.Name] == "github.com/boshu2/agentops/cli/internal/clicontract" {
			attached++
		}
		return true
	})
	if runnable != attached {
		return []Violation{{Rule: RuleRecursiveContracts, Path: path, Message: fmt.Sprintf("every runnable Cobra node needs exactly one attached contract: runnable=%d attached=%d", runnable, attached)}}, nil
	}
	return nil, nil
}

func checkOutputSource(path string, source []byte) ([]Violation, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, source, 0)
	if err != nil {
		return nil, fmt.Errorf("parse semantic source %s: %w", path, err)
	}
	aliases := sourceImportAliases(file)
	structured := false
	ast.Inspect(file, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "OutputStructured" {
			return true
		}
		ident, ok := selector.X.(*ast.Ident)
		if ok && aliases[ident.Name] == "github.com/boshu2/agentops/cli/internal/clicontract" {
			structured = true
		}
		return true
	})
	if !structured {
		return nil, nil
	}
	var violations []Violation
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "Fprint" && selector.Sel.Name != "Fprintln" && selector.Sel.Name != "Fprintf" {
			return true
		}
		ident, ok := selector.X.(*ast.Ident)
		if ok && aliases[ident.Name] == "fmt" {
			violations = append(violations, Violation{Rule: RuleOutput, Path: path, Line: fset.Position(call.Pos()).Line, Message: "structured output contract must use a structured encoder, not human text formatting"})
		}
		return true
	})
	return violations, nil
}

func checkUniversalEffectsSource(path string, source []byte) ([]Violation, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, source, 0)
	if err != nil {
		return nil, fmt.Errorf("parse semantic source %s: %w", path, err)
	}
	aliases := sourceImportAliases(file)
	var violations []Violation
	ast.Inspect(file, func(node ast.Node) bool {
		field, ok := node.(*ast.KeyValueExpr)
		if !ok {
			return true
		}
		key, ok := field.Key.(*ast.Ident)
		if !ok || key.Name != "PersistentPreRun" && key.Name != "PersistentPreRunE" {
			return true
		}
		function, ok := field.Value.(*ast.FuncLit)
		if !ok {
			return true
		}
		hasEffect := false
		ast.Inspect(function.Body, func(child ast.Node) bool {
			selector, ok := child.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			ident, ok := selector.X.(*ast.Ident)
			if !ok {
				return true
			}
			importPath := aliases[ident.Name]
			if importPath == "os" && osRule(selector.Sel.Name) != "" || importPath == "github.com/boshu2/agentops/cli/internal/adapters/worktreeconfig" {
				hasEffect = true
				return false
			}
			return true
		})
		if hasEffect {
			violations = append(violations, Violation{Rule: RuleEffects, Path: path, Line: fset.Position(field.Pos()).Line, Message: "universal pre-run lifecycle may not perform command-specific environment, filesystem, or process effects"})
		}
		return true
	})
	return violations, nil
}

func checkTrackerExecutionSource(path string, source []byte) ([]Violation, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, source, 0)
	if err != nil {
		return nil, fmt.Errorf("parse semantic source %s: %w", path, err)
	}
	execAliases := map[string]bool{}
	for _, spec := range file.Imports {
		if spec.Path.Value != `"os/exec"` {
			continue
		}
		name := "exec"
		if spec.Name != nil {
			name = spec.Name.Name
		}
		execAliases[name] = true
	}
	type launchState struct {
		node         ast.Node
		contextAware bool
		workDir      bool
		childEnv     bool
	}
	launches := map[string]*launchState{}
	ast.Inspect(file, func(node ast.Node) bool {
		assignment, ok := node.(*ast.AssignStmt)
		if !ok || len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 {
			return true
		}
		if name, ok := assignment.Lhs[0].(*ast.Ident); ok {
			call, ok := assignment.Rhs[0].(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			alias, aliasOK := selector.X.(*ast.Ident)
			if !aliasOK || !execAliases[alias.Name] {
				return true
			}
			launches[name.Name] = &launchState{node: call, contextAware: selector.Sel.Name == "CommandContext"}
			return true
		}
		left, ok := assignment.Lhs[0].(*ast.SelectorExpr)
		if !ok {
			return true
		}
		name, ok := left.X.(*ast.Ident)
		if !ok || launches[name.Name] == nil {
			return true
		}
		right, ok := assignment.Rhs[0].(*ast.SelectorExpr)
		if !ok {
			return true
		}
		switch {
		case left.Sel.Name == "Dir" && right.Sel.Name == "WorkDir":
			launches[name.Name].workDir = true
		case left.Sel.Name == "Env" && right.Sel.Name == "ChildEnv":
			launches[name.Name].childEnv = true
		}
		return true
	})
	if len(launches) == 0 {
		return []Violation{{Rule: RuleTrackerExecution, Path: path, Message: "tracker adapter has no context-aware process launch"}}, nil
	}
	var violations []Violation
	for _, launch := range launches {
		if launch.contextAware && launch.workDir && launch.childEnv {
			continue
		}
		line := fset.Position(launch.node.Pos()).Line
		violations = append(violations, Violation{Rule: RuleTrackerExecution, Path: path, Line: line, Message: "tracker launch must use caller context plus resolved WorkDir and ChildEnv"})
	}
	return violations, nil
}
