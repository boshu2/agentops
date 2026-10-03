// Package skillshealth audits the skills/ tree.
//
// It validates each skill's YAML frontmatter (name + description present,
// name matches the directory) and verifies that every references/*.md file is
// linked from SKILL.md.
//
// The audit is read-only: it never mutates skills/.
package skillshealth

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Report is the top-level audit result.
type Report struct {
	Skills    []SkillStatus `json:"skills"`
	Errors    []string      `json:"errors"`
	Generated string        `json:"generated_at"`
}

// SkillStatus captures per-skill audit state.
type SkillStatus struct {
	Name               string   `json:"name"`
	Path               string   `json:"path"`
	FrontmatterValid   bool     `json:"frontmatter_valid"`
	MissingFrontmatter []string `json:"missing_frontmatter,omitempty"`
	BrokenRefs         []string `json:"broken_refs,omitempty"`
}

// Options controls Audit behaviour.
type Options struct {
	SkillsDir string
	OnlySkill string
	Strict    bool
}

// referenceLinkPattern preserves explicit ./ and ../ targets, including
// sibling skill paths and angle-bracketed links. Bare references/<name>.md
// mentions (also used inside repo-qualified shell examples) remain local refs.
var referenceLinkPattern = regexp.MustCompile(`((?:\.\.?/[A-Za-z0-9_./-]*)?references/[A-Za-z0-9_./-]+\.md)`)

// Audit walks SkillsDir and produces a Report.
func Audit(opts Options) (*Report, error) {
	if strings.TrimSpace(opts.SkillsDir) == "" {
		opts.SkillsDir = "skills"
	}

	report := &Report{
		Skills:    []SkillStatus{},
		Errors:    []string{},
		Generated: time.Now().UTC().Format(time.RFC3339),
	}

	entries, err := os.ReadDir(opts.SkillsDir)
	if err != nil {
		return nil, fmt.Errorf("read skills dir %s: %w", opts.SkillsDir, err)
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		// Skip files (e.g., SKILL-TIERS.md) at the top level. Only walk dirs.
		if !e.IsDir() || strings.HasPrefix(e.Name(), "_") {
			continue
		}
		name := e.Name()
		if opts.OnlySkill != "" && name != opts.OnlySkill {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		status := auditOneSkill(opts.SkillsDir, name)
		report.Skills = append(report.Skills, status)

		if !status.FrontmatterValid {
			report.Errors = append(report.Errors,
				fmt.Sprintf("%s: missing frontmatter fields: %s",
					name, strings.Join(status.MissingFrontmatter, ", ")))
		}
		for _, br := range status.BrokenRefs {
			report.Errors = append(report.Errors,
				fmt.Sprintf("%s: broken reference: %s", name, br))
		}
	}

	return report, nil
}

func auditOneSkill(skillsDir, name string) SkillStatus {
	skillPath := filepath.Join(skillsDir, name, "SKILL.md")
	status := SkillStatus{
		Name: name,
		Path: skillPath,
	}

	data, err := os.ReadFile(skillPath)
	if err != nil {
		// No SKILL.md at all -> treat as missing both required fields.
		status.MissingFrontmatter = []string{"name", "description"}
		status.FrontmatterValid = false
		return status
	}
	body := string(data)

	fm := ParseFrontmatter(body)
	missing := ValidateFrontmatter(fm, name)
	status.MissingFrontmatter = missing
	status.FrontmatterValid = len(missing) == 0

	// Broken-references check: every references/*.md that exists on disk
	// must be linked from SKILL.md, and every link in SKILL.md must point
	// at a file that exists.
	skillDir := filepath.Join(skillsDir, name)
	status.BrokenRefs = findBrokenRefs(skillDir, body)
	return status
}

// ParseFrontmatter extracts the YAML frontmatter block delimited by leading
// `---` lines. It is intentionally line-based (no full YAML parser) because
// SKILL.md frontmatter is conventionally flat key:value with simple lists.
//
// Returns a map of top-level keys; nested values are stored as the raw
// remainder of the line. If there is no leading `---`, returns an empty map.
func ParseFrontmatter(content string) map[string]string {
	out := map[string]string{}
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return out
	}
	// Locate the opening fence: first non-empty line must be "---".
	start := -1
	for i, ln := range lines {
		t := strings.TrimSpace(ln)
		if t == "" {
			continue
		}
		if t == "---" {
			start = i
		}
		break
	}
	if start < 0 {
		return out
	}
	// Locate the closing fence.
	end := -1
	for i := start + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return out
	}
	// Track indentation: only top-level keys (zero leading spaces) count.
	keyPattern := regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_-]*)\s*:\s*(.*)$`)
	for i := start + 1; i < end; i++ {
		raw := lines[i]
		// Skip indented (nested) lines and comments.
		if len(raw) > 0 && (raw[0] == ' ' || raw[0] == '\t') {
			continue
		}
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		m := keyPattern.FindStringSubmatch(raw)
		if m == nil {
			continue
		}
		key := m[1]
		val := strings.TrimSpace(m[2])
		// Strip simple surrounding quotes.
		val = strings.Trim(val, `"'`)
		out[key] = val
	}
	return out
}

