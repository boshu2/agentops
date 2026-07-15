package goals

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// Directive-attribute keys recognized as structured executable-spec metadata.
const (
	AttrDirectiveID       = "Directive ID"
	AttrSteer             = "Steer"
	AttrSetpoint          = "Setpoint"
	AttrScenarios         = "Scenarios"
	AttrScenarioThreshold = "Scenario threshold"
	attrTags              = "Tags"
)

// directiveIDRe is the stable directive-ID format: "d-" followed by an
// alphanumeric then alphanumerics/hyphens. A stable ID is independent of a
// directive's display number.
var directiveIDRe = regexp.MustCompile(`^d-[a-z0-9][a-z0-9-]*$`)

// attrLineRe matches a "**Key:** value" directive-attribute line (trimmed).
var attrLineRe = regexp.MustCompile(`^\*\*([A-Za-z][A-Za-z0-9 ]*?):\*\*[ \t]?(.*)$`)

// nonSlugRe matches runs of characters not allowed in a slug.
var nonSlugRe = regexp.MustCompile(`[^a-z0-9]+`)

// knownAttrKeys is the allowlist of directive-attribute keys the patcher
// recognizes. Bold-prefixed prose lines that are not real metadata (e.g.
// "**Progress:**") are deliberately excluded so they are treated as body text
// and never used as insertion anchors.
var knownAttrKeys = map[string]bool{
	AttrDirectiveID:       true,
	AttrSteer:             true,
	AttrSetpoint:          true,
	AttrScenarios:         true,
	AttrScenarioThreshold: true,
	attrTags:              true,
}

// directiveAttr is one "**Key:** value" metadata line within a directive block.
type directiveAttr struct {
	key     string
	value   string
	lineIdx int // 0-based source line index
}

// ParsedDirective is a directive parsed from GOALS.md with source line ranges
// and structured attribute metadata.
//
// It is distinct from Directive (the GoalFile model): ParsedDirective carries
// the line addressing required for non-lossy patching, and the fields below
// reflect what is actually written in the file rather than a re-rendered view.
type ParsedDirective struct {
	Number            int      `json:"number"`
	Title             string   `json:"title"`
	StableID          string   `json:"directive_id,omitempty"`
	Steer             string   `json:"steer,omitempty"`
	Setpoint          string   `json:"setpoint,omitempty"`
	Scenarios         []string `json:"scenarios,omitempty"`
	ScenarioThreshold string   `json:"scenario_threshold,omitempty"`
	StartLine         int      `json:"start_line"` // 1-based line of the "### N. Title" heading
	EndLine           int      `json:"end_line"`   // 1-based last line of the block

	headingIdx int             // 0-based index of the "### N. Title" line
	endIdx     int             // 0-based exclusive end of the block
	attrs      []directiveAttr // every recognized "**Key:** value" line, in source order
}

// GoalsPatcher is a read-only parsed view over GOALS.md directive metadata.
type GoalsPatcher struct {
	lines []string
}

// NewGoalsPatcher builds a read-only view over raw GOALS.md content.
func NewGoalsPatcher(data []byte) (*GoalsPatcher, error) {
	if strings.TrimSpace(string(data)) == "" {
		return nil, fmt.Errorf("empty goals file")
	}
	return &GoalsPatcher{lines: strings.Split(string(data), "\n")}, nil
}

// LoadGoalsPatcher resolves the GOALS.md path, reads it, and returns a patcher
// plus the resolved path. It never writes either file.
func LoadGoalsPatcher(path string) (*GoalsPatcher, string, error) {
	resolved := ResolveGoalsPath(path)
	data, err := os.ReadFile(resolved)
	if err != nil {
		return nil, "", err
	}
	p, err := NewGoalsPatcher(data)
	if err != nil {
		return nil, "", fmt.Errorf("parsing %s: %w", resolved, err)
	}
	return p, resolved, nil
}

// Directives parses every directive block in the current buffer.
func (p *GoalsPatcher) Directives() []ParsedDirective {
	return directiveBlocks(p.lines)
}

// DirectiveByNumber returns the directive with the given display number.
func (p *GoalsPatcher) DirectiveByNumber(n int) (ParsedDirective, bool) {
	for _, d := range p.Directives() {
		if d.Number == n {
			return d, true
		}
	}
	return ParsedDirective{}, false
}

// DirectiveByStableID returns the directive declaring the given stable ID.
func (p *GoalsPatcher) DirectiveByStableID(id string) (ParsedDirective, bool) {
	for _, d := range p.Directives() {
		if d.StableID == id {
			return d, true
		}
	}
	return ParsedDirective{}, false
}

// ParseDirectiveBlocks parses GOALS.md content into directives with line ranges.
func ParseDirectiveBlocks(data []byte) ([]ParsedDirective, error) {
	p, err := NewGoalsPatcher(data)
	if err != nil {
		return nil, err
	}
	return p.Directives(), nil
}

