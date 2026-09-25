package skillshealth

import (
	"regexp"
	"strings"
)

// markdownProseLines blanks frontmatter and CommonMark-style fenced/indented
// examples while preserving source line numbers. It is not a Markdown renderer.
func markdownProseLines(text string) []string { return markdownLines(text, false) }

// markdownCommandLines retains literal command text in shell and unlabelled
// fences. Markdown/text examples remain excluded; syntax highlighting is not a
// prerequisite for recognizing an actual shell instruction.
func markdownCommandLines(text string) []string { return markdownLines(text, true) }

func markdownLines(text string, commands bool) []string {
	lines := strings.Split(text, "\n")
	var fence byte
	width := 0
	front := false
	executableFence := false
	preceding := ""
	for i, line := range lines {
		if i == 0 && strings.TrimSpace(line) == "---" {
			front = true
			lines[i] = ""
			continue
		}
		if front {
			if strings.TrimSpace(line) == "---" {
				front = false
			}
			lines[i] = ""
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		trim := strings.TrimSpace(line)
		if fence != 0 {
			if indent <= 3 && fenceRun(trim, fence) >= width && strings.TrimSpace(trim[fenceRun(trim, fence):]) == "" {
				fence = 0
				executableFence = false
			}
			if !executableFence {
				lines[i] = ""
			}
			continue
		}
		if indent >= 4 || strings.HasPrefix(line, "\t") {
			lines[i] = ""
			continue
		}
		if len(trim) > 0 && (trim[0] == '`' || trim[0] == '~') {
			run := fenceRun(trim, trim[0])
			if run >= 3 && (trim[0] != '`' || !strings.Contains(trim[run:], "`")) {
				fence = trim[0]
				width = run
				executableFence = commands && executableFenceInfo(trim[run:], preceding)
				lines[i] = ""
			}
		}
		if trim != "" && fence == 0 {
			preceding = trim
		}
	}
	return lines
}

var markdownNonInstruction = regexp.MustCompile(`(?i)^\s*(?:example\b|for example\b|e\.g\.|never\b|do not\b|don't\b|avoid\b)`)

func executableFenceInfo(info, preceding string) bool {
	if markdownNonInstruction.MatchString(preceding) {
		return false
	}
	fields := strings.Fields(strings.ToLower(info))
	if len(fields) == 0 {
		return true
	}
	switch fields[0] {
	case "bash", "sh", "shell", "zsh":
		return true
	}
	return false
}

func fenceRun(s string, marker byte) int {
	n := 0
	for n < len(s) && s[n] == marker {
		n++
	}
	return n
}

// stripInlineCode handles variable-length backtick spans, including literal
// shorter backticks inside a span. Unmatched delimiters remain ordinary prose.
func stripInlineCode(line string) string {
	for start := 0; start < len(line); {
		if line[start] != '`' {
			start++
			continue
		}
		width := fenceRun(line[start:], '`')
		end := start + width
		for end < len(line) {
			if line[end] != '`' {
				end++
				continue
			}
			n := fenceRun(line[end:], '`')
			if n == width {
				line = line[:start] + strings.Repeat(" ", end+n-start) + line[end+n:]
				break
			}
			end += n
		}
		start = end
	}
	return line
}

var markdownDefinition = regexp.MustCompile(`^ {0,3}\[([^\]]+)\]:\s*(?:<([^>]+)>|(\S+))`)
var markdownReference = regexp.MustCompile(`\[([^\]]+)\](?:\[([^\]]*)\])?`)

type markdownResource struct {
	target string
	line   int
}

func referenceKey(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), " ")) }

// markdownResources resolves used full, collapsed and shortcut references.
// Unused definitions and code examples do not declare required resources.
func markdownResources(lines []string) []markdownResource {
	defs := map[string]string{}
	for _, line := range lines {
		if m := markdownDefinition.FindStringSubmatch(line); m != nil {
			target := m[2]
			if target == "" {
				target = m[3]
			}
			key := referenceKey(m[1])
			if _, exists := defs[key]; !exists {
				defs[key] = target
			}
		}
	}
	var out []markdownResource
	for i, line := range lines {
		if markdownDefinition.MatchString(line) {
			continue
		}
		line = stripInlineCode(line)
		for _, m := range auditLink.FindAllStringSubmatch(line, -1) {
			out = append(out, markdownResource{m[1], i + 1})
		}
		for _, idx := range markdownReference.FindAllStringSubmatchIndex(line, -1) {
			if idx[1] < len(line) && line[idx[1]] == '(' {
				continue
			}
			label := line[idx[2]:idx[3]]
			if idx[4] >= 0 && idx[5] > idx[4] {
				label = line[idx[4]:idx[5]]
			}
			if target, ok := defs[referenceKey(label)]; ok {
				out = append(out, markdownResource{target, i + 1})
			}
		}
	}
	return out
}

var markdownRunPrefix = regexp.MustCompile("(?i)(?:^|[.!?;:]\\s*|\\bthen\\s+)(?:[-*] |[0-9]+[.)] )?(?:run|execute|invoke|use)\\s+[`\\\"']?$")
var markdownBareCommand = regexp.MustCompile("^\\s*(?:[-*] |[0-9]+[.)] )?[`\"']?$")
var markdownExamplePrefix = regexp.MustCompile(`(?i)^\s*(?:example:|for example[, :]|e\.g\.)`)
var scriptRunPrefix = regexp.MustCompile(`(?:^|;|&&|\|\||\bthen\b|\bdo\b)\s*$`)

// Direct invocations are affirmative prose directives (including inline command
// formatting), or command-position text in scripts. Examples and prohibitions
// do not become resource requirements merely by naming a command.
func directInvocations(line string, markdown bool) []string {
	var out []string
	if (markdown && markdownExamplePrefix.MatchString(line)) || (!markdown && strings.HasPrefix(strings.TrimSpace(line), "#")) {
		return out
	}
	for _, idx := range auditInvocation.FindAllStringSubmatchIndex(line, -1) {
		prefix := line[:idx[0]]
		if markdown && !markdownRunPrefix.MatchString(prefix) && !markdownBareCommand.MatchString(prefix) {
			continue
		}
		if !markdown && !scriptRunPrefix.MatchString(prefix) {
			continue
		}
		out = append(out, line[idx[2]:idx[3]])
	}
	return out
}
