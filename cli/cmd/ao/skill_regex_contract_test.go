// practices: [design-by-contract, code-complete]
package main

import (
	"regexp"
	"testing"
)

// =============================================================================
// Regex contract tests for patterns the CLI uses to parse SKILL.md content,
// council reports, learning files, and markdown structures.
//
// Each test group covers one or more regexp.MustCompile patterns from production
// code and verifies that they match expected inputs and reject invalid ones.
// =============================================================================

// --- Council Verdict regex (rpi_phased_verdicts.go:58) ---
// Pattern: `(?m)^## Council Verdict:\s*(PASS|WARN|FAIL)`

func TestSkillRegexContract_CouncilVerdict(t *testing.T) {
	pattern := `(?m)^## Council Verdict:\s*(PASS|WARN|FAIL)`
	re := regexp.MustCompile(pattern)

	tests := []struct {
		name      string
		input     string
		wantMatch bool
		wantGroup string // first capture group
	}{
		{"PASS verdict", "## Council Verdict: PASS", true, "PASS"},
		{"WARN verdict", "## Council Verdict: WARN", true, "WARN"},
		{"FAIL verdict", "## Council Verdict: FAIL", true, "FAIL"},
		{"extra spaces after colon", "## Council Verdict:  PASS", true, "PASS"},
		{"tab after colon", "## Council Verdict:\tPASS", true, "PASS"},
		{"embedded in multiline", "Some text\n## Council Verdict: WARN\nMore text", true, "WARN"},
		{"at start of string", "## Council Verdict: FAIL\n", true, "FAIL"},

		// Should NOT match
		{"empty string", "", false, ""},
		{"wrong heading level h1", "# Council Verdict: PASS", false, ""},
		{"wrong heading level h3", "### Council Verdict: PASS", false, ""},
		{"no space after ##", "##Council Verdict: PASS", false, ""},
		{"lowercase verdict", "## Council Verdict: pass", false, ""},
		{"unknown verdict OK", "## Council Verdict: OK", false, ""},
		{"missing colon", "## Council Verdict PASS", false, ""},
		{"indented line", "  ## Council Verdict: PASS", false, ""},
		{"partial text INFO", "## Council Verdict: INFO", false, ""},
		{"no verdict value", "## Council Verdict:", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := re.FindStringSubmatch(tt.input)
			got := len(matches) >= 2
			if got != tt.wantMatch {
				t.Errorf("pattern %q on %q: match=%v, want match=%v", pattern, tt.input, got, tt.wantMatch)
				return
			}
			if got && matches[1] != tt.wantGroup {
				t.Errorf("captured %q, want %q", matches[1], tt.wantGroup)
			}
		})
	}
}

// --- Structured findings regex (rpi_phased_verdicts.go:128) ---
// Pattern: `(?m)FINDING:\s*(.+?)\s*\|\s*FIX:\s*(.+?)\s*\|\s*REF:\s*(.+?)$`

func TestSkillRegexContract_StructuredFindings(t *testing.T) {
	pattern := `(?m)FINDING:\s*(.+?)\s*\|\s*FIX:\s*(.+?)\s*\|\s*REF:\s*(.+?)$`
	re := regexp.MustCompile(pattern)

	tests := []struct {
		name   string
		input  string
		want   bool
		groups []string // [finding, fix, ref]
	}{
		{
			"standard format",
			"FINDING: Missing nil check | FIX: Add guard | REF: inject.go:42",
			true,
			[]string{"Missing nil check", "Add guard", "inject.go:42"},
		},
		{
			"minimal whitespace",
			"FINDING:x|FIX:y|REF:z",
			true,
			[]string{"x", "y", "z"},
		},
		{
			"extra whitespace",
			"FINDING:  Unused var  |  FIX:  Remove it  |  REF:  pool.go:100",
			true,
			[]string{"Unused var", "Remove it", "pool.go:100"},
		},
		{
			"multiline document",
			"Line 1\nFINDING: Bug | FIX: Patch | REF: file.go\nLine 3",
			true,
			[]string{"Bug", "Patch", "file.go"},
		},

		// Should NOT match
		{"empty string", "", false, nil},
		{"missing FINDING prefix", "Bug | FIX: Patch | REF: file.go", false, nil},
		{"missing FIX field", "FINDING: Bug | REF: file.go", false, nil},
		{"missing REF field", "FINDING: Bug | FIX: Patch", false, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := re.FindStringSubmatch(tt.input)
			got := len(matches) >= 4
			if got != tt.want {
				t.Errorf("match=%v, want %v for %q", got, tt.want, tt.input)
				return
			}
			if got {
				for i, g := range tt.groups {
					actual := matches[i+1]
					// Trim because the regex uses non-greedy (.+?) with surrounding \s*
					if actual != g {
						t.Errorf("group[%d] = %q, want %q", i+1, actual, g)
					}
				}
			}
		})
	}
}

