package goals

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// directiveHeadingRe matches "### N. Title" where N is the directive number.
var directiveHeadingRe = regexp.MustCompile(`^###\s+(\d+)\.\s+(.+)$`)

// tableRowRe matches a markdown table row (non-separator).
var tableRowRe = regexp.MustCompile(`^\s*\|(.+)\|\s*$`)

// tableSepRe matches a markdown table separator row.
var tableSepRe = regexp.MustCompile(`^\s*\|[\s:|-]+\|\s*$`)

// ParseMarkdownGoals parses a GOALS.md file into a GoalFile.
// Sets Version=4 and Format="md".
func ParseMarkdownGoals(data []byte) (*GoalFile, error) {
	content := string(data)
	lines := strings.Split(content, "\n")

	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("empty goals file")
	}

	gf := &GoalFile{
		Version: 4,
		Format:  "md",
	}

	gf.Mission = parseMission(lines)
	gf.NorthStars = parseListSection(lines, "North Stars")
	gf.AntiStars = parseListSection(lines, "Anti Stars")
	gf.Directives = parseDirectives(lines)
	gf.Goals = parseGatesTable(lines)

	return gf, nil
}

// parseMission extracts the mission statement — the first non-empty paragraph
// after a line starting with "# " (case-insensitive match on "Goals").
func parseMission(lines []string) string {
	inHeader := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Look for "# Goals" or "# <anything>" as the H1
		if strings.HasPrefix(trimmed, "# ") {
			inHeader = true
			continue
		}

		if inHeader {
			// Skip empty lines after the heading
			if trimmed == "" {
				continue
			}
			// Skip if this line is a heading (section start)
			if strings.HasPrefix(trimmed, "#") {
				return ""
			}
			// First non-empty, non-heading line is the mission
			return trimmed
		}
	}
	return ""
}

// parseListSection extracts bullet items from a section matching the given heading.
// Matching is case-insensitive.
func parseListSection(lines []string, heading string) []string {
	var items []string
	inSection := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for H2 heading match (case-insensitive)
		if strings.HasPrefix(trimmed, "## ") {
			sectionTitle := strings.TrimSpace(strings.TrimPrefix(trimmed, "## "))
			if strings.EqualFold(sectionTitle, heading) {
				inSection = true
				continue
			}
			if inSection {
				// Hit another H2 — done with this section
				break
			}
			continue
		}

		// Check for any heading at H1/H3+ level — also ends the section
		if inSection && strings.HasPrefix(trimmed, "#") {
			break
		}

		if inSection {
			if trimmed == "" {
				continue
			}
			// Parse bullet items (- or *)
			if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
				item := strings.TrimSpace(trimmed[2:])
				if item != "" {
					items = append(items, item)
				}
			}
		}
	}

	return items
}

// parseDirectives extracts numbered H3 directives with optional body text and **Steer:** line.
func parseDirectives(lines []string) []Directive {
	// Find the Directives section (case-insensitive H2 match)
	directiveStart := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			sectionTitle := strings.TrimSpace(strings.TrimPrefix(trimmed, "## "))
			if strings.EqualFold(sectionTitle, "Directives") {
				directiveStart = i + 1
				break
			}
		}
	}

	if directiveStart < 0 {
		return nil // No directives section
	}

	var directives []Directive
	var current *Directive
	var bodyLines []string

	flushCurrent := func() {
		if current != nil {
			current.Description = strings.TrimSpace(strings.Join(bodyLines, "\n"))
			directives = append(directives, *current)
		}
	}

	for i := directiveStart; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])

		// End of directives section at next H2 or H1
		if strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "# ") {
			break
		}

		// Check for directive heading: ### N. Title
		if m := directiveHeadingRe.FindStringSubmatch(trimmed); m != nil {
			flushCurrent()
			num, _ := strconv.Atoi(m[1])
			current = &Directive{
				Number: num,
				Title:  strings.TrimSpace(m[2]),
			}
			bodyLines = nil
			continue
		}

		if current != nil {
			// Check for **Steer:** line
			if strings.HasPrefix(trimmed, "**Steer:**") {
				steerVal := strings.TrimSpace(strings.TrimPrefix(trimmed, "**Steer:**"))
				if steerVal != "" {
					current.Steer = steerVal
				}
				continue
			}
			// Preserve raw line content (including indentation) for directive body.
			// flushCurrent applies TrimSpace to the joined result, which strips
			// leading/trailing blank lines while keeping internal formatting.
			bodyLines = append(bodyLines, lines[i])
		}
	}

	flushCurrent()
	return directives
}

