package skillsapp

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/boshu2/agentops/cli/internal/evidencepath"
	"github.com/boshu2/agentops/cli/internal/skillshealth"
)

// BuildOptions controls the existing Skill Builder creation operation.
type BuildOptions struct {
	Repo, Mode, Slug, Source, Report string
	InitOnly                         bool
}

// BuildReport preserves the legacy receipt fields and explicitly names scaffold state.
type BuildReport struct {
	Mode               string   `json:"mode"`
	SkillName          string   `json:"skill_name"`
	FilesCreated       []string `json:"files_created"`
	StructureCheckPass bool     `json:"structure_check_pass"`
	SourceHint         string   `json:"source_hint,omitempty"`
	AuthoringState     string   `json:"authoring_state"`
	SemanticsEvaluated bool     `json:"semantics_evaluated"`
}

var buildSlug = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// Build creates only a blank source package; external content is never imported.
func Build(opts BuildOptions, out io.Writer) (result *BuildReport, resultErr error) {
	if !buildSlug.MatchString(opts.Slug) {
		return nil, fmt.Errorf("slug must be lowercase-hyphen: %s", opts.Slug)
	}
	root, err := filepath.Abs(opts.Repo)
	if err != nil {
		return nil, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	skillsDir := filepath.Join(root, "skills")
	info, err := os.Lstat(skillsDir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("skills must be a real directory: %s", skillsDir)
	}
	if err = buildInputs(root, skillsDir, opts); err != nil {
		return nil, err
	}
	tier, lists, err := buildMetadata(opts.Slug)
	if err != nil {
		return nil, err
	}
	target := filepath.Join(skillsDir, opts.Slug)
	if err = os.Mkdir(target, 0755); err != nil {
		return nil, fmt.Errorf("create target: %w", err)
	}
	source := fmt.Sprintf(`---
name: %s
description: 'TODO: state when this behavior applies.'
practices: []
skill_api_version: 1
hexagonal_role: supporting
consumes: []
produces: []
context_rel: []
user-invocable: true
metadata:
  tier: %s
  dependencies: %s
  capabilities: %s
  effects: %s
  canonical_status: canonical
  disposition: keep_specialist
  stability: experimental
  authoring_state: scaffold
---
# %s

TODO: State the applicable request, required inputs and limits of authority.

TODO: Describe the actual operation, its inline answer or required artifact,
how the caller knows it is done, and what happens on failure.

Replace these placeholders and remove metadata.authoring_state after authoring.
Choose headings and supporting files only when the behavior needs them.
Static checks do not establish semantic completeness or effectiveness.
`, opts.Slug, tier, lists["SKILL_DEPENDENCIES"], lists["SKILL_CAPABILITIES"], lists["SKILL_EFFECTS"], opts.Slug)
	if err = os.WriteFile(filepath.Join(target, "SKILL.md"), []byte(source), 0644); err != nil {
		return nil, err
	}
	report := &BuildReport{Mode: opts.Mode, SkillName: opts.Slug, FilesCreated: []string{"skills/" + opts.Slug + "/SKILL.md"}, SourceHint: opts.Source, AuthoringState: "scaffold"}
	var reportFile *os.File
	if opts.Report != "" {
		reportFile, err = os.OpenFile(opts.Report, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
		if err != nil {
			return report, err
		}
		defer func() {
			if closeErr := reportFile.Close(); resultErr == nil && closeErr != nil {
				resultErr = closeErr
			}
		}()
	}
	if err = writeBuildReport(reportFile, report); err != nil {
		return report, err
	}
	if !opts.InitOnly {
		findings, e := skillshealth.CheckSource(root, []string{target}, true)
		if e != nil {
			return report, e
		}
		if len(findings) > 0 {
			return report, fmt.Errorf("source structure: %v", findings)
		}
		for _, args := range [][]string{{"python3", "scripts/generate-skill-mesh.py"}, {"bash", "scripts/codex-sync.sh", "--only", opts.Slug}, {"bash", "scripts/regen-codex-hashes.sh", "--only", opts.Slug}} {
			cmd := exec.Command(args[0], args[1:]...)
			cmd.Dir = root
			cmd.Stdout = out
			cmd.Stderr = out
			if err = cmd.Run(); err != nil {
				return report, fmt.Errorf("projection incomplete: %w", err)
			}
		}
		report.StructureCheckPass = true
		if err = writeBuildReport(reportFile, report); err != nil {
			return report, err
		}
	}
	return report, nil
}

func checkBuildReport(root, path string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return fmt.Errorf("report must be an absolute external path")
	}
	parent := filepath.Dir(path)
	resolved, err := filepath.EvalSymlinks(parent)
	if err != nil || resolved != parent {
		return fmt.Errorf("report parent must exist without symlinks")
	}
	if path == root || strings.HasPrefix(path, root+string(filepath.Separator)) {
		return fmt.Errorf("report must be outside the source repository")
	}
	if _, err := evidencepath.Validate(parent, root); err != nil {
		return fmt.Errorf("report must be outside Git storage: %w", err)
	}
	if _, err = os.Lstat(path); !os.IsNotExist(err) {
		return fmt.Errorf("report destination already exists or is inaccessible")
	}
	return nil
}

func writeBuildReport(file *os.File, report *BuildReport) error {
	if file == nil {
		return nil
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if _, err = file.Seek(0, 0); err != nil {
		return err
	}
	if err = file.Truncate(0); err != nil {
		return err
	}
	if _, err = file.Write(append(data, '\n')); err != nil {
		return err
	}
	return file.Sync()
}

func buildInputs(root, skillsDir string, opts BuildOptions) error {
	var err error
	var info os.FileInfo
	switch opts.Mode {
	case "from-scratch":
		if opts.Source != "" {
			return fmt.Errorf("from-scratch takes no source")
		}
	case "from-template":
		if !buildSlug.MatchString(opts.Source) {
			return fmt.Errorf("invalid template slug")
		}
		if _, err = skillshealth.CheckSource(root, []string{"skills/" + opts.Source}, true); err != nil {
			return err
		}
		if _, err = os.Stat(filepath.Join(skillsDir, opts.Source, "SKILL.md")); err != nil {
			return err
		}
	case "absorb-external":
		info, err = os.Stat(opts.Source)
		if err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("external source must exist as a regular file")
		}
	default:
		return fmt.Errorf("unknown creation mode: %s", opts.Mode)
	}
	if opts.Report != "" {
		if err = checkBuildReport(root, opts.Report); err != nil {
			return err
		}
	}
	return nil
}

func buildMetadata(slug string) (string, map[string]string, error) {
	tier := os.Getenv("SKILL_TIER")
	if tier == "" {
		tier = "execution"
	}
	if !buildSlug.MatchString(tier) {
		return "", nil, fmt.Errorf("invalid skill tier")
	}
	lists := map[string]string{}
	for key, fallback := range map[string]string{"SKILL_DEPENDENCIES": "[]", "SKILL_CAPABILITIES": fmt.Sprintf("[%q]", strings.ReplaceAll(slug, "-", "_")), "SKILL_EFFECTS": "[]"} {
		value := os.Getenv(key)
		if value == "" {
			value = fallback
		}
		var values []string
		if err := json.Unmarshal([]byte(value), &values); err != nil || values == nil {
			return "", nil, fmt.Errorf("%s must be a JSON array of strings", key)
		}
		encoded, marshalErr := json.Marshal(values)
		if marshalErr != nil {
			return "", nil, marshalErr
		}
		lists[key] = string(encoded)
	}
	return tier, lists, nil
}
