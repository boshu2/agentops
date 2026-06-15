// practices: [fail-closed-safety, hexagonal-architecture]

// Package corpusscan is the CANONICAL deny/PII marker registry and matcher for
// the corpus public/publish pipeline (council verdict
// .agents/council/2026-06-15-corpus-private-public-seam-verdict.md, layer 3).
//
// This package is the SINGLE source of truth for the leak-detection marker set.
// Both `ao corpus scan` and the future CI check (epic ag-k7tq9, bead S7) MUST
// import this package — never re-derive the marker list. Adding or removing a
// marker is a privacy-critical change: edit ONLY here.
//
// FAIL-CLOSED CONTRACT: the scanner DETECTS and FAILS. It never auto-redacts,
// never modifies a file, and any single hit means the input is NOT publishable.
// On any ambiguity (read error, undecidable input) callers must treat the
// content as unsafe.
package corpusscan

import (
	"regexp"
	"strings"
)

// Marker is one canonical leak signal: a named, compiled pattern plus the
// privacy class it guards. Markers are matched case-insensitively (the patterns
// embed `(?i)`) with word boundaries where a bare substring would over-match
// (e.g. "navi" must not fire inside "navigation").
type Marker struct {
	// Name is the stable identifier reported on a hit (machine + human output).
	Name string
	// Class is the privacy category this marker guards.
	Class string
	// Pattern is the compiled, case-insensitive, word-bounded matcher.
	Pattern *regexp.Regexp
}

// Privacy classes for the marker set.
const (
	ClassFleet     = "fleet"      // host/infra topology that must never leak
	ClassClient    = "client"     // client names from AI Partner engagements
	ClassPeer      = "peer-agent" // peer agent / navigator names
	ClassPrivateNS = "private-ns" // private vault namespaces (.finance, .health)
	ClassMyth      = "myth"       // operator-side mythology / persona names
	ClassBrand     = "brand"      // backstage brand/system names (AgentOps, Codex)
	ClassLandmine  = "landmine"   // the Lena deposition-grade landmine phrases
)

// wordBounded wraps a literal term so it only matches as a whole word
// (case-insensitive). Word boundaries prevent absurd false positives like
// "navi" inside "navigation" or "shield" inside "windshield".
func wordBounded(term string) *regexp.Regexp {
	return regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(term) + `\b`)
}

// raw compiles a hand-written case-insensitive pattern (already including its
// own anchoring); used where wordBounded's \b semantics don't fit (IP regex,
// dotted private namespaces, multi-word phrases).
func raw(pat string) *regexp.Regexp {
	return regexp.MustCompile(`(?i)` + pat)
}

