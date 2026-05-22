// This file is the port conformance suite for the wiki bounded context
// (W4 — soc-wiki.7). A conformance suite is a single shared contract test
// that EVERY adapter of a given port must pass: the contract is written once
// against the port interface, then every concrete adapter is run through the
// identical assertions. When a future wave adds a second adapter for a port
// (an in-memory codec, a SQLite-backed index, an alternate freshness model),
// satisfying conformance is one new table row — the contract itself never
// forks.
//
// The suite covers the three wiki ports:
//
//   - ports.FrontmatterCodecPort   adapter: wiki.PortCodec
//   - ports.WikiIndexPort          adapter: wiki.WikiIndex
//   - ports.FreshnessPolicyPort    adapter: freshnessPortAdapter (test-local;
//     see its doc comment)
//
// Each port has exactly one production adapter today, so each conformance
// table currently has one row. The contract functions (conformFrontmatterCodec,
// conformWikiIndex, conformFreshnessPolicy) are written purely against the
// ports.* interface types, so they are reusable verbatim against any future
// adapter.
package wiki

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/ports"
)

// floatTol is the comparison tolerance for confidence and freshness floats.
const floatTol = 1e-9

// ---------------------------------------------------------------------------
// FrontmatterCodecPort conformance
// ---------------------------------------------------------------------------

// frontmatterCodecCase is one golden assertion in the FrontmatterCodecPort
// contract. Every adapter of the port must produce these exact results.
type frontmatterCodecCase struct {
	name string
	// text is the input document for the Decode / DecodeLines assertions.
	text string
	// wantHasFrontmatter is the expected FrontmatterDocument.HasFrontmatter.
	wantHasFrontmatter bool
	// wantBody is the expected FrontmatterDocument.Body.
	wantBody string
	// wantField, if non-empty, names a frontmatter key that must be present
	// in Fields with the value wantFieldValue.
	wantField      string
	wantFieldValue string
	// confInput / confValue / confRaw exercise ParseConfidence.
	confInput any
	confValue float64
	confRaw   string
}

// frontmatterCodecContract is the golden contract for ports.FrontmatterCodecPort.
// It is deliberately shared by every adapter row in TestConformance.
var frontmatterCodecContract = []frontmatterCodecCase{
	{
		name:               "valid frontmatter block",
		text:               "---\ntitle: hello\nconfidence: high\n---\nbody text",
		wantHasFrontmatter: true,
		wantBody:           "body text",
		wantField:          "title",
		wantFieldValue:     "hello",
		confInput:          "high",
		confValue:          ConfidenceHigh,
		confRaw:            "high",
	},
	{
		name:               "no frontmatter is a miss with verbatim body",
		text:               "plain document\nsecond line",
		wantHasFrontmatter: false,
		wantBody:           "plain document\nsecond line",
		confInput:          0.72,
		confValue:          0.72,
		confRaw:            "",
	},
	{
		name:               "unterminated block is a miss",
		text:               "---\ntitle: stuck\nno closing delimiter",
		wantHasFrontmatter: false,
		wantBody:           "---\ntitle: stuck\nno closing delimiter",
		confInput:          "medium",
		confValue:          ConfidenceMedium,
		confRaw:            "medium",
	},
	{
		name:               "malformed confidence coerces to default with Raw",
		text:               "---\nconfidence: probably\n---\ntail",
		wantHasFrontmatter: true,
		wantBody:           "tail",
		wantField:          "confidence",
		wantFieldValue:     "probably",
		confInput:          "probably",
		confValue:          ConfidenceDefault,
		confRaw:            "probably",
	},
}

