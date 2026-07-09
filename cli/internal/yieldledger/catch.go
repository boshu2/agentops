package yieldledger

import (
	"regexp"
	"sort"
	"strings"
)

// A Catch is a membrane REFUTE recorded as a class. Unlike an Escape — which
// requires a CONFIRMED→REFUTED PAIR and is structurally rare — a Catch is ANY
// REFUTED gate-verdict that carries a reason AND a domain. Catches are the
// ABUNDANT signal the smart membrane learns from: record them, recall them into
// future reviews, and measure whether a class RECURS compilably. (epic age-zpj5,
// PROVE-FIRST S1; ADR-0011 data-starvation is what this inverts.)
type Catch struct {
	// ClassKey is the catch-native, versioned class identity — see ClassKeyFor.
	// Two catches of the same class collide on it, so recall + recurrence work
	// without anyone authoring a regex.
	ClassKey string
	Domain   string
	Reason   string
	// AffectedPaths are concrete repo-relative FILE paths from the reviewed diff,
	// unioned across the class — INDEPENDENT of any detector, so a judgment-class
	// catch (no detector) is still path-recallable.
	AffectedPaths []string
	// HitCount is the number of DISTINCT (bead, head) occurrences of the class —
	// round-collapsed, so N REFUTED rounds on ONE (bead, head) count as ONE hit
	// (review rounds are not class recurrence). Beads lists the distinct beads.
	HitCount int
	Beads    []string
	// Detector* are present only on the detector-bearing subset (CompileCandidates),
	// carried from the first catch of the class that named a code pattern.
	DetectorPattern     string
	ConstraintPathGlobs string
	DetectorKind        string
	// Instances is one entry per DISTINCT (bead, head) occurrence of the class — the
	// per-instance (head_sha + paths) data the all-instances TP-replay needs (S4): a
	// detector is ASSESSED-COMPILABLE only if it hits EVERY stored bad instance.
	Instances []CatchInstance
}

// CatchInstance is one recorded occurrence of a catch class: the head_sha it was caught
// at and the files it touched. (epic age-zpj5, S4)
type CatchInstance struct {
	HeadSHA       string
	AffectedPaths []string
}

// classKeyVersion prefixes every class key so the normalization can evolve later
// without silently re-bucketing old rows (a v2 key never collides with a v1 key).
const classKeyVersion = "v1"

// ClassKeyFor computes the deterministic class key for a catch. The class-identity
// component is the SEMANTIC class slug when one was supplied (--class), else the
// normalized reason. Keying on a stable semantic slug is what makes a class
// CROSS-BEAD: the same --class on two DIFFERENT beads collides on one key, where a
// reason-derived key drifts with the (often bead-specific) verdict wording — that
// drift is exactly why cross-bead recurrence was invisible (age-jjt8). Shape:
// "v1:" + slug(domain) + "/" + slug(semanticClass | normalize(reason)) and, when a
// detector is present, + "/" + slug(detectorPattern). Pure and versioned: the same
// inputs always yield the same key.
//
// Backward compatibility: an empty semanticClass falls back to the reason path, so
// historical rows (which carry no class) key EXACTLY as before. A semantic-class
// slug that happens to equal a reason's normalized slug collides with it — the two
// ARE then the same class, which is the intended (and benign) semantics. Catching
// trivial rephrasings of the same FREE-TEXT reason as the same key is a NON-goal —
// the per-class human assessment in S4 is what confirms a class before it counts;
// the semantic class is the deliberate way to make a class stable on purpose.
func ClassKeyFor(domain, reason, detectorPattern, semanticClass string) string {
	ident := slugify(normalizeReason(reason))
	if s := slugify(semanticClass); s != "" {
		ident = s
	}
	key := classKeyVersion + ":" + slugify(domain) + "/" + ident
	if strings.TrimSpace(detectorPattern) != "" {
		key += "/" + slugify(detectorPattern)
	}
	return key
}

// classKeyIfCatch returns the class key for a REFUTED verdict that carries REAL
// classifiable content (non-empty, NON-sentinel domain+reason), else "". Stamped
// onto the body at emit so a catch row carries its class identity; DetectCatches
// recomputes the SAME key on read for historical rows that predate the stored field.
// Uses the SAME isClassifiableCatch predicate as the read side so a sentinel-stamped
// row (DomainUnclassified / ReasonUnspecified) is NEVER persisted with a fabricated
// class_key — floor-only at the ledger contract, not just in triage. (epic age-zpj5, S4)
func classKeyIfCatch(disposition, domain, reason, detectorPattern, semanticClass string) string {
	if disposition != DispositionRefuted {
		return ""
	}
	if !isClassifiableCatch(domain, reason) {
		return ""
	}
	return ClassKeyFor(domain, reason, detectorPattern, semanticClass)
}

