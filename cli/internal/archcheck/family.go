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
	"strconv"
	"strings"
)

type ownershipRecord struct {
	SchemaVersion int               `json:"schema_version"`
	Family        string            `json:"family"`
	Profiles      map[string]string `json:"profiles"`
	LegacySymbols []string          `json:"legacy_symbols"`
	LiveOwner     string            `json:"live_owner"`
	AllowedPaths  []string          `json:"allowed_paths"`
}

type lineageRecord struct {
	SchemaVersion   int    `json:"schema_version"`
	Family          string `json:"family"`
	FreezeSHA       string `json:"freeze_sha"`
	OwnershipSHA256 string `json:"ownership_sha256"`
	ScopeSHA256     string `json:"scope_sha256,omitempty"`
	MigrationState  string `json:"migration_state,omitempty"`
	AcceptedSHA     string `json:"accepted_sha,omitempty"`
}

func checkFamily(root, family string) ([]Violation, error) {
	familyDir := strings.ReplaceAll(family, "-", "_")
	baseline := filepath.Join(root, "cli", "testdata", "compatibility-baseline", "families", family)
	ownershipPath := filepath.Join(baseline, "ownership.json")
	lineagePath := filepath.Join(baseline, "lineage.json")

	ownershipBytes, err := os.ReadFile(ownershipPath)
	if err != nil {
		return nil, fmt.Errorf("family %s ownership: %w", family, err)
	}
	var ownership ownershipRecord
	if err := json.Unmarshal(ownershipBytes, &ownership); err != nil {
		return nil, fmt.Errorf("decode %s: %w", ownershipPath, err)
	}
	if !validOwnershipRecord(ownership, family) {
		return []Violation{{Rule: RuleOwnership, Path: relative(root, ownershipPath), Message: "invalid or mismatched ownership record"}}, nil
	}
	lineageBytes, err := os.ReadFile(lineagePath)
	if err != nil {
		return nil, fmt.Errorf("family %s lineage: %w", family, err)
	}
	var lineage lineageRecord
	if err := json.Unmarshal(lineageBytes, &lineage); err != nil {
		return nil, fmt.Errorf("decode %s: %w", lineagePath, err)
	}

	var violations []Violation
	if lineage.SchemaVersion != 1 || lineage.Family != family {
		violations = append(violations, Violation{Rule: RuleOwnership, Path: relative(root, lineagePath), Message: "invalid or mismatched lineage record"})
	}
	if !isFullCommitSHA(lineage.FreezeSHA) {
		violations = append(violations, Violation{Rule: RuleScope, Path: relative(root, lineagePath), Message: "lineage freeze_sha must be a full commit"})
	} else {
		violations = append(violations, verifyFrozenOwnership(root, ownershipPath, ownershipBytes, lineage)...)
	}

	moduleDir := filepath.Join(root, "cli", "internal", "commands", familyDir)
	modulePath := filepath.Join(moduleDir, "module.go")
	_, moduleErr := os.Stat(modulePath)
	if moduleErr != nil {
		violations = append(violations, Violation{Rule: RuleOwnership, Path: relative(root, modulePath), Message: "migrated family must have module.go"})
	} else {
		if lineage.MigrationState != "migrating" && lineage.MigrationState != "migrated" {
			violations = append(violations, Violation{Rule: RuleOwnership, Path: relative(root, lineagePath), Message: "module exists but lineage migration_state is neither migrating nor migrated"})
		}
		moduleViolations, checkErr := checkTree(root, moduleDir)
		if checkErr != nil {
			return nil, checkErr
		}
		violations = append(violations, moduleViolations...)
		violations = append(violations, checkModuleShape(root, moduleDir, modulePath, ownership)...)
	}
	if (lineage.MigrationState == "migrating" || lineage.MigrationState == "migrated") && moduleErr != nil {
		violations = append(violations, Violation{Rule: RuleOwnership, Path: relative(root, modulePath), Message: "migration lineage cannot lose its command module"})
	}

	ownerPath := filepath.Join(root, filepath.FromSlash(ownership.LiveOwner))
	if info, err := os.Stat(ownerPath); err != nil || !info.IsDir() {
		violations = append(violations, Violation{Rule: RuleOwnership, Path: ownership.LiveOwner, Message: "declared live owner directory does not exist"})
	}
	legacy, err := findLegacySymbols(root, ownership.LegacySymbols)
	if err != nil {
		return nil, err
	}
	violations = append(violations, legacy...)
	if isFullCommitSHA(lineage.FreezeSHA) {
		scopeEnd := "HEAD"
		switch lineage.MigrationState {
		case "migrating":
			if lineage.AcceptedSHA != "" {
				violations = append(violations, Violation{Rule: RuleScope, Path: relative(root, lineagePath), Message: "migrating lineage cannot declare accepted_sha"})
			}
		case "migrated":
			acceptedViolations := verifyAcceptedBoundary(root, lineagePath, modulePath, lineageBytes, lineage)
			violations = append(violations, acceptedViolations...)
			if isFullCommitSHA(lineage.AcceptedSHA) {
				scopeEnd = lineage.AcceptedSHA
			}
		}
		scopeViolations, err := checkAllowedScope(root, lineage.FreezeSHA, scopeEnd, ownership.AllowedPaths)
		if err != nil {
			return nil, err
		}
		violations = append(violations, scopeViolations...)
	}

	violations = dedupe(violations)
	sortViolations(violations)
	return violations, nil
}

