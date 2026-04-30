package context

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRender_TableDriven exercises the pure Render function across the
// behavior matrix: happy path, missing required input, blank-section
// fallbacks, custom CharsPerToken, and learning-ID propagation.
func TestRender_TableDriven(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		in             BriefRenderInput
		wantErr        bool
		wantErrSubstr  string
		wantMarkdown   string
		wantBeadID     string
		wantLearnings  []string
		wantTokenCount int
	}{
		{
			name: "happy path with all sections populated",
			in: BriefRenderInput{
				BeadID:        "agentops-0zh",
				Task:          "Port brief.Render() pattern.",
				Context:       "Olympus → agentopsd extraction.",
				Learnings:     "- learn-1: pure functions compound",
				Navigation:    "- `cli/internal/context/`",
				LearningIDs:   []string{"learn-1"},
				CharsPerToken: 4,
			},
			wantBeadID: "agentops-0zh",
			wantMarkdown: "# Briefing: agentops-0zh\n\n" +
				"## Task\nPort brief.Render() pattern.\n\n" +
				"## Context\nOlympus → agentopsd extraction.\n\n" +
				"## Learnings\n- learn-1: pure functions compound\n\n" +
				"## Navigation\n- `cli/internal/context/`\n",
			wantLearnings:  []string{"learn-1"},
			wantTokenCount: 49, // len(markdown) == 199, 199 / 4 == 49
		},
		{
			name: "blank sections fall back to default placeholders",
			in: BriefRenderInput{
				BeadID: "agentops-blank",
			},
			wantBeadID: "agentops-blank",
			wantMarkdown: "# Briefing: agentops-blank\n\n" +
				"## Task\nNo task content provided.\n\n" +
				"## Context\nNo context provided.\n\n" +
				"## Learnings\nNo relevant learnings found.\n\n" +
				"## Navigation\nNo AGENTS.md files found in repository.\n",
			wantLearnings:  nil,
			wantTokenCount: 48, // len(markdown) == 193, 193 / 4 == 48
		},
		{
			name: "whitespace-only sections also trigger fallbacks",
			in: BriefRenderInput{
				BeadID:     "agentops-ws",
				Task:       "   ",
				Context:    "\n\t\n",
				Learnings:  "",
				Navigation: " ",
			},
			wantBeadID: "agentops-ws",
			wantMarkdown: "# Briefing: agentops-ws\n\n" +
				"## Task\nNo task content provided.\n\n" +
				"## Context\nNo context provided.\n\n" +
				"## Learnings\nNo relevant learnings found.\n\n" +
				"## Navigation\nNo AGENTS.md files found in repository.\n",
			wantLearnings:  nil,
			wantTokenCount: 47, // len(markdown) == 190, 190 / 4 == 47
		},
		{
			name: "custom CharsPerToken affects only token count",
			in: BriefRenderInput{
				BeadID:        "agentops-tok",
				Task:          "T",
				Context:       "C",
				Learnings:     "L",
				Navigation:    "N",
				CharsPerToken: 8,
			},
			wantBeadID: "agentops-tok",
			wantMarkdown: "# Briefing: agentops-tok\n\n" +
				"## Task\nT\n\n" +
				"## Context\nC\n\n" +
				"## Learnings\nL\n\n" +
				"## Navigation\nN\n",
			wantLearnings:  nil,
			wantTokenCount: 10, // len(markdown) == 83, 83 / 8 == 10
		},
		{
			name: "zero CharsPerToken defaults to 4",
			in: BriefRenderInput{
				BeadID:        "agentops-def",
				Task:          "T",
				Context:       "C",
				Learnings:     "L",
				Navigation:    "N",
				CharsPerToken: 0,
			},
			wantBeadID: "agentops-def",
			wantMarkdown: "# Briefing: agentops-def\n\n" +
				"## Task\nT\n\n" +
				"## Context\nC\n\n" +
				"## Learnings\nL\n\n" +
				"## Navigation\nN\n",
			wantLearnings:  nil,
			wantTokenCount: 20, // len(markdown) == 83, 83 / 4 == 20
		},
		{
			name: "negative CharsPerToken defaults to 4",
			in: BriefRenderInput{
				BeadID:        "agentops-neg",
				Task:          "T",
				Context:       "C",
				Learnings:     "L",
				Navigation:    "N",
				CharsPerToken: -42,
			},
			wantBeadID: "agentops-neg",
			wantMarkdown: "# Briefing: agentops-neg\n\n" +
				"## Task\nT\n\n" +
				"## Context\nC\n\n" +
				"## Learnings\nL\n\n" +
				"## Navigation\nN\n",
			wantLearnings:  nil,
			wantTokenCount: 20, // len(markdown) == 83, 83 / 4 == 20
		},
		{
			name: "multiple learning IDs preserved in order",
			in: BriefRenderInput{
				BeadID:      "agentops-multi",
				Task:        "task",
				Context:     "ctx",
				Learnings:   "stuff",
				Navigation:  "nav",
				LearningIDs: []string{"a", "b", "c"},
			},
			wantBeadID:    "agentops-multi",
			wantLearnings: []string{"a", "b", "c"},
		},
		{
			name:          "empty BeadID is rejected",
			in:            BriefRenderInput{BeadID: ""},
			wantErr:       true,
			wantErrSubstr: "BeadID is required",
		},
		{
			name:          "whitespace-only BeadID is rejected",
			in:            BriefRenderInput{BeadID: "   \t\n"},
			wantErr:       true,
			wantErrSubstr: "BeadID is required",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			out, err := Render(tc.in)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("Render(%q): want error containing %q, got nil", tc.name, tc.wantErrSubstr)
				}
				if !strings.Contains(err.Error(), tc.wantErrSubstr) {
					t.Fatalf("Render(%q): error = %q, want substring %q", tc.name, err.Error(), tc.wantErrSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Render(%q): unexpected error: %v", tc.name, err)
			}

			if out.BeadID != tc.wantBeadID {
				t.Errorf("BeadID = %q, want %q", out.BeadID, tc.wantBeadID)
			}
			if tc.wantMarkdown != "" && out.Markdown != tc.wantMarkdown {
				t.Errorf("Markdown mismatch.\n--- got ---\n%s\n--- want ---\n%s", out.Markdown, tc.wantMarkdown)
			}
			if tc.wantTokenCount != 0 && out.TokenCount != tc.wantTokenCount {
				t.Errorf("TokenCount = %d, want %d (markdown len=%d)", out.TokenCount, tc.wantTokenCount, len(out.Markdown))
			}
			if !equalStringSlices(out.LearningIDs, tc.wantLearnings) {
				t.Errorf("LearningIDs = %v, want %v", out.LearningIDs, tc.wantLearnings)
			}
		})
	}
}