// markers is the canonical, ordered registry. Order is stable so hit reports
// are deterministic. Confirmed against the live corpus + the jargon-translator
// forbidden-vocab list (skills/jargon-translator/SKILL.md).
var markers = []Marker{
	// --- Fleet / infra topology ---
	{Name: "bushido", Class: ClassFleet, Pattern: wordBounded("bushido")},
	{Name: "mt-olympus", Class: ClassFleet, Pattern: raw(`\bmt-olympus\b`)},
	{Name: "tailscale", Class: ClassFleet, Pattern: wordBounded("tailscale")},
	{Name: "tailnet", Class: ClassFleet, Pattern: wordBounded("tailnet")},
	// Tailnet CGNAT range (100.64.0.0/10). Match the 100.x.x.x dotted-quad shape
	// used across the fleet docs. The leading edge rejects a digit/dot prefix so
	// it does not fire inside a longer number (e.g. "10.100.1.2"); the trailing
	// edge rejects only a further DIGIT — a trailing dot is a normal sentence end
	// ("...194.61.") and failing to match it would be a fail-OPEN leak hole.
	{Name: "tailnet-ip", Class: ClassFleet, Pattern: raw(`(?:^|[^0-9.])100\.\d{1,3}\.\d{1,3}\.\d{1,3}(?:[^0-9]|$)`)},
	{Name: "shield", Class: ClassFleet, Pattern: wordBounded("shield")},
	{Name: "databricks", Class: ClassFleet, Pattern: wordBounded("databricks")},

	// --- Client / engagement ---
	{Name: "ai-partner", Class: ClassClient, Pattern: raw(`\bai[-_ ]partner\b`)},
	{Name: "lena", Class: ClassClient, Pattern: wordBounded("Lena")},
	{Name: "cristina", Class: ClassClient, Pattern: wordBounded("Cristina")},

	// --- Peer agents / navigators ---
	{Name: "mossylantern", Class: ClassPeer, Pattern: wordBounded("mossylantern")},
	{Name: "emeraldjaguar", Class: ClassPeer, Pattern: wordBounded("emeraldjaguar")},
	// "navi" as a whole word (the navigator persona) — \b stops it from firing
	// inside "navigation", "navigate", "navy", etc.
	{Name: "navi", Class: ClassPeer, Pattern: wordBounded("navi")},

	// --- Private vault namespaces ---
	{Name: "dot-finance", Class: ClassPrivateNS, Pattern: raw(`(?:^|[^a-z0-9])\.finance\b`)},
	{Name: "dot-health", Class: ClassPrivateNS, Pattern: raw(`(?:^|[^a-z0-9])\.health\b`)},

	// --- Operator-side mythology / persona names ---
	// (jargon-translator forbidden-vocab: operator/myth words never appear in
	// client-facing surfaces). Bounded to whole words.
	{Name: "athena", Class: ClassMyth, Pattern: wordBounded("Athena")},
	{Name: "morpheus", Class: ClassMyth, Pattern: wordBounded("Morpheus")},
	{Name: "kwisatz-haderach", Class: ClassMyth, Pattern: raw(`\bkwisatz\s+haderach\b`)},
	{Name: "zettelkasten", Class: ClassMyth, Pattern: wordBounded("Zettelkasten")},
	{Name: "second-brain", Class: ClassMyth, Pattern: raw(`\bsecond[-\s]brain\b`)},

	// --- Backstage brand / system names ---
	// "AgentOps" and "the Codex" are backstage and forbidden in client-facing
	// copy per the jargon-translator non-negotiables.
	{Name: "agentops-brand", Class: ClassBrand, Pattern: wordBounded("AgentOps")},

	// --- Lena landmine phrases (deposition-grade; must NEVER be published) ---
	{Name: "landmine-licenses", Class: ClassLandmine, Pattern: raw(`licenses?\s+don'?t\s+(?:really\s+)?exist`)},
	{Name: "landmine-license-around", Class: ClassLandmine, Pattern: raw(`get\s+around\s+the\s+license`)},
	{Name: "landmine-auto-safe", Class: ClassLandmine, Pattern: raw(`auto\s+mode\s+is\s+safe\s+enough`)},
	{Name: "landmine-no-read", Class: ClassLandmine, Pattern: raw(`you\s+don'?t\s+have\s+to\s+read\s+it`)},
	{Name: "landmine-no-guardrails", Class: ClassLandmine, Pattern: raw(`(?:ai|agents?)\s+(?:doesn'?t|don'?t)\s+(?:really\s+)?need\s+guardrails`)},
}

// Markers returns a copy of the canonical marker registry. The slice header is
// fresh so callers cannot mutate the registry order; the *regexp.Regexp values
// are safe to share (compiled regexps are concurrency-safe for matching).
func Markers() []Marker {
	out := make([]Marker, len(markers))
	copy(out, markers)
	return out
}

// MarkerCount reports the number of markers in the canonical registry. Used by
// callers (and the CI check) to assert the registry is non-empty — an empty
// registry would silently make the fail-closed scanner pass everything.
func MarkerCount() int { return len(markers) }

// Hit is one match of a marker against a line of input.
type Hit struct {
	Marker string `json:"marker"`         // Marker.Name
	Class  string `json:"class"`          // Marker.Class
	Line   int    `json:"line"`           // 1-based line number
	Match  string `json:"match"`          // the matched text (for the human readout)
	Text   string `json:"text,omitempty"` // the offending line, trimmed (context)
}

// ScanText scans a single document's text for every marker and returns all hits
// in deterministic order (by marker registry order, then line number). An empty
// result means the text is clean. ScanText never modifies its input.
func ScanText(text string) []Hit {
	var hits []Hit
	lines := strings.Split(text, "\n")
	for _, m := range markers {
		for i, line := range lines {
			loc := m.Pattern.FindStringIndex(line)
			if loc == nil {
				continue
			}
			hits = append(hits, Hit{
				Marker: m.Name,
				Class:  m.Class,
				Line:   i + 1,
				Match:  strings.TrimSpace(line[loc[0]:loc[1]]),
				Text:   trimContext(line),
			})
		}
	}
	return hits
}

// trimContext clamps a line to a readable length for the human report.
func trimContext(line string) string {
	const max = 200
	s := strings.TrimSpace(line)
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}
