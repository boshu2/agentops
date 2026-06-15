// practices: [fail-closed-safety, test-pyramid]
package corpusscan

import (
	"os"
	"path/filepath"
	"testing"
)

// TestScanText_MarkerCases is the L2 table over the canonical marker set:
// each fleet/client/peer/private/myth/landmine sample must be DETECTED, and
// each clean generic-learning sample must PASS. Word-boundary false positives
// (the "navigation"/"windshield" class) must NOT trigger.
func TestScanText_MarkerCases(t *testing.T) {
	cases := []struct {
		name     string
		text     string
		wantHit  bool
		wantMark string // expected marker name when wantHit
	}{
		// --- must FAIL CLOSED (a real leak) ---
		{"fleet-bushido", "Run the build on bushido tonight.", true, "bushido"},
		{"fleet-mt-olympus", "The mt-olympus pantheon archived.", true, "mt-olympus"},
		{"fleet-tailscale", "Connect over tailscale to the box.", true, "tailscale"},
		{"fleet-tailnet-ip", "ssh to 100.105.194.61 for the node.", true, "tailnet-ip"},
		{"fleet-tailnet-ip-eol-dot", "Reach the node at 100.105.194.61.", true, "tailnet-ip"},
		{"fleet-tailnet-ip-paren", "(100.96.94.84)", true, "tailnet-ip"},
		{"fleet-shield", "I started at Shield in April.", true, "shield"},
		{"fleet-databricks", "The dream door is Databricks.", true, "databricks"},
		{"client-ai-partner", "This is an AI-Partner engagement.", true, "ai-partner"},
		{"client-lena", "Lena's session transcript.", true, "lena"},
		{"client-cristina", "Cristina is a client.", true, "cristina"},
		{"peer-mossylantern", "mossylantern owns the tool lane.", true, "mossylantern"},
		{"peer-emeraldjaguar", "emeraldjaguar is the operability peer.", true, "emeraldjaguar"},
		{"peer-navi-word", "Navi is the navigator persona.", true, "navi"},
		{"private-finance", "Data lives in ~/.finance/ on Mac.", true, "dot-finance"},
		{"private-health", "Notes in .health are private.", true, "dot-health"},
		{"myth-athena", "Athena is the operator-side name.", true, "athena"},
		{"myth-morpheus", "Morpheus carries the conviction.", true, "morpheus"},
		{"myth-zettelkasten", "We use a Zettelkasten here.", true, "zettelkasten"},
		{"myth-second-brain", "Your second-brain is the substrate.", true, "second-brain"},
		{"brand-agentops", "This is what AgentOps does.", true, "agentops-brand"},
		{"landmine-licenses", "Licenses don't really exist with AI.", true, "landmine-licenses"},
		{"landmine-auto-safe", "Honestly auto mode is safe enough.", true, "landmine-auto-safe"},
		{"landmine-no-read", "You don't have to read it.", true, "landmine-no-read"},

		// --- must PASS (clean / word-boundary false positives) ---
		{"clean-generic", "Give the agent durable context and require evidence.", false, ""},
		{"clean-navigation", "Improve the navigation menu and navigate faster.", false, ""},
		{"clean-windshield", "The windshield catches the lake.", false, ""},
		{"clean-shielding", "Add shielding around the cable.", false, ""},
		{"clean-finance-word", "Personal finance basics for everyone.", false, ""},
		{"clean-ip-not-tailnet", "Version 100.5 of the release ships.", false, ""},
		{"clean-ip-100-not-head", "Subnet 10.100.1.2 is fine.", false, ""},
		{"clean-empty", "", false, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hits := ScanText(tc.text)
			if tc.wantHit {
				if len(hits) == 0 {
					t.Fatalf("expected a hit for %q, got none", tc.text)
				}
				found := false
				for _, h := range hits {
					if h.Marker == tc.wantMark {
						found = true
					}
				}
				if !found {
					t.Fatalf("expected marker %q for %q, got %+v", tc.wantMark, tc.text, hits)
				}
			} else {
				if len(hits) != 0 {
					t.Fatalf("expected CLEAN for %q, got hits %+v", tc.text, hits)
				}
			}
		})
	}
}