// ValidateFrontmatter returns the list of missing required fields. Required:
// name (must equal dirName) and description (non-empty).
func ValidateFrontmatter(fm map[string]string, dirName string) []string {
	var missing []string
	name := strings.TrimSpace(fm["name"])
	if name == "" {
		missing = append(missing, "name")
	} else if name != dirName {
		missing = append(missing, "name (mismatch: got "+name+", want "+dirName+")")
	}
	if strings.TrimSpace(fm["description"]) == "" {
		missing = append(missing, "description")
	}
	return missing
}

// findBrokenRefs returns reference paths that are linked in body but missing
// on disk, plus references/*.md files that exist on disk but are not linked
// from body.
func findBrokenRefs(skillDir, body string) []string {
	var broken []string

	// Strip frontmatter region so we don't pick up YAML accidentally.
	scanBody := body
	if strings.HasPrefix(strings.TrimSpace(scanBody), "---") {
		idx := strings.Index(scanBody, "---")
		if idx >= 0 {
			rest := scanBody[idx+3:]
			if end := strings.Index(rest, "---"); end >= 0 {
				scanBody = rest[end+3:]
			}
		}
	}

	linked := map[string]bool{}
	for _, m := range referenceLinkPattern.FindAllStringSubmatch(scanBody, -1) {
		if len(m) < 2 {
			continue
		}
		ref := m[1]
		full := filepath.Join(skillDir, filepath.FromSlash(ref))
		linked[full] = true
		if _, err := os.Stat(full); err != nil {
			broken = append(broken, ref+" (linked but missing on disk)")
		}
	}

	// Walk references/ directory; flag files not linked from SKILL.md.
	refsDir := filepath.Join(skillDir, "references")
	entries, err := os.ReadDir(refsDir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				// Walk one level down for nested references; many skills
				// nest reference packs.
				sub, err := os.ReadDir(filepath.Join(refsDir, e.Name()))
				if err != nil {
					continue
				}
				for _, s := range sub {
					if s.IsDir() || filepath.Ext(s.Name()) != ".md" {
						continue
					}
					rel := e.Name() + "/" + s.Name()
					if !linked[filepath.Join(refsDir, rel)] {
						broken = append(broken, "references/"+rel+" (on disk but unlinked)")
					}
				}
				continue
			}
			if filepath.Ext(e.Name()) != ".md" {
				continue
			}
			if !linked[filepath.Join(refsDir, e.Name())] {
				broken = append(broken, "references/"+e.Name()+" (on disk but unlinked)")
			}
		}
	}

	sort.Strings(broken)
	return broken
}
