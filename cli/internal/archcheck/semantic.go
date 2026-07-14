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
	"strings"
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
	Source       string `json:"source"`
	SourceSHA256 string `json:"source_sha256"`
	Generator    string `json:"generator"`
	Path         string `json:"path"`
	SHA256       string `json:"sha256"`
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

func checkSemanticSeal(root, expectedCandidate string) ([]Violation, error) {
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
		return checkEvidenceBinding(root, manifest, expectedCandidate), nil
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

func checkEvidenceBinding(root string, manifest semanticSealManifest, expectedCandidate string) []Violation {
	invalid := func(path, message string) []Violation {
		return []Violation{{Rule: RuleEvidenceBinding, Path: path, Message: message}}
	}
	if !validHexDigest(expectedCandidate, 40) || !validHexDigest(manifest.CandidateSHA, 40) || manifest.Evidence == nil || !validHexDigest(manifest.Evidence.SHA256, 64) {
		return invalid(semanticSealFilename, "candidate SHA and evidence digest must be complete lowercase hex digests")
	}
	if manifest.CandidateSHA != expectedCandidate {
		return invalid(semanticSealFilename, "evidence manifest is not bound to the externally supplied candidate")
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

// SemanticProductionGate executes one adversarial canary for every semantic
// rule registered in the production architecture gate. It deliberately does
// not depend on a fixture-root semantic-seal manifest.
func SemanticProductionGate(expectedCandidate string) ([]Rule, error) {
	if !validHexDigest(expectedCandidate, 40) {
		return nil, fmt.Errorf("externally resolved candidate SHA is required")
	}
	type canary struct {
		rule Rule
		run  func() ([]Violation, error)
	}
	canaries := []canary{
		{RuleContext, func() ([]Violation, error) {
			return CheckSource("cli/internal/commands/probe/module.go", []byte(`package probe; import "context"; func run(){ _ = context.Background() }`))
		}},
		{RuleTrackerExecution, func() ([]Violation, error) {
			return checkTrackerExecutionSource("tracker.go", []byte(`package probe; import ("context"; "os/exec"); type resolution struct { Binary, WorkDir string; ChildEnv []string }; func launch(ctx context.Context, resolved resolution) error { command := exec.CommandContext(context.Background(), resolved.Binary); command.Dir = resolved.WorkDir; command.Env = resolved.ChildEnv; return command.Run() }`))
		}},
		{RuleEffects, func() ([]Violation, error) {
			return checkUniversalEffectsSource("root.go", []byte(`package probe; import ("os"; "github.com/spf13/cobra"); func prepare(*cobra.Command, []string) error { _, err := os.Getwd(); return err }; var root = &cobra.Command{PersistentPreRunE: prepare}`))
		}},
		{RuleOutput, func() ([]Violation, error) {
			return checkOutputSource("module.go", []byte(`package probe; import ("encoding/json"; "github.com/spf13/cobra"; "github.com/boshu2/agentops/cli/internal/clicontract"); func contract() clicontract.CommandContract { return clicontract.CommandContract{Output: clicontract.OutputStructured} }; func command() *cobra.Command { return &cobra.Command{RunE: func(command *cobra.Command, _ []string) error { return json.NewEncoder(command.OutOrStdout()).Encode(true) }} }`))
		}},
		{RuleRecursiveContracts, func() ([]Violation, error) {
			return checkRecursiveContractsSource("module.go", []byte(`package probe; import ("github.com/spf13/cobra"; "github.com/boshu2/agentops/cli/internal/clicontract"); func command() *cobra.Command { root := &cobra.Command{RunE: run}; child := &cobra.Command{RunE: run}; root.AddCommand(child); _ = clicontract.Attach(root, clicontract.CommandContract{}); _ = clicontract.Attach(root, clicontract.CommandContract{}); return root }; func run(*cobra.Command, []string) error { return nil }`))
		}},
	}
	for _, canary := range canaries {
		violations, err := canary.run()
		if err != nil {
			return nil, err
		}
		if !violationsContainRule(violations, canary.rule) {
			return nil, fmt.Errorf("%s canary escaped", canary.rule)
		}
	}

	root, err := os.MkdirTemp("", "agentops-semantic-gate-")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(root) }()
	changedSource := []byte("changed source\n")
	generated := []byte("generated from source\n")
	if err := os.WriteFile(filepath.Join(root, "source.txt"), changedSource, 0o600); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(root, "generated.txt"), generated, 0o600); err != nil {
		return nil, err
	}
	generatedViolations := checkGeneratedEvidence(root, semanticSealManifest{
		Sources: []string{"source.txt"},
		Generated: []semanticGenerated{{
			Source: "source.txt", SourceSHA256: digestBytes([]byte("source\n")), Generator: "archcheck.prefix-generated-from.v1",
			Path: "generated.txt", SHA256: digestBytes(generated),
		}},
	})
	if !violationsContainRule(generatedViolations, RuleGeneratedEvidence) {
		return nil, fmt.Errorf("%s canary escaped", RuleGeneratedEvidence)
	}

	fabricatedCandidate := strings.Repeat("a", 40)
	if fabricatedCandidate == expectedCandidate {
		fabricatedCandidate = strings.Repeat("b", 40)
	}
	evidenceDocument := semanticEvidenceDocument{SchemaVersion: 1, Class: "evidence-binding", CandidateSHA: fabricatedCandidate, SourceDigests: map[string]string{"source.txt": digestBytes(changedSource)}}
	evidenceData, err := json.Marshal(evidenceDocument)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(root, "evidence.json"), evidenceData, 0o600); err != nil {
		return nil, err
	}
	evidenceViolations := checkEvidenceBinding(root, semanticSealManifest{
		Class: "evidence-binding", Sources: []string{"source.txt"}, CandidateSHA: fabricatedCandidate,
		Evidence: &semanticEvidence{Path: "evidence.json", SHA256: digestBytes(evidenceData)},
	}, expectedCandidate)
	if !violationsContainRule(evidenceViolations, RuleEvidenceBinding) {
		return nil, fmt.Errorf("%s canary escaped", RuleEvidenceBinding)
	}

	return []Rule{RuleContext, RuleTrackerExecution, RuleEffects, RuleOutput, RuleRecursiveContracts, RuleGeneratedEvidence, RuleEvidenceBinding}, nil
}

