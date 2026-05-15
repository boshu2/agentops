// practices: [design-by-contract, code-complete]
package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestCouncilVerdictHeadingContract verifies that the wrapper skills used by
// extractCouncilVerdict (pre-mortem, vibe, post-mortem) each contain the
// exact heading "## Council Verdict:" that the CLI regex depends on.
//
// The regex in rpi_phased_processing.go is:
//
//	regexp.MustCompile(`(?m)^## Council Verdict:\s*(PASS|WARN|FAIL)`)
//
// If any wrapper skill is missing this heading, the CLI will fail to extract
// the verdict from council reports produced by those skills.
func TestCouncilVerdictHeadingContract(t *testing.T) {
	// Walk up from cli/cmd/ao to find the repo root (the directory containing skills/).
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}

	// Ascend until we find a skills/ directory or exhaust the path.
	repoRoot := ""
	dir := cwd
	for {
		candidate := filepath.Join(dir, "skills")
		if info, statErr := os.Stat(candidate); statErr == nil && info.IsDir() {
			repoRoot = dir
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root without finding skills/.
			break
		}
		dir = parent
	}

	if repoRoot == "" {
		t.Skip("skills/ directory not found in any ancestor of cwd; skipping contract test")
	}

	skillsDir := filepath.Join(repoRoot, "skills")

	wrapperSkills := []string{
		"pre-mortem",
		"vibe",
		"post-mortem",
	}

	const requiredHeading = "## Council Verdict:"

	for _, skill := range wrapperSkills {
		skill := skill // capture loop variable
		t.Run(skill, func(t *testing.T) {
			// Billboard architecture: a skill's detail may live in references/.
			// The verdict-heading contract holds if SKILL.md OR any
			// references/*.md carries the heading instruction — the skill is
			// checked as a unit, not SKILL.md alone.
			skillDir := filepath.Join(skillsDir, skill)
			skillFile := filepath.Join(skillDir, "SKILL.md")
			if _, err := os.Stat(skillFile); err != nil {
				t.Fatalf("could not stat %s: %v", skillFile, err)
			}

			candidates := []string{skillFile}
			refs, _ := filepath.Glob(filepath.Join(skillDir, "references", "*.md"))
			candidates = append(candidates, refs...)

			found := false
			for _, f := range candidates {
				data, readErr := os.ReadFile(f)
				if readErr != nil {
					continue
				}
				if strings.Contains(string(data), requiredHeading) {
					found = true
					break
				}
			}

			if !found {
				t.Errorf(
					"%s skill (SKILL.md + references/) is missing the required heading %q\n"+
						"The CLI regex in extractCouncilVerdict depends on this heading being present.\n"+
						"Regex: `(?m)^## Council Verdict:\\s*(PASS|WARN|FAIL)`",
					skill, requiredHeading,
				)
			}
		})
	}
}

