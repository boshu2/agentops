package main

import (
	"math"
	"sort"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// artifactTypeFromPath
// ---------------------------------------------------------------------------

func TestArtifactTypeFromPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "learnings dir", path: "/home/user/.agents/learnings/mutex.md", want: "learning"},
		{name: "patterns dir", path: "/home/user/.agents/patterns/retry.md", want: "pattern"},
		{name: "research dir", path: "/home/user/.agents/research/deep-dive.md", want: "research"},
		{name: "retros dir", path: "/home/user/.agents/retro/2026-02-22.md", want: "retro"},
		{name: "candidates dir", path: "/home/user/.agents/candidates/idea.md", want: "candidate"},
		{name: "unknown dir", path: "/home/user/.agents/misc/notes.md", want: "unknown"},
		{name: "empty path", path: "", want: "unknown"},
		{name: "nested learnings", path: "/a/b/learnings/sub/deep.md", want: "learning"},
		{name: "no agents prefix", path: "/learnings/foo.md", want: "learning"},
		{name: "ambiguous first match wins", path: "/learnings/patterns/both.md", want: "learning"},
		{name: "just a filename", path: "file.md", want: "unknown"},
		{name: "trailing slash only", path: "/learnings/", want: "learning"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := artifactTypeFromPath(tt.path)
			if got != tt.want {
				t.Errorf("artifactTypeFromPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// appendCategoryKeywords
// ---------------------------------------------------------------------------

func TestAppendCategoryKeywords(t *testing.T) {
	tests := []struct {
		name     string
		keywords []string
		category string
		tags     []string
		want     []string
	}{
		{
			name:     "category and tags appended",
			keywords: []string{"existing"},
			category: "Testing",
			tags:     []string{"Go", "Coverage"},
			want:     []string{"existing", "testing", "go", "coverage"},
		},
		{
			name:     "empty category skipped",
			keywords: []string{"a"},
			category: "",
			tags:     []string{"tag1"},
			want:     []string{"a", "tag1"},
		},
		{
			name:     "empty tags list",
			keywords: []string{"a"},
			category: "Cat",
			tags:     nil,
			want:     []string{"a", "cat"},
		},
		{
			name:     "whitespace-only tags skipped",
			keywords: nil,
			category: "",
			tags:     []string{"  ", "\t", ""},
			want:     nil,
		},
		{
			name:     "tags trimmed before append",
			keywords: nil,
			category: "",
			tags:     []string{"  go  ", " rust "},
			want:     []string{"go", "rust"},
		},
		{
			name:     "all empty inputs",
			keywords: nil,
			category: "",
			tags:     nil,
			want:     nil,
		},
		{
			name:     "nil keywords with category",
			keywords: nil,
			category: "DevOps",
			tags:     nil,
			want:     []string{"devops"},
		},
		{
			name:     "uppercase category lowercased",
			keywords: []string{},
			category: "INFRASTRUCTURE",
			tags:     []string{"AWS", "Terraform"},
			want:     []string{"infrastructure", "aws", "terraform"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := appendCategoryKeywords(tt.keywords, tt.category, tt.tags)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d; got %v", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("index %d: got %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// extractKeywords
// ---------------------------------------------------------------------------

func TestExtractKeywords_Comprehensive(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		wantKeys   []string // must be present
		wantAbsent []string // must NOT be present
		wantLen    int      // -1 = skip len check
	}{
		{
			name:     "pattern and fix detected",
			content:  "Here is a pattern: do X. Also fix: Y.",
			wantKeys: []string{"pattern", "fix"},
			wantLen:  -1,
		},
		{
			name:     "case insensitive detection",
			content:  "Found an ERROR: something went wrong. DEPLOY: to prod.",
			wantKeys: []string{"error", "deploy"},
			wantLen:  -1,
		},
		{
			name:     "all patterns detected",
			content:  "pattern: a\nsolution: b\nlearning: c\ndecision: d\nfix: e\nissue: f\nerror: g\nwarning: h\nconfig: i\nsetup: j\ninstall: k\ndeploy: l",
			wantKeys: []string{"pattern", "solution", "learning", "decision", "fix", "issue", "error", "warning", "config", "setup", "install", "deploy"},
			wantLen:  12,
		},
		{
			name:     "tags metadata extracted",
			content:  "# Title\n\n**Tags**: auth, database, security\n",
			wantKeys: []string{"auth", "database", "security"},
			wantLen:  -1,
		},
		{
			name:     "keywords metadata extracted",
			content:  "**Keywords**: mutex, concurrency\n",
			wantKeys: []string{"mutex", "concurrency"},
			wantLen:  -1,
		},
		{
			name:    "empty content",
			content: "",
			wantLen: 0,
		},
		{
			name:    "no matches",
			content: "Just some plain text with no special colon patterns.",
			wantLen: 0,
		},
		{
			name:       "dedup across patterns and tags",
			content:    "This has a pattern: thing.\n**Tags**: pattern, other",
			wantKeys:   []string{"pattern", "other"},
			wantAbsent: nil,
			wantLen:    -1,
		},
		{
			name:     "empty tag values skipped",
			content:  "**Tags**: , , valid, \n",
			wantKeys: []string{"valid"},
			wantLen:  -1,
		},
		{
			name:     "tags on both Tags and Keywords lines",
			content:  "**Tags**: a, b\n**Keywords**: c, d\n",
			wantKeys: []string{"a", "b", "c", "d"},
			wantLen:  -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractKeywords(tt.content)

			if tt.wantLen >= 0 && len(got) != tt.wantLen {
				t.Errorf("len = %d, want %d; got %v", len(got), tt.wantLen, got)
			}

			gotSet := make(map[string]bool)
			for _, k := range got {
				gotSet[k] = true
			}
			for _, want := range tt.wantKeys {
				if !gotSet[want] {
					t.Errorf("missing keyword %q in %v", want, got)
				}
			}
			for _, absent := range tt.wantAbsent {
				if gotSet[absent] {
					t.Errorf("unexpected keyword %q in %v", absent, got)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// extractCategoryAndTags
// ---------------------------------------------------------------------------

func TestExtractCategoryAndTags(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantCat  string
		wantTags []string
	}{
		{
			name:     "YAML frontmatter with category and tags",
			content:  "---\ncategory: testing\ntags: [go, coverage, rpi]\n---\n\n# My Learning",
			wantCat:  "testing",
			wantTags: []string{"go", "coverage", "rpi"},
		},
		{
			name:     "YAML category only",
			content:  "---\ncategory: \"go-patterns\"\n---\n\ncontent here",
			wantCat:  "go-patterns",
			wantTags: nil,
		},
		{
			name:     "markdown format category and tags",
			content:  "# My Learning\n\n**Category**: workflow\n**Tags**: automation, ci, testing",
			wantCat:  "workflow",
			wantTags: []string{"automation", "ci", "testing"},
		},
		{
			name:     "no metadata returns empty",
			content:  "# My Learning\n\nJust some plain content.",
			wantCat:  "",
			wantTags: nil,
		},
		{
			name:     "empty content",
			content:  "",
			wantCat:  "",
			wantTags: nil,
		},
		{
			name:    "YAML category wins over markdown category",
			content: "---\ncategory: yaml-category\n---\n**Category**: markdown-category",
			wantCat: "yaml-category",
			// markdown tags still get appended
			wantTags: nil,
		},
		{
			name:    "quoted YAML tags",
			content: "---\ntags: [\"go\", \"test\"]\n---\n",
			wantCat: "",
			// parseBracketedList applies Trim then TrimSpace; for ` "test"` the
			// leading space prevents Trim from stripping the left quote, so the
			// second element retains a leading quote character.
			wantTags: []string{"go", `"test`},
		},
		{
			name:    "YAML and markdown tags combine",
			content: "---\ntags: [yaml-tag]\n---\n**Tags**: md-tag",
			wantCat: "",
			// YAML tags + markdown tags appended
			wantTags: []string{"yaml-tag", "md-tag"},
		},
		{
			name:     "markdown category when no YAML frontmatter",
			content:  "Some content\n**Category**: ops\n**Tags**: deploy, k8s",
			wantCat:  "ops",
			wantTags: []string{"deploy", "k8s"},
		},
		{
			name:     "single quoted YAML category",
			content:  "---\ncategory: 'infra'\n---\n",
			wantCat:  "infra",
			wantTags: nil,
		},
		{
			name:     "frontmatter not at start of file is ignored",
			content:  "some text\n---\ncategory: wrong\n---\n",
			wantCat:  "",
			wantTags: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cat, tags := extractCategoryAndTags(tt.content)
			if cat != tt.wantCat {
				t.Errorf("category = %q, want %q", cat, tt.wantCat)
			}
			if tt.wantTags == nil {
				if len(tags) != 0 {
					t.Errorf("expected no tags, got %v", tags)
				}
			} else {
				if len(tags) != len(tt.wantTags) {
					t.Fatalf("tags len = %d, want %d; got %v", len(tags), len(tt.wantTags), tags)
				}
				for i, want := range tt.wantTags {
					if tags[i] != want {
						t.Errorf("tag[%d] = %q, want %q", i, tags[i], want)
					}
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// extractFrontmatterMeta
// ---------------------------------------------------------------------------

func TestExtractFrontmatterMeta(t *testing.T) {
	tests := []struct {
		name     string
		lines    []string
		wantCat  string
		wantTags []string
	}{
		{
			name:     "category and tags",
			lines:    []string{"category: testing", "tags: [go, coverage]", "---"},
			wantCat:  "testing",
			wantTags: []string{"go", "coverage"},
		},
		{
			name:     "category only",
			lines:    []string{"category: infra", "---"},
			wantCat:  "infra",
			wantTags: nil,
		},
		{
			name:     "tags only",
			lines:    []string{"tags: [a, b]", "---"},
			wantCat:  "",
			wantTags: []string{"a", "b"},
		},
		{
			name:     "stops at closing ---",
			lines:    []string{"category: first", "---", "category: second"},
			wantCat:  "first",
			wantTags: nil,
		},
		{
			name:     "double-quoted category stripped",
			lines:    []string{`category: "my-cat"`, "---"},
			wantCat:  "my-cat",
			wantTags: nil,
		},
		{
			name:     "single-quoted category stripped",
			lines:    []string{"category: 'my-cat'", "---"},
			wantCat:  "my-cat",
			wantTags: nil,
		},
		{
			name:     "empty lines array",
			lines:    []string{},
			wantCat:  "",
			wantTags: nil,
		},
		{
			name:     "no closing separator",
			lines:    []string{"category: unclosed", "tags: [x]"},
			wantCat:  "unclosed",
			wantTags: []string{"x"},
		},
		{
			name:     "whitespace around values",
			lines:    []string{"  category:   spaced  ", "---"},
			wantCat:  "spaced",
			wantTags: nil,
		},
		{
			name:     "non-bracketed tags return nil",
			lines:    []string{"tags: not-bracketed", "---"},
			wantCat:  "",
			wantTags: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cat, tags := extractFrontmatterMeta(tt.lines)
			if cat != tt.wantCat {
				t.Errorf("category = %q, want %q", cat, tt.wantCat)
			}
			if tt.wantTags == nil {
				if len(tags) != 0 {
					t.Errorf("expected no tags, got %v", tags)
				}
			} else {
				if len(tags) != len(tt.wantTags) {
					t.Fatalf("tags len = %d, want %d; got %v", len(tags), len(tt.wantTags), tags)
				}
				for i, want := range tt.wantTags {
					if tags[i] != want {
						t.Errorf("tag[%d] = %q, want %q", i, tags[i], want)
					}
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// extractMarkdownMeta
// ---------------------------------------------------------------------------

func TestExtractMarkdownMeta(t *testing.T) {
	tests := []struct {
		name     string
		lines    []string
		wantCat  string
		wantTags []string
	}{
		{
			name:     "category and tags",
			lines:    []string{"**Category**: workflow", "**Tags**: a, b, c"},
			wantCat:  "workflow",
			wantTags: []string{"a", "b", "c"},
		},
		{
			name:     "category only",
			lines:    []string{"**Category**: infra"},
			wantCat:  "infra",
			wantTags: nil,
		},
		{
			name:     "tags only",
			lines:    []string{"**Tags**: go, rust"},
			wantCat:  "",
			wantTags: []string{"go", "rust"},
		},
		{
			name:     "first category wins",
			lines:    []string{"**Category**: first", "**Category**: second"},
			wantCat:  "first",
			wantTags: nil,
		},
		{
			name:     "multiple tag lines accumulate",
			lines:    []string{"**Tags**: a", "**Tags**: b"},
			wantCat:  "",
			wantTags: []string{"a", "b"},
		},
		{
			name:     "empty lines",
			lines:    []string{},
			wantCat:  "",
			wantTags: nil,
		},
		{
			name:     "whitespace trimmed",
			lines:    []string{"  **Category**:   ops  "},
			wantCat:  "ops",
			wantTags: nil,
		},
		{
			name:     "non-metadata lines ignored",
			lines:    []string{"# Title", "Some content", "**Category**: found"},
			wantCat:  "found",
			wantTags: nil,
		},
		{
			name:     "empty tag values skipped",
			lines:    []string{"**Tags**: , , valid, "},
			wantCat:  "",
			wantTags: []string{"valid"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cat, tags := extractMarkdownMeta(tt.lines)
			if cat != tt.wantCat {
				t.Errorf("category = %q, want %q", cat, tt.wantCat)
			}
			if tt.wantTags == nil {
				if len(tags) != 0 {
					t.Errorf("expected no tags, got %v", tags)
				}
			} else {
				if len(tags) != len(tt.wantTags) {
					t.Fatalf("tags len = %d, want %d; got %v", len(tags), len(tt.wantTags), tags)
				}
				for i, want := range tt.wantTags {
					if tags[i] != want {
						t.Errorf("tag[%d] = %q, want %q", i, tags[i], want)
					}
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseBracketedList
// ---------------------------------------------------------------------------

func TestParseBracketedList(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want []string
	}{
		{
			name: "simple list",
			s:    "[a, b, c]",
			want: []string{"a", "b", "c"},
		},
		{
			name: "quoted values — first stripped, rest retain leading quote",
			// parseBracketedList applies Trim(cutset) before TrimSpace;
			// for elements with a leading space before the quote, Trim
			// cannot reach the left quote, so it survives TrimSpace.
			s:    `["go", "rust", "python"]`,
			want: []string{"go", `"rust`, `"python`},
		},
		{
			name: "single quoted — first stripped, rest retain leading quote",
			s:    "['a', 'b']",
			want: []string{"a", `'b`},
		},
		{
			name: "not bracketed returns nil",
			s:    "a, b, c",
			want: nil,
		},
		{
			name: "missing closing bracket returns nil",
			s:    "[a, b",
			want: nil,
		},
		{
			name: "missing opening bracket returns nil",
			s:    "a, b]",
			want: nil,
		},
		{
			name: "empty brackets",
			s:    "[]",
			want: nil,
		},
		{
			name: "single element",
			s:    "[solo]",
			want: []string{"solo"},
		},
		{
			name: "whitespace elements skipped",
			s:    "[a, , , b]",
			want: []string{"a", "b"},
		},
		{
			name: "spaces around values trimmed",
			s:    "[  go  ,  rust  ]",
			want: []string{"go", "rust"},
		},
		{
			name: "empty string",
			s:    "",
			want: nil,
		},
		{
			name: "just brackets no space",
			s:    "[ ]",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseBracketedList(tt.s)
			if tt.want == nil {
				if got != nil {
					t.Errorf("parseBracketedList(%q) = %v, want nil", tt.s, got)
				}
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d; got %v", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("index %d: got %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// splitCSV
// ---------------------------------------------------------------------------

func TestSplitCSV(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want []string
	}{
		{
			name: "simple CSV",
			s:    "a, b, c",
			want: []string{"a", "b", "c"},
		},
		{
			name: "no spaces",
			s:    "x,y,z",
			want: []string{"x", "y", "z"},
		},
		{
			name: "extra whitespace trimmed",
			s:    "  alpha ,  beta  ,  gamma  ",
			want: []string{"alpha", "beta", "gamma"},
		},
		{
			name: "empty values skipped",
			s:    "a,,b, ,c",
			want: []string{"a", "b", "c"},
		},
		{
			name: "single value",
			s:    "solo",
			want: []string{"solo"},
		},
		{
			name: "empty string",
			s:    "",
			want: nil,
		},
		{
			name: "all commas",
			s:    ",,,",
			want: nil,
		},
		{
			name: "whitespace only values",
			s:    " , , ",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitCSV(tt.s)
			if tt.want == nil {
				if got != nil {
					t.Errorf("splitCSV(%q) = %v, want nil", tt.s, got)
				}
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d; got %v", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("index %d: got %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseMemRLMetadata
// ---------------------------------------------------------------------------

func TestParseMemRLMetadata(t *testing.T) {
	const defaultUtility = 0.5 // types.InitialUtility

	tests := []struct {
		name         string
		content      string
		wantUtility  float64
		wantMaturity string
	}{
		{
			name:         "both utility and maturity",
			content:      "# Learning\n\n**Utility**: 0.85\n**Maturity**: validated\n",
			wantUtility:  0.85,
			wantMaturity: "validated",
		},
		{
			name:         "utility only",
			content:      "**Utility**: 0.7\n",
			wantUtility:  0.7,
			wantMaturity: "provisional",
		},
		{
			name:         "maturity only",
			content:      "**Maturity**: hardened\n",
			wantUtility:  defaultUtility,
			wantMaturity: "hardened",
		},
		{
			name:         "no metadata defaults",
			content:      "# Just a title\n\nSome content.",
			wantUtility:  defaultUtility,
			wantMaturity: "provisional",
		},
		{
			name:         "empty content",
			content:      "",
			wantUtility:  defaultUtility,
			wantMaturity: "provisional",
		},
		{
			name:         "bullet-prefixed utility",
			content:      "- **Utility**: 0.95\n- **Maturity**: locked\n",
			wantUtility:  0.95,
			wantMaturity: "locked",
		},
		{
			name:         "utility 0.0 not boosted",
			content:      "**Utility**: 0.0\n",
			wantUtility:  0.0,
			wantMaturity: "provisional",
		},
		{
			name:         "utility 1.0",
			content:      "**Utility**: 1.0\n",
			wantUtility:  1.0,
			wantMaturity: "provisional",
		},
		{
			name:         "malformed utility keeps default",
			content:      "**Utility**: not-a-number\n",
			wantUtility:  defaultUtility,
			wantMaturity: "provisional",
		},
		{
			name:         "mixed with other content",
			content:      "# Title\n\nSome explanation.\n\n**Utility**: 0.6\n\nMore text.\n\n**Maturity**: tested\n",
			wantUtility:  0.6,
			wantMaturity: "tested",
		},
		{
			name:         "bullet-prefixed both",
			content:      "- **Utility**: 0.3\n- **Maturity**: experimental\n",
			wantUtility:  0.3,
			wantMaturity: "experimental",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			utility, maturity := parseMemRLMetadata(tt.content)
			if math.Abs(utility-tt.wantUtility) > 1e-9 {
				t.Errorf("utility = %f, want %f", utility, tt.wantUtility)
			}
			if maturity != tt.wantMaturity {
				t.Errorf("maturity = %q, want %q", maturity, tt.wantMaturity)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// createSearchSnippet
// ---------------------------------------------------------------------------

func TestCreateSearchSnippet(t *testing.T) {
	tests := []struct {
		name    string
		content string
		query   string
		maxLen  int
		check   func(t *testing.T, snippet string)
	}{
		{
			name:    "exact match found",
			content: "This document explains the mutex pattern for Go concurrency.",
			query:   "mutex pattern",
			maxLen:  150,
			check: func(t *testing.T, snippet string) {
				if !strings.Contains(strings.ToLower(snippet), "mutex pattern") {
					t.Errorf("snippet should contain query, got %q", snippet)
				}
			},
		},
		{
			name:    "first term fallback",
			content: "Learn about mutex and channels separately for different patterns.",
			query:   "mutex channels together",
			maxLen:  150,
			check: func(t *testing.T, snippet string) {
				if !strings.Contains(strings.ToLower(snippet), "mutex") {
					t.Errorf("snippet should contain first query term, got %q", snippet)
				}
			},
		},
		{
			name:    "no match returns start of content",
			content: "This is some content about databases and SQL queries.",
			query:   "zzzznonexistent",
			maxLen:  20,
			check: func(t *testing.T, snippet string) {
				if !strings.HasSuffix(snippet, "...") {
					t.Errorf("truncated snippet should end with ..., got %q", snippet)
				}
				if len(snippet) > 20 {
					t.Errorf("snippet length %d exceeds maxLen 20", len(snippet))
				}
			},
		},
		{
			name:    "short content no match returned as-is",
			content: "Short",
			query:   "missing",
			maxLen:  150,
			check: func(t *testing.T, snippet string) {
				if snippet != "Short" {
					t.Errorf("expected short content as-is, got %q", snippet)
				}
			},
		},
		{
			name:    "empty content",
			content: "",
			query:   "anything",
			maxLen:  150,
			check: func(t *testing.T, snippet string) {
				if snippet != "" {
					t.Errorf("expected empty snippet, got %q", snippet)
				}
			},
		},
		{
			name:    "match near start no leading ellipsis",
			content: "mutex pattern is used for shared state protection.",
			query:   "mutex",
			maxLen:  150,
			check: func(t *testing.T, snippet string) {
				if strings.HasPrefix(snippet, "...") {
					t.Errorf("snippet near start should not have leading ..., got %q", snippet)
				}
			},
		},
		{
			name:    "match deep in content has leading ellipsis",
			content: strings.Repeat("x", 200) + "TARGET found here" + strings.Repeat("y", 200),
			query:   "TARGET",
			maxLen:  100,
			check: func(t *testing.T, snippet string) {
				if !strings.HasPrefix(snippet, "...") {
					t.Errorf("snippet from deep in content should have leading ..., got %q", snippet)
				}
			},
		},
		{
			name:    "match not at end has trailing ellipsis",
			content: strings.Repeat("a", 100) + "NEEDLE" + strings.Repeat("b", 300),
			query:   "NEEDLE",
			maxLen:  50,
			check: func(t *testing.T, snippet string) {
				if !strings.HasSuffix(snippet, "...") {
					t.Errorf("snippet not reaching end should have trailing ..., got %q", snippet)
				}
			},
		},
		{
			name:    "newlines replaced with spaces",
			content: "line1\nNEEDLE\nline3",
			query:   "NEEDLE",
			maxLen:  150,
			check: func(t *testing.T, snippet string) {
				if strings.Contains(snippet, "\n") {
					t.Errorf("snippet should not contain newlines, got %q", snippet)
				}
			},
		},
		{
			name:    "case insensitive match",
			content: "The Mutex Pattern is important.",
			query:   "mutex pattern",
			maxLen:  150,
			check: func(t *testing.T, snippet string) {
				if !strings.Contains(snippet, "Mutex Pattern") {
					t.Errorf("snippet should preserve original case, got %q", snippet)
				}
			},
		},
		{
			name:    "empty query matches at index 0",
			content: "Some content here that is longer than max length for testing purposes really.",
			query:   "",
			maxLen:  20,
			check: func(t *testing.T, snippet string) {
				// empty string matches at index 0 via strings.Index, so the
				// window starts at 0 and extends maxLen chars, then "..." is
				// appended (the function does not enforce maxLen strictly).
				if !strings.HasPrefix(snippet, "Some content") {
					t.Errorf("expected snippet to start with content prefix, got %q", snippet)
				}
				if !strings.HasSuffix(snippet, "...") {
					t.Errorf("expected trailing ellipsis, got %q", snippet)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snippet := createSearchSnippet(tt.content, tt.query, tt.maxLen)
			tt.check(t, snippet)
		})
	}
}

// ---------------------------------------------------------------------------
// accumulateEntryStats
// ---------------------------------------------------------------------------

func TestAccumulateEntryStats(t *testing.T) {
	now := time.Date(2026, 2, 22, 10, 0, 0, 0, time.UTC)
	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	later := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	earlier := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	zeroUtilityAt := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	negativeUtilityAt := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	emptyTypeAt := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)

	tests := []accumulateEntryStatsCase{
		{
			name: "single entry updates all fields",
			entries: []IndexEntry{
				{Type: "learning", Utility: 0.8, IndexedAt: now},
			},
			checkpoints: []accumulateEntryStatsExpectation{
				{
					totalEntries: 1,
					byType:       map[string]int{"learning": 1},
					totalUtility: 0.8,
					utilityCount: 1,
					oldestEntry:  now,
					newestEntry:  now,
				},
			},
		},
		{
			name: "multiple entries track oldest and newest",
			entries: []IndexEntry{
				{Type: "learning", Utility: 0.5, IndexedAt: t2},
				{Type: "pattern", Utility: 0.9, IndexedAt: t1},
				{Type: "learning", Utility: 0.7, IndexedAt: t3},
			},
			checkpoints: []accumulateEntryStatsExpectation{
				{
					totalEntries: 3,
					byType:       map[string]int{"learning": 2, "pattern": 1},
					totalUtility: 0.5 + 0.9 + 0.7,
					utilityCount: 3,
					oldestEntry:  t1,
					newestEntry:  t3,
				},
			},
		},
		{
			name: "zero utility not counted",
			entries: []IndexEntry{
				{Type: "retro", Utility: 0.0, IndexedAt: zeroUtilityAt},
			},
			checkpoints: []accumulateEntryStatsExpectation{
				{
					totalEntries: 1,
					byType:       map[string]int{"retro": 1},
					totalUtility: 0.0,
					utilityCount: 0,
					oldestEntry:  zeroUtilityAt,
					newestEntry:  zeroUtilityAt,
				},
			},
		},
		{
			name: "negative utility not counted",
			entries: []IndexEntry{
				{Type: "research", Utility: -0.1, IndexedAt: negativeUtilityAt},
			},
			checkpoints: []accumulateEntryStatsExpectation{
				{
					totalEntries: 1,
					byType:       map[string]int{"research": 1},
					totalUtility: 0.0,
					utilityCount: 0,
					oldestEntry:  negativeUtilityAt,
					newestEntry:  negativeUtilityAt,
				},
			},
		},
		{
			name: "oldest set on first entry then replaced by earlier",
			entries: []IndexEntry{
				{Type: "a", IndexedAt: later},
				{Type: "a", IndexedAt: earlier},
			},
			checkpoints: []accumulateEntryStatsExpectation{
				{
					totalEntries: 1,
					byType:       map[string]int{"a": 1},
					totalUtility: 0.0,
					utilityCount: 0,
					oldestEntry:  later,
					newestEntry:  later,
				},
				{
					totalEntries: 2,
					byType:       map[string]int{"a": 2},
					totalUtility: 0.0,
					utilityCount: 0,
					oldestEntry:  earlier,
					newestEntry:  later,
				},
			},
		},
		{
			name: "empty type counted",
			entries: []IndexEntry{
				{Type: "", IndexedAt: emptyTypeAt},
			},
			checkpoints: []accumulateEntryStatsExpectation{
				{
					totalEntries: 1,
					byType:       map[string]int{"": 1},
					totalUtility: 0.0,
					utilityCount: 0,
					oldestEntry:  emptyTypeAt,
					newestEntry:  emptyTypeAt,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runAccumulateEntryStatsCase(t, tt)
		})
	}
}

type accumulateEntryStatsCase struct {
	name        string
	entries     []IndexEntry
	checkpoints []accumulateEntryStatsExpectation
}

type accumulateEntryStatsExpectation struct {
	totalEntries int
	byType       map[string]int
	totalUtility float64
	utilityCount int
	oldestEntry  time.Time
	newestEntry  time.Time
}

type accumulateEntryStatsFixture struct {
	stats        *IndexStats
	totalUtility float64
	utilityCount int
}

func runAccumulateEntryStatsCase(t *testing.T, tc accumulateEntryStatsCase) {
	t.Helper()

	fixture := newAccumulateEntryStatsFixture()
	if len(tc.checkpoints) != 1 && len(tc.checkpoints) != len(tc.entries) {
		t.Fatalf("checkpoints = %d, want 1 or %d", len(tc.checkpoints), len(tc.entries))
	}

	for i, entry := range tc.entries {
		accumulateEntryStats(fixture.stats, entry, &fixture.totalUtility, &fixture.utilityCount)
		if len(tc.checkpoints) == len(tc.entries) {
			assertAccumulateEntryStatsExpectation(t, fixture, tc.checkpointAt(i))
		}
	}
	if len(tc.checkpoints) == 1 {
		assertAccumulateEntryStatsExpectation(t, fixture, tc.checkpoints[0])
	}
}

func (tc accumulateEntryStatsCase) checkpointAt(index int) accumulateEntryStatsExpectation {
	if len(tc.checkpoints) == 1 {
		return tc.checkpoints[0]
	}
	return tc.checkpoints[index]
}

func newAccumulateEntryStatsFixture() *accumulateEntryStatsFixture {
	return &accumulateEntryStatsFixture{
		stats: &IndexStats{ByType: make(map[string]int)},
	}
}

func assertAccumulateEntryStatsExpectation(t *testing.T, fixture *accumulateEntryStatsFixture, want accumulateEntryStatsExpectation) {
	t.Helper()

	if fixture.stats.TotalEntries != want.totalEntries {
		t.Errorf("TotalEntries = %d, want %d", fixture.stats.TotalEntries, want.totalEntries)
	}
	for key, value := range want.byType {
		if fixture.stats.ByType[key] != value {
			t.Errorf("ByType[%q] = %d, want %d", key, fixture.stats.ByType[key], value)
		}
	}
	if math.Abs(fixture.totalUtility-want.totalUtility) > 1e-9 {
		t.Errorf("totalUtility = %f, want %f", fixture.totalUtility, want.totalUtility)
	}
	if fixture.utilityCount != want.utilityCount {
		t.Errorf("utilityCount = %d, want %d", fixture.utilityCount, want.utilityCount)
	}
	if !fixture.stats.OldestEntry.Equal(want.oldestEntry) {
		t.Errorf("OldestEntry = %v, want %v", fixture.stats.OldestEntry, want.oldestEntry)
	}
	if !fixture.stats.NewestEntry.Equal(want.newestEntry) {
		t.Errorf("NewestEntry = %v, want %v", fixture.stats.NewestEntry, want.newestEntry)
	}
}

// ---------------------------------------------------------------------------
// Integration: extractKeywords dedup across pattern matches and tag lines
// ---------------------------------------------------------------------------

func TestExtractKeywords_NoDuplicates(t *testing.T) {
	content := "pattern: here\n**Tags**: pattern, other"
	got := extractKeywords(content)
	seen := map[string]int{}
	for _, k := range got {
		seen[k]++
	}
	for k, count := range seen {
		if count > 1 {
			t.Errorf("keyword %q appears %d times, expected 1 (dedup)", k, count)
		}
	}
}

// ---------------------------------------------------------------------------
// Integration: extractCategoryAndTags combines YAML + markdown tags
// ---------------------------------------------------------------------------

func TestExtractCategoryAndTags_CombinedTags(t *testing.T) {
	content := "---\ncategory: yaml-cat\ntags: [yaml-tag]\n---\n\n**Tags**: md-tag1, md-tag2\n"
	cat, tags := extractCategoryAndTags(content)
	if cat != "yaml-cat" {
		t.Errorf("category = %q, want yaml-cat", cat)
	}
	// YAML tags come first, then markdown tags appended
	sort.Strings(tags)
	expected := []string{"md-tag1", "md-tag2", "yaml-tag"}
	sort.Strings(expected)
	if len(tags) != len(expected) {
		t.Fatalf("tags = %v, want %v", tags, expected)
	}
	for i := range tags {
		if tags[i] != expected[i] {
			t.Errorf("tag[%d] = %q, want %q", i, tags[i], expected[i])
		}
	}
}
