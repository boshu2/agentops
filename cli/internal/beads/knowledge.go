package beads

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

type CitationStatus string

const (
	CitationFresh   CitationStatus = "FRESH"
	CitationStale   CitationStatus = "STALE"
	CitationUnknown CitationStatus = "UNKNOWN"
)

type Citation struct {
	Kind     string         `json:"kind"`
	Raw      string         `json:"raw"`
	Status   CitationStatus `json:"status"`
	Reason   string         `json:"reason"`
	Resolved string         `json:"resolved"`
}

type VerifyReport struct {
	BeadID      string     `json:"bead_id"`
	Title       string     `json:"title"`
	Status      string     `json:"status"`
	Citations   []Citation `json:"citations"`
	StaleCount  int        `json:"stale_count"`
	FreshCount  int        `json:"fresh_count"`
	TotalCount  int        `json:"total_count"`
	BDAvailable bool       `json:"bd_available"`
}

type ParsedBead struct {
	ID          string
	Title       string
	Status      string
	Description string
	CloseReason string
}

func (bead *ParsedBead) Body() string {
	if bead.CloseReason != "" {
		return bead.CloseReason
	}
	return bead.Description
}

func ParseBDShow(raw string) (*ParsedBead, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("empty bd show output")
	}
	parsed := &ParsedBead{}
	lines := strings.Split(raw, "\n")
	header := regexp.MustCompile(`^[○●✓]?\s*(\S+)\s*·\s*(.*?)\s*\[([^\[\]]*)\]\s*$`)
	for index, line := range lines {
		if parsed.ID == "" {
			if match := header.FindStringSubmatch(line); match != nil {
				parsed.ID = strings.TrimSpace(match[1])
				parsed.Title = strings.TrimSpace(match[2])
				parsed.Status = strings.TrimSpace(match[3])
				continue
			}
		}
		if strings.HasPrefix(line, "Close reason:") {
			parsed.CloseReason = strings.TrimSpace(strings.TrimPrefix(line, "Close reason:"))
			continue
		}
		if strings.HasPrefix(line, "DESCRIPTION") {
			tail := strings.Join(lines[index+1:], "\n")
			if marker := strings.LastIndex(tail, "\n[rerun:"); marker >= 0 {
				tail = tail[:marker]
			}
			tail = strings.TrimSpace(tail)
			if strings.HasPrefix(tail, "[rerun:") {
				tail = ""
			}
			parsed.Description = tail
			break
		}
	}
	if parsed.ID == "" && parsed.Description == "" && parsed.CloseReason == "" {
		return nil, fmt.Errorf("could not parse bd show output: %q", Truncate(raw, 80))
	}
	return parsed, nil
}

func ExtractCitations(description string) []Citation {
	var citations []Citation
	seen := make(map[string]bool)
	filePattern := regexp.MustCompile(`(?:^|[^\w.])([.\w][\w./-]*\.(?:go|py|sh|md|yaml|yml|json|ts|tsx|js|jsx|rs|toml))(?::(\d+))?`)
	for _, match := range filePattern.FindAllStringSubmatch(description, -1) {
		raw := match[1]
		if match[2] != "" {
			raw += ":" + match[2]
		}
		key := "file:" + raw
		if !seen[key] {
			seen[key] = true
			citations = append(citations, Citation{Kind: "file", Raw: raw})
		}
	}
	functionPattern := regexp.MustCompile(`\bfunc\s+(?:\([^)]*\)\s*)?([A-Za-z_]\w*)`)
	for _, match := range functionPattern.FindAllStringSubmatch(description, -1) {
		key := "func:" + match[1]
		if !seen[key] {
			seen[key] = true
			citations = append(citations, Citation{Kind: "function", Raw: "func " + match[1]})
		}
	}
	backtickPattern := regexp.MustCompile("`([A-Za-z_][\\w.]{2,})`")
	for _, match := range backtickPattern.FindAllStringSubmatch(description, -1) {
		symbol := match[1]
		if strings.Contains(symbol, "/") {
			continue
		}
		key := "sym:" + symbol
		if !seen[key] {
			seen[key] = true
			citations = append(citations, Citation{Kind: "symbol", Raw: "`" + symbol + "`"})
		}
	}
	sort.SliceStable(citations, func(left, right int) bool {
		if citations[left].Kind != citations[right].Kind {
			return citations[left].Kind < citations[right].Kind
		}
		return citations[left].Raw < citations[right].Raw
	})
	return citations
}

