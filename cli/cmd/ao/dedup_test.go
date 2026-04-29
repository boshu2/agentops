package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractMarkdownBody(t *testing.T) {
	input := "---\ntitle: Test Learning\ndate: 2026-01-15\n---\nThis is the body content.\nIt spans multiple lines."
	got := extractMarkdownBody(input)
	want := "This is the body content.\nIt spans multiple lines."
	if got != want {
		t.Errorf("extractMarkdownBody with frontmatter:\n  got:  %q\n  want: %q", got, want)
	}
}

func TestExtractMarkdownBody_NoFrontMatter(t *testing.T) {
	input := "Just plain content.\nNo frontmatter here."
	got := extractMarkdownBody(input)
	if got != input {
		t.Errorf("extractMarkdownBody without frontmatter:\n  got:  %q\n  want: %q", got, input)
	}
}

func TestExtractJSONLBody(t *testing.T) {
	input := `{"content":"some content","title":"title"}`
	got := extractJSONLBody(input)
	want := "some content"
	if got != want {
		t.Errorf("extractJSONLBody with content field:\n  got:  %q\n  want: %q", got, want)
	}
}

func TestExtractJSONLBody_TitleFallback(t *testing.T) {
	input := `{"title":"just title"}`
	got := extractJSONLBody(input)
	want := "just title"
	if got != want {
		t.Errorf("extractJSONLBody title fallback:\n  got:  %q\n  want: %q", got, want)
	}
}

func TestHashNormalizedContent_IdenticalContent(t *testing.T) {
	a := "  This is Some  Content  "
	b := "this is some content"
	hashA := hashNormalizedContent(a)
	hashB := hashNormalizedContent(b)
	if hashA != hashB {
		t.Errorf("expected identical hashes for whitespace/case variants:\n  a: %s\n  b: %s", hashA, hashB)
	}
}

func TestHashNormalizedContent_DifferentContent(t *testing.T) {
	a := "This is about mutexes and concurrency"
	b := "This is about database connections"
	hashA := hashNormalizedContent(a)
	hashB := hashNormalizedContent(b)
	if hashA == hashB {
		t.Errorf("expected different hashes for different content, both got: %s", hashA)
	}
}

func TestRunDedup_NoDuplicates(t *testing.T) {
	// Create temp directory structure
	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatalf("creating learnings dir: %v", err)
	}

	// Write two .md files with different body content
	file1 := filepath.Join(learningsDir, "learn-01.md")
	file2 := filepath.Join(learningsDir, "learn-02.md")

	if err := os.WriteFile(file1, []byte("---\ntitle: First\n---\nContent about mutexes."), 0o644); err != nil {
		t.Fatalf("writing file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("---\ntitle: Second\n---\nContent about databases."), 0o644); err != nil {
		t.Fatalf("writing file2: %v", err)
	}

	// Change to temp dir for the command
	t.Chdir(tmp)

	// Force JSON output so we can parse the result
	origOutput := output
	output = "json"
	defer func() { output = origOutput }()

	// Capture stdout
	r, w, _ := os.Pipe()
	origStdout := os.Stdout
	os.Stdout = w

	err := runDedup(nil, nil)

	_ = w.Close()
	os.Stdout = origStdout

	if err != nil {
		t.Fatalf("runDedup returned error: %v", err)
	}

	var result DedupResult
	if decErr := json.NewDecoder(r).Decode(&result); decErr != nil {
		t.Fatalf("decoding JSON output: %v", decErr)
	}

	if result.TotalFiles != 2 {
		t.Errorf("TotalFiles = %d, want 2", result.TotalFiles)
	}
	if result.DuplicateGroups != 0 {
		t.Errorf("DuplicateGroups = %d, want 0", result.DuplicateGroups)
	}
	if result.DuplicateFiles != 0 {
		t.Errorf("DuplicateFiles = %d, want 0", result.DuplicateFiles)
	}
}