func validOwnershipRecord(ownership ownershipRecord, family string) bool {
	if ownership.SchemaVersion != 1 || ownership.Family != family || strings.TrimSpace(ownership.LiveOwner) == "" || len(ownership.AllowedPaths) == 0 || len(ownership.Profiles) != 4 {
		return false
	}
	for _, profile := range []string{"default", "flywheel", "legacy", "combined"} {
		value, ok := ownership.Profiles[profile]
		if !ok || value != "present" && value != "absent" {
			return false
		}
	}
	return true
}

func isFullCommitSHA(value string) bool {
	if len(value) != 40 {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			if character < 'a' || character > 'f' {
				return false
			}
		}
	}
	return true
}

func discoverMigratedFamilies(root string) ([]string, error) {
	familiesRoot := filepath.Join(root, "cli", "testdata", "compatibility-baseline", "families")
	familySet := map[string]bool{}
	entries, err := os.ReadDir(familiesRoot)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			familySet[entry.Name()] = true
		}
	}
	history, _ := gitOutput(root, "log", "--diff-filter=A", "--name-only", "--format=", "--", "cli/testdata/compatibility-baseline/families")
	const prefix = "cli/testdata/compatibility-baseline/families/"
	for _, path := range strings.Fields(history) {
		if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, "/ownership.json") {
			continue
		}
		family := strings.TrimSuffix(strings.TrimPrefix(path, prefix), "/ownership.json")
		if family != "" && !strings.Contains(family, "/") {
			familySet[family] = true
		}
	}
	allFamilies := make([]string, 0, len(familySet))
	for family := range familySet {
		allFamilies = append(allFamilies, family)
	}
	sort.Strings(allFamilies)
	var families []string
	for _, family := range allFamilies {
		moduleDir := filepath.Join(root, "cli", "internal", "commands", strings.ReplaceAll(family, "-", "_"))
		if info, err := os.Stat(moduleDir); err == nil && info.IsDir() {
			families = append(families, family)
			continue
		}
		moduleRel := relative(root, filepath.Join(moduleDir, "module.go"))
		introduced, _ := gitOutput(root, "log", "--diff-filter=A", "--format=%H", "--", moduleRel)
		if strings.TrimSpace(introduced) != "" {
			families = append(families, family)
			continue
		}
		lineageBytes, err := os.ReadFile(filepath.Join(familiesRoot, family, "lineage.json"))
		if err != nil {
			continue
		}
		var lineage lineageRecord
		if json.Unmarshal(lineageBytes, &lineage) == nil && (lineage.MigrationState == "migrating" || lineage.MigrationState == "migrated") {
			families = append(families, family)
		}
	}
	sort.Strings(families)
	return families, nil
}