// TestRender_Deterministic asserts the pure-function contract: the same input
// produces byte-identical output across repeated invocations. If anyone
// reintroduces a clock, ID minter, or env read, this fails immediately.
func TestRender_Deterministic(t *testing.T) {
	t.Parallel()

	in := BriefRenderInput{
		BeadID:      "agentops-det",
		Task:        "deterministic task",
		Context:     "deterministic context",
		Learnings:   "deterministic learnings",
		Navigation:  "deterministic navigation",
		LearningIDs: []string{"l-1", "l-2"},
	}

	first, err := Render(in)
	if err != nil {
		t.Fatalf("Render first call: %v", err)
	}
	second, err := Render(in)
	if err != nil {
		t.Fatalf("Render second call: %v", err)
	}

	if first.Markdown != second.Markdown {
		t.Fatalf("non-deterministic Markdown:\nfirst:\n%s\nsecond:\n%s", first.Markdown, second.Markdown)
	}
	if first.TokenCount != second.TokenCount {
		t.Errorf("non-deterministic TokenCount: %d vs %d", first.TokenCount, second.TokenCount)
	}
	if !equalStringSlices(first.LearningIDs, second.LearningIDs) {
		t.Errorf("non-deterministic LearningIDs: %v vs %v", first.LearningIDs, second.LearningIDs)
	}
}

