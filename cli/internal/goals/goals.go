// Package goals manages GOALS.yaml and GOALS.md fitness specifications,
// including loading, validation, drift detection, and historical measurement snapshots.
package goals

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// GoalType categorizes goals by domain.
type GoalType string

const (
	GoalTypeHealth       GoalType = "health"
	GoalTypeArchitecture GoalType = "architecture"
	GoalTypeQuality      GoalType = "quality"
	GoalTypeMeta         GoalType = "meta"
)

// ContinuousMetric defines a metric and threshold for continuous evaluation.
type ContinuousMetric struct {
	Metric    string  `yaml:"metric"`
	Threshold float64 `yaml:"threshold"`
}

// Goal represents a single goal entry.
type Goal struct {
	ID          string            `yaml:"id"`
	Description string            `yaml:"description"`
	Check       string            `yaml:"check"`
	Weight      int               `yaml:"weight"`
	Type        GoalType          `yaml:"type,omitempty"`
	Pillar      string            `yaml:"pillar,omitempty"`
	Continuous  *ContinuousMetric `yaml:"continuous,omitempty"`
	Tags        []string          `yaml:"tags,omitempty"`
	// AffectsFiles is the canonical fix-files glob list for this goal.
	// Populated from the goals-affects-files.yaml sidecar (kept separate so
	// GOALS.md tables stay readable). The nightly routine cross-references
	// this against open-PR file diffs to compute the open-PR blocker matrix
	// and skip goals another in-flight PR already owns.
	AffectsFiles []string `yaml:"affects_files,omitempty" json:"affects_files,omitempty"`
}

// Directive represents a strategic intent directive (GOALS.md only).
type Directive struct {
	Number      int    `json:"number" yaml:"-"`
	Title       string `json:"title" yaml:"-"`
	Description string `json:"description" yaml:"-"`
	Steer       string `json:"steer" yaml:"-"`
}

// GoalFile is the top-level structure of a goals file (YAML or Markdown).
type GoalFile struct {
	Version    int         `yaml:"version" json:"version"`
	Mission    string      `yaml:"mission,omitempty" json:"mission,omitempty"`
	Goals      []Goal      `yaml:"goals" json:"goals"`
	Format     string      `yaml:"-" json:"format,omitempty"`
	NorthStars []string    `yaml:"-" json:"north_stars,omitempty"`
	AntiStars  []string    `yaml:"-" json:"anti_stars,omitempty"`
	Directives []Directive `yaml:"-" json:"directives,omitempty"`
}

// ValidationError describes a validation problem with a specific goal field.
type ValidationError struct {
	GoalID  string
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("goal %q field %q: %s", e.GoalID, e.Field, e.Message)
}

// KebabRe matches kebab-case identifiers (e.g. "my-goal-id").
var KebabRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// ValidTypes enumerates the allowed GoalType values.
var ValidTypes = map[GoalType]bool{
	GoalTypeHealth:       true,
	GoalTypeArchitecture: true,
	GoalTypeQuality:      true,
	GoalTypeMeta:         true,
}

// defaultGoalTypes sets the default Type to GoalTypeHealth for any goal that has none.
func defaultGoalTypes(goals []Goal) {
	for i := range goals {
		if goals[i].Type == "" {
			goals[i].Type = GoalTypeHealth
		}
	}
}

// DetectFormat determines the goals file format from path and filesystem context.
// Returns "md" for markdown files, "yaml" for YAML files.
// If the path is "GOALS.yaml" (default) and a GOALS.md exists in the same directory,
// returns "md" to prefer the markdown format.
func DetectFormat(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".md" {
		return "md"
	}
	if ext == ".yaml" || ext == ".yml" {
		// Check if a GOALS.md exists alongside the YAML file
		dir := filepath.Dir(path)
		mdPath := filepath.Join(dir, "GOALS.md")
		if _, err := os.Stat(mdPath); err == nil {
			return "md"
		}
		return "yaml"
	}
	// No extension or unknown — check for GOALS.md in same dir
	dir := filepath.Dir(path)
	mdPath := filepath.Join(dir, "GOALS.md")
	if _, err := os.Stat(mdPath); err == nil {
		return "md"
	}
	return "yaml"
}

// ResolveGoalsPath returns the actual file path based on format detection.
// If the detected format is "md" but the given path is not a .md file,
// it returns the GOALS.md path in the same directory.
func ResolveGoalsPath(path string) string {
	format := DetectFormat(path)
	if format == "md" && strings.ToLower(filepath.Ext(path)) != ".md" {
		return filepath.Join(filepath.Dir(path), "GOALS.md")
	}
	return path
}