func violationsContainRule(violations []Violation, rule Rule) bool {
	for _, violation := range violations {
		if violation.Rule == rule {
			return true
		}
	}
	return false
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
		sourceData, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(generated.Source)))
		if err != nil {
			violations = append(violations, Violation{Rule: RuleGeneratedEvidence, Path: generated.Source, Message: "generated evidence source is missing"})
			continue
		}
		if !validHexDigest(generated.SourceSHA256, 64) || digestBytes(sourceData) != generated.SourceSHA256 {
			violations = append(violations, Violation{Rule: RuleGeneratedEvidence, Path: generated.Source, Message: "generated evidence source digest mismatch"})
			continue
		}
		if generated.Generator != "archcheck.prefix-generated-from.v1" {
			violations = append(violations, Violation{Rule: RuleGeneratedEvidence, Path: semanticSealFilename, Message: "generated evidence must declare a supported deterministic generator"})
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
			continue
		}
		expected := []byte("generated from " + strings.TrimSuffix(string(sourceData), "\n") + "\n")
		if string(data) != string(expected) {
			violations = append(violations, Violation{Rule: RuleGeneratedEvidence, Path: generated.Path, Message: "generated evidence does not equal deterministic generator recomputation"})
		}
	}
	return violations
}

// recursiveCommandNode is a discovered cobra command literal plus whether it is runnable.
type recursiveCommandNode struct {
	node     ast.Node
	runnable bool
}

// collectCommandDefinition records a cobra command literal assigned to an identifier.
func collectCommandDefinition(node ast.Node, aliases map[string]string, commands map[string]recursiveCommandNode) {
	assignment, ok := node.(*ast.AssignStmt)
	if !ok || len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 {
		return
	}
	name, nameOK := assignment.Lhs[0].(*ast.Ident)
	literal, literalOK := cobraCommandLiteral(assignment.Rhs[0], aliases)
	if nameOK && literalOK {
		commands[name.Name] = recursiveCommandNode{node: literal, runnable: cobraCommandRunnable(literal)}
	}
}