func TestRunDedup_IncludesPatterns(t *testing.T) {
	// Create temp directory with both learnings and patterns
	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	patternsDir := filepath.Join(tmp, ".agents", "patterns")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatalf("creating learnings dir: %v", err)
	}
	if err := os.MkdirAll(patternsDir, 0o755); err != nil {
		t.Fatalf("creating patterns dir: %v", err)
	}

	// Write one file in learnings and one in patterns with same body, different frontmatter
	learningFile := filepath.Join(learningsDir, "learn-mutex.md")
	patternFile := filepath.Join(patternsDir, "pattern-mutex.md")

	if err := os.WriteFile(learningFile, []byte("---\ntitle: Mutex Learning\ndate: 2026-01-10\n---\nAlways use mutex for shared state."), 0o644); err != nil {
		t.Fatalf("writing learning file: %v", err)
	}
	if err := os.WriteFile(patternFile, []byte("---\ntitle: Mutex Pattern\ndate: 2026-02-20\n---\nAlways use mutex for shared state."), 0o644); err != nil {
		t.Fatalf("writing pattern file: %v", err)
	}

	// Change to temp dir for the command
	t.Chdir(tmp)

	// Force JSON output
	origOutput := output
	output = "json"
	defer func() { output = origOutput }()

	// Capture stdout
	r, w, _ := os.Pipe()
	origStdout := os.Stdout
	os.Stdout = w

	err := runDedup(nil, nil)

	_ = w.Close()
	os.Stdout = origStdout

	if err != nil {
		t.Fatalf("runDedup returned error: %v", err)
	}

	var result DedupResult
	if decErr := json.NewDecoder(r).Decode(&result); decErr != nil {
		t.Fatalf("decoding JSON output: %v", decErr)
	}

	if result.TotalFiles != 2 {
		t.Errorf("TotalFiles = %d, want 2", result.TotalFiles)
	}
	if result.DuplicateGroups != 1 {
		t.Errorf("DuplicateGroups = %d, want 1", result.DuplicateGroups)
	}
	if result.DuplicateFiles != 2 {
		t.Errorf("DuplicateFiles = %d, want 2", result.DuplicateFiles)
	}
	if len(result.Groups) != 1 {
		t.Fatalf("len(Groups) = %d, want 1", len(result.Groups))
	}
	// Verify files come from different directories
	hasLearning := false
	hasPattern := false
	for _, f := range result.Groups[0].Files {
		if filepath.Dir(f) == filepath.Join(".agents", "learnings") {
			hasLearning = true
		}
		if filepath.Dir(f) == filepath.Join(".agents", "patterns") {
			hasPattern = true
		}
	}
	if !hasLearning || !hasPattern {
		t.Errorf("expected files from both learnings and patterns dirs, got: %v", result.Groups[0].Files)
	}
}

func TestRunDedup_FindsDuplicates(t *testing.T) {
	// Create temp directory structure
	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatalf("creating learnings dir: %v", err)
	}

	// Write two .md files with identical body content but different frontmatter
	file1 := filepath.Join(learningsDir, "learn-a.md")
	file2 := filepath.Join(learningsDir, "learn-b.md")

	if err := os.WriteFile(file1, []byte("---\ntitle: First Copy\ndate: 2026-01-10\n---\nAlways use mutex for shared state."), 0o644); err != nil {
		t.Fatalf("writing file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("---\ntitle: Second Copy\ndate: 2026-02-15\n---\nAlways use mutex for shared state."), 0o644); err != nil {
		t.Fatalf("writing file2: %v", err)
	}

	// Change to temp dir for the command
	t.Chdir(tmp)

	// Force JSON output so we can parse the result
	origOutput := output
	output = "json"
	defer func() { output = origOutput }()

	// Capture stdout
	r, w, _ := os.Pipe()
	origStdout := os.Stdout
	os.Stdout = w

	err := runDedup(nil, nil)

	_ = w.Close()
	os.Stdout = origStdout

	if err != nil {
		t.Fatalf("runDedup returned error: %v", err)
	}

	var result DedupResult
	if decErr := json.NewDecoder(r).Decode(&result); decErr != nil {
		t.Fatalf("decoding JSON output: %v", decErr)
	}

	if result.TotalFiles != 2 {
		t.Errorf("TotalFiles = %d, want 2", result.TotalFiles)
	}
	if result.DuplicateGroups != 1 {
		t.Errorf("DuplicateGroups = %d, want 1", result.DuplicateGroups)
	}
	if result.DuplicateFiles != 2 {
		t.Errorf("DuplicateFiles = %d, want 2", result.DuplicateFiles)
	}
	if len(result.Groups) != 1 {
		t.Fatalf("len(Groups) = %d, want 1", len(result.Groups))
	}
	if result.Groups[0].Count != 2 {
		t.Errorf("Groups[0].Count = %d, want 2", result.Groups[0].Count)
	}
}