// directiveSectionStart returns the 0-based index of the first line after the
// "## Directives" heading, or -1 when the section is absent.
func directiveSectionStart(lines []string) int {
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			title := strings.TrimSpace(strings.TrimPrefix(trimmed, "## "))
			if strings.EqualFold(title, "Directives") {
				return i + 1
			}
		}
	}
	return -1
}

// directiveBlocks parses every "### N. Title" block inside the Directives
// section, recording each block's source line range.
func directiveBlocks(lines []string) []ParsedDirective {
	start := directiveSectionStart(lines)
	if start < 0 {
		return nil
	}
	sectionEnd := len(lines)
	var heads []int
	for i := start; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "# ") {
			sectionEnd = i
			break
		}
		if directiveHeadingRe.MatchString(trimmed) {
			heads = append(heads, i)
		}
	}
	out := make([]ParsedDirective, 0, len(heads))
	for hi, h := range heads {
		endIdx := sectionEnd
		if hi+1 < len(heads) {
			endIdx = heads[hi+1]
		}
		out = append(out, parseDirectiveBlock(lines, h, endIdx))
	}
	return out
}

// parseDirectiveBlock parses one directive block spanning lines[h:endIdx].
func parseDirectiveBlock(lines []string, h, endIdx int) ParsedDirective {
	m := directiveHeadingRe.FindStringSubmatch(strings.TrimSpace(lines[h]))
	num, _ := strconv.Atoi(m[1])
	d := ParsedDirective{
		Number:     num,
		Title:      strings.TrimSpace(m[2]),
		headingIdx: h,
		endIdx:     endIdx,
		StartLine:  h + 1,
		EndLine:    endIdx,
	}
	for i := h + 1; i < endIdx; i++ {
		am := attrLineRe.FindStringSubmatch(strings.TrimSpace(lines[i]))
		if am == nil {
			continue
		}
		key := strings.TrimSpace(am[1])
		if !knownAttrKeys[key] {
			continue
		}
		val := strings.TrimSpace(am[2])
		d.attrs = append(d.attrs, directiveAttr{key: key, value: val, lineIdx: i})
		applyAttr(&d, key, val)
	}
	return d
}

// applyAttr copies a recognized attribute value into the structured fields.
func applyAttr(d *ParsedDirective, key, val string) {
	switch key {
	case AttrDirectiveID:
		d.StableID = val
	case AttrSteer:
		d.Steer = val
	case AttrSetpoint:
		d.Setpoint = val
	case AttrScenarios:
		d.Scenarios = splitScenarioList(val)
	case AttrScenarioThreshold:
		d.ScenarioThreshold = val
	}
}

// splitScenarioList splits a comma/semicolon-separated scenario-ID list.
func splitScenarioList(s string) []string {
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' })
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// validateAttribute checks that an attribute value is well-formed.
func validateAttribute(key, value string) error {
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("%s value must be a single line", key)
	}
	switch key {
	case AttrDirectiveID:
		if !directiveIDRe.MatchString(value) {
			return fmt.Errorf("invalid Directive ID %q (must match %s)", value, directiveIDRe.String())
		}
	case AttrScenarioThreshold:
		f, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil || f < 0 || f > 1 {
			return fmt.Errorf("invalid Scenario threshold %q (must be a number in [0,1])", value)
		}
	case AttrSteer, AttrSetpoint, AttrScenarios:
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s value must not be empty", key)
		}
	}
	return nil
}

// Validate reports line-numbered errors for malformed structured directive
// metadata (bad Directive ID, out-of-range Scenario threshold, duplicate
// stable IDs). It returns nil when all directive metadata is well-formed.
func (p *GoalsPatcher) Validate() []error {
	var errs []error
	seen := map[string]int{}
	for _, d := range p.Directives() {
		for _, a := range d.attrs {
			if err := validateAttribute(a.key, a.value); err != nil {
				errs = append(errs, fmt.Errorf("GOALS.md:%d: %w", a.lineIdx+1, err))
				continue
			}
			if a.key != AttrDirectiveID {
				continue
			}
			if prev, dup := seen[a.value]; dup {
				errs = append(errs, fmt.Errorf("GOALS.md:%d: duplicate Directive ID %q (also declared on line %d)", a.lineIdx+1, a.value, prev))
			} else {
				seen[a.value] = a.lineIdx + 1
			}
		}
	}
	return errs
}

// SlugifyDirectiveID derives a deterministic display tag from a directive
// title when an older GOALS.md file has no explicit stable ID.
func SlugifyDirectiveID(title string) string {
	slug := nonSlugRe.ReplaceAllString(strings.ToLower(title), "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		return "d-directive"
	}
	return "d-" + slug
}
