package main

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestKnowledgeOrderedItem(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "simple numeric", in: "1. do thing", want: "do thing"},
		{name: "multi-digit", in: "12. twelve things", want: "twelve things"},
		{name: "trims surrounding whitespace", in: "3.   spaced out  ", want: "spaced out"},
		{name: "not ordered list", in: "- dashed item", want: ""},
		{name: "no space after dot", in: "1.tight", want: ""},
		{name: "letter prefix rejected", in: "a. something", want: ""},
		{name: "empty", in: "", want: ""},
		{name: "leading dot", in: ". nothing", want: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := knowledgeOrderedItem(tc.in)
			if got != tc.want {
				t.Fatalf("knowledgeOrderedItem(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestExtractKnowledgeListItems(t *testing.T) {
	text := `# Title

## Core Beliefs

- belief one
- belief two
1. ordered belief
- belief two

## Other Section

- unrelated item

## Operating Principles

- principle one
`
	got := extractKnowledgeListItems(text, "## Core Beliefs")
	want := []string{"belief one", "belief two", "ordered belief"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Core Beliefs = %v, want %v", got, want)
	}

	gotOther := extractKnowledgeListItems(text, "## Operating Principles")
	wantOther := []string{"principle one"}
	if !reflect.DeepEqual(gotOther, wantOther) {
		t.Fatalf("Operating Principles = %v, want %v", gotOther, wantOther)
	}

	if got := extractKnowledgeListItems(text, "## Missing"); len(got) != 0 {
		t.Fatalf("missing heading = %v, want empty", got)
	}
}

func TestRankKnowledgeContextLines_RanksByTokenScore(t *testing.T) {
	items := []string{
		"alpha beta gamma",
		"gamma delta epsilon",
		"nothing matches here",
	}
	got := rankKnowledgeContextLines("gamma", items, 3)
	// Items 0 and 1 each contain "gamma" (score 2). They retain original order
	// as a stable sort tie-break. Item 2 scores 0 and comes last.
	want := []string{"alpha beta gamma", "gamma delta epsilon", "nothing matches here"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ranked = %v, want %v", got, want)
	}
}

func TestRankKnowledgeContextLines_AppliesLimit(t *testing.T) {
	items := []string{"alpha", "beta", "gamma"}
	got := rankKnowledgeContextLines("beta", items, 2)
	// "beta" ranks first; limit cuts at 2.
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (got %v)", len(got), got)
	}
	if got[0] != "beta" {
		t.Fatalf("first = %q, want %q", got[0], "beta")
	}
}

func TestRankKnowledgeContextLines_NoMatchesPreservesOrder(t *testing.T) {
	items := []string{"alpha", "beta", "gamma"}
	got := rankKnowledgeContextLines("unrelated", items, 0)
	want := []string{"alpha", "beta", "gamma"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("no-match = %v, want %v", got, want)
	}
}

func TestRankKnowledgeContextLines_NormalizesWhitespace(t *testing.T) {
	items := []string{"   alpha   beta   "}
	got := rankKnowledgeContextLines("", items, 0)
	want := []string{"alpha beta"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalize = %v, want %v", got, want)
	}
}

func TestRankKnowledgeContextLines_EmptyInput(t *testing.T) {
	if got := rankKnowledgeContextLines("q", nil, 5); got != nil {
		t.Fatalf("nil items = %v, want nil", got)
	}
}

func TestKnowledgeAgentsRoot_FallsBackToCwdWhenNoGit(t *testing.T) {
	dir := t.TempDir()
	// No .git — findGitRoot returns "" and the cwd should be used.
	got := knowledgeAgentsRoot(dir)
	want := filepath.Join(dir, ".agents")
	if got != want {
		t.Fatalf("knowledgeAgentsRoot(%q) = %q, want %q", dir, got, want)
	}
}

func TestDisplayKnowledgeContextPath_EmptyReturnsEmpty(t *testing.T) {
	if got := displayKnowledgeContextPath("/tmp", ""); got != "" {
		t.Fatalf("empty path = %q, want \"\"", got)
	}
}

func TestDisplayKnowledgeContextPath_RelativeToCwd(t *testing.T) {
	cwd := t.TempDir()
	path := filepath.Join(cwd, "sub", "artifact.md")
	got := displayKnowledgeContextPath(cwd, path)
	want := filepath.Join("sub", "artifact.md")
	if got != want {
		t.Fatalf("rel path = %q, want %q", got, want)
	}
}

func TestDisplayKnowledgeContextPath_AbsoluteWhenOutsideCwd(t *testing.T) {
	cwd := t.TempDir()
	other := t.TempDir()
	path := filepath.Join(other, "elsewhere.md")
	got := displayKnowledgeContextPath(cwd, path)
	if got != path {
		t.Fatalf("outside path = %q, want %q", got, path)
	}
}