// --- Fallback findings regex (rpi_phased_verdicts.go:145) ---
// Pattern: `(?m)^\d+\.\s+\*\*(.+?)\*\*\s*[—–-]\s*(.+)$`

func TestSkillRegexContract_FallbackFindings(t *testing.T) {
	pattern := `(?m)^\d+\.\s+\*\*(.+?)\*\*\s*[—–-]\s*(.+)$`
	re := regexp.MustCompile(pattern)

	tests := []struct {
		name  string
		input string
		want  bool
		title string
		desc  string
	}{
		{"em dash", "1. **Missing guard** — inject.go needs nil check", true, "Missing guard", "inject.go needs nil check"},
		{"en dash", "2. **Race condition** – goroutine access", true, "Race condition", "goroutine access"},
		{"hyphen", "3. **Dead code** - unused helper", true, "Dead code", "unused helper"},
		{"double digit", "12. **Complex issue** — needs refactor", true, "Complex issue", "needs refactor"},
		{"multiline doc", "Intro\n1. **Bug** — description\n2. **Fix** — more", true, "Bug", "description"},

		// Should NOT match
		{"empty string", "", false, "", ""},
		{"no number prefix", "**Bold text** — description", false, "", ""},
		{"no bold markers", "1. Plain text — description", false, "", ""},
		{"no separator", "1. **Bold text** description only", false, "", ""},
		{"letter prefix", "a. **Bold** — desc", false, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := re.FindStringSubmatch(tt.input)
			got := len(matches) >= 3
			if got != tt.want {
				t.Errorf("match=%v, want %v for %q", got, tt.want, tt.input)
				return
			}
			if got {
				if matches[1] != tt.title {
					t.Errorf("title = %q, want %q", matches[1], tt.title)
				}
				if matches[2] != tt.desc {
					t.Errorf("desc = %q, want %q", matches[2], tt.desc)
				}
			}
		})
	}
}

// --- Learning header regex (pool_ingest.go:218) ---
// Pattern: `(?m)^# Learning:\s*(.+)\s*$`

func TestSkillRegexContract_LearningHeader(t *testing.T) {
	pattern := `(?m)^# Learning:\s*(.+)\s*$`
	re := regexp.MustCompile(pattern)

	tests := []struct {
		name  string
		input string
		want  bool
		title string
	}{
		{"standard header", "# Learning: Always validate JWT claims", true, "Always validate JWT claims"},
		// Note: the regex captures trailing spaces in the group; production code
		// calls strings.TrimSpace on the captured group afterward.
		{"extra spaces", "# Learning:  Use context.WithCancel  ", true, "Use context.WithCancel  "},
		{"multiline with header", "Intro\n# Learning: Lesson one\nBody text", true, "Lesson one"},

		// Should NOT match
		{"empty string", "", false, ""},
		{"h2 heading", "## Learning: Not h1", false, ""},
		{"no colon", "# Learning Always validate", false, ""},
		{"indented", "  # Learning: Indented", false, ""},
		{"no title after colon", "# Learning:", false, ""},
		{"just hash", "#Learning: No space", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := re.FindStringSubmatch(tt.input)
			got := len(matches) >= 2
			if got != tt.want {
				t.Errorf("match=%v, want %v for %q", got, tt.want, tt.input)
				return
			}
			if got && matches[1] != tt.title {
				t.Errorf("title = %q, want %q", matches[1], tt.title)
			}
		})
	}
}

// --- Metadata line regexes (pool_ingest.go:220-222) ---
// ID:         `(?m)^\*\*ID:?\*\*:?\s*(.+)\s*$`
// Category:   `(?m)^\*\*Category:?\*\*:?\s*(.+)\s*$`
// Confidence: `(?m)^\*\*Confidence:?\*\*:?\s*(.+)\s*$`