// LoadGoals reads and parses a goals file (YAML or Markdown).
// Accepts YAML versions 1, 2, or 3. Markdown files use version 4.
// Version 1 is accepted with a deprecation warning.
// Defaults Goal.Type to "health" if empty.
func LoadGoals(path string) (*GoalFile, error) {
	format := DetectFormat(path)
	if format == "md" {
		actualPath := ResolveGoalsPath(path)
		data, err := os.ReadFile(actualPath)
		if err != nil {
			return nil, err
		}
		gf, err := ParseMarkdownGoals(data)
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", actualPath, err)
		}
		defaultGoalTypes(gf.Goals)
		mergeAffectsFilesSidecar(gf, sidecarPathFor(actualPath))
		return gf, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var gf GoalFile
	if err := yaml.Unmarshal(data, &gf); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	if gf.Version < 1 || gf.Version > 3 {
		return nil, fmt.Errorf("unsupported version %d (expected 1, 2, or 3)", gf.Version)
	}

	if gf.Version == 1 {
		fmt.Fprintf(os.Stderr, "WARNING: %s uses version 1 (deprecated). Run 'ao goals migrate' to upgrade to version 2.\n", path)
	}

	gf.Format = "yaml"
	defaultGoalTypes(gf.Goals)
	mergeAffectsFilesSidecar(&gf, sidecarPathFor(path))
	return &gf, nil
}

// sidecarPathFor returns the conventional location of the affects-files
// sidecar (goals-affects-files.yaml) given a goals file path. Defaults to a
// sibling of the goals file in the same directory.
func sidecarPathFor(goalsPath string) string {
	return filepath.Join(filepath.Dir(goalsPath), "goals-affects-files.yaml")
}

// affectsFilesSidecar mirrors the on-disk YAML schema:
//
//	affects_files:
//	  <goal_id>:
//	    - <glob>
type affectsFilesSidecar struct {
	AffectsFiles map[string][]string `yaml:"affects_files"`
}

// mergeAffectsFilesSidecar reads the sidecar (if it exists) and populates
// AffectsFiles on each matching Goal. Unknown goal IDs in the sidecar are
// silently ignored so the sidecar can be edited ahead of removing a goal.
// A missing or unreadable sidecar is not an error: the field stays empty
// and gates parse identically to the pre-sidecar behavior.
func mergeAffectsFilesSidecar(gf *GoalFile, path string) {
	if gf == nil || path == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var sc affectsFilesSidecar
	if err := yaml.Unmarshal(data, &sc); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: parsing %s: %v\n", path, err)
		return
	}
	if len(sc.AffectsFiles) == 0 {
		return
	}
	for i := range gf.Goals {
		if globs, ok := sc.AffectsFiles[gf.Goals[i].ID]; ok && len(globs) > 0 {
			gf.Goals[i].AffectsFiles = append([]string(nil), globs...)
		}
	}
}

// MigrateV1ToV2 converts a v1 GoalFile to v2 by adding default fields.
func MigrateV1ToV2(gf *GoalFile) {
	gf.Version = 2
	if gf.Mission == "" {
		gf.Mission = "Project fitness goals"
	}
	defaultGoalTypes(gf.Goals)
}

// ValidateGoals checks a GoalFile for structural correctness.
// Returns an empty slice if all goals are valid.
func ValidateGoals(gf *GoalFile) []ValidationError {
	var errs []ValidationError
	seen := make(map[string]bool)
	for _, g := range gf.Goals {
		errs = append(errs, validateGoalID(g, seen)...)
		errs = append(errs, validateGoalFields(g)...)
	}
	return errs
}

// validateGoalID checks ID presence, uniqueness, and format.
func validateGoalID(g Goal, seen map[string]bool) []ValidationError {
	var errs []ValidationError
	if g.ID == "" {
		return append(errs, ValidationError{GoalID: g.ID, Field: "id", Message: "required"})
	}
	if seen[g.ID] {
		errs = append(errs, ValidationError{GoalID: g.ID, Field: "id", Message: "duplicate"})
	}
	seen[g.ID] = true
	if !KebabRe.MatchString(g.ID) {
		errs = append(errs, ValidationError{GoalID: g.ID, Field: "id", Message: "must be kebab-case"})
	}
	return errs
}

// requireField appends a "required" validation error if value is empty.
func requireField(errs *[]ValidationError, goalID, field, value string) {
	if value == "" {
		*errs = append(*errs, ValidationError{GoalID: goalID, Field: field, Message: "required"})
	}
}

// validateGoalFields checks description, check, weight, and type fields.
func validateGoalFields(g Goal) []ValidationError {
	var errs []ValidationError
	requireField(&errs, g.ID, "description", g.Description)
	requireField(&errs, g.ID, "check", g.Check)
	if g.Weight < 1 || g.Weight > 10 {
		errs = append(errs, ValidationError{GoalID: g.ID, Field: "weight", Message: "must be 1-10"})
	}
	if g.Type != "" && !ValidTypes[g.Type] {
		errs = append(errs, ValidationError{GoalID: g.ID, Field: "type", Message: fmt.Sprintf("invalid type %q", g.Type)})
	}
	return errs
}
