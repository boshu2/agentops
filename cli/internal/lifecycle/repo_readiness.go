package lifecycle

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/boshu2/agentops/cli/internal/autodev"
	"github.com/boshu2/agentops/cli/internal/goals"
	"github.com/boshu2/agentops/cli/internal/paths"
)

// ReadinessLayer names one setup layer in the AgentOps repo onboarding model.
type ReadinessLayer string

const (
	LayerCore         ReadinessLayer = "core"
	LayerGoals        ReadinessLayer = "goals"
	LayerInstructions ReadinessLayer = "instructions"
	LayerHooks        ReadinessLayer = "hooks"
	LayerTracking     ReadinessLayer = "tracking"
	LayerProduct      ReadinessLayer = "product"
	LayerReadme       ReadinessLayer = "readme"
	LayerProgram      ReadinessLayer = "program"
	LayerSchedule     ReadinessLayer = "schedule"
)

// ReadinessItem is one inspectable artifact or capability in a repo setup.
type ReadinessItem struct {
	Layer    ReadinessLayer `json:"layer"`
	Name     string         `json:"name"`
	Path     string         `json:"path,omitempty"`
	Present  bool           `json:"present"`
	Required bool           `json:"required"`
	Action   string         `json:"action,omitempty"`
	Detail   string         `json:"detail,omitempty"`
}

// ReadinessReport is the structured repo-readiness view used by setup commands.
type ReadinessReport struct {
	Root      string          `json:"root"`
	AgentsDir string          `json:"agents_dir"`
	Template  string          `json:"template"`
	DryRun    bool            `json:"dry_run"`
	Ready     bool            `json:"ready"`
	Items     []ReadinessItem `json:"items"`
}

// ReadinessOptions controls repo readiness inspection and application.
type ReadinessOptions struct {
	Template string
	Force    bool
	DryRun   bool
	Minimal  bool
	NoBeads  bool
}

// CoreAgentSubdirs are the canonical .agents subdirectories created by repo setup.
var CoreAgentSubdirs = []string{
	"briefings",
	"packets",
	"research",
	"products",
	"knowledge",
	"knowledge/pending",
	"playbooks",
	"synthesis",
	"specs",
	"retro",
	"handoff",
	"learnings",
	"patterns",
	"council",
	"plans",
	"rpi",
	"ao",
}

// CoreStorageSubdirs are the storage subdirectories under .agents/ao.
var CoreStorageSubdirs = []string{
	"ao/sessions",
	"ao/index",
	"ao/provenance",
}

// CoreAgentDirPaths returns the legacy repo-relative .agents paths used by ao init.
func CoreAgentDirPaths() []string {
	out := make([]string, 0, len(CoreAgentSubdirs))
	for _, subdir := range CoreAgentSubdirs {
		out = append(out, filepath.ToSlash(filepath.Join(".agents", subdir)))
	}
	return out
}

// ResolveReadinessPaths returns the canonical AgentOps state paths for root.
func ResolveReadinessPaths(root string) (*paths.Paths, error) {
	absRoot, err := normalizeReadinessRoot(root)
	if err != nil {
		return nil, err
	}
	return paths.ResolveFromRoot(absRoot), nil
}