func verifyAcceptedBoundary(root, lineagePath, modulePath string, current []byte, lineage lineageRecord) []Violation {
	rel := relative(root, lineagePath)
	moduleRel := relative(root, modulePath)
	if !isFullCommitSHA(lineage.AcceptedSHA) {
		return []Violation{{Rule: RuleScope, Path: rel, Message: "migrated lineage accepted_sha must be a full commit"}}
	}
	var violations []Violation
	if command := exec.Command("git", "-C", root, "merge-base", "--is-ancestor", lineage.FreezeSHA, lineage.AcceptedSHA); command.Run() != nil {
		violations = append(violations, Violation{Rule: RuleScope, Path: rel, Message: "accepted_sha must descend from freeze_sha"})
	}
	introduced, err := gitOutput(root, "log", "-S\"accepted_sha\"", "--format=%H", "--", rel)
	commits := strings.Fields(introduced)
	if err != nil || len(commits) != 1 {
		violations = append(violations, Violation{Rule: RuleScope, Path: rel, Message: "accepted_sha must have exactly one Git introduction commit"})
		return violations
	}
	sealSHA := commits[0]
	sealed, err := gitOutputBytes(root, "show", sealSHA+":"+rel)
	if err != nil || !slices.Equal(current, sealed) {
		violations = append(violations, Violation{Rule: RuleScope, Path: rel, Message: "lineage changed after its accepted_sha seal commit"})
	}
	parents, err := gitOutput(root, "rev-list", "--parents", "-n", "1", sealSHA)
	parentFields := strings.Fields(parents)
	if err != nil || len(parentFields) != 2 || parentFields[1] != lineage.AcceptedSHA {
		violations = append(violations, Violation{Rule: RuleScope, Path: rel, Message: "accepted_sha seal commit must have exactly one parent and it must be accepted_sha"})
	}
	changed, err := gitOutput(root, "diff", "--name-only", lineage.AcceptedSHA+".."+sealSHA)
	paths := strings.Fields(changed)
	if err != nil || len(paths) != 1 || paths[0] != rel {
		violations = append(violations, Violation{Rule: RuleScope, Path: rel, Message: "accepted_sha seal commit may change only lineage.json"})
	}
	if command := exec.Command("git", "-C", root, "merge-base", "--is-ancestor", sealSHA, "HEAD"); command.Run() != nil {
		violations = append(violations, Violation{Rule: RuleScope, Path: rel, Message: "accepted_sha seal commit is not an ancestor of HEAD"})
	}
	if command := exec.Command("git", "-C", root, "cat-file", "-e", lineage.FreezeSHA+":"+moduleRel); command.Run() == nil {
		violations = append(violations, Violation{Rule: RuleOwnership, Path: moduleRel, Message: "command module must be absent at freeze_sha"})
	}
	if command := exec.Command("git", "-C", root, "cat-file", "-e", lineage.AcceptedSHA+":"+moduleRel); command.Run() != nil {
		violations = append(violations, Violation{Rule: RuleOwnership, Path: moduleRel, Message: "command module must be present at accepted_sha"})
	}
	introduced, err = gitOutput(root, "log", "--diff-filter=A", "--format=%H", "--", moduleRel)
	moduleIntroductions := strings.Fields(introduced)
	if err != nil || len(moduleIntroductions) != 1 {
		violations = append(violations, Violation{Rule: RuleOwnership, Path: moduleRel, Message: "command module must have exactly one Git introduction commit"})
		return violations
	}
	moduleIntroduction := moduleIntroductions[0]
	if command := exec.Command("git", "-C", root, "merge-base", "--is-ancestor", lineage.FreezeSHA, moduleIntroduction); command.Run() != nil {
		violations = append(violations, Violation{Rule: RuleOwnership, Path: moduleRel, Message: "command module introduction must descend from freeze_sha"})
	}
	if command := exec.Command("git", "-C", root, "merge-base", "--is-ancestor", moduleIntroduction, lineage.AcceptedSHA); command.Run() != nil {
		violations = append(violations, Violation{Rule: RuleOwnership, Path: moduleRel, Message: "command module introduction must be included in accepted_sha"})
	}
	return violations
}

func gitOutputBytes(root string, args ...string) ([]byte, error) {
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("git %v: %w", args, err)
	}
	return output, nil
}

func verifyFrozenOwnership(root, ownershipPath string, current []byte, lineage lineageRecord) []Violation {
	rel := relative(root, ownershipPath)
	var violations []Violation
	show := exec.Command("git", "-C", root, "show", lineage.FreezeSHA+":"+rel)
	frozen, err := show.Output()
	if err != nil {
		return []Violation{{Rule: RuleOwnership, Path: rel, Message: "freeze_sha does not contain the ownership record"}}
	}
	sum := sha256.Sum256(frozen)
	if hex.EncodeToString(sum[:]) != lineage.OwnershipSHA256 {
		violations = append(violations, Violation{Rule: RuleOwnership, Path: rel, Message: "lineage digest does not match ownership bytes at freeze_sha"})
	}
	if !slices.Equal(current, frozen) {
		violations = append(violations, Violation{Rule: RuleOwnership, Path: rel, Message: "ownership record changed after its freeze commit"})
	}
	introduced := exec.Command("git", "-C", root, "log", "--diff-filter=A", "--format=%H", "--", rel)
	output, err := introduced.Output()
	commits := strings.Fields(string(output))
	if err != nil || len(commits) != 1 || commits[0] != lineage.FreezeSHA {
		violations = append(violations, Violation{Rule: RuleOwnership, Path: rel, Message: "freeze_sha must be the ownership record's sole Git introduction commit"})
	}
	return violations
}