// collectCommandGraphEdges records AddCommand parent→child edges and clicontract attachment counts.
func collectCommandGraphEdges(node ast.Node, aliases map[string]string, edges map[string][]string, attachments map[string]int) {
	call, ok := node.(*ast.CallExpr)
	if !ok {
		return
	}
	selector, selectorOK := call.Fun.(*ast.SelectorExpr)
	if !selectorOK {
		return
	}
	if selector.Sel.Name == "AddCommand" {
		if parent, parentOK := selector.X.(*ast.Ident); parentOK {
			for _, argument := range call.Args {
				if child, childOK := argument.(*ast.Ident); childOK {
					edges[parent.Name] = append(edges[parent.Name], child.Name)
				}
			}
		}
	}
	if selector.Sel.Name == "Attach" {
		pkg, pkgOK := selector.X.(*ast.Ident)
		if pkgOK && aliases[pkg.Name] == "github.com/boshu2/agentops/cli/internal/clicontract" && len(call.Args) > 0 {
			if command, commandOK := call.Args[0].(*ast.Ident); commandOK {
				attachments[command.Name]++
			}
		}
	}
}

// collectCommandRoots records identifiers returned as command roots.
func collectCommandRoots(node ast.Node, commands map[string]recursiveCommandNode, roots map[string]bool) {
	statement, ok := node.(*ast.ReturnStmt)
	if !ok {
		return
	}
	for _, result := range statement.Results {
		root, rootOK := result.(*ast.Ident)
		if !rootOK {
			continue
		}
		if _, commandOK := commands[root.Name]; commandOK {
			roots[root.Name] = true
		}
	}
}

func checkRecursiveContractsSource(path string, source []byte) ([]Violation, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, source, 0)
	if err != nil {
		return nil, fmt.Errorf("parse semantic source %s: %w", path, err)
	}
	aliases := sourceImportAliases(file)
	commands := map[string]recursiveCommandNode{}
	edges := map[string][]string{}
	attachments := map[string]int{}
	roots := map[string]bool{}
	ast.Inspect(file, func(node ast.Node) bool {
		collectCommandDefinition(node, aliases, commands)
		collectCommandGraphEdges(node, aliases, edges, attachments)
		collectCommandRoots(node, commands, roots)
		return true
	})
	reachable := map[string]bool{}
	var visit func(string)
	visit = func(name string) {
		if reachable[name] {
			return
		}
		reachable[name] = true
		for _, child := range edges[name] {
			visit(child)
		}
	}
	for root := range roots {
		visit(root)
	}
	var violations []Violation
	for name, command := range commands {
		if !reachable[name] {
			if attachments[name] > 0 {
				violations = append(violations, Violation{Rule: RuleRecursiveContracts, Path: path, Line: fset.Position(command.node.Pos()).Line, Message: fmt.Sprintf("contract attached to unreachable command %s", name)})
			}
			continue
		}
		if command.runnable && attachments[name] != 1 {
			violations = append(violations, Violation{Rule: RuleRecursiveContracts, Path: path, Line: fset.Position(command.node.Pos()).Line, Message: fmt.Sprintf("reachable runnable %s needs exactly one attached contract: got %d", name, attachments[name])})
		}
	}
	return violations, nil
}

func cobraCommandLiteral(expression ast.Expr, aliases map[string]string) (*ast.CompositeLit, bool) {
	if unary, ok := expression.(*ast.UnaryExpr); ok {
		expression = unary.X
	}
	literal, ok := expression.(*ast.CompositeLit)
	if !ok {
		return nil, false
	}
	selector, ok := literal.Type.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Command" {
		return nil, false
	}
	pkg, ok := selector.X.(*ast.Ident)
	return literal, ok && aliases[pkg.Name] == "github.com/spf13/cobra"
}

func cobraCommandRunnable(literal *ast.CompositeLit) bool {
	for _, element := range literal.Elts {
		field, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := field.Key.(*ast.Ident)
		if ok && (key.Name == "Run" || key.Name == "RunE") {
			return true
		}
	}
	return false
}

// collectStructuredFunctions reports the functions that call clicontract.OutputStructured
// and whether any structured contract exists at all.
func collectStructuredFunctions(file *ast.File, aliases map[string]string) (map[string]bool, bool) {
	structuredFunctions := map[string]bool{}
	structured := false
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Body == nil {
			continue
		}
		found := false
		ast.Inspect(function.Body, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "OutputStructured" {
				return true
			}
			pkg, ok := selector.X.(*ast.Ident)
			if ok && aliases[pkg.Name] == "github.com/boshu2/agentops/cli/internal/clicontract" {
				found = true
			}
			return true
		})
		if found {
			structured = true
			structuredFunctions[function.Name.Name] = true
		}
	}
	return structuredFunctions, structured
}