// findSkillsDir walks up from the current working directory to find the
// skills/ directory. Returns the path if found, or "" if not found.
func findSkillsDir(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	dir := cwd
	for {
		candidate := filepath.Join(dir, "skills")
		if info, statErr := os.Stat(candidate); statErr == nil && info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// TestSkillContract_FrontmatterYAMLParseable verifies every SKILL.md that has
// YAML frontmatter (--- delimiters) can be parsed by the same extractFrontmatter
// function that the CLI uses for --for context injection.
func TestSkillContract_FrontmatterYAMLParseable(t *testing.T) {
	skillsDir := findSkillsDir(t)
	if skillsDir == "" {
		t.Skip("skills/ directory not found; skipping frontmatter contract test")
	}
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		t.Fatalf("read skills dir: %v", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		t.Run(name, func(t *testing.T) {
			skillFile := filepath.Join(skillsDir, name, "SKILL.md")
			data, err := os.ReadFile(skillFile)
			if err != nil {
				t.Skipf("no SKILL.md: %v", err)
			}

			content := string(data)
			if !strings.HasPrefix(strings.TrimSpace(content), "---") {
				t.Skip("no frontmatter (no leading ---)")
			}

			fm, err := extractFrontmatter(content)
			if err != nil {
				t.Errorf("extractFrontmatter failed for %s/SKILL.md: %v", name, err)
			}
			if fm == "" {
				t.Errorf("%s/SKILL.md starts with --- but extractFrontmatter returned empty", name)
			}
		})
	}
}

// TestSkillContract_CouncilVerdictRegexMatchesReportFormat verifies that the
// Council Verdict regex in extractCouncilVerdict actually matches the format
// that council wrapper skills instruct the model to produce.
func TestSkillContract_CouncilVerdictRegexMatchesReportFormat(t *testing.T) {
	re := regexp.MustCompile(`(?m)^## Council Verdict:\s*(PASS|WARN|FAIL)`)

	// These are the formats that council wrapper skills produce
	validFormats := []struct {
		name  string
		input string
		want  string
	}{
		{"PASS verdict", "## Council Verdict: PASS", "PASS"},
		{"WARN verdict", "## Council Verdict: WARN", "WARN"},
		{"FAIL verdict", "## Council Verdict: FAIL", "FAIL"},
		{"PASS with trailing text", "## Council Verdict: PASS\nSome details follow", "PASS"},
		{"embedded in report", "# Vibe Report\n\nSome analysis\n\n## Council Verdict: WARN\n\n## Details", "WARN"},
	}

	for _, tc := range validFormats {
		t.Run(tc.name, func(t *testing.T) {
			matches := re.FindStringSubmatch(tc.input)
			if len(matches) < 2 {
				t.Errorf("regex did not match %q", tc.input)
				return
			}
			if matches[1] != tc.want {
				t.Errorf("captured %q, want %q", matches[1], tc.want)
			}
		})
	}

	// These should NOT match
	invalidFormats := []struct {
		name  string
		input string
	}{
		{"wrong heading level", "# Council Verdict: PASS"},
		{"### heading level", "### Council Verdict: PASS"},
		{"no space after ##", "##Council Verdict: PASS"},
		{"lowercase verdict", "## Council Verdict: pass"},
		{"unknown verdict", "## Council Verdict: OK"},
		{"missing colon", "## Council Verdict PASS"},
	}

	for _, tc := range invalidFormats {
		t.Run("no_match/"+tc.name, func(t *testing.T) {
			if re.MatchString(tc.input) {
				t.Errorf("regex should NOT match %q", tc.input)
			}
		})
	}
}

// TestSkillContract_FindingsRegexMatchesReportFormat verifies that the two
// findings extraction regexes in extractCouncilFindings match the actual
// format produced by council reports.
func TestSkillContract_FindingsRegexMatchesReportFormat(t *testing.T) {
	// Structured findings format: FINDING: ... | FIX: ... | REF: ...
	reStructured := regexp.MustCompile(`(?m)FINDING:\s*(.+?)\s*\|\s*FIX:\s*(.+?)\s*\|\s*REF:\s*(.+?)$`)

	t.Run("structured_findings", func(t *testing.T) {
		tests := []struct {
			name    string
			input   string
			finding string
			fix     string
			ref     string
		}{
			{
				"standard format",
				"FINDING: Missing error handling | FIX: Add nil check | REF: cmd/ao/inject.go:42",
				"Missing error handling",
				"Add nil check",
				"cmd/ao/inject.go:42",
			},
			{
				"extra whitespace",
				"FINDING:  Unused variable  |  FIX:  Remove or use it  |  REF:  pool_ingest.go:100",
				"Unused variable",
				"Remove or use it",
				"pool_ingest.go:100",
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				matches := reStructured.FindStringSubmatch(tc.input)
				if len(matches) < 4 {
					t.Fatalf("regex did not match %q", tc.input)
				}
				if strings.TrimSpace(matches[1]) != tc.finding {
					t.Errorf("finding = %q, want %q", strings.TrimSpace(matches[1]), tc.finding)
				}
				if strings.TrimSpace(matches[2]) != tc.fix {
					t.Errorf("fix = %q, want %q", strings.TrimSpace(matches[2]), tc.fix)
				}
				if strings.TrimSpace(matches[3]) != tc.ref {
					t.Errorf("ref = %q, want %q", strings.TrimSpace(matches[3]), tc.ref)
				}
			})
		}
	})

	// Fallback findings format: numbered list with bold title and em-dash separator
	reFallback := regexp.MustCompile(`(?m)^\d+\.\s+\*\*(.+?)\*\*\s*[—–-]\s*(.+)$`)

	t.Run("fallback_findings", func(t *testing.T) {
		tests := []struct {
			name  string
			input string
			title string
			desc  string
		}{
			{
				"em dash separator",
				"1. **Missing guard clause** — inject.go should check nil before access",
				"Missing guard clause",
				"inject.go should check nil before access",
			},
			{
				"en dash separator",
				"2. **Race condition** – goroutine access without mutex",
				"Race condition",
				"goroutine access without mutex",
			},
			{
				"hyphen separator",
				"3. **Dead code** - unused helper function on line 42",
				"Dead code",
				"unused helper function on line 42",
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				matches := reFallback.FindStringSubmatch(tc.input)
				if len(matches) < 3 {
					t.Fatalf("regex did not match %q", tc.input)
				}
				if matches[1] != tc.title {
					t.Errorf("title = %q, want %q", matches[1], tc.title)
				}
				if strings.TrimSpace(matches[2]) != tc.desc {
					t.Errorf("desc = %q, want %q", strings.TrimSpace(matches[2]), tc.desc)
				}
			})
		}
	})
}