func checkModuleShape(root, moduleDir, modulePath string, ownership ownershipRecord) []Violation {
	fset := token.NewFileSet()
	files, _ := filepath.Glob(filepath.Join(moduleDir, "*.go"))
	newModuleCount := 0
	contractCount := 0
	validProfileMasks := 0
	actualProfiles := map[string]bool{}
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			continue
		}
		aliases := sourceImportAliases(file)
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if isModuleConstructor(function) {
				newModuleCount++
			}
			if !isModuleContract(function, aliases) {
				continue
			}
			returned := returnedContractLiterals(function.Body, aliases)
			contractCount += len(returned)
			for _, literal := range returned {
				if collectLiteralProfiles(literal, aliases, actualProfiles) {
					validProfileMasks++
				}
			}
		}
	}
	var violations []Violation
	if newModuleCount != 1 {
		violations = append(violations, Violation{Rule: RuleOwnership, Path: relative(root, modulePath), Message: fmt.Sprintf("want exactly one NewModule production owner, found %d", newModuleCount)})
	}
	if contractCount != 1 {
		violations = append(violations, Violation{Rule: RuleProfileReachability, Path: relative(root, modulePath), Message: fmt.Sprintf("want exactly one explicit CommandContract, found %d", contractCount)})
	}
	if validProfileMasks != 1 {
		violations = append(violations, Violation{Rule: RuleProfileReachability, Path: relative(root, modulePath), Message: "Profiles must be one OR-only mask of exact clicontract profile constants"})
	}
	for _, profile := range []string{"default", "flywheel", "legacy", "combined"} {
		want := ownership.Profiles[profile] == "present"
		if actualProfiles[profile] != want {
			violations = append(violations, Violation{Rule: RuleProfileReachability, Path: relative(root, modulePath), Message: fmt.Sprintf("profile %s reachability=%t, ownership wants %t", profile, actualProfiles[profile], want)})
		}
	}
	return violations
}

func isModuleContract(function *ast.FuncDecl, aliases map[string]string) bool {
	if function.Name.Name != "Contract" || receiverType(function) != "Module" || function.Body == nil {
		return false
	}
	if function.Type.Params != nil && len(function.Type.Params.List) != 0 {
		return false
	}
	if function.Type.Results == nil || len(function.Type.Results.List) != 1 {
		return false
	}
	selector, ok := function.Type.Results.List[0].Type.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "CommandContract" {
		return false
	}
	alias, ok := selector.X.(*ast.Ident)
	return ok && aliases[alias.Name] == "github.com/boshu2/agentops/cli/internal/clicontract"
}

func isModuleConstructor(function *ast.FuncDecl) bool {
	if function.Name.Name != "NewModule" || function.Recv != nil || function.Type.Results == nil || len(function.Type.Results.List) != 1 {
		return false
	}
	result := function.Type.Results.List[0].Type
	if pointer, ok := result.(*ast.StarExpr); ok {
		result = pointer.X
	}
	ident, ok := result.(*ast.Ident)
	return ok && ident.Name == "Module"
}

func sourceImportAliases(file *ast.File) map[string]string {
	aliases := map[string]string{}
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		name := filepath.Base(path)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		aliases[name] = path
	}
	return aliases
}

func receiverType(function *ast.FuncDecl) string {
	if function.Recv == nil || len(function.Recv.List) != 1 {
		return ""
	}
	expression := function.Recv.List[0].Type
	if pointer, ok := expression.(*ast.StarExpr); ok {
		expression = pointer.X
	}
	ident, _ := expression.(*ast.Ident)
	if ident == nil {
		return ""
	}
	return ident.Name
}

func isCLIContractLiteral(literal *ast.CompositeLit, aliases map[string]string) bool {
	selector, ok := literal.Type.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "CommandContract" {
		return false
	}
	alias, ok := selector.X.(*ast.Ident)
	return ok && aliases[alias.Name] == "github.com/boshu2/agentops/cli/internal/clicontract"
}

