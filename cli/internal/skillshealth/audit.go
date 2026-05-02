// Package skillshealth audits the skills/ tree and its codex parity sibling.
//
// It validates each skill's YAML frontmatter (name + description present,
// name matches the directory), verifies that every references/*.md file is
// linked from SKILL.md, and reports parity drift against skills-codex/.
//
// The audit is read-only: it never mutates skills/ or skills-codex/.
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
	Skills      []SkillStatus `json:"skills"`
	Errors      []string      `json:"errors"`
	ParityDrift []string      `json:"parity_drift"`
	Generated   string        `json:"generated_at"`
}

// SkillStatus captures per-skill audit state.
type SkillStatus struct {
	Name               string   `json:"name"`
	Path               string   `json:"path"`
	FrontmatterValid   bool     `json:"frontmatter_valid"`
	MissingFrontmatter []string `json:"missing_frontmatter,omitempty"`
	BrokenRefs         []string `json:"broken_refs,omitempty"`
	CodexParity        string   `json:"codex_parity"` // "matched" | "missing" | "diverged" | "n/a"
}

// Options controls Audit behaviour.
type Options struct {
	SkillsDir, CodexDir string
	OnlySkill           string
	Strict              bool
}

// referenceLinkPattern matches markdown references to references/<name>.md
// (with or without leading paths or angle-bracketed link forms).
var referenceLinkPattern = regexp.MustCompile(`references/([A-Za-z0-9_./-]+\.md)`)

// Audit walks SkillsDir and CodexDir and produces a Report.
func Audit(opts Options) (*Report, error) {
	if strings.TrimSpace(opts.SkillsDir) == "" {
		opts.SkillsDir = "skills"
	}
	if strings.TrimSpace(opts.CodexDir) == "" {
		opts.CodexDir = "skills-codex"
	}

	report := &Report{
		Skills:      []SkillStatus{},
		Errors:      []string{},
		ParityDrift: []string{},
		Generated:   time.Now().UTC().Format(time.RFC3339),
	}

	entries, err := os.ReadDir(opts.SkillsDir)
	if err != nil {
		return nil, fmt.Errorf("read skills dir %s: %w", opts.SkillsDir, err)
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		// Skip files (e.g., SKILL-TIERS.md) at the top level. Only walk dirs.
		if !e.IsDir() {
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
		status := auditOneSkill(opts.SkillsDir, opts.CodexDir, name)
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
		if status.CodexParity == "missing" || status.CodexParity == "diverged" {
			report.ParityDrift = append(report.ParityDrift,
				fmt.Sprintf("%s: %s", name, status.CodexParity))
		}
	}

	return report, nil
}

func auditOneSkill(skillsDir, codexDir, name string) SkillStatus {
	skillPath := filepath.Join(skillsDir, name, "SKILL.md")
	status := SkillStatus{
		Name:        name,
		Path:        skillPath,
		CodexParity: "n/a",
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

	// Codex parity.
	status.CodexParity = compareCodexParity(codexDir, name, fm["description"])
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
		linked[ref] = true
		full := filepath.Join(skillDir, "references", ref)
		if _, err := os.Stat(full); err != nil {
			broken = append(broken, "references/"+ref+" (linked but missing on disk)")
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
					if !linked[rel] {
						broken = append(broken, "references/"+rel+" (on disk but unlinked)")
					}
				}
				continue
			}
			if filepath.Ext(e.Name()) != ".md" {
				continue
			}
			if !linked[e.Name()] {
				broken = append(broken, "references/"+e.Name()+" (on disk but unlinked)")
			}
		}
	}

	sort.Strings(broken)
	return broken
}

// compareCodexParity returns "matched", "missing", or "diverged" based on
// presence and description-similarity of the codex sibling.
func compareCodexParity(codexDir, name, sourceDesc string) string {
	codexPath := filepath.Join(codexDir, name, "SKILL.md")
	data, err := os.ReadFile(codexPath)
	if err != nil {
		return "missing"
	}
	codexFM := ParseFrontmatter(string(data))
	codexDesc := strings.TrimSpace(codexFM["description"])
	srcDesc := strings.TrimSpace(sourceDesc)
	// Empty descriptions on either side: cannot compare meaningfully.
	if srcDesc == "" || codexDesc == "" {
		if srcDesc == "" && codexDesc == "" {
			return "matched"
		}
		return "diverged"
	}
	if descriptionsClose(srcDesc, codexDesc) {
		return "matched"
	}
	return "diverged"
}

// descriptionsClose returns true if two descriptions are likely the same
// intent. We consider them close when one is a prefix of the other (modulo
// whitespace/punctuation) or they share most content tokens. The codex
// converter may rewrap text or substitute Codex-specific tool names, so
// strict equality is too brittle for parity drift detection.
func descriptionsClose(a, b string) bool {
	la, lb := strings.ToLower(a), strings.ToLower(b)
	if la == lb {
		return true
	}
	// Normalize whitespace and punctuation.
	norm := func(s string) string {
		s = strings.ToLower(s)
		var sb strings.Builder
		prevSpace := false
		for _, r := range s {
			if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
				sb.WriteRune(r)
				prevSpace = false
			} else if !prevSpace {
				sb.WriteRune(' ')
				prevSpace = true
			}
		}
		return strings.TrimSpace(sb.String())
	}
	na, nb := norm(la), norm(lb)
	if na == nb {
		return true
	}
	if strings.HasPrefix(na, nb) || strings.HasPrefix(nb, na) {
		return true
	}
	// Token overlap: >=60% of the shorter side's tokens appear in the longer.
	ta := strings.Fields(na)
	tb := strings.Fields(nb)
	if len(ta) == 0 || len(tb) == 0 {
		return false
	}
	short, long := ta, tb
	if len(tb) < len(ta) {
		short, long = tb, ta
	}
	longSet := map[string]bool{}
	for _, t := range long {
		longSet[t] = true
	}
	hits := 0
	for _, t := range short {
		if longSet[t] {
			hits++
		}
	}
	return float64(hits)/float64(len(short)) >= 0.60
}
