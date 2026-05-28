// Package evolve renders the /evolve loop-mode cron prompt from a versioned
// template at skills/evolve/templates/cron-loop-mode.md.
//
// Load-bearing prompt sections (3-ADR cite, 7-step unblock ladder, Layer-3
// authority, no-self-stop) drift when manually rebuilt every cycle. This
// package externalizes the template and guards drift mechanically via
// VERBATIM-PRESERVE markers whose SHA-256 hashes are stored in the
// frontmatter and re-checked on every render.
//
// Marker syntax:
//
//	<!-- VERBATIM-PRESERVE:start name="<name>" -->
//	... inner content ...
//	<!-- VERBATIM-PRESERVE:end -->
//
// SHA-256 input definition: the renderer hashes the raw bytes strictly
// between the closing "-->" of the start marker and the opening "<!--" of
// the end marker — i.e., the substring excluding both comment delimiters.
// Operators who intentionally edit a marker MUST recompute its hash and
// update the frontmatter's verbatim_markers map; one-off recomputation:
//
//	go run ./cli/cmd/ao evolve template-hash <path>   (future surface)
//
// or simply call ComputeMarkerSHA on the extracted inner content.
package evolve

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

// CronContext is the data passed into the loop-mode cron template. All
// fields are required to be set by the caller (zero values render as empty
// strings / empty lists, which is acceptable for the first cycle).
type CronContext struct {
	// ShippedCommits enumerates commits landed since the last cycle. Each
	// entry renders as "<Sha> (<Bead>[ #<Scenario>]);" — joined into a
	// single line. Empty list → empty rendering.
	ShippedCommits []ShippedCommit

	// NextRecommendedBead is the bead-id the prior cycle's tail suggested
	// the operator queue next. Advisory; Layer-3 authority may override.
	NextRecommendedBead string

	// SubBeadsFiledThisCycle is the list of bead-ids the agent filed via
	// `bd create` during this cycle (typically discovered-from links).
	SubBeadsFiledThisCycle []string

	// TestsDelta is a human-readable summary of test-suite movement, e.g.
	// "+3 passing, 0 new failures" or "no test changes".
	TestsDelta string

	// CronSelfAdjustCounter is the cycle number, incremented each time the
	// cron template is rendered for a new tick. Used in the prompt header.
	CronSelfAdjustCounter int
}

// ShippedCommit describes a single commit landed during the prior cycle.
type ShippedCommit struct {
	Sha      string
	Bead     string
	Scenario string
}

// templateFrontmatter mirrors the YAML head of the cron template.
type templateFrontmatter struct {
	TemplateVersion int               `yaml:"template_version"`
	VerbatimMarkers map[string]string `yaml:"verbatim_markers"`
}

// markerRegexp matches a VERBATIM-PRESERVE block and captures (name, inner).
// Inner content is the substring strictly between the closing "-->" of the
// start marker and the opening "<!--" of the end marker.
var markerRegexp = regexp.MustCompile(
	`<!--\s*VERBATIM-PRESERVE:start\s+name="([^"]+)"\s*-->` +
		`([\s\S]*?)` +
		`<!--\s*VERBATIM-PRESERVE:end\s*-->`,
)

// frontmatterRegexp matches a leading YAML frontmatter block.
var frontmatterRegexp = regexp.MustCompile(`\A---\r?\n([\s\S]*?)\r?\n---\r?\n`)

// Render parses the template at templatePath, validates VERBATIM-PRESERVE
// markers against the stored SHA-256 map, then renders with ctx using
// text/template. Returns the rendered output and any error.
//
// On marker drift, returns the error from VerifyMarkers without rendering.
func Render(templatePath string, ctx CronContext) (string, error) {
	raw, err := os.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("evolve: reading template %q: %w", templatePath, err)
	}
	if err := verifyMarkersFromBytes(raw); err != nil {
		return "", err
	}

	body, err := stripFrontmatter(raw)
	if err != nil {
		return "", fmt.Errorf("evolve: stripping frontmatter in %q: %w", templatePath, err)
	}

	tmpl, err := template.New("cron-loop-mode").Parse(string(body))
	if err != nil {
		return "", fmt.Errorf("evolve: parsing template %q: %w", templatePath, err)
	}

	var out bytes.Buffer
	if err := tmpl.Execute(&out, ctx); err != nil {
		return "", fmt.Errorf("evolve: executing template %q: %w", templatePath, err)
	}
	return out.String(), nil
}