// reasonStopwords are dropped from a reason before keying so incidental glue words
// don't perturb the class. Kept deliberately small — over-aggressive stopwording
// would over-merge distinct reasons.
var reasonStopwords = map[string]bool{
	"a": true, "an": true, "the": true, "of": true, "to": true, "in": true,
	"on": true, "is": true, "was": true, "for": true, "and": true, "or": true,
	"with": true, "without": true, "that": true, "this": true, "it": true,
	"as": true, "at": true, "by": true, "be": true, "are": true, "but": true,
}

// beadRefContext matches a bead REFERENCE in the "for <id>" / "bead <id>" context
// the pawl emits (e.g. "pawl-review REFUTED for age-55qz.2 (see evidence)"). The id
// is a prefix + one-or-more hyphen segments + optional dot segments — USER-GENERAL,
// so ANY prefix (age-, bd-, a user's own myproj-) matches; "age-" is NOT hardcoded.
// Group 2 is the candidate id; stripBeadRefs strips it only when it also carries a
// DIGIT (see below). Multi-hyphen slugs (age-focus-membrane-bookkeeper-m1wg.13) are
// captured whole so the entire id collapses, not just its first segment.
var beadRefContext = regexp.MustCompile(`(?i)\b(for|bead)\s+([a-z][a-z0-9]*(?:-[a-z0-9]+)*(?:\.[a-z0-9]+)*)`)

// stripBeadRefs removes bead-id fragments from a reason BEFORE keying, so two catch
// reasons that differ ONLY by their bead id ("...for age-55qz.2..." vs "...for age-z1pv...")
// collapse to the SAME class instead of fragmenting into sparse singletons (age-oxzc).
//
// It combines BOTH safe levers so it strips bead ids but never ordinary hyphenated
// words: (a) the id must contain a DIGIT — real bead slugs do (55qz, z1pv, m1wg,
// 04h2.2), whereas "cross-family" / "tmux-send-keys" / "send-keys" do NOT; AND (b) it
// only fires in the "for <id>" / "bead <id>" context the pawl emits — so a compound
// like "pawl-review" (no for/bead before it) or "cross-family" is left untouched even
// though the former is hyphenated. Requiring BOTH is deliberately conservative:
// TRADEOFF — a digit-less bead id ("age-landq-self") or an id NOT in for/bead context
// survives as its own class (a rare miss), which we accept over the worse failure of
// mangling a real technical term. On the live ledger every "for X-<digit>" is in fact
// a bead ref, so the over-strip risk is empirically zero while 14/15 fragmented pawl
// classes collapse. Input is already lowercased by the caller.
func stripBeadRefs(lowered string) string {
	return beadRefContext.ReplaceAllStringFunc(lowered, func(m string) string {
		sub := beadRefContext.FindStringSubmatch(m) // sub[1]=for|bead, sub[2]=candidate id
		if strings.ContainsAny(sub[2], "0123456789") {
			return sub[1] + " " // keep the connective, drop the id
		}
		return m // no digit → not a bead ref we strip (precision over over-stripping)
	})
}

// normalizeReason lowercases, strips bead-id references (see stripBeadRefs), strips
// non-alphanumerics to spaces, collapses whitespace, drops stopwords, and keeps the
// first 8 significant tokens joined by '-'. Pure; same reason → same normalized form.
func normalizeReason(reason string) string {
	lowered := stripBeadRefs(strings.ToLower(reason))
	var b strings.Builder
	for _, r := range lowered {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte(' ')
		}
	}
	tokens := strings.Fields(b.String())
	kept := make([]string, 0, len(tokens))
	for _, t := range tokens {
		if reasonStopwords[t] {
			continue
		}
		kept = append(kept, t)
		if len(kept) == 8 {
			break
		}
	}
	return strings.Join(kept, "-")
}