// conformFrontmatterCodec runs the FrontmatterCodecPort golden contract against
// adapter. It is written only against ports.FrontmatterCodecPort, so any
// adapter of the port can be passed.
func conformFrontmatterCodec(t *testing.T, adapter ports.FrontmatterCodecPort) {
	t.Helper()
	for _, tc := range frontmatterCodecContract {

		t.Run(tc.name, func(t *testing.T) {
			// Contract: Decode returns a non-nil Fields map on every input.
			doc := adapter.Decode(tc.text)
			if doc.Fields == nil {
				t.Fatalf("Decode(%q): Fields is nil; contract requires a non-nil map", tc.name)
			}
			if doc.HasFrontmatter != tc.wantHasFrontmatter {
				t.Errorf("Decode HasFrontmatter = %v, want %v", doc.HasFrontmatter, tc.wantHasFrontmatter)
			}
			if doc.Body != tc.wantBody {
				t.Errorf("Decode Body = %q, want %q", doc.Body, tc.wantBody)
			}
			if tc.wantField != "" {
				got, ok := doc.Fields[tc.wantField]
				if !ok {
					t.Errorf("Decode Fields missing key %q", tc.wantField)
				} else if gotStr, _ := got.(string); gotStr != tc.wantFieldValue {
					t.Errorf("Decode Fields[%q] = %v, want %q", tc.wantField, got, tc.wantFieldValue)
				}
			}

			// Contract: DecodeLines must match Decode for the same logical input.
			lineDoc := adapter.DecodeLines(splitLinesForTest(tc.text))
			if lineDoc.HasFrontmatter != doc.HasFrontmatter {
				t.Errorf("DecodeLines HasFrontmatter = %v, want %v (Decode parity)",
					lineDoc.HasFrontmatter, doc.HasFrontmatter)
			}
			if lineDoc.Body != doc.Body {
				t.Errorf("DecodeLines Body = %q, want %q (Decode parity)", lineDoc.Body, doc.Body)
			}

			// Contract: ParseConfidence always returns a Value in [0,1].
			conf := adapter.ParseConfidence(tc.confInput)
			if conf.Value < 0 || conf.Value > 1 {
				t.Errorf("ParseConfidence(%v) Value = %v, out of [0,1]", tc.confInput, conf.Value)
			}
			if math.Abs(conf.Value-tc.confValue) > floatTol {
				t.Errorf("ParseConfidence(%v) Value = %v, want %v", tc.confInput, conf.Value, tc.confValue)
			}
			if conf.Raw != tc.confRaw {
				t.Errorf("ParseConfidence(%v) Raw = %q, want %q", tc.confInput, conf.Raw, tc.confRaw)
			}
		})
	}
}

// splitLinesForTest splits text on "\n" for the DecodeLines parity assertion.
// It mirrors how FrontmatterCodec.Decode itself splits, so the two entry
// points receive identical logical input.
func splitLinesForTest(text string) []string {
	out := []string{}
	cur := ""
	for _, r := range text {
		if r == '\n' {
			out = append(out, cur)
			cur = ""
			continue
		}
		cur += string(r)
	}
	out = append(out, cur)
	return out
}

// ---------------------------------------------------------------------------
// WikiIndexPort conformance
// ---------------------------------------------------------------------------