// VerifyMarkers parses the template at templatePath, recomputes the SHA-256
// of each VERBATIM-PRESERVE block's inner content, and compares each to
// the frontmatter's verbatim_markers map. Returns nil on full match.
//
// On any drift, returns an error whose message lists each drifted marker in
// the form:
//
//	VERBATIM-PRESERVE drift detected: marker '<name>' has SHA <got>, expected <want>
//
// Errors stack one-per-line if multiple markers drifted.
func VerifyMarkers(templatePath string) error {
	raw, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("evolve: reading template %q: %w", templatePath, err)
	}
	return verifyMarkersFromBytes(raw)
}

// verifyMarkersFromBytes is the shared implementation between Render and
// VerifyMarkers; pulled out to avoid double file reads.
func verifyMarkersFromBytes(raw []byte) error {
	fm, err := parseFrontmatter(raw)
	if err != nil {
		return fmt.Errorf("evolve: parsing frontmatter: %w", err)
	}
	if fm.VerbatimMarkers == nil {
		fm.VerbatimMarkers = map[string]string{}
	}

	found := extractMarkers(raw)

	// Build sorted union of marker names so error output is deterministic.
	names := unionNames(fm.VerbatimMarkers, found)

	var drifts []string
	for _, name := range names {
		want, wantOK := fm.VerbatimMarkers[name]
		gotInner, gotOK := found[name]
		switch {
		case wantOK && !gotOK:
			drifts = append(drifts, fmt.Sprintf(
				"VERBATIM-PRESERVE drift detected: marker %q declared in frontmatter but missing from body",
				name,
			))
		case !wantOK && gotOK:
			drifts = append(drifts, fmt.Sprintf(
				"VERBATIM-PRESERVE drift detected: marker %q present in body but not declared in frontmatter (got SHA %s)",
				name, ComputeMarkerSHA(gotInner),
			))
		default:
			got := ComputeMarkerSHA(gotInner)
			if got != want {
				drifts = append(drifts, fmt.Sprintf(
					"VERBATIM-PRESERVE drift detected: marker '%s' has SHA %s, expected %s",
					name, got, want,
				))
			}
		}
	}
	if len(drifts) > 0 {
		return errors.New(strings.Join(drifts, "\n"))
	}
	return nil
}

// ComputeMarkerSHA returns the lowercase hex SHA-256 of the marker's inner
// content, exactly as the verifier hashes it. The input is the raw bytes
// strictly between the "-->" of the start marker and the "<!--" of the
// end marker; ComputeMarkerSHA does NOT trim or normalize.
func ComputeMarkerSHA(inner string) string {
	sum := sha256.Sum256([]byte(inner))
	return hex.EncodeToString(sum[:])
}

// parseFrontmatter extracts and decodes the leading YAML frontmatter block
// from raw. Returns an error if the block is missing or malformed.
func parseFrontmatter(raw []byte) (templateFrontmatter, error) {
	var fm templateFrontmatter
	m := frontmatterRegexp.FindSubmatch(raw)
	if m == nil {
		return fm, errors.New("missing leading --- frontmatter block")
	}
	if err := yaml.Unmarshal(m[1], &fm); err != nil {
		return fm, fmt.Errorf("yaml decode: %w", err)
	}
	return fm, nil
}

// stripFrontmatter returns raw with the leading frontmatter block removed.
// If no frontmatter is present, returns raw unchanged.
func stripFrontmatter(raw []byte) ([]byte, error) {
	m := frontmatterRegexp.FindIndex(raw)
	if m == nil {
		return raw, nil
	}
	return raw[m[1]:], nil
}

// extractMarkers scans raw for every VERBATIM-PRESERVE block and returns a
// map from marker name → inner content (verbatim, no trim).
func extractMarkers(raw []byte) map[string]string {
	out := map[string]string{}
	for _, m := range markerRegexp.FindAllSubmatch(raw, -1) {
		name := string(m[1])
		inner := string(m[2])
		out[name] = inner
	}
	return out
}

// unionNames returns the sorted union of keys from a and b.
func unionNames(a map[string]string, b map[string]string) []string {
	set := map[string]struct{}{}
	for k := range a {
		set[k] = struct{}{}
	}
	for k := range b {
		set[k] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