// collectOutputRunnable records runnable cobra commands defined by assignment or returned inline.
func collectOutputRunnable(node ast.Node, aliases map[string]string, fset *token.FileSet, runnables map[string]ast.Node) {
	if assignment, ok := node.(*ast.AssignStmt); ok && len(assignment.Lhs) == 1 && len(assignment.Rhs) == 1 {
		name, nameOK := assignment.Lhs[0].(*ast.Ident)
		literal, literalOK := cobraCommandLiteral(assignment.Rhs[0], aliases)
		if nameOK && literalOK && cobraCommandRunnable(literal) {
			runnables[name.Name] = literal
		}
	}
	if statement, ok := node.(*ast.ReturnStmt); ok {
		for _, result := range statement.Results {
			literal, literalOK := cobraCommandLiteral(result, aliases)
			if literalOK && cobraCommandRunnable(literal) {
				runnables[fmt.Sprintf("@%d", fset.Position(literal.Pos()).Line)] = literal
			}
		}
	}
}

// collectStructuredAttachment counts clicontract.Attach calls that bind a structured contract.
func collectStructuredAttachment(node ast.Node, aliases map[string]string, structuredFunctions map[string]bool, structuredAttachments map[string]int) {
	call, ok := node.(*ast.CallExpr)
	if !ok {
		return
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Attach" || len(call.Args) < 2 {
		return
	}
	pkg, pkgOK := selector.X.(*ast.Ident)
	command, commandOK := call.Args[0].(*ast.Ident)
	contractCall, callOK := call.Args[1].(*ast.CallExpr)
	if !callOK {
		return
	}
	contractName, nameOK := contractCall.Fun.(*ast.Ident)
	if pkgOK && commandOK && nameOK && aliases[pkg.Name] == "github.com/boshu2/agentops/cli/internal/clicontract" && structuredFunctions[contractName.Name] {
		structuredAttachments[command.Name]++
	}
}

// appendHumanTextViolations flags fmt.Fprint* human-text formatting under a structured contract.
func appendHumanTextViolations(file *ast.File, aliases map[string]string, fset *token.FileSet, path string, violations []Violation) []Violation {
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
	return violations
}

func checkOutputSource(path string, source []byte) ([]Violation, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, source, 0)
	if err != nil {
		return nil, fmt.Errorf("parse semantic source %s: %w", path, err)
	}
	aliases := sourceImportAliases(file)
	structuredFunctions, structured := collectStructuredFunctions(file, aliases)
	runnables := map[string]ast.Node{}
	structuredAttachments := map[string]int{}
	ast.Inspect(file, func(node ast.Node) bool {
		collectOutputRunnable(node, aliases, fset, runnables)
		collectStructuredAttachment(node, aliases, structuredFunctions, structuredAttachments)
		return true
	})
	if !structured {
		return nil, nil
	}
	var violations []Violation
	for name, node := range runnables {
		if structuredAttachments[name] != 1 {
			violations = append(violations, Violation{Rule: RuleOutput, Path: path, Line: fset.Position(node.Pos()).Line, Message: fmt.Sprintf("structured output contract must be attached exactly once to runnable %s", name)})
		}
	}
	violations = appendHumanTextViolations(file, aliases, fset, path, violations)
	return violations, nil
}