// placeholderReasonTokens are the disposition/route boilerplate a reason-less pawl
// REFUTE stamps ("pawl-review REFUTED (see evidence)"). They carry NO defect content.
// Kept SEPARATE from reasonStopwords: those are English glue; these are membrane-
// verdict words that a real defect sentence would never consist of ENTIRELY.
var placeholderReasonTokens = map[string]bool{
	"pawl": true, "review": true, "refuted": true, "confirmed": true,
	"see": true, "evidence": true,
}

// placeholderMaxRawLen is the raw-length floor below which a reason cannot carry any
// defect content — it catches bare single-token reasons ("r") the token pass keeps.
const placeholderMaxRawLen = 2

// pawlBoilerplateReason matches the pawl's own verdict STAMP as a reason — a reason
// that BEGINS "pawl-review REFUTED/CONFIRMED …" (whatever bead id or "(see evidence)"
// trails it). This is never a defect description; a real reason names the DEFECT
// ("gate-routing gap …", "missing t.Cleanup …"), never the verdict. Anchoring here
// catches the DIGIT-LESS bead-id variants ("… for age-landq-self (see evidence)") that
// stripBeadRefs deliberately leaves intact, without touching any real defect sentence.
var pawlBoilerplateReason = regexp.MustCompile(`(?i)^\s*pawl[- ]review\s+(refuted|confirmed)\b`)

// IsPlaceholderReason reports whether a catch reason is a NON-substantive placeholder:
// a reason-less pawl verdict ("pawl-review REFUTED (see evidence)"), a bare token
// ("r"), the bare disposition word ("REFUTED"), or anything that reduces to disposition
// boilerplate + bead-id refs with no defect content. Such a reason names no defect, so
// a pre-mortem checklist built from it is pure noise — `ao membrane digest` filters
// these by default (age-7758).
//
// CONSERVATIVE by construction — it must NEVER mis-flag a real defect sentence:
//   - it reuses the SAME normalization the class key uses (lowercase, strip bead-id
//     refs via stripBeadRefs, drop stopwords), so incidental glue never counts; and
//   - it declares a placeholder only when ZERO substantive tokens survive — a token is
//     substantive unless it is disposition boilerplate or a residual bead-id/version
//     fragment (contains a digit). One real content word ("unguarded", "gate-routing",
//     "fail") is enough to make the whole reason substantive.
func IsPlaceholderReason(reason string) bool {
	if len(strings.TrimSpace(reason)) <= placeholderMaxRawLen {
		return true // "", "r" — too short to carry any defect content
	}
	if pawlBoilerplateReason.MatchString(reason) {
		return true // the pawl verdict stamp itself — no defect content, any bead id
	}
	normalized := normalizeReason(reason) // lowercase, strip bead refs, drop stopwords
	if normalized == "" {
		return true // all-stopword / all-bead-ref reason
	}
	for _, tok := range strings.Split(normalized, "-") {
		if tok == "" || placeholderReasonTokens[tok] {
			continue
		}
		if strings.ContainsAny(tok, "0123456789") {
			continue // residual bead-id / version fragment, not defect content
		}
		return false // a surviving content token → substantive
	}
	return true
}