func TestSkillRegexContract_MetadataLines(t *testing.T) {
	// These share the same structure: **Field:** value or **Field**: value
	metaPatterns := []struct {
		name    string
		pattern string
	}{
		{"ID", `(?m)^\*\*ID:?\*\*:?\s*(.+)\s*$`},
		{"Category", `(?m)^\*\*Category:?\*\*:?\s*(.+)\s*$`},
		{"Confidence", `(?m)^\*\*Confidence:?\*\*:?\s*(.+)\s*$`},
	}

	for _, mp := range metaPatterns {
		re := regexp.MustCompile(mp.pattern)
		t.Run(mp.name, func(t *testing.T) {
			// Both "**Field:** value" and "**Field**: value" forms should match
			tests := []struct {
				name  string
				input string
				want  bool
				value string
			}{
				{"colon inside bold", "**" + mp.name + ":** some-value", true, "some-value"},
				{"colon outside bold", "**" + mp.name + "**: some-value", true, "some-value"},
				{"colon both places", "**" + mp.name + ":**:  some-value", true, "some-value"},
				{"no colon at all", "**" + mp.name + "** some-value", true, "some-value"},
				{"multiline context", "Intro\n**" + mp.name + ":** val\nMore", true, "val"},

				// Should NOT match
				{"empty string", "", false, ""},
				{"no bold markers", mp.name + ": value", false, ""},
				{"indented", "  **" + mp.name + ":** value", false, ""},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					matches := re.FindStringSubmatch(tt.input)
					got := len(matches) >= 2
					if got != tt.want {
						t.Errorf("match=%v, want %v for %q", got, tt.want, tt.input)
						return
					}
					if got && matches[1] != tt.value {
						t.Errorf("value = %q, want %q", matches[1], tt.value)
					}
				})
			}
		})
	}
}

// --- Frontmatter regex (pool_ingest.go:223) ---
// Pattern: `(?s)^---\s*\n(.*?)\n---\s*\n`

func TestSkillRegexContract_Frontmatter(t *testing.T) {
	pattern := `(?s)^---\s*\n(.*?)\n---\s*\n`
	re := regexp.MustCompile(pattern)

	tests := []struct {
		name    string
		input   string
		want    bool
		content string
	}{
		{
			"standard frontmatter",
			"---\nname: test\ncontext: fork\n---\n# Heading",
			true,
			"name: test\ncontext: fork",
		},
		{
			"spaces after dashes",
			"---  \nkey: val\n---  \nbody",
			true,
			"key: val",
		},
		{
			"single field",
			"---\nfoo: bar\n---\n",
			true,
			"foo: bar",
		},

		// Should NOT match
		{"empty string", "", false, ""},
		{"no opening dashes", "key: val\n---\n", false, ""},
		{"no closing dashes", "---\nkey: val\n", false, ""},
		{"dashes in middle of file", "text\n---\nkey: val\n---\n", false, ""},
		{"four dashes", "----\nkey: val\n---\n", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := re.FindStringSubmatch(tt.input)
			got := len(matches) >= 2
			if got != tt.want {
				t.Errorf("match=%v, want %v for %q", got, tt.want, tt.input)
				return
			}
			if got && matches[1] != tt.content {
				t.Errorf("content = %q, want %q", matches[1], tt.content)
			}
		})
	}
}

// --- Date extraction regexes (pool_ingest.go:224-225, index.go:29-30) ---
// Markdown: `(?m)^\*\*Date:?\*\*:?\s*(\d{4}-\d{2}-\d{2})\s*$`
// YAML:     `(?m)^date:\s*(\d{4}-\d{2}-\d{2})\s*$`
// Filename: `^(\d{4}-\d{2}-\d{2})`

