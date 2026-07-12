// Package archcheck enforces the Go CLI's command-module dependency and effect
// boundaries before a migrated family is allowed to replace its legacy owner.
package archcheck

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type Rule string

const (
	RuleProcess             Rule = "effect.process"
	RuleFilesystem          Rule = "effect.filesystem"
	RuleEnvironment         Rule = "effect.environment"
	RuleNetwork             Rule = "effect.network"
	RuleTracker             Rule = "effect.tracker"
	RuleClock               Rule = "effect.clock"
	RuleServiceBag          Rule = "dependency.service-bag"
	RuleCompositionImport   Rule = "dependency.composition"
	RuleConcreteAdapter     Rule = "dependency.concrete-adapter"
	RuleOwnership           Rule = "family.ownership"
	RuleProfileReachability Rule = "family.profile-reachability"
	RuleLegacySymbol        Rule = "family.legacy-symbol"
	RuleScope               Rule = "family.scope"
)

type Violation struct {
	Rule    Rule
	Path    string
	Line    int
	Message string
}

func (v Violation) String() string {
	if v.Line > 0 {
		return fmt.Sprintf("%s:%d: %s: %s", v.Path, v.Line, v.Rule, v.Message)
	}
	return fmt.Sprintf("%s: %s: %s", v.Path, v.Rule, v.Message)
}

type Options struct {
	Root        string
	Family      string
	AllMigrated bool
	VerifyScope string
}

func Check(options Options) ([]Violation, error) {
	root, err := filepath.Abs(options.Root)
	if err != nil {
		return nil, err
	}
	commandsRoot := filepath.Join(root, "cli", "internal", "commands")
	if options.Family != "" {
		violations, err := checkFamily(root, options.Family)
		if err != nil {
			return nil, err
		}
		if options.VerifyScope != "" {
			scopeViolations, err := verifyScopeEvidence(root, options.Family, options.VerifyScope)
			if err != nil {
				return nil, err
			}
			violations = append(violations, scopeViolations...)
		}
		violations = dedupe(violations)
		sortViolations(violations)
		return violations, nil
	}
	var violations []Violation
	if _, statErr := os.Stat(commandsRoot); statErr == nil {
		boundaryViolations, err := checkTree(root, commandsRoot)
		if err != nil {
			return nil, err
		}
		violations = append(violations, boundaryViolations...)
	} else if !os.IsNotExist(statErr) {
		return nil, statErr
	}
	families, err := discoverMigratedFamilies(root)
	if err != nil {
		return nil, err
	}
	for _, family := range families {
		familyViolations, err := checkFamily(root, family)
		if err != nil {
			return nil, err
		}
		violations = append(violations, familyViolations...)
	}
	violations = dedupe(violations)
	sortViolations(violations)
	return violations, nil
}

func checkTree(root, tree string) ([]Violation, error) {
	var violations []Violation
	err := filepath.WalkDir(tree, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		found, err := CheckSource(filepath.ToSlash(rel), data)
		if err != nil {
			return err
		}
		violations = append(violations, found...)
		return nil
	})
	sortViolations(violations)
	return violations, err
}