// InspectRepoReadiness returns a deterministic readiness report for root.
func InspectRepoReadiness(root string, opts ReadinessOptions) (*ReadinessReport, error) {
	absRoot, err := normalizeReadinessRoot(root)
	if err != nil {
		return nil, err
	}
	statePaths := paths.ResolveFromRoot(absRoot)
	template := normalizeSeedTemplate(absRoot, opts.Template)

	report := &ReadinessReport{
		Root:      absRoot,
		AgentsDir: statePaths.AgentsDir,
		Template:  template,
		DryRun:    opts.DryRun,
		Ready:     true,
	}

	addRequired := func(item ReadinessItem) {
		item.Required = true
		if !item.Present {
			report.Ready = false
		}
		report.Items = append(report.Items, item)
	}
	addOptional := func(item ReadinessItem) {
		item.Required = false
		report.Items = append(report.Items, item)
	}

	for _, subdir := range CoreAgentSubdirs {
		path := filepath.Join(statePaths.AgentsDir, subdir)
		addRequired(ReadinessItem{
			Layer:   LayerCore,
			Name:    filepath.ToSlash(filepath.Join(".agents", subdir)),
			Path:    path,
			Present: isDir(path),
			Action:  "create directory",
		})
	}
	for _, subdir := range CoreStorageSubdirs {
		path := filepath.Join(statePaths.AgentsDir, subdir)
		addRequired(ReadinessItem{
			Layer:   LayerCore,
			Name:    filepath.ToSlash(filepath.Join(".agents", subdir)),
			Path:    path,
			Present: isDir(path),
			Action:  "create storage directory",
		})
	}

	nestedGitignore := filepath.Join(statePaths.AgentsDir, ".gitignore")
	addRequired(ReadinessItem{
		Layer:   LayerCore,
		Name:    ".agents/.gitignore",
		Path:    nestedGitignore,
		Present: isFile(nestedGitignore),
		Action:  "create deny-all nested gitignore",
	})

	goalsPath, goalsPresent := existingFirstFile(absRoot, "GOALS.md", "GOALS.yaml")
	if goalsPath == "" {
		goalsPath = filepath.Join(absRoot, "GOALS.md")
	}
	addRequired(ReadinessItem{
		Layer:   LayerGoals,
		Name:    filepath.Base(goalsPath),
		Path:    goalsPath,
		Present: goalsPresent,
		Action:  "create GOALS.md",
		Detail:  fmt.Sprintf("template=%s", template),
	})

	claudePath := filepath.Join(absRoot, "CLAUDE.md")
	claudePresent := false
	claudeAction := "create CLAUDE.md with AgentOps section"
	if data, err := os.ReadFile(claudePath); err == nil {
		claudePresent = HasSeedMarker(string(data))
		claudeAction = "append AgentOps section to CLAUDE.md"
	}
	addRequired(ReadinessItem{
		Layer:   LayerInstructions,
		Name:    "CLAUDE.md AgentOps section",
		Path:    claudePath,
		Present: claudePresent,
		Action:  claudeAction,
	})

	addOptional(ReadinessItem{
		Layer:   LayerTracking,
		Name:    "beads tracker",
		Path:    filepath.Join(absRoot, ".beads"),
		Present: isDir(filepath.Join(absRoot, ".beads")),
		Action:  "bd init --prefix <prefix>",
	})
	addOptional(ReadinessItem{
		Layer:   LayerHooks,
		Name:    "session hooks",
		Present: false,
		Action:  "ao init --hooks",
	})
	addOptional(ReadinessItem{
		Layer:   LayerProduct,
		Name:    "PRODUCT.md",
		Path:    filepath.Join(absRoot, "PRODUCT.md"),
		Present: isFile(filepath.Join(absRoot, "PRODUCT.md")),
		Action:  "$product",
	})
	addOptional(ReadinessItem{
		Layer:   LayerReadme,
		Name:    "README.md",
		Path:    filepath.Join(absRoot, "README.md"),
		Present: isFile(filepath.Join(absRoot, "README.md")),
		Action:  "$readme",
	})

	programRel := autodev.ResolveProgramPath(absRoot)
	programPath := filepath.Join(absRoot, "PROGRAM.md")
	programPresent := false
	if programRel != "" {
		programPath = filepath.Join(absRoot, programRel)
		programPresent = true
	}
	addOptional(ReadinessItem{
		Layer:   LayerProgram,
		Name:    "PROGRAM.md or AUTODEV.md",
		Path:    programPath,
		Present: programPresent,
		Action:  `ao autodev init "your objective"`,
	})
	addOptional(ReadinessItem{
		Layer:   LayerSchedule,
		Name:    ".agents/schedule.yaml",
		Path:    filepath.Join(statePaths.AgentsDir, "schedule.yaml"),
		Present: isFile(filepath.Join(statePaths.AgentsDir, "schedule.yaml")),
		Action:  "ao init --with-schedule",
	})

	return report, nil
}

// PlanRepoSeed returns the readiness report that would be applied.
func PlanRepoSeed(root string, opts ReadinessOptions) (*ReadinessReport, error) {
	opts.DryRun = true
	return InspectRepoReadiness(root, opts)
}