func TestSkillRegexContract_DateExtraction(t *testing.T) {
	t.Run("markdown_date", func(t *testing.T) {
		re := regexp.MustCompile(`(?m)^\*\*Date:?\*\*:?\s*(\d{4}-\d{2}-\d{2})\s*$`)
		tests := []struct {
			name  string
			input string
			want  bool
			date  string
		}{
			{"colon inside bold", "**Date:** 2026-03-04", true, "2026-03-04"},
			{"colon outside bold", "**Date**: 2026-01-15", true, "2026-01-15"},
			{"no colon", "**Date** 2026-12-31", true, "2026-12-31"},

			{"empty", "", false, ""},
			{"wrong format", "**Date:** March 4 2026", false, ""},
			{"no bold", "Date: 2026-03-04", false, ""},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				matches := re.FindStringSubmatch(tt.input)
				got := len(matches) >= 2
				if got != tt.want {
					t.Errorf("match=%v, want %v for %q", got, tt.want, tt.input)
				}
				if got && matches[1] != tt.date {
					t.Errorf("date = %q, want %q", matches[1], tt.date)
				}
			})
		}
	})

	t.Run("yaml_date", func(t *testing.T) {
		re := regexp.MustCompile(`(?m)^date:\s*(\d{4}-\d{2}-\d{2})\s*$`)
		tests := []struct {
			name  string
			input string
			want  bool
			date  string
		}{
			{"standard", "date: 2026-03-04", true, "2026-03-04"},
			{"in frontmatter", "name: test\ndate: 2026-01-01\nfoo: bar", true, "2026-01-01"},
			{"extra spaces", "date:  2026-06-15  ", true, "2026-06-15"},

			{"empty", "", false, ""},
			{"indented", "  date: 2026-03-04", false, ""},
			{"uppercase", "Date: 2026-03-04", false, ""},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				matches := re.FindStringSubmatch(tt.input)
				got := len(matches) >= 2
				if got != tt.want {
					t.Errorf("match=%v, want %v for %q", got, tt.want, tt.input)
				}
				if got && matches[1] != tt.date {
					t.Errorf("date = %q, want %q", matches[1], tt.date)
				}
			})
		}
	})

	t.Run("filename_date", func(t *testing.T) {
		re := regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})`)
		tests := []struct {
			name  string
			input string
			want  bool
			date  string
		}{
			{"learning file", "2026-03-04-parallel-worktree.md", true, "2026-03-04"},
			{"date only", "2026-01-01", true, "2026-01-01"},

			{"empty", "", false, ""},
			{"no leading date", "learning-2026-03-04.md", false, ""},
			{"partial date", "2026-03", false, ""},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				matches := re.FindStringSubmatch(tt.input)
				got := len(matches) >= 2
				if got != tt.want {
					t.Errorf("match=%v, want %v for %q", got, tt.want, tt.input)
				}
				if got && matches[1] != tt.date {
					t.Errorf("date = %q, want %q", matches[1], tt.date)
				}
			})
		}
	})
}

// --- Session hint regex (pool_ingest.go:226) ---
// Pattern: `\bag-[a-z0-9]+\b`

func TestSkillRegexContract_SessionHint(t *testing.T) {
	pattern := `\bag-[a-z0-9]+\b`
	re := regexp.MustCompile(pattern)

	tests := []struct {
		name  string
		input string
		want  bool
		match string
	}{
		{"standard session ID", "worked on ag-xyz in this session", true, "ag-xyz"},
		{"alphanumeric", "fixed in ag-abc123", true, "ag-abc123"},
		{"at start", "ag-test is the issue", true, "ag-test"},
		{"at end", "see ag-foo", true, "ag-foo"},

		// Should NOT match
		{"empty", "", false, ""},
		{"no ag prefix", "session-xyz", false, ""},
		{"uppercase after ag", "ag-XYZ", false, ""},
		{"no word boundary", "flag-test", false, ""},
		{"ag alone", "ag-", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match := re.FindString(tt.input)
			got := match != ""
			if got != tt.want {
				t.Errorf("match=%v, want %v for %q", got, tt.want, tt.input)
			}
			if got && match != tt.match {
				t.Errorf("matched %q, want %q", match, tt.match)
			}
		})
	}
}

// --- Canonical session ID patterns (canonical_identity.go:12-14) ---

func TestSkillRegexContract_CanonicalSessionPatterns(t *testing.T) {
	t.Run("session_prefix", func(t *testing.T) {
		re := regexp.MustCompile(`^session-\d{8}-\d{6}$`)
		tests := []struct {
			name  string
			input string
			want  bool
		}{
			{"valid", "session-20260304-143022", true},
			{"missing prefix", "20260304-143022", false},
			{"wrong prefix", "sess-20260304-143022", false},
			{"short date", "session-2026030-143022", false},
			{"extra chars", "session-20260304-143022x", false},
			{"empty", "", false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if re.MatchString(tt.input) != tt.want {
					t.Errorf("match=%v, want %v for %q", !tt.want, tt.want, tt.input)
				}
			})
		}
	})

	t.Run("timestamp_only", func(t *testing.T) {
		re := regexp.MustCompile(`^\d{8}-\d{6}$`)
		tests := []struct {
			name  string
			input string
			want  bool
		}{
			{"valid", "20260304-143022", true},
			{"with prefix", "session-20260304-143022", false},
			{"short", "2026030-143022", false},
			{"empty", "", false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if re.MatchString(tt.input) != tt.want {
					t.Errorf("match=%v, want %v for %q", !tt.want, tt.want, tt.input)
				}
			})
		}
	})

	t.Run("uuid_format", func(t *testing.T) {
		re := regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$`)
		tests := []struct {
			name  string
			input string
			want  bool
		}{
			{"valid uuid", "a1b2c3d4-e5f6-7890-abcd-ef1234567890", true},
			{"all zeros", "00000000-0000-0000-0000-000000000000", true},
			{"uppercase rejected", "A1B2C3D4-E5F6-7890-ABCD-EF1234567890", false},
			{"short segment", "a1b2c3d-e5f6-7890-abcd-ef1234567890", false},
			{"empty", "", false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if re.MatchString(tt.input) != tt.want {
					t.Errorf("match=%v, want %v for %q", !tt.want, tt.want, tt.input)
				}
			})
		}
	})
}