func TestRunDedup_MergeKeepsHighestUtility(t *testing.T) {
	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatalf("creating learnings dir: %v", err)
	}

	// Two files with identical body but different utility values
	highFile := filepath.Join(learningsDir, "learn-high.md")
	lowFile := filepath.Join(learningsDir, "learn-low.md")

	if err := os.WriteFile(highFile, []byte("---\ntitle: High Utility\nutility: 0.9\n---\nAlways use mutex for shared state."), 0o644); err != nil {
		t.Fatalf("writing high file: %v", err)
	}
	if err := os.WriteFile(lowFile, []byte("---\ntitle: Low Utility\nutility: 0.3\n---\nAlways use mutex for shared state."), 0o644); err != nil {
		t.Fatalf("writing low file: %v", err)
	}

	// Change to temp dir
	t.Chdir(tmp)

	// Enable merge flag, disable dry-run
	origMerge := dedupMerge
	dedupMerge = true
	defer func() { dedupMerge = origMerge }()

	origDryRun := dryRun
	dryRun = false
	defer func() { dryRun = origDryRun }()

	// Capture stdout (merge prints to stdout)
	_, w, _ := os.Pipe()
	origStdout := os.Stdout
	os.Stdout = w

	err := runDedup(nil, nil)

	_ = w.Close()
	os.Stdout = origStdout

	if err != nil {
		t.Fatalf("runDedup --merge returned error: %v", err)
	}

	// Verify: high-utility file should still exist
	if _, statErr := os.Stat(highFile); os.IsNotExist(statErr) {
		t.Errorf("high-utility file was removed, should have been kept: %s", highFile)
	}

	// Verify: low-utility file should be gone from original location
	if _, statErr := os.Stat(lowFile); !os.IsNotExist(statErr) {
		t.Errorf("low-utility file still exists at original location, should have been archived: %s", lowFile)
	}

	// Verify: low-utility file should be in archive
	archivePath := filepath.Join(tmp, ".agents", "archive", "dedup", "learn-low.md")
	if _, statErr := os.Stat(archivePath); os.IsNotExist(statErr) {
		t.Errorf("low-utility file not found in archive: %s", archivePath)
	}
}