func checkUniversalEffectsSource(path string, source []byte) ([]Violation, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, source, 0)
	if err != nil {
		return nil, fmt.Errorf("parse semantic source %s: %w", path, err)
	}
	aliases := sourceImportAliases(file)
	functions := map[string]*ast.BlockStmt{}
	for _, declaration := range file.Decls {
		if function, ok := declaration.(*ast.FuncDecl); ok && function.Body != nil {
			functions[function.Name.Name] = function.Body
		}
	}
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
		var body *ast.BlockStmt
		switch value := field.Value.(type) {
		case *ast.FuncLit:
			body = value.Body
		case *ast.Ident:
			body = functions[value.Name]
		}
		if body == nil {
			return true
		}
		hasEffect := false
		ast.Inspect(body, func(child ast.Node) bool {
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

// trackerLaunchState tracks a discovered exec launch and the guarantees it satisfies.
type trackerLaunchState struct {
	node         ast.Node
	contextAware bool
	resolvedName string
	workDir      bool
	childEnv     bool
}

// collectExecAliases returns the local aliases under which "os/exec" is imported.
func collectExecAliases(file *ast.File) map[string]bool {
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
	return execAliases
}

// collectContextParams returns the parameter names of type context.Context on a function.
func collectContextParams(function *ast.FuncDecl, aliases map[string]string) map[string]bool {
	contextParams := map[string]bool{}
	if function.Type.Params == nil {
		return contextParams
	}
	for _, field := range function.Type.Params.List {
		selector, selectorOK := field.Type.(*ast.SelectorExpr)
		if !selectorOK {
			continue
		}
		pkg, pkgOK := selector.X.(*ast.Ident)
		if !pkgOK || selector.Sel.Name != "Context" || aliases[pkg.Name] != "context" {
			continue
		}
		for _, name := range field.Names {
			contextParams[name.Name] = true
		}
	}
	return contextParams
}

// recordTrackerLaunch registers an exec launch assigned to name, capturing whether it is
// context-aware and the identifier that resolved the binary.
func recordTrackerLaunch(name *ast.Ident, assignment *ast.AssignStmt, execAliases map[string]bool, contextParams map[string]bool, launches map[string]*trackerLaunchState) {
	call, ok := assignment.Rhs[0].(*ast.CallExpr)
	if !ok {
		return
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	alias, aliasOK := selector.X.(*ast.Ident)
	if !aliasOK || !execAliases[alias.Name] {
		return
	}
	state := &trackerLaunchState{node: call}
	if selector.Sel.Name == "CommandContext" && len(call.Args) >= 2 {
		caller, callerOK := call.Args[0].(*ast.Ident)
		binary, binaryOK := call.Args[1].(*ast.SelectorExpr)
		if !binaryOK {
			launches[name.Name] = state
			return
		}
		resolved, resolvedOK := binary.X.(*ast.Ident)
		if callerOK && contextParams[caller.Name] && resolvedOK && binary.Sel.Name == "Binary" {
			state.contextAware = true
			state.resolvedName = resolved.Name
		}
	}
	launches[name.Name] = state
}

// applyTrackerLaunchField applies a Dir=WorkDir / Env=ChildEnv field assignment to a launch.
func applyTrackerLaunchField(assignment *ast.AssignStmt, launches map[string]*trackerLaunchState) {
	left, ok := assignment.Lhs[0].(*ast.SelectorExpr)
	if !ok {
		return
	}
	name, ok := left.X.(*ast.Ident)
	if !ok || launches[name.Name] == nil {
		return
	}
	right, ok := assignment.Rhs[0].(*ast.SelectorExpr)
	if !ok {
		return
	}
	resolved, resolvedOK := right.X.(*ast.Ident)
	if !resolvedOK || resolved.Name != launches[name.Name].resolvedName {
		return
	}
	switch {
	case left.Sel.Name == "Dir" && right.Sel.Name == "WorkDir":
		launches[name.Name].workDir = true
	case left.Sel.Name == "Env" && right.Sel.Name == "ChildEnv":
		launches[name.Name].childEnv = true
	}
}

func checkTrackerExecutionSource(path string, source []byte) ([]Violation, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, source, 0)
	if err != nil {
		return nil, fmt.Errorf("parse semantic source %s: %w", path, err)
	}
	aliases := sourceImportAliases(file)
	execAliases := collectExecAliases(file)
	launches := map[string]*trackerLaunchState{}
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Body == nil {
			continue
		}
		contextParams := collectContextParams(function, aliases)
		ast.Inspect(function.Body, func(node ast.Node) bool {
			assignment, ok := node.(*ast.AssignStmt)
			if !ok || len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 {
				return true
			}
			if name, ok := assignment.Lhs[0].(*ast.Ident); ok {
				recordTrackerLaunch(name, assignment, execAliases, contextParams, launches)
				return true
			}
			applyTrackerLaunchField(assignment, launches)
			return true
		})
	}
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
