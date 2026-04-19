// Package formatter provides output formatters for agentops sessions.
package formatter

import (
	"fmt"
	"io"
	"strings"
	"text/template"

	"github.com/boshu2/agentops/cli/internal/storage"
	"github.com/boshu2/agentops/cli/pkg/vault"
)

// markdownTmplSource holds the template text. Package-level var so tests
// can inject an invalid template to exercise the parse-error path.
var markdownTmplSource = markdownTemplate

// MarkdownFormatter outputs sessions as Obsidian-compatible markdown.
type MarkdownFormatter struct {
	// VaultPath is the detected Obsidian vault path (empty if not in vault).
	VaultPath string

	// UseWikiLinks enables [[wiki-links]] syntax (only if in vault).
	UseWikiLinks bool
}

// NewMarkdownFormatter creates a markdown formatter with vault detection.
func NewMarkdownFormatter() *MarkdownFormatter {
	mf := &MarkdownFormatter{}
	mf.detectVault()
	return mf
}

// detectVault looks for .obsidian directory to determine if we're in a vault.
func (mf *MarkdownFormatter) detectVault() {
	mf.VaultPath = vault.DetectVault("")
	mf.UseWikiLinks = mf.VaultPath != ""
}

// Format writes the session as markdown.
func (mf *MarkdownFormatter) Format(w io.Writer, session *storage.Session) error {
	// Build template data
	data := mf.buildTemplateData(session)

	// Execute template
	tmpl, err := template.New("session").Funcs(mf.templateFuncs()).Parse(markdownTmplSource)
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	return tmpl.Execute(w, data)
}

// Extension returns the file extension for markdown.
func (mf *MarkdownFormatter) Extension() string {
	return ".md"
}

// templateData holds all data for the markdown template.
type templateData struct {
	// YAML frontmatter fields
	SessionID string
	Date      string
	Summary   string
	Tags      []string

	// Content sections
	Decisions    []string
	Knowledge    []string
	FilesChanged []string
	Issues       []string
	ToolCalls    map[string]int
	Tokens       storage.TokenUsage

	// Link formatting
	UseWikiLinks bool
}

// buildTemplateData prepares data for the template.
func (mf *MarkdownFormatter) buildTemplateData(session *storage.Session) *templateData {
	return &templateData{
		SessionID:    session.ID,
		Date:         session.Date.Format("2006-01-02"),
		Summary:      session.Summary,
		Tags:         mf.extractTags(session),
		Decisions:    session.Decisions,
		Knowledge:    session.Knowledge,
		FilesChanged: session.FilesChanged,
		Issues:       session.Issues,
		ToolCalls:    session.ToolCalls,
		Tokens:       session.Tokens,
		UseWikiLinks: mf.UseWikiLinks,
	}
}

// extractTags generates tags from session content.
func (mf *MarkdownFormatter) extractTags(session *storage.Session) []string {
	tags := []string{"agentops", "session"}

	// Add date-based tag
	tags = append(tags, session.Date.Format("2006-01"))

	// Could add more intelligent tagging here
	return tags
}

// templateFuncs returns custom template functions.
func (mf *MarkdownFormatter) templateFuncs() template.FuncMap {
	return template.FuncMap{
		"link": func(text, path string) string {
			if mf.UseWikiLinks {
				// Wiki-link format for Obsidian
				return fmt.Sprintf("[[%s|%s]]", path, text)
			}
			// Standard markdown link
			return fmt.Sprintf("[%s](%s)", text, path)
		},
		"fileLink": func(path string) string {
			if mf.UseWikiLinks {
				return fmt.Sprintf("[[%s]]", path)
			}
			return fmt.Sprintf("`%s`", path)
		},
		"issueLink": func(issueID string) string {
			if mf.UseWikiLinks {
				return fmt.Sprintf("[[issues/%s|%s]]", issueID, issueID)
			}
			return fmt.Sprintf("`%s`", issueID)
		},
		"join": strings.Join,
		"hasContent": func(s []string) bool {
			return len(s) > 0
		},
		"hasToolCalls": func(m map[string]int) bool {
			return len(m) > 0
		},
	}
}

const markdownTemplate = `---
session_id: {{ .SessionID }}
date: {{ .Date }}
summary: "{{ .Summary }}"
tags:
{{- range .Tags }}
  - {{ . }}
{{- end }}
---

# {{ .Summary }}

**Session:** {{ .SessionID }}
**Date:** {{ .Date }}

{{- if hasContent .Decisions }}

## Decisions

{{- range .Decisions }}
- {{ . }}
{{- end }}
{{- end }}

{{- if hasContent .Knowledge }}

## Knowledge

{{- range .Knowledge }}
- {{ . }}
{{- end }}
{{- end }}

{{- if hasContent .FilesChanged }}

## Files Changed

{{- range .FilesChanged }}
- {{ fileLink . }}
{{- end }}
{{- end }}

{{- if hasContent .Issues }}

## Issues

{{- range .Issues }}
- {{ issueLink . }}
{{- end }}
{{- end }}

{{- if hasToolCalls .ToolCalls }}

## Tool Usage

| Tool | Count |
|------|-------|
{{- range $tool, $count := .ToolCalls }}
| {{ $tool }} | {{ $count }} |
{{- end }}
{{- end }}

{{- if .Tokens.Total }}

## Tokens

- **Input:** {{ .Tokens.Input }}
- **Output:** {{ .Tokens.Output }}
- **Total:** {{ if .Tokens.Estimated }}~{{ end }}{{ .Tokens.Total }}{{ if .Tokens.Estimated }} (estimated){{ end }}
{{- end }}
`