// conformWikiIndex runs the WikiIndexPort golden contract against adapter.
// makeFile writes a file under one of the adapter's corpus roots and returns
// its path; touchFile rewrites an already-indexed file with new content. Both
// hooks let the contract drive adapters whose storage layout differs without
// the contract knowing the layout.
//
// The contract asserts the four WikiIndexPort guarantees from the port doc:
//
//  1. Reindex of a fresh corpus reports every file as Added.
//  2. Records returns the index in path-sorted order.
//  3. Reindex after a content change reports exactly the changed path as
//     Updated and rewrites nothing else (the incremental guarantee).
//  4. Reindex after a deletion reports the missing path as Removed.
func conformWikiIndex(
	t *testing.T,
	adapter ports.WikiIndexPort,
	makeFile func(name, content string) string,
	touchFile func(path, content string),
) {
	t.Helper()
	ctx := context.Background()

	alpha := makeFile("alpha.md", "# alpha\noriginal alpha body\n")
	beta := makeFile("beta.md", "# beta\noriginal beta body\n")

	// (1) Fresh corpus: both files reported Added, nothing Updated/Removed.
	first, err := adapter.Reindex(ctx)
	if err != nil {
		t.Fatalf("Reindex (fresh corpus): %v", err)
	}
	if got := len(first.Added); got != 2 {
		t.Fatalf("fresh Reindex Added count = %d, want 2 (%v)", got, first.Added)
	}
	if !containsPath(first.Added, alpha) || !containsPath(first.Added, beta) {
		t.Errorf("fresh Reindex Added = %v, want both %q and %q", first.Added, alpha, beta)
	}
	if len(first.Updated) != 0 || len(first.Removed) != 0 {
		t.Errorf("fresh Reindex Updated=%v Removed=%v, want both empty", first.Updated, first.Removed)
	}

	// (2) Records returns path-sorted order.
	recs, err := adapter.Records()
	if err != nil {
		t.Fatalf("Records: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("Records count = %d, want 2", len(recs))
	}
	if recs[0].Path > recs[1].Path {
		t.Errorf("Records not path-sorted: %q before %q", recs[0].Path, recs[1].Path)
	}
	for _, rec := range recs {
		if rec.ContentHash == "" {
			t.Errorf("Records: record %q has empty ContentHash", rec.Path)
		}
	}

	// (3) Incremental: change exactly one file; only it is Updated.
	hashBefore := hashByPath(recs)
	touchFile(alpha, "# alpha\nCHANGED alpha body\n")
	second, err := adapter.Reindex(ctx)
	if err != nil {
		t.Fatalf("Reindex (after change): %v", err)
	}
	if len(second.Updated) != 1 || second.Updated[0] != alpha {
		t.Fatalf("incremental Reindex Updated = %v, want exactly [%q]", second.Updated, alpha)
	}
	if len(second.Added) != 0 || len(second.Removed) != 0 {
		t.Errorf("incremental Reindex Added=%v Removed=%v, want both empty", second.Added, second.Removed)
	}
	afterChange := hashByPath(mustRecords(t, adapter))
	if afterChange[beta] != hashBefore[beta] {
		t.Errorf("incremental: unchanged file %q hash mutated (%q -> %q)",
			beta, hashBefore[beta], afterChange[beta])
	}
	if afterChange[alpha] == hashBefore[alpha] {
		t.Errorf("incremental: changed file %q hash did not update", alpha)
	}

	// (4) Deletion: remove a file; it is reported Removed.
	if err := os.Remove(beta); err != nil {
		t.Fatalf("remove %q: %v", beta, err)
	}
	third, err := adapter.Reindex(ctx)
	if err != nil {
		t.Fatalf("Reindex (after delete): %v", err)
	}
	if len(third.Removed) != 1 || third.Removed[0] != beta {
		t.Errorf("Reindex Removed = %v, want exactly [%q]", third.Removed, beta)
	}
	if len(third.Updated) != 0 || len(third.Added) != 0 {
		t.Errorf("delete Reindex Added=%v Updated=%v, want both empty", third.Added, third.Updated)
	}
}

// containsPath reports whether paths contains target.
func containsPath(paths []string, target string) bool {
	for _, p := range paths {
		if p == target {
			return true
		}
	}
	return false
}

// hashByPath indexes records by absolute path -> content hash.
func hashByPath(recs []ports.WikiIndexRecord) map[string]string {
	out := make(map[string]string, len(recs))
	for _, r := range recs {
		out[r.Path] = r.ContentHash
	}
	return out
}

// mustRecords fetches Records or fails the test.
func mustRecords(t *testing.T, adapter ports.WikiIndexPort) []ports.WikiIndexRecord {
	t.Helper()
	recs, err := adapter.Records()
	if err != nil {
		t.Fatalf("Records: %v", err)
	}
	return recs
}

// newWikiIndexUnderTest builds the production WikiIndex adapter rooted at a
// fresh temp corpus and returns it together with the file-mutation hooks the
// conformance contract drives it through. The corpus root is AO_AGENTS_DIR so
// the CorpusLocator resolves directly to it.
func newWikiIndexUnderTest(t *testing.T) (ports.WikiIndexPort, func(string, string) string, func(string, string)) {
	t.Helper()
	dir := t.TempDir()
	corpus := filepath.Join(dir, "corpus")
	if err := os.MkdirAll(corpus, 0o750); err != nil {
		t.Fatalf("mkdir corpus: %v", err)
	}
	t.Setenv("AO_AGENTS_DIR", corpus)
	t.Setenv("AO_HOME", "")

	idx, err := NewWikiIndex(filepath.Join(dir, "index.jsonl"), dir)
	if err != nil {
		t.Fatalf("NewWikiIndex: %v", err)
	}

	makeFile := func(name, content string) string {
		p := filepath.Join(corpus, name)
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatalf("write %q: %v", p, err)
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			t.Fatalf("abs %q: %v", p, err)
		}
		return abs
	}
	touchFile := func(path, content string) {
		// Bump mtime forward so the change is unambiguous even on
		// coarse-grained filesystem clocks.
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("rewrite %q: %v", path, err)
		}
		future := time.Now().Add(2 * time.Second)
		if err := os.Chtimes(path, future, future); err != nil {
			t.Fatalf("chtimes %q: %v", path, err)
		}
	}
	return idx, makeFile, touchFile
}

