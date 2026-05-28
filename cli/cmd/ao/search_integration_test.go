// practices: [wiki-knowledge-surface, ai-assisted-dev]
package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestSearch_Integration_MatchingQuery(t *testing.T) {
	dir := chdirTemp(t)
	setupAgentsDir(t, dir)
	writeFile(t, filepath.Join(dir, ".agents", "learnings", "auth-lesson.md"),
		"---\nname: auth lesson\n---\nAuthentication bypass found in middleware\n")

	oldDryRun := dryRun
	dryRun = false
	oldLocal := searchUseLocal
	oldCASS := searchUseCASS
	oldSC := searchUseSC
	oldType := searchType
	oldLimit := searchLimit
	oldCite := searchCiteType
	t.Cleanup(func() {
		dryRun = oldDryRun
		searchUseLocal = oldLocal
		searchUseCASS = oldCASS
		searchUseSC = oldSC
		searchType = oldType
		searchLimit = oldLimit
		searchCiteType = oldCite
	})
	searchUseLocal = true
	searchUseCASS = false
	searchUseSC = false
	searchType = ""
	searchLimit = 10
	searchCiteType = ""

	out, err := captureStdout(t, func() error {
		return runSearch(searchCmd, []string{"authentication"})
	})
	if err != nil {
		t.Fatalf("search returned error: %v", err)
	}
	if !strings.Contains(strings.ToLower(out), "auth") {
		t.Errorf("expected output to contain 'auth', got: %s", out)
	}
}

func TestSearch_Integration_NoMatch(t *testing.T) {
	dir := chdirTemp(t)
	setupAgentsDir(t, dir)
	writeFile(t, filepath.Join(dir, ".agents", "learnings", "auth-lesson.md"),
		"---\nname: auth lesson\n---\nAuthentication bypass found in middleware\n")

	oldDryRun := dryRun
	dryRun = false
	oldLocal := searchUseLocal
	oldCASS := searchUseCASS
	oldSC := searchUseSC
	oldType := searchType
	oldLimit := searchLimit
	oldCite := searchCiteType
	t.Cleanup(func() {
		dryRun = oldDryRun
		searchUseLocal = oldLocal
		searchUseCASS = oldCASS
		searchUseSC = oldSC
		searchType = oldType
		searchLimit = oldLimit
		searchCiteType = oldCite
	})
	searchUseLocal = true
	searchUseCASS = false
	searchUseSC = false
	searchType = ""
	searchLimit = 10
	searchCiteType = ""

	out, err := captureStdout(t, func() error {
		return runSearch(searchCmd, []string{"zzzznonexistent"})
	})
	if err != nil {
		t.Fatalf("search returned error: %v", err)
	}
	if !strings.Contains(out, "No results found") {
		t.Errorf("expected 'No results found' message, got: %s", out)
	}
}

func TestSearch_Integration_JSONOutput(t *testing.T) {
	dir := chdirTemp(t)
	setupAgentsDir(t, dir)
	// Put a searchable file in patterns/ (searched via grepFiles) and also in
	// learnings/ as markdown so both paths can match.
	writeFile(t, filepath.Join(dir, ".agents", "patterns", "mutex-pattern.md"),
		"---\nname: mutex pattern\n---\nUse sync.Mutex for shared state protection\n")

	oldDryRun := dryRun
	dryRun = false
	oldLocal := searchUseLocal
	oldCASS := searchUseCASS
	oldSC := searchUseSC
	oldType := searchType
	oldLimit := searchLimit
	oldCite := searchCiteType
	oldOutput := output
	t.Cleanup(func() {
		dryRun = oldDryRun
		searchUseLocal = oldLocal
		searchUseCASS = oldCASS
		searchUseSC = oldSC
		searchType = oldType
		searchLimit = oldLimit
		searchCiteType = oldCite
		output = oldOutput
	})
	searchUseLocal = true
	searchUseCASS = false
	searchUseSC = false
	searchType = ""
	searchLimit = 10
	searchCiteType = ""
	output = "json"

	out, err := captureStdout(t, func() error {
		return runSearch(searchCmd, []string{"mutex"})
	})
	if err != nil {
		t.Fatalf("search returned error: %v", err)
	}

	var results []searchResult
	if err := json.Unmarshal([]byte(out), &results); err != nil {
		t.Fatalf("expected valid JSON output, got parse error: %v\nraw: %s", err, out)
	}
	if len(results) == 0 {
		t.Error("expected at least one JSON result for 'mutex' query")
	}
	foundMutex := false
	for _, r := range results {
		if strings.Contains(r.Path, "mutex-pattern") {
			foundMutex = true
			break
		}
	}
	if !foundMutex {
		t.Errorf("expected result path containing 'mutex-pattern', got: %+v", results)
	}
}
