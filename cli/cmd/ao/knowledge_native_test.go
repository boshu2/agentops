package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestKnowledgeBriefingOutputPath_UsesSluggedGoal(t *testing.T) {
	agents := "/tmp/agents"
	got := knowledgeBriefingOutputPath(agents, "Refactor Dream Council")
	today := time.Now().Format("2006-01-02")
	wantSuffix := today + "-refactor-dream-council.md"
	wantDir := filepath.Join(agents, "briefings")
	if filepath.Dir(got) != wantDir {
		t.Fatalf("dir = %q, want %q", filepath.Dir(got), wantDir)
	}
	if filepath.Base(got) != wantSuffix {
		t.Fatalf("base = %q, want %q", filepath.Base(got), wantSuffix)
	}
}

func TestKnowledgeBriefingOutputPath_WhitespaceSlug(t *testing.T) {
	agents := "/tmp/agents"
	// Pool.Slugify returns the "cand" sentinel (never "") so the filename
	// always has a real slug segment.
	got := knowledgeBriefingOutputPath(agents, "   ")
	today := time.Now().Format("2006-01-02")
	if !strings.HasSuffix(got, today+"-cand.md") {
		t.Fatalf("base = %q, want suffix %q", filepath.Base(got), today+"-cand.md")
	}
}

func TestKnowledgeBriefingOutputPath_LowercasesAndDelimits(t *testing.T) {
	agents := "/tmp/agents"
	got := knowledgeBriefingOutputPath(agents, "Hello World!")
	today := time.Now().Format("2006-01-02")
	wantBase := today + "-hello-world.md"
	if filepath.Base(got) != wantBase {
		t.Fatalf("base = %q, want %q", filepath.Base(got), wantBase)
	}
}