// TestRender_LearningIDsDefensiveCopy asserts that mutating the input slice
// after Render() does not affect the output. The output owns its own slice.
func TestRender_LearningIDsDefensiveCopy(t *testing.T) {
	t.Parallel()

	ids := []string{"a", "b", "c"}
	out, err := Render(BriefRenderInput{
		BeadID:      "agentops-copy",
		Task:        "t",
		Context:     "c",
		Learnings:   "l",
		Navigation:  "n",
		LearningIDs: ids,
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	// Mutate the caller's slice.
	ids[0] = "MUTATED"

	if out.LearningIDs[0] != "a" {
		t.Errorf("output LearningIDs leaked input slice: got %q, want %q", out.LearningIDs[0], "a")
	}
}

// TestRenderAndPersist_WritesExactRenderOutput is the L2 integration test:
// drive the thin shell against a real tempdir and assert the bytes on disk
// equal Render()'s Markdown exactly, and the returned struct equals the pure
// Render result.
func TestRenderAndPersist_WritesExactRenderOutput(t *testing.T) {
	t.Parallel()

	in := BriefRenderInput{
		BeadID:      "agentops-persist",
		Task:        "persist task",
		Context:     "persist context",
		Learnings:   "persist learnings",
		Navigation:  "persist navigation",
		LearningIDs: []string{"l-99"},
	}

	pure, err := Render(in)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	dir := t.TempDir()
	dst := filepath.Join(dir, "subdir", "briefing.md")

	out, err := RenderAndPersist(in, dst)
	if err != nil {
		t.Fatalf("RenderAndPersist: %v", err)
	}

	if out.Markdown != pure.Markdown {
		t.Errorf("returned Markdown != pure Render Markdown")
	}
	if out.BeadID != pure.BeadID {
		t.Errorf("returned BeadID = %q, want %q", out.BeadID, pure.BeadID)
	}
	if out.TokenCount != pure.TokenCount {
		t.Errorf("returned TokenCount = %d, want %d", out.TokenCount, pure.TokenCount)
	}
	if !equalStringSlices(out.LearningIDs, pure.LearningIDs) {
		t.Errorf("returned LearningIDs = %v, want %v", out.LearningIDs, pure.LearningIDs)
	}

	gotBytes, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("reading written file: %v", err)
	}
	if string(gotBytes) != pure.Markdown {
		t.Errorf("on-disk content mismatch.\n--- on disk ---\n%s\n--- want ---\n%s", string(gotBytes), pure.Markdown)
	}

	// Verify parent dir was created with the expected mode (0755).
	parentInfo, err := os.Stat(filepath.Dir(dst))
	if err != nil {
		t.Fatalf("statting parent dir: %v", err)
	}
	if !parentInfo.IsDir() {
		t.Errorf("expected parent to be a directory")
	}
}

// TestRenderAndPersist_ErrorPaths covers the failure surfaces: bad input
// rejected by Render and bad destination paths rejected by the shell.
func TestRenderAndPersist_ErrorPaths(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		in            BriefRenderInput
		dst           string
		wantErrSubstr string
	}{
		{
			name:          "render error propagates with prefix",
			in:            BriefRenderInput{BeadID: ""},
			dst:           filepath.Join(t.TempDir(), "x.md"),
			wantErrSubstr: "BeadID is required",
		},
		{
			name:          "empty dst is rejected",
			in:            BriefRenderInput{BeadID: "agentops-x", Task: "t"},
			dst:           "",
			wantErrSubstr: "dst path is required",
		},
		{
			name:          "whitespace dst is rejected",
			in:            BriefRenderInput{BeadID: "agentops-x", Task: "t"},
			dst:           "   ",
			wantErrSubstr: "dst path is required",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := RenderAndPersist(tc.in, tc.dst)
			if err == nil {
				t.Fatalf("want error containing %q, got nil", tc.wantErrSubstr)
			}
			if !strings.Contains(err.Error(), tc.wantErrSubstr) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tc.wantErrSubstr)
			}
		})
	}
}

// TestRenderAndPersist_OverwritesExistingFile asserts that re-running with the
// same destination overwrites cleanly. Briefings get re-rendered as beads
// evolve; a stale-truncate bug here would silently corrupt context.
func TestRenderAndPersist_OverwritesExistingFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dst := filepath.Join(dir, "briefing.md")

	// Pre-populate the destination with longer junk content.
	junk := strings.Repeat("XXXXXXXXXX\n", 100)
	if err := os.WriteFile(dst, []byte(junk), 0o644); err != nil {
		t.Fatalf("seeding dst: %v", err)
	}

	in := BriefRenderInput{
		BeadID:     "agentops-over",
		Task:       "short",
		Context:    "short",
		Learnings:  "short",
		Navigation: "short",
	}
	out, err := RenderAndPersist(in, dst)
	if err != nil {
		t.Fatalf("RenderAndPersist: %v", err)
	}

	gotBytes, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("reading dst: %v", err)
	}
	if string(gotBytes) != out.Markdown {
		t.Errorf("on-disk content not overwritten.\n--- on disk ---\n%s\n--- want ---\n%s", string(gotBytes), out.Markdown)
	}
	if strings.Contains(string(gotBytes), "XXXXXXXXXX") {
		t.Errorf("overwrite did not truncate; junk content still present")
	}
}

// equalStringSlices is a small test helper. Returns true iff a and b have the
// same length and identical elements at each index. Treats nil and []string{}
// as equal (both length 0).
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