func returnedContractLiterals(body *ast.BlockStmt, aliases map[string]string) []*ast.CompositeLit {
	var literals []*ast.CompositeLit
	returnCount := 0
	ast.Inspect(body, func(node ast.Node) bool {
		if _, nested := node.(*ast.FuncLit); nested {
			return false
		}
		statement, ok := node.(*ast.ReturnStmt)
		if !ok {
			return true
		}
		returnCount++
		if len(statement.Results) != 1 {
			return true
		}
		expression := statement.Results[0]
		if parenthesized, ok := expression.(*ast.ParenExpr); ok {
			expression = parenthesized.X
		}
		literal, ok := expression.(*ast.CompositeLit)
		if ok && isCLIContractLiteral(literal, aliases) {
			literals = append(literals, literal)
		}
		return true
	})
	if returnCount != 1 {
		return nil
	}
	return literals
}

func collectLiteralProfiles(literal *ast.CompositeLit, aliases map[string]string, profiles map[string]bool) bool {
	fields := 0
	valid := false
	for _, element := range literal.Elts {
		field, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := field.Key.(*ast.Ident)
		if !ok || key.Name != "Profiles" {
			continue
		}
		fields++
		parsed, ok := parseProfileMask(field.Value, aliases)
		if ok {
			valid = true
			for profile := range parsed {
				profiles[profile] = true
			}
		}
	}
	return fields == 1 && valid
}

func parseProfileMask(expression ast.Expr, aliases map[string]string) (map[string]bool, bool) {
	switch value := expression.(type) {
	case *ast.ParenExpr:
		return parseProfileMask(value.X, aliases)
	case *ast.BinaryExpr:
		if value.Op != token.OR {
			return nil, false
		}
		left, leftOK := parseProfileMask(value.X, aliases)
		right, rightOK := parseProfileMask(value.Y, aliases)
		if !leftOK || !rightOK {
			return nil, false
		}
		for profile := range right {
			left[profile] = true
		}
		return left, true
	case *ast.SelectorExpr:
		alias, ok := value.X.(*ast.Ident)
		if !ok || aliases[alias.Name] != "github.com/boshu2/agentops/cli/internal/clicontract" {
			return nil, false
		}
		profiles := map[string]bool{}
		switch value.Sel.Name {
		case "ProfileDefault":
			profiles["default"] = true
		case "ProfileFlywheel":
			profiles["flywheel"] = true
		case "ProfileLegacy":
			profiles["legacy"] = true
		case "ProfileCombined":
			profiles["combined"] = true
		default:
			return nil, false
		}
		return profiles, true
	default:
		return nil, false
	}
}

func findLegacySymbols(root string, symbols []string) ([]Violation, error) {
	wanted := make(map[string]bool, len(symbols))
	for _, symbol := range symbols {
		wanted[symbol] = true
	}
	if len(wanted) == 0 {
		return nil, nil
	}
	cmdRoot := filepath.Join(root, "cli", "cmd", "ao")
	var violations []Violation
	err := filepath.WalkDir(cmdRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(node ast.Node) bool {
			ident, ok := node.(*ast.Ident)
			if ok && wanted[ident.Name] {
				violations = append(violations, Violation{Rule: RuleLegacySymbol, Path: relative(root, path), Line: fset.Position(ident.Pos()).Line, Message: fmt.Sprintf("legacy symbol %s remains", ident.Name)})
			}
			return true
		})
		return nil
	})
	if os.IsNotExist(err) {
		return nil, nil
	}
	return violations, err
}

func checkAllowedScope(root, freezeSHA, endSHA string, allowed []string) ([]Violation, error) {
	command := exec.Command("git", "-C", root, "diff", "--name-only", freezeSHA+".."+endSHA)
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("family scope diff from %s to %s: %w", freezeSHA, endSHA, err)
	}
	var violations []Violation
	for _, path := range strings.Fields(string(output)) {
		if matchesAny(path, allowed) {
			continue
		}
		violations = append(violations, Violation{Rule: RuleScope, Path: path, Message: "changed path is outside frozen allowed_paths"})
	}
	return violations, nil
}

func matchesAny(path string, patterns []string) bool {
	for _, pattern := range patterns {
		pattern = filepath.ToSlash(pattern)
		switch {
		case strings.HasSuffix(pattern, "/**") && strings.HasPrefix(path, strings.TrimSuffix(pattern, "**")):
			return true
		case !strings.Contains(pattern, "*") && path == pattern:
			return true
		case strings.Contains(pattern, "*"):
			prefix, suffix, _ := strings.Cut(pattern, "*")
			if strings.HasPrefix(path, prefix) && strings.HasSuffix(path, suffix) {
				return true
			}
		}
	}
	return false
}

func relative(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}