// ApplyRepoSeed creates the required core, goals, and instruction artifacts.
func ApplyRepoSeed(root string, opts ReadinessOptions) (*ReadinessReport, error) {
	absRoot, err := normalizeReadinessRoot(root)
	if err != nil {
		return nil, err
	}
	if opts.DryRun {
		return PlanRepoSeed(absRoot, opts)
	}

	statePaths := paths.ResolveFromRoot(absRoot)
	for _, subdir := range append(append([]string{}, CoreAgentSubdirs...), CoreStorageSubdirs...) {
		if err := os.MkdirAll(filepath.Join(statePaths.AgentsDir, subdir), 0o700); err != nil {
			return nil, fmt.Errorf("create %s: %w", subdir, err)
		}
	}
	if err := ensureNestedGitignoreFile(filepath.Join(statePaths.AgentsDir, ".gitignore")); err != nil {
		return nil, err
	}
	if err := ensureSeedGoals(absRoot, normalizeSeedTemplate(absRoot, opts.Template), opts.Force); err != nil {
		return nil, err
	}
	if err := ensureSeedInstructions(absRoot, opts.Force); err != nil {
		return nil, err
	}
	return InspectRepoReadiness(absRoot, opts)
}

// DetectSeedTemplate inspects project files to determine the seed template.
func DetectSeedTemplate(root string) string {
	stat := func(rel string) bool {
		info, err := os.Stat(filepath.Join(root, rel))
		return err == nil && !info.IsDir()
	}
	switch {
	case stat("go.mod") || stat("cli/go.mod"):
		return "go-cli"
	case stat("package.json"):
		return "web-app"
	case stat("pyproject.toml"):
		return "python-lib"
	case stat("Cargo.toml"):
		return "rust-cli"
	default:
		return "generic"
	}
}

func normalizeSeedTemplate(root, template string) string {
	template = strings.TrimSpace(template)
	if template == "" {
		template = DetectSeedTemplate(root)
	}
	if !ValidTemplates[template] {
		return "generic"
	}
	return template
}

func normalizeReadinessRoot(root string) (string, error) {
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve root: %w", err)
	}
	info, err := os.Stat(absRoot)
	if err != nil {
		return "", fmt.Errorf("stat root: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("root is not a directory: %s", absRoot)
	}
	return absRoot, nil
}

func existingFirstFile(root string, names ...string) (string, bool) {
	for _, name := range names {
		path := filepath.Join(root, name)
		if isFile(path) {
			return path, true
		}
	}
	return "", false
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func ensureNestedGitignoreFile(path string) error {
	if isFile(path) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create %s parent: %w", path, err)
	}
	content := "# Do not commit this directory - session artifacts, absolute paths, sensitive output.\n*\n!.gitignore\n!README.md\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	return nil
}

func ensureSeedGoals(root, template string, force bool) error {
	path := filepath.Join(root, "GOALS.md")
	if !force && (isFile(path) || isFile(filepath.Join(root, "GOALS.yaml"))) {
		return nil
	}
	gf := BuildSeedGoalFile(root, template)
	gf.Goals = append(gf.Goals, goals.DetectGates(root)...)
	if err := os.WriteFile(path, []byte(goals.RenderGoalsMD(gf)), 0o600); err != nil {
		return fmt.Errorf("write GOALS.md: %w", err)
	}
	return nil
}

func ensureSeedInstructions(root string, force bool) error {
	path := filepath.Join(root, "CLAUDE.md")
	if data, err := os.ReadFile(path); err == nil {
		if HasSeedMarker(string(data)) && !force {
			return nil
		}
		content := string(data)
		if !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += ClaudeMDSeedSection
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			return fmt.Errorf("update CLAUDE.md: %w", err)
		}
		return nil
	}

	header := fmt.Sprintf("# %s\n", filepath.Base(root))
	if err := os.WriteFile(path, []byte(header+ClaudeMDSeedSection), 0o600); err != nil {
		return fmt.Errorf("create CLAUDE.md: %w", err)
	}
	return nil
}