type LintReport struct {
	StatusFilter string         `json:"status_filter"`
	TotalBeads   int            `json:"total_beads"`
	CleanBeads   int            `json:"clean_beads"`
	StaleBeads   int            `json:"stale_beads"`
	ErrorBeads   int            `json:"error_beads"`
	PerBead      []VerifyReport `json:"per_bead"`
}

func ParseBeadIDs(raw []byte) []string {
	pattern := regexp.MustCompile(`\b([a-z]{2,6}-[0-9a-z][\w.]*)\b`)
	seen := make(map[string]bool)
	var ids []string
	for _, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimLeft(line, " \t├─└│")
		if trimmed == "" {
			continue
		}
		first := []rune(trimmed)[0]
		if first != '○' && first != '●' && first != '✓' {
			continue
		}
		if match := pattern.FindStringSubmatch(line); match != nil && !seen[match[1]] {
			seen[match[1]] = true
			ids = append(ids, match[1])
		}
	}
	return ids
}

type LearningFrontmatter struct {
	Title      string   `json:"title" yaml:"title"`
	BeadID     string   `json:"bead_id" yaml:"bead_id"`
	Source     string   `json:"source" yaml:"source"`
	Date       string   `json:"date" yaml:"date"`
	Tags       []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Maturity   string   `json:"maturity" yaml:"maturity"`
	Provenance string   `json:"provenance" yaml:"provenance"`
}

func RenderLearningBody(frontmatter LearningFrontmatter, parsed *ParsedBead) string {
	var body strings.Builder
	body.WriteString("---\n")
	fmt.Fprintf(&body, "title: %q\n", frontmatter.Title)
	fmt.Fprintf(&body, "bead_id: %s\n", frontmatter.BeadID)
	fmt.Fprintf(&body, "source: %s\n", frontmatter.Source)
	fmt.Fprintf(&body, "date: %s\n", frontmatter.Date)
	fmt.Fprintf(&body, "maturity: %s\n", frontmatter.Maturity)
	fmt.Fprintf(&body, "provenance: %q\n", frontmatter.Provenance)
	body.WriteString("tags:\n")
	for _, tag := range frontmatter.Tags {
		fmt.Fprintf(&body, "  - %s\n", tag)
	}
	body.WriteString("---\n\n")
	fmt.Fprintf(&body, "# %s\n\n", frontmatter.Title)
	fmt.Fprintf(&body, "Harvested from closed bead [%s] on %s.\n\n", frontmatter.BeadID, frontmatter.Date)
	body.WriteString("## Closure reason\n\n")
	body.WriteString(parsed.Body())
	body.WriteString("\n")
	return body.String()
}

func IsClosedStatus(status string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(status))
	for _, token := range []string{"CLOSED", "DONE", "RESOLVED"} {
		if strings.Contains(normalized, token) {
			return true
		}
	}
	return false
}

func Slugify(title string, maxLength int) string {
	var slug strings.Builder
	lastDash := false
	for _, character := range strings.ToLower(title) {
		switch {
		case character >= 'a' && character <= 'z', character >= '0' && character <= '9':
			slug.WriteRune(character)
			lastDash = false
		default:
			if !lastDash && slug.Len() > 0 {
				slug.WriteRune('-')
				lastDash = true
			}
		}
	}
	value := strings.TrimRight(slug.String(), "-")
	if len(value) > maxLength {
		value = strings.TrimRight(value[:maxLength], "-")
	}
	if value == "" {
		return "untitled"
	}
	return value
}

func Truncate(value string, length int) string {
	if len(value) <= length {
		return value
	}
	return value[:length] + "..."
}