// buildGateColumnMap takes header row cells and returns a column index map.
// Default mapping: {"id": 0, "check": 1, "weight": 2, "description": 3}.
// "tags" is opt-in: only present in the map when the header declares it.
// Header cell names are matched case-insensitively to override default positions.
func buildGateColumnMap(cells []string) map[string]int {
	colMap := map[string]int{"id": 0, "check": 1, "weight": 2, "description": 3}
	for j, cell := range cells {
		lower := strings.ToLower(strings.TrimSpace(cell))
		switch lower {
		case "id":
			colMap["id"] = j
		case "check":
			colMap["check"] = j
		case "weight":
			colMap["weight"] = j
		case "description":
			colMap["description"] = j
		case "tags":
			colMap["tags"] = j
		}
	}
	return colMap
}

// parseTagsCell splits a Tags cell into a slice of tag strings.
// Tags are comma- or semicolon-separated, lowercased, and trimmed; empty
// entries are dropped. Backticks wrapping the cell value are stripped.
// Returns nil for empty input so an unset Tags field stays nil rather than
// becoming an empty slice (preserves omitempty behavior in serialization).
func parseTagsCell(s string) []string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '`' && s[len(s)-1] == '`' {
		s = s[1 : len(s)-1]
	}
	if s == "" {
		return nil
	}
	splitter := func(r rune) bool { return r == ',' || r == ';' }
	parts := strings.FieldsFunc(s, splitter)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.ToLower(strings.TrimSpace(p))
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// parseGateRow extracts a Goal from a data row's cells using the column index map.
// Strips backticks from the check field, defaults weight to 5 on parse error,
// and falls back description to ID if empty.
func parseGateRow(cells []string, colMap map[string]int) Goal {
	g := Goal{Type: GoalTypeHealth}
	if idx, ok := colMap["id"]; ok && idx < len(cells) {
		g.ID = strings.TrimSpace(cells[idx])
	}
	if idx, ok := colMap["check"]; ok && idx < len(cells) {
		s := strings.TrimSpace(cells[idx])
		if len(s) >= 2 && s[0] == '`' && s[len(s)-1] == '`' {
			s = s[1 : len(s)-1]
		}
		g.Check = s
	}
	if idx, ok := colMap["weight"]; ok && idx < len(cells) {
		w, err := strconv.Atoi(strings.TrimSpace(cells[idx]))
		if err != nil || w < 1 || w > 10 {
			w = 5
		}
		g.Weight = w
	}
	if idx, ok := colMap["description"]; ok && idx < len(cells) {
		g.Description = strings.TrimSpace(cells[idx])
	}
	if g.Description == "" {
		g.Description = g.ID
	}
	if idx, ok := colMap["tags"]; ok && idx < len(cells) {
		g.Tags = parseTagsCell(cells[idx])
	}
	return g
}

// parseGatesTable extracts goals from a markdown table under the "Gates" section.
// Expected columns: ID | Check | Weight | Description (order may vary if header is present).
func parseGatesTable(lines []string) []Goal {
	// Find the Gates section (case-insensitive H2 match)
	gatesStart := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			sectionTitle := strings.TrimSpace(strings.TrimPrefix(trimmed, "## "))
			if strings.EqualFold(sectionTitle, "Gates") {
				gatesStart = i + 1
				break
			}
		}
	}

	if gatesStart < 0 {
		return nil // No gates section
	}

	var goals []Goal
	headerFound := false
	colMap := map[string]int{"id": 0, "check": 1, "weight": 2, "description": 3}

	for i := gatesStart; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])

		// End of gates section at next heading
		if strings.HasPrefix(trimmed, "#") {
			break
		}

		if trimmed == "" {
			continue
		}

		// Must be a table row
		if !tableRowRe.MatchString(trimmed) {
			continue
		}

		// Skip separator rows
		if tableSepRe.MatchString(trimmed) {
			continue
		}

		cells := splitTableRow(trimmed)

		// First non-separator row is the header
		if !headerFound {
			headerFound = true
			colMap = buildGateColumnMap(cells)
			continue
		}

		// Parse data row
		g := parseGateRow(cells, colMap)
		if g.ID != "" {
			goals = append(goals, g)
		}
	}

	return goals
}

// splitTableRow splits a markdown table row into cells, stripping outer pipes.
// Escaped pipes (\|) within cell content are preserved during splitting and then
// unescaped to literal | after the split.
func splitTableRow(row string) []string {
	// Remove leading/trailing whitespace and pipes
	row = strings.TrimSpace(row)
	row = strings.Trim(row, "|")

	// Split on unescaped pipe characters. We use a placeholder to preserve \|
	// sequences during the split, then restore them as literal |.
	const placeholder = "\x00PIPE\x00"
	row = strings.ReplaceAll(row, `\|`, placeholder)
	parts := strings.Split(row, "|")
	cells := make([]string, len(parts))
	for i, p := range parts {
		cell := strings.TrimSpace(p)
		// Unescape \| back to |
		cell = strings.ReplaceAll(cell, placeholder, "|")
		cells[i] = cell
	}
	return cells
}