// --- Issue ID pattern (forge.go:46) ---
// Pattern: `\b([a-z]{2,3})-([a-z0-9]{3,7}(?:-[a-z0-9]+)?)\b`

func TestSkillRegexContract_IssueIDPattern(t *testing.T) {
	pattern := `\b([a-z]{2,3})-([a-z0-9]{3,7}(?:-[a-z0-9]+)?)\b`
	re := regexp.MustCompile(pattern)

	tests := []struct {
		name   string
		input  string
		want   bool
		prefix string
		id     string
	}{
		{"standard na issue", "working on na-83w", true, "na", "83w"},
		{"three letter prefix", "fix for abc-def123", true, "abc", "def123"},
		{"with subpart", "issue ag-xyz-sub1", true, "ag", "xyz-sub1"},
		{"two char prefix", "see na-test in the log", true, "na", "test"},
		{"in sentence", "closed bd-abc123 today", true, "bd", "abc123"},

		// Should NOT match
		{"empty", "", false, "", ""},
		{"single letter prefix", "a-test", false, "", ""},
		{"four letter prefix", "abcd-test", false, "", ""},
		{"uppercase prefix", "NA-test", false, "", ""},
		{"short id", "na-ab", false, "", ""},
		{"long id", "na-abcdefgh", false, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := re.FindStringSubmatch(tt.input)
			got := len(matches) >= 3
			if got != tt.want {
				t.Errorf("match=%v, want %v for %q", got, tt.want, tt.input)
				return
			}
			if got {
				if matches[1] != tt.prefix {
					t.Errorf("prefix = %q, want %q", matches[1], tt.prefix)
				}
				if matches[2] != tt.id {
					t.Errorf("id = %q, want %q", matches[2], tt.id)
				}
			}
		})
	}
}

// --- Specificity score patterns (pool_ingest.go:532-555) ---
// File extension: `\b[a-zA-Z0-9_./-]+\.(go|ts|js|py|sh|yaml|yml|json|md)\b`
// Bullet list:    `(?m)^\s*[-*]\s+`
// Action verbs:   `(?i)\b(run|add|remove|use|ensure|check|grep|rg|fix|avoid|prefer|must|should)\b`

