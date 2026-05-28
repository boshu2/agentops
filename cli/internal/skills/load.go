package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Load reads every skills/<name>/SKILL.md under skillsDir, parses its
// frontmatter, and returns the catalog as SkillMeta sorted by name. Directories
// without a readable SKILL.md are skipped. There is no static index file — the
// catalog is derived from the tree on every call, so a newly added skill is
// discovered on the next invocation.
func Load(skillsDir string) ([]SkillMeta, error) {
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return nil, fmt.Errorf("read skills dir %s: %w", skillsDir, err)
	}
	metas := make([]SkillMeta, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(skillsDir, e.Name(), "SKILL.md")
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		meta := parseSkillMeta(string(data))
		meta.Path = path
		if meta.Name == "" {
			meta.Name = e.Name()
		}
		metas = append(metas, meta)
	}
	sort.Slice(metas, func(i, j int) bool { return metas[i].Name < metas[j].Name })
	return metas, nil
}

// parseSkillMeta extracts name, description, and best-effort triggers from a
// SKILL.md frontmatter block. It is line-based rather than a full YAML parser
// because SKILL.md frontmatter is conventionally flat key:value with simple
// lists; triggers may appear top-level or nested under metadata, as a block
// list or an inline [a, b] list.
func parseSkillMeta(content string) SkillMeta {
	lines := frontmatterLines(content)
	var meta SkillMeta
	for i := 0; i < len(lines); i++ {
		raw := lines[i]
		trimmed := strings.TrimSpace(raw)
		switch {
		case indentOf(raw) == 0 && strings.HasPrefix(trimmed, "name:"):
			meta.Name = unquoteScalar(strings.TrimPrefix(trimmed, "name:"))
		case indentOf(raw) == 0 && strings.HasPrefix(trimmed, "description:"):
			meta.Description = unquoteScalar(strings.TrimPrefix(trimmed, "description:"))
		case trimmed == "triggers:" || strings.HasPrefix(trimmed, "triggers:"):
			val := strings.TrimSpace(strings.TrimPrefix(trimmed, "triggers:"))
			if val != "" {
				meta.Triggers = append(meta.Triggers, parseInlineList(val)...)
			} else {
				meta.Triggers = append(meta.Triggers, collectBlockListItems(lines, i+1, indentOf(raw))...)
			}
		}
	}
	return meta
}

// frontmatterLines returns the lines inside the leading `---` fenced YAML block,
// or nil if there is no frontmatter.
func frontmatterLines(content string) []string {
	lines := strings.Split(content, "\n")
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
		return nil
	}
	for i := start + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return lines[start+1 : i]
		}
	}
	return nil
}

// indentOf returns the count of leading space/tab runes.
func indentOf(s string) int {
	n := 0
	for _, r := range s {
		if r != ' ' && r != '\t' {
			break
		}
		n++
	}
	return n
}

// unquoteScalar trims surrounding whitespace and a single layer of matching
// quotes from a YAML scalar value.
func unquoteScalar(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// parseInlineList parses a YAML inline list "[a, b, c]" into its items. A value
// that is not bracketed is treated as a single scalar item.
func parseInlineList(val string) []string {
	val = strings.TrimSpace(val)
	if strings.HasPrefix(val, "[") && strings.HasSuffix(val, "]") {
		val = val[1 : len(val)-1]
	}
	var out []string
	for _, part := range strings.Split(val, ",") {
		if item := unquoteScalar(part); item != "" {
			out = append(out, item)
		}
	}
	return out
}

// collectBlockListItems reads consecutive YAML block-list items ("- item")
// that are indented deeper than parentIndent, starting at line index from.
func collectBlockListItems(lines []string, from, parentIndent int) []string {
	var out []string
	for i := from; i < len(lines); i++ {
		raw := lines[i]
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if !strings.HasPrefix(trimmed, "- ") {
			break
		}
		if indentOf(raw) <= parentIndent {
			break
		}
		if item := unquoteScalar(strings.TrimPrefix(trimmed, "- ")); item != "" {
			out = append(out, item)
		}
	}
	return out
}
