package evolve

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// canonicalTemplatePath returns the path to the canonical cron template
// relative to this _test.go file. The skills/ tree lives two directories
// up from cli/internal/evolve/.
func canonicalTemplatePath(t *testing.T) string {
	t.Helper()
	// cli/internal/evolve/template_test.go → ../../../skills/...
	p, err := filepath.Abs(filepath.Join("..", "..", "..", "skills", "evolve", "templates", "cron-loop-mode.md"))
	if err != nil {
		t.Fatalf("resolving canonical template path: %v", err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("canonical template not found at %s: %v", p, err)
	}
	return p
}

var updateGolden = flag.Bool("update", false, "rewrite golden files instead of asserting against them")

// loadContextFixture decodes the JSON fixture at path into a CronContext.
func loadContextFixture(t *testing.T, path string) CronContext {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading fixture %s: %v", path, err)
	}
	var ctx CronContext
	if err := json.Unmarshal(raw, &ctx); err != nil {
		t.Fatalf("decoding fixture %s: %v", path, err)
	}
	return ctx
}

// TestRender_GoldenFixture is the primary L2 test: load the fixture
// context, render the canonical template, byte-compare against the golden.
func TestRender_GoldenFixture(t *testing.T) {
	tmpl := canonicalTemplatePath(t)
	ctx := loadContextFixture(t, filepath.Join("testdata", "cron-fixture-1.json"))

	got, err := Render(tmpl, ctx)
	if err != nil {
		t.Fatalf("Render() returned error: %v", err)
	}

	goldenPath := filepath.Join("testdata", "cron-fixture-1.golden.md")
	if *updateGolden {
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatalf("writing golden: %v", err)
		}
		t.Logf("updated golden at %s", goldenPath)
		return
	}

	wantBytes, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("reading golden: %v", err)
	}
	want := string(wantBytes)
	if got != want {
		t.Fatalf("Render() output does not match golden.\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// TestRender_DriftedMarker asserts that Render rejects a template whose
// VERBATIM-PRESERVE marker content has been tampered with.
func TestRender_DriftedMarker(t *testing.T) {
	ctx := loadContextFixture(t, filepath.Join("testdata", "cron-fixture-1.json"))
	drifted := filepath.Join("testdata", "cron-template-drifted.md")

	_, err := Render(drifted, ctx)
	if err == nil {
		t.Fatal("Render() on drifted template returned nil error; want drift error")
	}
	if !strings.Contains(err.Error(), "VERBATIM-PRESERVE drift detected: marker 'no-self-stop'") {
		t.Fatalf("Render() error does not name the drifted marker; got: %v", err)
	}
}

// TestVerifyMarkers_Clean verifies the canonical template passes.
func TestVerifyMarkers_Clean(t *testing.T) {
	if err := VerifyMarkers(canonicalTemplatePath(t)); err != nil {
		t.Fatalf("VerifyMarkers() on canonical template: %v", err)
	}
}

// TestVerifyMarkers_Drift verifies the drifted fixture is rejected and the
// specific marker name appears in the error message.
func TestVerifyMarkers_Drift(t *testing.T) {
	err := VerifyMarkers(filepath.Join("testdata", "cron-template-drifted.md"))
	if err == nil {
		t.Fatal("VerifyMarkers() on drifted template returned nil; want error")
	}
	if !strings.Contains(err.Error(), "marker 'no-self-stop'") {
		t.Fatalf("error did not name the drifted marker; got: %v", err)
	}
	if !strings.Contains(err.Error(), "VERBATIM-PRESERVE drift detected") {
		t.Fatalf("error missing drift sentinel; got: %v", err)
	}
}

// TestParseFrontmatter is an L1 unit covering valid + malformed YAML heads.
func TestParseFrontmatter(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr bool
		check   func(t *testing.T, fm templateFrontmatter)
	}{
		{
			name: "valid full",
			input: "---\n" +
				"template_version: 1\n" +
				"verbatim_markers:\n" +
				"  alpha: deadbeef\n" +
				"  beta: cafef00d\n" +
				"---\nbody here\n",
			check: func(t *testing.T, fm templateFrontmatter) {
				if fm.TemplateVersion != 1 {
					t.Errorf("template_version = %d, want 1", fm.TemplateVersion)
				}
				if fm.VerbatimMarkers["alpha"] != "deadbeef" {
					t.Errorf("alpha = %q, want deadbeef", fm.VerbatimMarkers["alpha"])
				}
				if fm.VerbatimMarkers["beta"] != "cafef00d" {
					t.Errorf("beta = %q, want cafef00d", fm.VerbatimMarkers["beta"])
				}
			},
		},
		{
			name:    "missing frontmatter block",
			input:   "no frontmatter here\nbody\n",
			wantErr: true,
		},
		{
			name: "malformed yaml",
			input: "---\n" +
				"template_version: : :\n" +
				"---\nbody\n",
			wantErr: true,
		},
		{
			name: "empty markers map",
			input: "---\n" +
				"template_version: 2\n" +
				"---\nbody\n",
			check: func(t *testing.T, fm templateFrontmatter) {
				if fm.TemplateVersion != 2 {
					t.Errorf("template_version = %d, want 2", fm.TemplateVersion)
				}
				if len(fm.VerbatimMarkers) != 0 {
					t.Errorf("verbatim_markers should be empty; got %v", fm.VerbatimMarkers)
				}
			},
		},
	}
	for _, tc := range cases {

		t.Run(tc.name, func(t *testing.T) {
			fm, err := parseFrontmatter([]byte(tc.input))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error; got fm=%+v", fm)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.check != nil {
				tc.check(t, fm)
			}
		})
	}
}

// TestComputeMarkerSHA is an L1 unit covering hashing of known inputs.
func TestComputeMarkerSHA(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "empty",
			in:   "",
			// sha256("") = e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
			want: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name: "newline only",
			in:   "\n",
			want: "01ba4719c80b6fe911b091a7c05124b64eeece964e09c058ef8f9805daca546b",
		},
		{
			name: "abc",
			in:   "abc",
			want: "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
		},
		{
			name: "with surrounding whitespace not trimmed",
			in:   "  abc  ",
			// must NOT equal sha256("abc") = ba7816bf...; trimming would break drift detection
			want: "e1df0bfff7ba8ed81085b91ad7dde5d31777855835b80db6d99b56ef9f3aaa6b",
		},
	}
	for _, tc := range cases {

		t.Run(tc.name, func(t *testing.T) {
			got := ComputeMarkerSHA(tc.in)
			if got != tc.want {
				t.Errorf("ComputeMarkerSHA(%q) = %s, want %s", tc.in, got, tc.want)
			}
		})
	}
}