// ---------------------------------------------------------------------------
// FreshnessPolicyPort conformance
// ---------------------------------------------------------------------------

// freshnessPortAdapter adapts wiki.FreshnessPolicy to ports.FreshnessPolicyPort.
//
// The production FreshnessPolicy.Evaluate takes a wiki.Claim, while the port's
// Evaluate takes the claim's class strings — so FreshnessPolicy does not
// satisfy ports.FreshnessPolicyPort directly. This thin adapter bridges the
// two. It lives in the conformance test (not the production package) because
// W4 — soc-wiki.7 is a pure test-addition bead and freshness.go is owned by an
// earlier wave; promoting the adapter into production is a follow-up. Defining
// it here still lets the FreshnessPolicyPort contract be exercised, and proves
// the conformance pattern: a future production adapter is one more table row.
type freshnessPortAdapter struct {
	policy FreshnessPolicy
}

// Evaluate implements ports.FreshnessPolicyPort by delegating to the wrapped
// wiki.FreshnessPolicy and translating between the port and domain shapes.
func (a freshnessPortAdapter) Evaluate(
	claimVolatility, claimAuthority string,
	signal ports.FreshnessChangeSignal,
	verifiedAt time.Time,
) ports.FreshnessVerdict {
	claim := Claim{
		VolatilityClass: VolatilityClass(claimVolatility),
		AuthorityClass:  AuthorityClass(claimAuthority),
	}
	domainSignal := ChangeSignal{
		Kind:       ChangeSignalKind(signal.Kind),
		ObservedAt: signal.ObservedAt,
		Detail:     signal.Detail,
	}
	res := a.policy.Evaluate(claim, domainSignal, verifiedAt)
	return ports.FreshnessVerdict{
		State:         string(res.State),
		Reason:        res.Reason,
		NextReviewAt:  res.NextReviewAt,
		RefreshAction: res.RefreshAction,
	}
}

// freshnessCase is one assertion in the FreshnessPolicyPort contract.
type freshnessCase struct {
	name       string
	volatility string
	authority  string
	signal     ports.FreshnessChangeSignal
	verifiedAt time.Time
	wantState  string
}

// freshnessRefClock is the pinned reference time the contract evaluates
// against, so the clock-aging cases are deterministic.
var freshnessRefClock = time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)

// freshnessPolicyContract is the golden contract for ports.FreshnessPolicyPort.
// It encodes the three port-doc guarantees: Evaluate is total, always sets a
// non-zero NextReviewAt, and a release-bound claim with a post-verification
// change signal goes aging|stale.
var freshnessPolicyContract = []freshnessCase{
	{
		name:       "release-bound claim with newer release signal goes stale",
		volatility: "release-bound",
		authority:  "code",
		signal: ports.FreshnessChangeSignal{
			Kind:       ports.FreshnessSignalRelease,
			ObservedAt: freshnessRefClock.Add(-1 * time.Hour),
		},
		verifiedAt: freshnessRefClock.Add(-24 * time.Hour),
		wantState:  "stale",
	},
	{
		name:       "fast claim with file-hash signal ages",
		volatility: "fast",
		authority:  "agents",
		signal: ports.FreshnessChangeSignal{
			Kind:       ports.FreshnessSignalFileHash,
			ObservedAt: freshnessRefClock.Add(-1 * time.Hour),
		},
		verifiedAt: freshnessRefClock.Add(-24 * time.Hour),
		wantState:  "aging",
	},
	{
		name:       "invariant claim recently verified with no signal stays fresh",
		volatility: "invariant",
		authority:  "code",
		signal:     ports.FreshnessChangeSignal{Kind: ports.FreshnessSignalNone},
		verifiedAt: freshnessRefClock.Add(-24 * time.Hour),
		wantState:  "fresh",
	},
	{
		name:       "release-bound claim past review interval with no signal goes stale",
		volatility: "release-bound",
		authority:  "code",
		signal:     ports.FreshnessChangeSignal{Kind: ports.FreshnessSignalNone},
		// release-bound base interval is 30d; authority=code multiplier 1.5x
		// => 45d. 60 days elapsed exceeds it.
		verifiedAt: freshnessRefClock.Add(-60 * 24 * time.Hour),
		wantState:  "stale",
	},
	{
		name:       "unrecognized classes are handled by conservative fallback",
		volatility: "made-up-volatility",
		authority:  "made-up-authority",
		signal:     ports.FreshnessChangeSignal{Kind: ports.FreshnessSignalNone},
		verifiedAt: freshnessRefClock.Add(-24 * time.Hour),
		wantState:  "fresh",
	},
}