func TestReadUtilityFromFile_Markdown(t *testing.T) {
	tmp := t.TempDir()

	// File with utility in frontmatter
	withUtility := filepath.Join(tmp, "with-utility.md")
	if err := os.WriteFile(withUtility, []byte("---\ntitle: Test\nutility: 0.8\n---\nBody"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := readUtilityFromFile(withUtility); got != 0.8 {
		t.Errorf("readUtilityFromFile(with utility) = %f, want 0.8", got)
	}

	// File without utility
	noUtility := filepath.Join(tmp, "no-utility.md")
	if err := os.WriteFile(noUtility, []byte("---\ntitle: Test\n---\nBody"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := readUtilityFromFile(noUtility); got != 0.5 {
		t.Errorf("readUtilityFromFile(no utility) = %f, want 0.5", got)
	}

	// File with no frontmatter
	noFM := filepath.Join(tmp, "no-fm.md")
	if err := os.WriteFile(noFM, []byte("Just plain content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := readUtilityFromFile(noFM); got != 0.5 {
		t.Errorf("readUtilityFromFile(no frontmatter) = %f, want 0.5", got)
	}
}

func TestReadUtilityFromFile_JSONL(t *testing.T) {
	tmp := t.TempDir()

	// JSONL with utility as number
	withUtility := filepath.Join(tmp, "with-utility.jsonl")
	if err := os.WriteFile(withUtility, []byte(`{"content":"test","utility":0.7}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := readUtilityFromFile(withUtility); got != 0.7 {
		t.Errorf("readUtilityFromFile(jsonl with utility) = %f, want 0.7", got)
	}

	// JSONL without utility
	noUtility := filepath.Join(tmp, "no-utility.jsonl")
	if err := os.WriteFile(noUtility, []byte(`{"content":"test"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := readUtilityFromFile(noUtility); got != 0.5 {
		t.Errorf("readUtilityFromFile(jsonl no utility) = %f, want 0.5", got)
	}
}

// --- Tests for extracted helpers ---

func TestCollectDedupFiles_BothDirs(t *testing.T) {
	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	patternsDir := filepath.Join(tmp, ".agents", "patterns")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatalf("creating learnings dir: %v", err)
	}
	if err := os.MkdirAll(patternsDir, 0o755); err != nil {
		t.Fatalf("creating patterns dir: %v", err)
	}

	// Write files in both directories
	if err := os.WriteFile(filepath.Join(learningsDir, "learn-01.md"), []byte("---\ntitle: L1\n---\nBody 1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(learningsDir, "learn-02.jsonl"), []byte(`{"content":"c2"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(patternsDir, "pattern-01.md"), []byte("---\ntitle: P1\n---\nBody 3"), 0o644); err != nil {
		t.Fatal(err)
	}

	files, err := collectDedupFiles(tmp)
	if err != nil {
		t.Fatalf("collectDedupFiles returned error: %v", err)
	}
	if len(files) != 3 {
		t.Errorf("expected 3 files, got %d: %v", len(files), files)
	}
}

func TestCollectDedupFiles_MissingDir(t *testing.T) {
	tmp := t.TempDir()
	// Only create learnings, no patterns directory
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatalf("creating learnings dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(learningsDir, "learn-01.md"), []byte("---\ntitle: L1\n---\nBody"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(learningsDir, "learn-02.md"), []byte("---\ntitle: L2\n---\nBody2"), 0o644); err != nil {
		t.Fatal(err)
	}

	files, err := collectDedupFiles(tmp)
	if err != nil {
		t.Fatalf("collectDedupFiles returned error: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("expected 2 files from learnings only, got %d: %v", len(files), files)
	}
}

func TestCollectDedupFiles_EmptyDirs(t *testing.T) {
	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	patternsDir := filepath.Join(tmp, ".agents", "patterns")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatalf("creating learnings dir: %v", err)
	}
	if err := os.MkdirAll(patternsDir, 0o755); err != nil {
		t.Fatalf("creating patterns dir: %v", err)
	}

	files, err := collectDedupFiles(tmp)
	if err != nil {
		t.Fatalf("collectDedupFiles returned error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files from empty dirs, got %d: %v", len(files), files)
	}
	// Verify it returns a non-nil slice (dirs exist but are empty)
	if files == nil {
		t.Error("expected non-nil empty slice when dirs exist but are empty, got nil")
	}
}

func TestGroupByContentHash_IdenticalContent(t *testing.T) {
	tmp := t.TempDir()
	file1 := filepath.Join(tmp, "a.md")
	file2 := filepath.Join(tmp, "b.md")

	// Same body content, different frontmatter
	if err := os.WriteFile(file1, []byte("---\ntitle: First\n---\nAlways use mutex for shared state."), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file2, []byte("---\ntitle: Second\n---\nAlways use mutex for shared state."), 0o644); err != nil {
		t.Fatal(err)
	}

	result := groupByContentHash([]string{file1, file2})

	if len(result) != 1 {
		t.Fatalf("expected 1 hash group for identical content, got %d", len(result))
	}
	for _, group := range result {
		if len(group) != 2 {
			t.Errorf("expected 2 files in group, got %d", len(group))
		}
	}
}

func TestGroupByContentHash_DifferentContent(t *testing.T) {
	tmp := t.TempDir()
	file1 := filepath.Join(tmp, "a.md")
	file2 := filepath.Join(tmp, "b.md")

	if err := os.WriteFile(file1, []byte("---\ntitle: First\n---\nContent about mutexes."), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file2, []byte("---\ntitle: Second\n---\nContent about databases."), 0o644); err != nil {
		t.Fatal(err)
	}

	result := groupByContentHash([]string{file1, file2})

	if len(result) != 2 {
		t.Fatalf("expected 2 hash groups for different content, got %d", len(result))
	}
	for _, group := range result {
		if len(group) != 1 {
			t.Errorf("expected 1 file per group, got %d", len(group))
		}
	}
}

func TestMergeDedupGroups_DryRun(t *testing.T) {
	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatalf("creating learnings dir: %v", err)
	}

	// Create two duplicate files
	file1 := filepath.Join(learningsDir, "learn-high.md")
	file2 := filepath.Join(learningsDir, "learn-low.md")
	body := "Always use mutex for shared state."
	if err := os.WriteFile(file1, []byte("---\ntitle: High\nutility: 0.9\n---\n"+body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file2, []byte("---\ntitle: Low\nutility: 0.3\n---\n"+body), 0o644); err != nil {
		t.Fatal(err)
	}

	// Build hash groups using the real function
	hashToFiles := groupByContentHash([]string{file1, file2})

	// Capture stdout to suppress dry-run output
	_, w, _ := os.Pipe()
	origStdout := os.Stdout
	os.Stdout = w

	err := mergeDedupGroups(hashToFiles, tmp, true) // dryRun=true

	_ = w.Close()
	os.Stdout = origStdout

	if err != nil {
		t.Fatalf("mergeDedupGroups dry-run returned error: %v", err)
	}

	// Verify no files were moved
	if _, statErr := os.Stat(file1); os.IsNotExist(statErr) {
		t.Error("file1 was removed during dry-run, should still exist")
	}
	if _, statErr := os.Stat(file2); os.IsNotExist(statErr) {
		t.Error("file2 was removed during dry-run, should still exist")
	}

	// Verify archive directory was NOT created
	archiveDir := filepath.Join(tmp, ".agents", "archive", "dedup")
	if _, statErr := os.Stat(archiveDir); !os.IsNotExist(statErr) {
		t.Error("archive directory was created during dry-run, should not exist")
	}
}

func TestRunDedup_NoDirs(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer func() { _ = os.Chdir(origDir) }()
	r, w, _ := os.Pipe()
	origStdout := os.Stdout
	os.Stdout = w
	err := runDedup(nil, nil)
	_ = w.Close()
	os.Stdout = origStdout
	if err != nil {
		t.Fatalf("no dirs: %v", err)
	}
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	if !containsStr(string(buf[:n]), "No learnings or patterns") {
		t.Errorf("expected no-dirs, got: %q", string(buf[:n]))
	}
}

func TestRunDedup_EmptyDirs(t *testing.T) {
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, ".agents", "learnings"), 0o755)
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer func() { _ = os.Chdir(origDir) }()
	r, w, _ := os.Pipe()
	origStdout := os.Stdout
	os.Stdout = w
	err := runDedup(nil, nil)
	_ = w.Close()
	os.Stdout = origStdout
	if err != nil {
		t.Fatalf("empty: %v", err)
	}
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	if !containsStr(string(buf[:n]), "No learning or pattern files") {
		t.Errorf("expected empty msg, got: %q", string(buf[:n]))
	}
}

func TestRunDedup_TextOutput(t *testing.T) {
	tmp := t.TempDir()
	ld := filepath.Join(tmp, ".agents", "learnings")
	os.MkdirAll(ld, 0o755)
	os.WriteFile(filepath.Join(ld, "a.md"), []byte("---\ntitle: A\n---\nUnique A."), 0o644)
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer func() { _ = os.Chdir(origDir) }()
	origOutput := output
	output = "table"
	defer func() { output = origOutput }()
	r, w, _ := os.Pipe()
	origStdout := os.Stdout
	os.Stdout = w
	err := runDedup(nil, nil)
	_ = w.Close()
	os.Stdout = origStdout
	if err != nil {
		t.Fatalf("text: %v", err)
	}
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	got := string(buf[:n])
	if !containsStr(got, "Dedup Scan Results") {
		t.Errorf("expected header, got: %q", got)
	}
}

func TestRunDedup_TextOutputWithDuplicates(t *testing.T) {
	tmp := t.TempDir()
	ld := filepath.Join(tmp, ".agents", "learnings")
	os.MkdirAll(ld, 0o755)
	body := "Identical body."
	os.WriteFile(filepath.Join(ld, "d1.md"), []byte("---\ntitle: D1\n---\n"+body), 0o644)
	os.WriteFile(filepath.Join(ld, "d2.md"), []byte("---\ntitle: D2\n---\n"+body), 0o644)
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer func() { _ = os.Chdir(origDir) }()
	origOutput := output
	output = "table"
	defer func() { output = origOutput }()
	r, w, _ := os.Pipe()
	origStdout := os.Stdout
	os.Stdout = w
	err := runDedup(nil, nil)
	_ = w.Close()
	os.Stdout = origStdout
	if err != nil {
		t.Fatalf("text dups: %v", err)
	}
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	if !containsStr(string(buf[:n]), "Duplicate Groups:") {
		t.Errorf("expected groups, got: %q", string(buf[:n]))
	}
}

func TestRunDedup_MergeDryRun(t *testing.T) {
	tmp := t.TempDir()
	ld := filepath.Join(tmp, ".agents", "learnings")
	os.MkdirAll(ld, 0o755)
	body := "Same merge."
	os.WriteFile(filepath.Join(ld, "m1.md"), []byte("---\nutility: 0.9\n---\n"+body), 0o644)
	os.WriteFile(filepath.Join(ld, "m2.md"), []byte("---\nutility: 0.3\n---\n"+body), 0o644)
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer func() { _ = os.Chdir(origDir) }()
	origMerge := dedupMerge
	dedupMerge = true
	defer func() { dedupMerge = origMerge }()
	origDryRun := dryRun
	dryRun = true
	defer func() { dryRun = origDryRun }()
	r, w, _ := os.Pipe()
	origStdout := os.Stdout
	os.Stdout = w
	err := runDedup(nil, nil)
	_ = w.Close()
	os.Stdout = origStdout
	if err != nil {
		t.Fatalf("merge dry: %v", err)
	}
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	if !containsStr(string(buf[:n]), "dry-run") {
		t.Errorf("expected dry-run, got: %q", string(buf[:n]))
	}
}

func TestRunDedup_MergeNoDuplicates(t *testing.T) {
	tmp := t.TempDir()
	ld := filepath.Join(tmp, ".agents", "learnings")
	os.MkdirAll(ld, 0o755)
	os.WriteFile(filepath.Join(ld, "u.md"), []byte("---\ntitle: U\n---\nUnique."), 0o644)
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer func() { _ = os.Chdir(origDir) }()
	origMerge := dedupMerge
	dedupMerge = true
	defer func() { dedupMerge = origMerge }()
	origOutput := output
	output = "json"
	defer func() { output = origOutput }()
	r, w, _ := os.Pipe()
	origStdout := os.Stdout
	os.Stdout = w
	err := runDedup(nil, nil)
	_ = w.Close()
	os.Stdout = origStdout
	if err != nil {
		t.Fatalf("merge no dups: %v", err)
	}
	var result DedupResult
	json.NewDecoder(r).Decode(&result)
	if result.DuplicateGroups != 0 {
		t.Errorf("DuplicateGroups = %d, want 0", result.DuplicateGroups)
	}
}

func TestReadUtilityFromJSONL_StringUtility(t *testing.T) {
	if got := readUtilityFromJSONL(`{"utility":"0.85"}`, 0.5); got != 0.85 {
		t.Errorf("string = %f, want 0.85", got)
	}
}

func TestReadUtilityFromJSONL_InvalidJSON(t *testing.T) {
	if got := readUtilityFromJSONL("bad", 0.5); got != 0.5 {
		t.Errorf("invalid = %f, want 0.5", got)
	}
}

func TestReadUtilityFromJSONL_EmptyText(t *testing.T) {
	if got := readUtilityFromJSONL("", 0.5); got != 0.5 {
		t.Errorf("empty = %f, want 0.5", got)
	}
}

func TestReadUtilityFromJSONL_UnparseableString(t *testing.T) {
	if got := readUtilityFromJSONL(`{"utility":"xyz"}`, 0.5); got != 0.5 {
		t.Errorf("nan = %f, want 0.5", got)
	}
}

func TestReadUtilityFromFrontmatter_UnparseableValue(t *testing.T) {
	if got := readUtilityFromFrontmatter("---\nutility: xyz\n---\nB", 0.5); got != 0.5 {
		t.Errorf("nan = %f, want 0.5", got)
	}
}

func TestReadUtilityFromFrontmatter_NoClosingDelimiter(t *testing.T) {
	if got := readUtilityFromFrontmatter("---\nutility: 0.9\ntitle: X", 0.5); got != 0.9 {
		t.Errorf("no close = %f, want 0.9", got)
	}
}

func TestExtractLearningBody_JSONL(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "l.jsonl")
	os.WriteFile(p, []byte(`{"content":"jb"}`+"\n"), 0o644)
	if got := extractLearningBody(p); got != "jb" {
		t.Errorf("JSONL = %q, want jb", got)
	}
}

func TestExtractLearningBody_Nonexistent(t *testing.T) {
	if got := extractLearningBody("/no/file.md"); got != "" {
		t.Errorf("nonexist = %q", got)
	}
}

func TestExtractMarkdownBody_UnclosedFrontmatter(t *testing.T) {
	text := "---\ntitle: X\nContent"
	if got := extractMarkdownBody(text); got != text {
		t.Errorf("unclosed = %q", got)
	}
}

func TestExtractJSONLBody_Empty(t *testing.T) {
	if got := extractJSONLBody(""); got != "" {
		t.Errorf("empty = %q", got)
	}
}

func TestExtractJSONLBody_InvalidJSON(t *testing.T) {
	if got := extractJSONLBody("bad"); got != "" {
		t.Errorf("invalid = %q", got)
	}
}

func TestExtractJSONLBody_NoContentOrTitle(t *testing.T) {
	if got := extractJSONLBody(`{"utility":0.5}`); got != "" {
		t.Errorf("no content = %q", got)
	}
}

func TestReadUtilityFromFile_JSONLStringUtility(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "s.jsonl")
	os.WriteFile(p, []byte(`{"utility":"0.65"}`+"\n"), 0o644)
	if got := readUtilityFromFile(p); got != 0.65 {
		t.Errorf("str = %f, want 0.65", got)
	}
}

func TestReadUtilityFromFile_Nonexistent(t *testing.T) {
	if got := readUtilityFromFile("/no/file.md"); got != 0.5 {
		t.Errorf("nonexist = %f, want 0.5", got)
	}
}

func TestCollectDedupFiles_NeitherDir(t *testing.T) {
	files, err := collectDedupFiles(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if files != nil {
		t.Errorf("expected nil, got %v", files)
	}
}

func TestGroupByContentHash_SkipsNonexistent(t *testing.T) {
	result := groupByContentHash([]string{"/no/file.md"})
	if len(result) != 0 {
		t.Errorf("expected empty, got %d", len(result))
	}
}

func TestPickHighestUtility_SingleFile(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "only.md")
	os.WriteFile(p, []byte("---\nutility: 0.8\n---\nB"), 0o644)
	kept, archived := pickHighestUtility([]string{p})
	if kept != p {
		t.Errorf("not kept: %q", kept)
	}
	if len(archived) != 0 {
		t.Errorf("archived = %d", len(archived))
	}
}

func TestPickHighestUtility_ThreeFiles(t *testing.T) {
	tmp := t.TempDir()
	lo := filepath.Join(tmp, "lo.md")
	mi := filepath.Join(tmp, "mi.md")
	hi := filepath.Join(tmp, "hi.md")
	os.WriteFile(lo, []byte("---\nutility: 0.2\n---\nB"), 0o644)
	os.WriteFile(mi, []byte("---\nutility: 0.5\n---\nB"), 0o644)
	os.WriteFile(hi, []byte("---\nutility: 0.9\n---\nB"), 0o644)
	kept, archived := pickHighestUtility([]string{lo, mi, hi})
	if kept != hi {
		t.Errorf("expected hi, got %q", kept)
	}
	if len(archived) != 2 {
		t.Errorf("archived = %d, want 2", len(archived))
	}
}

func TestBuildDedupResult_RelativePathFallback(t *testing.T) {
	h := map[string][]string{
		"abc123abc123abc123abc123abc123abc123abc123abc123abc123abc123abc1": {"/a/b.md", "/a/c.md"},
	}
	result := buildDedupResult(h, 2, "/nonexistent")
	if result.DuplicateGroups != 1 {
		t.Errorf("groups = %d", result.DuplicateGroups)
	}
	if len(result.Groups[0].Files) != 2 {
		t.Errorf("files = %d", len(result.Groups[0].Files))
	}
}