func TestSkillRegexContract_SpecificityPatterns(t *testing.T) {
	t.Run("file_extensions", func(t *testing.T) {
		re := regexp.MustCompile(`\b[a-zA-Z0-9_./-]+\.(go|ts|js|py|sh|yaml|yml|json|md)\b`)
		tests := []struct {
			name  string
			input string
			want  bool
		}{
			{"go file", "edit inject.go", true},
			{"typescript", "check main.ts", true},
			{"python", "run test.py", true},
			{"yaml", "update config.yaml", true},
			{"json", "parse data.json", true},
			{"markdown", "read README.md", true},
			{"path with slash", "cli/cmd/ao/inject.go reference", true},
			{"dotted path", "internal.parser.go", true},

			{"no extension", "just a word", false},
			{"unknown ext", "file.xyz", false},
			{"extension only", ".go", false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if re.MatchString(tt.input) != tt.want {
					t.Errorf("match=%v, want %v for %q", !tt.want, tt.want, tt.input)
				}
			})
		}
	})

	t.Run("bullet_list", func(t *testing.T) {
		re := regexp.MustCompile(`(?m)^\s*[-*]\s+`)
		tests := []struct {
			name  string
			input string
			want  bool
		}{
			{"dash bullet", "- item one", true},
			{"asterisk bullet", "* item one", true},
			{"indented bullet", "  - nested item", true},
			{"multiline", "text\n- bullet\nmore", true},

			{"empty", "", false},
			{"no bullet", "just text", false},
			{"dash without space", "-noSpace", false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if re.MatchString(tt.input) != tt.want {
					t.Errorf("match=%v, want %v for %q", !tt.want, tt.want, tt.input)
				}
			})
		}
	})

	t.Run("action_verbs", func(t *testing.T) {
		re := regexp.MustCompile(`(?i)\b(run|add|remove|use|ensure|check|grep|rg|fix|avoid|prefer|must|should)\b`)
		tests := []struct {
			name  string
			input string
			want  bool
		}{
			{"run", "run the tests", true},
			{"must uppercase", "MUST validate input", true},
			{"should mixed", "Should check errors", true},
			{"avoid", "avoid using eval", true},
			{"fix", "fix the nil pointer", true},
			{"grep", "grep for patterns", true},

			{"empty", "", false},
			{"no verbs", "the quick brown fox", false},
			{"partial match running", "running tests", false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if re.MatchString(tt.input) != tt.want {
					t.Errorf("match=%v, want %v for %q", !tt.want, tt.want, tt.input)
				}
			})
		}
	})
}

// --- RPI run ID pattern (rpi_serve.go:35) ---
// Pattern: `^(rpi-[a-f0-9]{8,12}|[a-f0-9]{12})$`

func TestSkillRegexContract_RPIRunID(t *testing.T) {
	pattern := `^(rpi-[a-f0-9]{8,12}|[a-f0-9]{12})$`
	re := regexp.MustCompile(pattern)

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"rpi prefix 8 hex", "rpi-abcdef01", true},
		{"rpi prefix 10 hex", "rpi-abcdef0123", true},
		{"rpi prefix 12 hex", "rpi-abcdef012345", true},
		{"bare 8 hex digits", "abcdef01", false},
		{"12 hex digits", "abcdef012345", true},

		{"empty", "", false},
		{"rpi with short hex", "rpi-abcde", false},
		{"10 hex digits bare", "abcdef0123", false},
		{"uppercase hex", "ABCDEF01", false},
		{"rpi uppercase", "RPI-abcdef01", false},
		{"extra chars", "rpi-abcdef01x", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if re.MatchString(tt.input) != tt.want {
				t.Errorf("match=%v, want %v for %q", !tt.want, tt.want, tt.input)
			}
		})
	}
}

// --- Context-related regexes (context.go, context_assemble.go) ---