// TestSkillContract_AllSKILLMDHaveDescription verifies each skill directory
// contains a SKILL.md with at least a top-level heading (#).
func TestSkillContract_AllSKILLMDHaveDescription(t *testing.T) {
	skillsDir := findSkillsDir(t)
	if skillsDir == "" {
		t.Skip("skills/ directory not found; skipping")
	}
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		t.Fatalf("read skills dir: %v", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		t.Run(name, func(t *testing.T) {
			skillFile := filepath.Join(skillsDir, name, "SKILL.md")
			data, err := os.ReadFile(skillFile)
			if err != nil {
				t.Fatalf("could not read %s/SKILL.md: %v", name, err)
			}
			content := string(data)
			if !strings.Contains(content, "# ") {
				t.Errorf("%s/SKILL.md has no heading (no '# ' found)", name)
			}
		})
	}
}

// TestSkillContract_ReferencesLinkedInSKILLMD verifies the heal.sh contract:
// every file in skills/<name>/references/ must be linked or referenced in the
// corresponding SKILL.md.
func TestSkillContract_ReferencesLinkedInSKILLMD(t *testing.T) {
	skillsDir := findSkillsDir(t)
	if skillsDir == "" {
		t.Skip("skills/ directory not found; skipping")
	}
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		t.Fatalf("read skills dir: %v", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillName := entry.Name()
		refsDir := filepath.Join(skillsDir, skillName, "references")
		if _, err := os.Stat(refsDir); os.IsNotExist(err) {
			continue // no references directory, skip
		}

		refEntries, err := os.ReadDir(refsDir)
		if err != nil {
			t.Logf("warning: could not read %s/references: %v", skillName, err)
			continue
		}

		skillFile := filepath.Join(skillsDir, skillName, "SKILL.md")
		skillData, err := os.ReadFile(skillFile)
		if err != nil {
			t.Errorf("could not read %s/SKILL.md: %v", skillName, err)
			continue
		}
		skillContent := string(skillData)

		for _, ref := range refEntries {
			if ref.IsDir() || strings.HasPrefix(ref.Name(), ".") {
				continue
			}
			refName := ref.Name()
			t.Run(skillName+"/"+refName, func(t *testing.T) {
				if !strings.Contains(skillContent, refName) {
					t.Errorf("%s/SKILL.md does not reference %s (from references/ directory)", skillName, refName)
				}
			})
		}
	}
}
