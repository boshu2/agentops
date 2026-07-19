package main

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// spineMarkerRegion isolates the machine-checked spine list in
// docs/architecture/go-cli.md. Everything between the markers must be the exact
// set of published root commands, one backtick-wrapped token each.
var (
	spineRegionRe = regexp.MustCompile(`(?s)<!-- spine:begin.*?-->(.*?)<!-- spine:end -->`)
	spineTokenRe  = regexp.MustCompile("`([^`]+)`")
)

// TestGoCLIDocSpineMatchesApprovedSpine binds the architecture doc's published
// spine list to approvedDefaultSpine. TestDefaultSpineMatchesCathedralCutAllowlist
// compares the cobra tree to the map; this closes the third edge — the prose the
// map is supposed to describe — so a command added to (or dropped from) the map
// without touching docs/architecture/go-cli.md fails here.
func TestGoCLIDocSpineMatchesApprovedSpine(t *testing.T) {
	path := findArchitectureDoc(t)
	raw, err := os.ReadFile(path) // #nosec G304 -- repo-internal doc resolved by walking up from the test cwd
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	region := spineRegionRe.FindSubmatch(raw)
	if region == nil {
		t.Fatalf("%s: could not find the <!-- spine:begin -->..<!-- spine:end --> region", path)
	}
	docSpine := map[string]bool{}
	for _, match := range spineTokenRe.FindAllStringSubmatch(string(region[1]), -1) {
		docSpine[match[1]] = true
	}

	var missing, extra []string
	for command := range approvedDefaultSpine {
		if !docSpine[command] {
			missing = append(missing, command)
		}
	}
	for command := range docSpine {
		if !approvedDefaultSpine[command] {
			extra = append(extra, command)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing) != 0 || len(extra) != 0 {
		t.Fatalf("docs/architecture/go-cli.md spine drift vs approvedDefaultSpine\n"+
			"missing from doc: %s\nextra in doc: %s\n"+
			"update the <!-- spine --> region to match cmd/ao/default_spine_test.go",
			strings.Join(missing, ", "), strings.Join(extra, ", "))
	}
}

// findArchitectureDoc walks up from the test's working directory to the repo
// root and returns the path to docs/architecture/go-cli.md.
func findArchitectureDoc(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for i := 0; i < 8; i++ {
		candidate := filepath.Join(dir, "docs", "architecture", "go-cli.md")
		if _, statErr := os.Stat(candidate); statErr == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatal("docs/architecture/go-cli.md not found walking up from test cwd")
	return ""
}