func TestSkillRegexContract_ContextPatterns(t *testing.T) {
	t.Run("filename_sanitizer", func(t *testing.T) {
		// Pattern: `[^a-zA-Z0-9._-]+`
		re := regexp.MustCompile(`[^a-zA-Z0-9._-]+`)
		tests := []struct {
			name   string
			input  string
			result string // after ReplaceAllString with "-"
		}{
			{"clean filename", "my-file_v1.2.txt", "my-file_v1.2.txt"},
			{"spaces", "my file name", "my-file-name"},
			{"special chars", "foo@bar#baz", "foo-bar-baz"},
			{"slashes", "path/to/file", "path-to-file"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got := re.ReplaceAllString(tt.input, "-")
				if got != tt.result {
					t.Errorf("sanitized = %q, want %q", got, tt.result)
				}
			})
		}
	})

	t.Run("context_issue_pattern", func(t *testing.T) {
		// Pattern: `(?i)\bag-[a-z0-9]+\b`
		re := regexp.MustCompile(`(?i)\bag-[a-z0-9]+\b`)
		tests := []struct {
			name  string
			input string
			want  bool
		}{
			{"lowercase", "working on ag-test", true},
			{"uppercase", "see AG-TEST", true},
			{"mixed case", "issue Ag-Xyz", true},

			{"empty", "", false},
			{"no ag prefix", "issue bd-test", false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if re.MatchString(tt.input) != tt.want {
					t.Errorf("match=%v, want %v for %q", !tt.want, tt.want, tt.input)
				}
			})
		}
	})

	t.Run("env_var_line", func(t *testing.T) {
		// Pattern: `(?i).*(KEY|TOKEN|SECRET|PASSWORD|API).*`
		re := regexp.MustCompile(`(?i).*(KEY|TOKEN|SECRET|PASSWORD|API).*`)
		tests := []struct {
			name  string
			input string
			want  bool
		}{
			{"api key", "GITHUB_API_KEY=abc", true},
			{"token", "my auth token is here", true},
			{"secret", "SECRET_VALUE=hidden", true},
			{"password", "user password reset", true},
			{"lowercase", "api endpoint url", true},

			{"empty", "", false},
			{"unrelated", "just regular text", false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if re.MatchString(tt.input) != tt.want {
					t.Errorf("match=%v, want %v for %q", !tt.want, tt.want, tt.input)
				}
			})
		}
	})

	t.Run("jwt_pattern", func(t *testing.T) {
		// Pattern: `eyJ[A-Za-z0-9_-]+\.eyJ[A-Za-z0-9_-]+`
		re := regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.eyJ[A-Za-z0-9_-]+`)
		tests := []struct {
			name  string
			input string
			want  bool
		}{
			{"valid jwt prefix", "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkw", true},
			{"in context", "Bearer eyJhbGciOiJSUzI1NiJ9.eyJpc3MiOiJ0ZXN0In0 more text", true},

			{"empty", "", false},
			{"partial", "eyJhbGci", false},
			{"no second eyJ", "eyJhbGci.notJWT", false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if re.MatchString(tt.input) != tt.want {
					t.Errorf("match=%v, want %v for %q", !tt.want, tt.want, tt.input)
				}
			})
		}
	})
}

// --- Extraction patterns from parser/extractor.go ---

func TestSkillRegexContract_KnowledgeExtractionPatterns(t *testing.T) {
	t.Run("decision_bold_marker", func(t *testing.T) {
		re := regexp.MustCompile(`(?i)\*\*Decision\*\*:`)
		tests := []struct {
			name  string
			input string
			want  bool
		}{
			{"standard", "**Decision**: Use X over Y", true},
			{"lowercase", "**decision**: use X", true},
			{"mixed case", "**DECISION**: picked X", true},

			{"no bold", "Decision: Use X", false},
			{"single asterisk", "*Decision*: Use X", false},
			{"no colon", "**Decision** Use X", false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if re.MatchString(tt.input) != tt.want {
					t.Errorf("match=%v, want %v for %q", !tt.want, tt.want, tt.input)
				}
			})
		}
	})

	t.Run("decided_to_use", func(t *testing.T) {
		re := regexp.MustCompile(`(?i)decided to use \w+`)
		tests := []struct {
			name  string
			input string
			want  bool
		}{
			{"standard", "decided to use context for cancellation", true},
			{"uppercase", "DECIDED TO USE mutex for safety", true},

			{"past tense different", "I was deciding to use X", false},
			{"missing word", "decided to use", false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if re.MatchString(tt.input) != tt.want {
					t.Errorf("match=%v, want %v for %q", !tt.want, tt.want, tt.input)
				}
			})
		}
	})

	t.Run("chose_over", func(t *testing.T) {
		re := regexp.MustCompile(`(?i)chose (\w+) (over|instead of) \w+`)
		tests := []struct {
			name  string
			input string
			want  bool
		}{
			{"chose over", "chose mutex over channel", true},
			{"chose instead of", "chose Go instead of Python", true},

			{"wrong preposition", "chose Go because of speed", false},
			{"empty", "", false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if re.MatchString(tt.input) != tt.want {
					t.Errorf("match=%v, want %v for %q", !tt.want, tt.want, tt.input)
				}
			})
		}
	})

	t.Run("solution_bold_marker", func(t *testing.T) {
		re := regexp.MustCompile(`(?i)\*\*Solution\*\*:`)
		tests := []struct {
			name  string
			input string
			want  bool
		}{
			{"standard", "**Solution**: Add nil check before dereference", true},
			{"lowercase", "**solution**: restart the service", true},

			{"no bold", "Solution: restart", false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if re.MatchString(tt.input) != tt.want {
					t.Errorf("match=%v, want %v for %q", !tt.want, tt.want, tt.input)
				}
			})
		}
	})

	t.Run("url_pattern", func(t *testing.T) {
		re := regexp.MustCompile(`https?://[^\s]+`)
		tests := []struct {
			name  string
			input string
			want  bool
		}{
			{"https", "see https://example.com/docs", true},
			{"http", "link: http://localhost:8080/api", true},
			{"with path", "https://github.com/boshu2/agentops/issues/42", true},

			{"empty", "", false},
			{"no scheme", "example.com/docs", false},
			{"ftp", "ftp://files.example.com", false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if re.MatchString(tt.input) != tt.want {
					t.Errorf("match=%v, want %v for %q", !tt.want, tt.want, tt.input)
				}
			})
		}
	})
}