// conformFreshnessPolicy runs the FreshnessPolicyPort golden contract against
// adapter. It is written only against ports.FreshnessPolicyPort.
func conformFreshnessPolicy(t *testing.T, adapter ports.FreshnessPolicyPort) {
	t.Helper()
	for _, tc := range freshnessPolicyContract {

		t.Run(tc.name, func(t *testing.T) {
			verdict := adapter.Evaluate(tc.volatility, tc.authority, tc.signal, tc.verifiedAt)

			// Contract: State is the expected closed-set value.
			if verdict.State != tc.wantState {
				t.Errorf("Evaluate State = %q, want %q", verdict.State, tc.wantState)
			}
			// Contract: State is always one of the three legal values.
			switch verdict.State {
			case "fresh", "aging", "stale":
			default:
				t.Errorf("Evaluate State = %q, not in {fresh,aging,stale}", verdict.State)
			}
			// Contract: NextReviewAt is always non-zero.
			if verdict.NextReviewAt.IsZero() {
				t.Errorf("Evaluate NextReviewAt is zero; contract requires a non-zero time")
			}
			// Contract: a non-fresh verdict carries a refresh action.
			if verdict.State != "fresh" && verdict.RefreshAction == "" {
				t.Errorf("Evaluate State=%q but RefreshAction is empty", verdict.State)
			}
			// Contract: every verdict carries a machine-stable reason.
			if verdict.Reason == "" {
				t.Errorf("Evaluate Reason is empty")
			}
		})
	}
}

// newFreshnessPortAdapter builds the FreshnessPolicyPort adapter under test,
// pinned to the contract's deterministic reference clock.
func newFreshnessPortAdapter() ports.FreshnessPolicyPort {
	return freshnessPortAdapter{
		policy: FreshnessPolicy{Now: func() time.Time { return freshnessRefClock }},
	}
}

// ---------------------------------------------------------------------------
// TestConformance — the single entry point
// ---------------------------------------------------------------------------

// TestConformance is the wiki bounded context's port conformance suite. For
// each wiki port it enumerates that port's adapters and runs every adapter
// through the same golden contract function. Each port currently has one
// production adapter, so each table has one row; a future adapter satisfies
// conformance by adding a row, never by editing the contract.
func TestConformance(t *testing.T) {
	t.Run("FrontmatterCodecPort", func(t *testing.T) {
		adapters := []struct {
			name    string
			adapter ports.FrontmatterCodecPort
		}{
			{name: "wiki.PortCodec", adapter: NewPortCodec()},
		}
		for _, a := range adapters {

			t.Run(a.name, func(t *testing.T) {
				conformFrontmatterCodec(t, a.adapter)
			})
		}
	})

	t.Run("WikiIndexPort", func(t *testing.T) {
		adapters := []struct {
			name string
			make func(t *testing.T) (ports.WikiIndexPort, func(string, string) string, func(string, string))
		}{
			{name: "wiki.WikiIndex", make: newWikiIndexUnderTest},
		}
		for _, a := range adapters {

			t.Run(a.name, func(t *testing.T) {
				adapter, makeFile, touchFile := a.make(t)
				conformWikiIndex(t, adapter, makeFile, touchFile)
			})
		}
	})

	t.Run("FreshnessPolicyPort", func(t *testing.T) {
		adapters := []struct {
			name string
			make func() ports.FreshnessPolicyPort
		}{
			{name: "wiki.freshnessPortAdapter", make: newFreshnessPortAdapter},
		}
		for _, a := range adapters {

			t.Run(a.name, func(t *testing.T) {
				conformFreshnessPolicy(t, a.make())
			})
		}
	})
}
