package skillshealth

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// SourceFinding is a located source-package defect, not a semantic verdict.
type SourceFinding struct{ Code, Path, Message string }

var sourceSlug = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// CheckSource checks explicit canonical targets without reading outside their package.
// Scaffolds may be projected during creation, but ordinary checks flag their state.
func CheckSource(repo string, targets []string, allowScaffold bool) ([]SourceFinding, error) {
	root, err := filepath.Abs(repo)
	if err != nil {
		return nil, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	var findings []SourceFinding
	for _, target := range targets {
		path := target
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, path)
		}
		if err := sourceTarget(root, target, path); err != nil {
			return nil, err
		}
		rel := "skills/" + filepath.Base(path)
		add := func(code, msg string) { findings = append(findings, SourceFinding{code, rel, msg}) }
		skill := filepath.Join(path, "SKILL.md")
		fileInfo, e := os.Lstat(skill)
		if e != nil || !fileInfo.Mode().IsRegular() {
			add("MISSING_SKILL", "SKILL.md must be a regular source file")
			continue
		}
		content, e := os.ReadFile(skill)
		if e != nil {
			return nil, e
		}
		parts := strings.SplitN(string(content), "---", 3)
		if len(parts) != 3 || strings.TrimSpace(parts[0]) != "" {
			add("INVALID_FRONTMATTER", "leading YAML frontmatter is missing")
			continue
		}
		var data map[string]any
		if e = yaml.Unmarshal([]byte(parts[1]), &data); e != nil || data == nil {
			add("INVALID_FRONTMATTER", "frontmatter must be a YAML mapping")
			continue
		}
		name, _ := data["name"].(string)
		if name != filepath.Base(path) || !sourceSlug.MatchString(name) {
			add("NAME_MISMATCH", "name must match the lowercase-hyphen directory slug")
		}
		desc, _ := data["description"].(string)
		if strings.TrimSpace(desc) == "" {
			add("MISSING_DESC", "description must be nonempty")
		}
		if data["skill_api_version"] != 1 {
			add("MISSING_API_VERSION", "skill_api_version must be 1")
		}
		meta, _ := data["metadata"].(map[string]any)
		disposition, _ := meta["disposition"].(string)
		if disposition == "" {
			add("MISSING_DISPOSITION", "metadata.disposition must be nonempty")
		}
		if !allowScaffold && meta["authoring_state"] == "scaffold" {
			add("INCOMPLETE_SCAFFOLD", "replace the scaffold with actual behavior and remove metadata.authoring_state; semantic review remains required")
		}
		for _, target := range sourcePackageResources(parts[2]) {
			resource := filepath.Join(path, target)
			actual, e := filepath.EvalSymlinks(resource)
			if e != nil {
				add("DEAD_REF", "missing "+target)
				continue
			}
			if actual != resource || !strings.HasPrefix(actual, path+string(filepath.Separator)) {
				add("UNSAFE_REF", "resource escapes package or uses symlink: "+target)
			}
		}
	}
	return findings, nil
}

func sourceTarget(root, target, path string) error {
	// Refuse traversal spelling before normalization, including an in-tree escape and return.
	for _, part := range strings.Split(filepath.ToSlash(target), "/") {
		if part == ".." {
			return fmt.Errorf("target traversal is not accepted: %s", target)
		}
	}
	info, e := os.Lstat(path)
	if e != nil {
		return fmt.Errorf("target does not exist: %s", target)
	}
	resolved, e := filepath.EvalSymlinks(path)
	if e != nil || info.Mode()&os.ModeSymlink != 0 || resolved != filepath.Clean(path) || filepath.Dir(resolved) != filepath.Join(root, "skills") || !info.IsDir() {
		return fmt.Errorf("target is not a real direct skill package: %s", target)
	}
	return nil
}

func sourcePackageResources(body string) []string {
	var out []string
	for _, ref := range markdownResources(markdownProseLines(body)) {
		target := strings.TrimPrefix(ref.target, "./")
		if !strings.HasPrefix(target, "references/") && !strings.HasPrefix(target, "scripts/") && !strings.HasPrefix(target, "assets/") {
			continue
		}
		out = append(out, strings.SplitN(strings.SplitN(target, "#", 2)[0], "?", 2)[0])
	}
	return out
}