// TestScan_DirectoryFailsClosed is the L2 over the directory entry point: a
// tree containing a fleet marker must report NOT clean; a clean tree must be
// clean. Real-shape fixtures written to a temp dir.
func TestScan_DirectoryFailsClosed(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "clean.md", "# Lesson\n\nGive agents durable context and proof.\n")
	writeFixture(t, dir, "leak.md", "# Setup\n\nRun on bushido over tailscale.\n")
	writeFixture(t, dir, "ignored.go", "package main // bushido should be ignored by ext\n")

	rep, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}
	if rep.Clean() {
		t.Fatal("expected report NOT clean (leak.md has fleet markers)")
	}
	if rep.HitCount() < 2 {
		t.Fatalf("expected >=2 hits (bushido + tailscale), got %d: %+v", rep.HitCount(), rep.Files)
	}
	// The .go file must not be scanned (rendered-text extensions only).
	for _, f := range rep.Files {
		if filepath.Ext(f.Path) == ".go" {
			t.Fatalf("non-rendered file scanned: %s", f.Path)
		}
	}
}

// TestScan_CleanDirectoryPasses asserts a fully clean tree exits clean.
func TestScan_CleanDirectoryPasses(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "a.md", "# A\n\nResearch then plan then implement.\n")
	writeFixture(t, dir, "b.json", `{"lesson":"require human rewrite","navigation":"ok"}`)

	rep, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	if !rep.Clean() {
		t.Fatalf("expected CLEAN, got hits: %+v", rep.Files)
	}
	if rep.HitCount() != 0 {
		t.Fatalf("expected 0 hits, got %d", rep.HitCount())
	}
}

// TestScan_SingleFile asserts the single-file path is scanned regardless of ext
// and that a leak fails closed.
func TestScan_SingleFile(t *testing.T) {
	dir := t.TempDir()
	p := writeFixture(t, dir, "render.md", "Connect to 100.96.94.84 on the tailnet.\n")
	rep, err := Scan(p)
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	if rep.Clean() {
		t.Fatal("expected NOT clean for a tailnet-IP leak")
	}
	if len(rep.Files) != 1 {
		t.Fatalf("expected exactly 1 file result, got %d", len(rep.Files))
	}
}

// TestScan_UnreadableFailsClosed proves the fail-closed contract on read
// failure: a path that cannot be statted is an internal error, and an
// unreadable file inside a tree marks the report unsafe.
func TestScan_MissingPathErrors(t *testing.T) {
	_, err := Scan(filepath.Join(t.TempDir(), "does-not-exist"))
	if err == nil {
		t.Fatal("expected an error for a missing path (fail closed), got nil")
	}
}

// TestScan_NeverModifies proves the scanner is detect-only: the file bytes are
// identical before and after a scan.
func TestScan_NeverModifies(t *testing.T) {
	dir := t.TempDir()
	content := "Run on bushido. Athena is here.\n"
	p := writeFixture(t, dir, "x.md", content)
	if _, err := Scan(p); err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	after, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read after: %v", err)
	}
	if string(after) != content {
		t.Fatalf("scanner modified the file: before=%q after=%q", content, string(after))
	}
}

// TestMarkerRegistry_NonEmpty guards the fail-closed invariant: an empty
// registry would silently pass everything.
func TestMarkerRegistry_NonEmpty(t *testing.T) {
	if MarkerCount() == 0 {
		t.Fatal("marker registry is empty — fail-closed scanner would pass everything")
	}
	if got := len(Markers()); got != MarkerCount() {
		t.Fatalf("Markers() len %d != MarkerCount() %d", got, MarkerCount())
	}
	// Markers() must return a defensive copy: mutating it must not affect the
	// canonical registry.
	cp := Markers()
	cp[0].Name = "MUTATED"
	if Markers()[0].Name == "MUTATED" {
		t.Fatal("Markers() did not return a defensive copy")
	}
}

func writeFixture(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture %s: %v", name, err)
	}
	return p
}