// --- Session outcome regexes (session_outcome.go:63-72) ---

func TestSkillRegexContract_SessionOutcomePatterns(t *testing.T) {
	t.Run("test_pass", func(t *testing.T) {
		re := regexp.MustCompile(`(?i)(PASSED|tests? passed|✓|ok$|All tests passed)`)
		tests := []struct {
			name  string
			input string
			want  bool
		}{
			{"PASSED", "PASSED", true},
			{"test passed", "test passed", true},
			{"tests passed", "tests passed", true},
			{"checkmark", "✓", true},
			{"ok at end", "ok", true},
			{"all tests passed", "All tests passed", true},

			{"empty", "", false},
			{"FAILED", "FAILED", false},
			{"ok in middle", "ok then more", false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if re.MatchString(tt.input) != tt.want {
					t.Errorf("match=%v, want %v for %q", !tt.want, tt.want, tt.input)
				}
			})
		}
	})

	t.Run("test_fail", func(t *testing.T) {
		re := regexp.MustCompile(`(?i)(FAILED|FAILURE|tests? failed|✗|ERROR.*test)`)
		tests := []struct {
			name  string
			input string
			want  bool
		}{
			{"FAILED", "FAILED", true},
			{"FAILURE", "FAILURE", true},
			{"test failed", "test failed", true},
			{"x mark", "✗", true},
			{"error test", "ERROR in test suite", true},

			{"empty", "", false},
			{"PASSED", "PASSED", false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if re.MatchString(tt.input) != tt.want {
					t.Errorf("match=%v, want %v for %q", !tt.want, tt.want, tt.input)
				}
			})
		}
	})

	t.Run("exit_zero", func(t *testing.T) {
		re := regexp.MustCompile(`exit (code|status):?\s*0`)
		tests := []struct {
			name  string
			input string
			want  bool
		}{
			{"exit code 0", "exit code 0", true},
			{"exit status 0", "exit status 0", true},
			{"with colon", "exit code: 0", true},

			{"exit code 1", "exit code 1", false},
			{"empty", "", false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if re.MatchString(tt.input) != tt.want {
					t.Errorf("match=%v, want %v for %q", !tt.want, tt.want, tt.input)
				}
			})
		}
	})

	t.Run("exit_nonzero", func(t *testing.T) {
		re := regexp.MustCompile(`exit (code|status):?\s*[1-9]\d*`)
		tests := []struct {
			name  string
			input string
			want  bool
		}{
			{"exit code 1", "exit code 1", true},
			{"exit status 127", "exit status 127", true},
			{"with colon", "exit code: 2", true},

			{"exit code 0", "exit code 0", false},
			{"empty", "", false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if re.MatchString(tt.input) != tt.want {
					t.Errorf("match=%v, want %v for %q", !tt.want, tt.want, tt.input)
				}
			})
		}
	})
}

// --- Shell safety regex (rpi_phased_tmux.go:329) ---
// Pattern: `[^a-zA-Z0-9_-]`

func TestSkillRegexContract_ShellSafety(t *testing.T) {
	re := regexp.MustCompile(`[^a-zA-Z0-9_-]`)

	tests := []struct {
		name  string
		input string
		safe  bool // true if NO match (all chars are safe)
	}{
		{"alphanumeric", "abc123", true},
		{"with hyphen", "my-session", true},
		{"with underscore", "my_session", true},
		{"with space", "my session", false},
		{"with semicolon", "cmd;evil", false},
		{"with backtick", "cmd`evil`", false},
		{"with dollar", "var$HOME", false},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasDangerous := re.MatchString(tt.input)
			if hasDangerous == tt.safe {
				t.Errorf("safe=%v, want %v for %q", !tt.safe, tt.safe, tt.input)
			}
		})
	}
}