// CheckSource analyzes one production source file. It deliberately uses the Go
// AST: comments and string literals cannot masquerade as effects.
func CheckSource(path string, source []byte) ([]Violation, error) {
	if strings.HasSuffix(path, "_test.go") || !isCommandPath(path) {
		return nil, nil
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, source, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	checker := sourceChecker{
		path:    path,
		fset:    fset,
		aliases: map[string]string{},
	}

	for _, spec := range file.Imports {
		checker.checkImport(spec)
	}

	ast.Inspect(file, checker.inspectNode)

	checker.violations = dedupe(checker.violations)
	sortViolations(checker.violations)
	return checker.violations, nil
}

type sourceChecker struct {
	path       string
	fset       *token.FileSet
	aliases    map[string]string
	violations []Violation
}

func (c *sourceChecker) add(rule Rule, node ast.Node, message string) {
	line := 0
	if node != nil {
		line = c.fset.Position(node.Pos()).Line
	}
	c.violations = append(c.violations, Violation{Rule: rule, Path: c.path, Line: line, Message: message})
}

func (c *sourceChecker) checkImport(spec *ast.ImportSpec) {
	importPath, err := strconv.Unquote(spec.Path.Value)
	if err != nil {
		return
	}
	name := filepath.Base(importPath)
	if spec.Name != nil && spec.Name.Name != "_" && spec.Name.Name != "." {
		name = spec.Name.Name
	}
	c.aliases[name] = importPath
	c.checkDotImport(spec, importPath)

	switch {
	case strings.Contains(importPath, "/internal/composition") || strings.Contains(importPath, "/internal/cliapp"):
		c.add(RuleCompositionImport, spec, "command modules cannot import root/composition packages")
	case strings.Contains(importPath, "/internal/adapters/"):
		c.add(RuleConcreteAdapter, spec, "command modules depend on ports, not concrete adapters")
	case strings.Contains(importPath, "/internal/trackerresolve"):
		c.add(RuleTracker, spec, "tracker resolution is an adapter boundary")
	case importPath == "os/exec" || importPath == "syscall":
		c.add(RuleProcess, spec, "process execution belongs in an adapter")
	case importPath == "io/ioutil":
		c.add(RuleFilesystem, spec, "filesystem access belongs in an adapter")
	case importPath == "net" || strings.HasPrefix(importPath, "net/http") || strings.HasPrefix(importPath, "net/rpc"):
		c.add(RuleNetwork, spec, "network access belongs in an adapter")
	}
}

func (c *sourceChecker) checkDotImport(spec *ast.ImportSpec, importPath string) {
	if spec.Name == nil || spec.Name.Name != "." {
		return
	}
	switch importPath {
	case "os", "os/exec", "syscall":
		c.add(RuleProcess, spec, "dot import can hide direct process/environment/filesystem effects")
	case "path/filepath", "io/ioutil":
		c.add(RuleFilesystem, spec, "dot import can hide direct filesystem effects")
	case "time":
		c.add(RuleClock, spec, "dot import can hide direct clock effects")
	}
}

func (c *sourceChecker) inspectNode(node ast.Node) bool {
	switch item := node.(type) {
	case *ast.TypeSpec:
		if isServiceBagName(item.Name.Name) || containsServiceBagType(item.Type) {
			c.add(RuleServiceBag, item, "shared dependency/service bags are forbidden")
		}
	case *ast.Field:
		if containsServiceBagType(item.Type) {
			c.add(RuleServiceBag, item, "shared dependency/service bags are forbidden")
		}
	case *ast.SelectorExpr:
		c.checkSelector(item)
	}
	return true
}

func (c *sourceChecker) checkSelector(item *ast.SelectorExpr) {
	ident, ok := item.X.(*ast.Ident)
	if !ok {
		return
	}
	switch c.aliases[ident.Name] {
	case "os":
		rule := osRule(item.Sel.Name)
		if rule != "" {
			c.add(rule, item, fmt.Sprintf("os.%s is a direct effect", item.Sel.Name))
		}
	case "time":
		if isClockSelector(item.Sel.Name) {
			c.add(RuleClock, item, fmt.Sprintf("time.%s requires an injected clock", item.Sel.Name))
		}
	case "path/filepath":
		if isFilesystemSelector(item.Sel.Name) {
			c.add(RuleFilesystem, item, fmt.Sprintf("filepath.%s reads the filesystem", item.Sel.Name))
		}
	}
}

func containsServiceBagType(expression ast.Expr) bool {
	found := false
	ast.Inspect(expression, func(node ast.Node) bool {
		ident, ok := node.(*ast.Ident)
		if ok && isServiceBagName(ident.Name) {
			found = true
			return false
		}
		return !found
	})
	return found
}

func isCommandPath(path string) bool {
	path = filepath.ToSlash(path)
	return strings.Contains(path, "/internal/commands/") || strings.HasPrefix(path, "cli/internal/commands/")
}

func isServiceBagName(name string) bool {
	switch name {
	case "Dependencies", "DependencyBag", "ServiceBag", "Services", "App":
		return true
	default:
		return false
	}
}

func osRule(selector string) Rule {
	switch selector {
	case "Getenv", "LookupEnv", "Environ", "Setenv", "Unsetenv", "ExpandEnv", "Clearenv", "Args", "Hostname", "UserHomeDir", "Executable", "Getuid", "Geteuid", "Getgid", "Getegid", "Getgroups":
		return RuleEnvironment
	case "StartProcess", "FindProcess", "Exit", "Interrupt", "Kill", "Stdout", "Stderr", "Stdin", "Pipe":
		return RuleProcess
	case "Open", "OpenFile", "Create", "CreateTemp", "ReadFile", "WriteFile", "ReadDir", "Mkdir", "MkdirAll", "MkdirTemp", "Remove", "RemoveAll", "Rename", "Stat", "Lstat", "Chmod", "Chown", "Lchown", "Chtimes", "Truncate", "Symlink", "Link", "Readlink", "Getwd", "Chdir", "TempDir":
		return RuleFilesystem
	default:
		return ""
	}
}

func isFilesystemSelector(selector string) bool {
	switch selector {
	case "EvalSymlinks", "Glob", "Walk", "WalkDir":
		return true
	default:
		return false
	}
}

func isClockSelector(selector string) bool {
	switch selector {
	case "Now", "Since", "Until", "Sleep", "After", "AfterFunc", "NewTimer", "NewTicker", "Tick":
		return true
	default:
		return false
	}
}

func dedupe(in []Violation) []Violation {
	seen := map[string]bool{}
	out := make([]Violation, 0, len(in))
	for _, violation := range in {
		key := fmt.Sprintf("%s:%d:%s", violation.Path, violation.Line, violation.Rule)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, violation)
	}
	return out
}

func sortViolations(violations []Violation) {
	sort.Slice(violations, func(i, j int) bool {
		if violations[i].Path != violations[j].Path {
			return violations[i].Path < violations[j].Path
		}
		if violations[i].Line != violations[j].Line {
			return violations[i].Line < violations[j].Line
		}
		return violations[i].Rule < violations[j].Rule
	})
}