// slugify lowercases and collapses any run of non-alphanumerics to a single '-',
// trimming leading/trailing '-'. An empty input yields "".
func slugify(s string) string {
	lowered := strings.ToLower(s)
	var b strings.Builder
	prevDash := false
	for _, r := range lowered {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevDash = false
		} else if !prevDash {
			b.WriteByte('-')
			prevDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

// DetectCatches returns every REFUTED gate-verdict that carries a reason AND a
// domain, grouped into classes by ClassKey. Judgment-class catches (no detector)
// ARE included — the key is computed from (domain, reason, detector) on READ, so
// the function works on historical catches that predate the stored ClassKey field.
//
// Round-collapse: HitCount counts DISTINCT (bead, head) occurrences, so multiple
// REFUTED rounds on the SAME (bead, head) — i.e. review rounds — count as ONE hit
// for the class, never as class recurrence. AffectedPaths are unioned across the
// class. The result is ordered by first appearance for determinism.
// isClassifiableCatch reports whether a REFUTED gate-verdict carries REAL (non-empty,
// NON-sentinel) domain+reason — the content a catch class is keyed from. The writer
// stamps DomainUnclassified / ReasonUnspecified on a reason-less overturning-REFUTED
// (StampEscapeSentinels); those sentinels are NOT real classifiable content — they
// belong in the unclassified floor, never a synthesized class. (epic age-zpj5, S4)
func isClassifiableCatch(domain, reason string) bool {
	d := strings.TrimSpace(domain)
	r := strings.TrimSpace(reason)
	return d != "" && r != "" && d != DomainUnclassified && r != ReasonUnspecified
}

func DetectCatches(l *Ledger) []Catch {
	if l == nil {
		return nil
	}
	type occ struct{ bead, head string }
	classes := map[string]*Catch{}
	counted := map[string]map[occ]bool{}
	order := []string{}

	for _, ev := range l.Events {
		if ev.Event != EventGateVerdict || ev.GateVerdict == nil {
			continue
		}
		gv := ev.GateVerdict
		if gv.Disposition != DispositionRefuted {
			continue
		}
		// A catch must carry REAL classifiable content. A bare REFUTED with no
		// reason/domain — OR one the writer SENTINEL-stamped (DomainUnclassified /
		// ReasonUnspecified) for a reason-less overturn — is an unclassified floor,
		// NEVER a fabricated class (no-fabrication; epic age-zpj5, S4).
		if !isClassifiableCatch(gv.Domain, gv.Reason) {
			continue
		}
		// Key from the stored semantic Class when present (cross-bead by design), else
		// the normalized reason. gv.Class is "" on every historical row, so those key
		// EXACTLY as they did before this field existed — the read path is fully
		// backward compatible, and a legacy bead-keyed reason stays its own class (never
		// retroactively merged; the ledger is never rewritten). (age-jjt8)
		ck := ClassKeyFor(gv.Domain, gv.Reason, gv.DetectorPattern, gv.Class)
		c, ok := classes[ck]
		if !ok {
			c = &Catch{
				ClassKey:            ck,
				Domain:              gv.Domain,
				Reason:              gv.Reason,
				DetectorPattern:     gv.DetectorPattern,
				ConstraintPathGlobs: gv.ConstraintPathGlobs,
				DetectorKind:        gv.DetectorKind,
			}
			classes[ck] = c
			counted[ck] = map[occ]bool{}
			order = append(order, ck)
		}
		c.AffectedPaths = unionStrings(c.AffectedPaths, gv.AffectedPaths)
		o := occ{bead: ev.BeadID, head: gv.HeadSHA}
		if !counted[ck][o] {
			counted[ck][o] = true
			c.HitCount++
			c.Beads = appendUnique(c.Beads, ev.BeadID)
			c.Instances = append(c.Instances, CatchInstance{
				HeadSHA:       gv.HeadSHA,
				AffectedPaths: append([]string(nil), gv.AffectedPaths...),
			})
		}
	}

	out := make([]Catch, 0, len(order))
	for _, ck := range order {
		out = append(out, *classes[ck])
	}
	return out
}

// CompileCandidates returns the subset of catches that carry a detector pattern —
// the compilable subset Axis-2 assesses (S4). Judgment-class catches (the bulk)
// are excluded: they have no mechanical detector and stay as recall-only memory.
func CompileCandidates(catches []Catch) []Catch {
	var out []Catch
	for _, c := range catches {
		if strings.TrimSpace(c.DetectorPattern) != "" {
			out = append(out, c)
		}
	}
	return out
}

// unionStrings appends to dst every element of add not already present, preserving
// dst's order then add's order, sorting the merged tail for determinism.
func unionStrings(dst, add []string) []string {
	if len(add) == 0 {
		return dst
	}
	seen := map[string]bool{}
	for _, s := range dst {
		seen[s] = true
	}
	fresh := []string{}
	for _, s := range add {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		fresh = append(fresh, s)
	}
	sort.Strings(fresh)
	return append(dst, fresh...)
}

// cleanStrings returns s with empty strings removed, as a NON-nil slice — so an
// emitted array field is always schema-conformant: a required array (refuter_families)
// is never JSON null, AND no item violates the closed schema's items.minLength:1
// (validateGateVerdictEvent does not validate slice items, so the Writer sanitizes
// here). An omitempty field (affected_paths) cleaned to len 0 is omitted, not null.
// (schema↔Go consistency, S1)
func cleanStrings(s []string) []string {
	out := make([]string, 0, len(s))
	for _, v := range s {
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

// appendUnique appends s to dst iff not already present.
func appendUnique(dst []string, s string) []string {
	for _, e := range dst {
		if e == s {
			return dst
		}
	}
	return append(dst, s)
}
