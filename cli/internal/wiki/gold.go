// This file implements the GoldCompiler — the raw-lead-to-gold bridge of the
// knowledge flywheel. It mines a repo's private .agents/ corpus (the "raw"
// layer) into a sanitized, public-safe, OKF-compliant wiki in .ao/wiki/ (the
// "gold" layer).
//
// OKF = Google Cloud's Open Knowledge Format: portable markdown + structured
// frontmatter, `type` required, reserved index.md (catalog) and log.md
// (history), file-path-is-identity, plain-markdown cross-links. It formalizes
// Karpathy's LLM-wiki pattern into an interoperable surface any agent or tool
// can read without a proprietary SDK.
//
// The gap this closes: ao already PRODUCES .agents/ (forge/curate/sessions)
// and REFINES it (the promotion ratchet's maturity/confidence fields), but
// never COMPILES it to portable gold. .agents/ is private (secrets, $HOME
// paths, session UUIDs) and noisy (provisional captures beside canonical
// truth). GoldCompiler does the two things the gap needs:
//
//  1. MINE     — a promotion gate; only durable entries cross into gold.
//  2. SANITIZE — secrets/$HOME (canonical llm.Redact) + session UUIDs scrubbed
//     BEFORE anything is written.
//
// then EMIT OKF and report a LINT summary. ao compile already renders
// .agents/ -> .agents/compiled/ for in-repo agents; GoldCompiler is the
// distinct publish step: sanitized, durability-gated, OKF-portable, to
// .ao/wiki/.
package wiki

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/llm"
)

// goldMineDirs are the .agents/ subdirectories whose entries are candidates
// for promotion into the gold wiki. Ephemeral/working dirs (runs, pool,
// handoff, briefings, sessions) are deliberately excluded — gold is durable
// knowledge, not session scratch.
var goldMineDirs = []string{
	"learnings", "findings", "patterns", "research",
	"design", "decisions", "council", "planning-rules",
}

// goldConfidenceFloor is the default confidence threshold for promotion when
// no stronger durability signal (maturity/tier/rewards) is present.
const goldConfidenceFloor = 0.70

var (
	durableMaturity = map[string]bool{
		"established": true, "canonical": true, "promoted": true,
		"stable": true, "verified": true,
	}
	noiseMaturity = map[string]bool{
		"provisional": true, "draft": true, "pending": true, "candidate": true,
	}
	// okfType maps a .agents type to the OKF `type` field (the one required
	// field; producer-defined per the spec).
	okfType = map[string]string{
		"learning": "Learning", "finding": "Finding", "pattern": "Pattern",
		"decision": "Decision", "research": "Research", "design": "Design",
		"council": "Verdict", "planning-rule": "PlanningRule",
	}
	// okfStatus maps maturity to a handbook trust-status label
	// (authoritative / draft / historical / deprecated / proposal).
	okfStatus = map[string]string{
		"canonical": "authoritative", "established": "authoritative",
		"promoted": "authoritative", "stable": "authoritative",
		"verified": "authoritative", "provisional": "draft", "draft": "draft",
		"deprecated": "deprecated", "superseded": "historical",
	}
	uuidPattern = regexp.MustCompile(`\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b`)
	// wikilinkPattern matches [[slug]] references. Anchored to a slug-shaped
	// token (alnum first char, then slug chars) so it does NOT match bash
	// double-bracket tests like [[ -f x ]], [[:space:]], or [[ $rc -eq 0 ]].
	wikilinkPattern = regexp.MustCompile(`\[\[[A-Za-z0-9][A-Za-z0-9_/.-]*\]\]`)
	// tagLinePattern matches structured tag lines in an Applicability section
	// ("- Work shapes: a, b", "- Scope tags: c", "- Languages: go, python").
	tagLinePattern = regexp.MustCompile(`(?i)^\s*[-*]?\s*(?:work shapes?|scope tags?|languages?|tags?):\s*(.+)$`)
	// homePathPattern catches ANY user's home dir, not just the current $HOME
	// that llm.Redact scrubs. A publish-to-gold step must be conservative:
	// another machine's /Users/<someone> or /home/<someone> is a private-
	// surface leak even though it isn't the redactor's $HOME.
	goldHomePattern = regexp.MustCompile(`/(Users|home)/[A-Za-z0-9._\-]+`)
)

// GoldCompiler compiles a .agents/ corpus into an OKF wiki under OutDir.
type GoldCompiler struct {
	// AgentsDir is the resolved private corpus root (the raw layer).
	AgentsDir string
	// OutDir is the gold wiki root (e.g. .ao/wiki).
	OutDir string
	// ConfidenceFloor overrides goldConfidenceFloor when > 0.
	ConfidenceFloor float64
	// Now is injectable for deterministic output; defaults to time.Now.
	Now func() time.Time
}

// GoldStats is the outcome of a Compile run.
type GoldStats struct {
	Scanned    int            `json:"scanned"`
	Promoted   int            `json:"promoted"`
	Rejected   int            `json:"rejected"`
	Redactions int            `json:"redactions"`
	Rejections map[string]int `json:"rejections"`
	ByType     map[string]int `json:"by_type"`
	Links      int            `json:"links"`
	Lint       []string       `json:"lint"`
}

type goldDoc struct {
	okfType, title, description, timestamp, status, sourceDigest, body string
	resource                                                           string // OKF standard: sanitized source pointer
	tags                                                               []string
	confidence                                                         float64
	category, slug                                                     string
	srcKeys                                                            []string // identifiers this doc can be referenced by
	relatedRefs                                                        []string // explicit cross-refs from frontmatter
}

func (c *GoldCompiler) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now().UTC()
}

func (c *GoldCompiler) floor() float64 {
	if c.ConfidenceFloor > 0 {
		return c.ConfidenceFloor
	}
	return goldConfidenceFloor
}

// isDurable is the promotion gate: it decides whether a raw entry has earned a
// place in the gold layer, and returns a human reason either way (never a
// silent drop). The whole point of the bridge — most raw captures are
// provisional noise; gold is reviewed, durable knowledge.
func (c *GoldCompiler) isDurable(fields map[string]any, body string) (bool, string) {
	maturity := strings.ToLower(fieldStr(fields, "maturity"))
	conf := ParseConfidence(fields["confidence"]).Value
	hasConf := fields["confidence"] != nil
	tier := strings.ToLower(fieldStr(fields, "tier"))
	rewards := fieldFloat(fields, "reward_count") + fieldFloat(fields, "helpful_count")
	harms := fieldFloat(fields, "harmful_count")

	switch {
	case durableMaturity[maturity]:
		return true, "maturity=" + maturity
	case tier == "gold":
		return true, "tier=gold"
	case rewards > 0 && harms == 0:
		return true, fmt.Sprintf("rewards=%.0f", rewards)
	case hasConf && conf >= c.floor() && !noiseMaturity[maturity]:
		return true, fmt.Sprintf("confidence=%.2f", conf)
	case noiseMaturity[maturity]:
		return false, "maturity=" + maturity
	case len(strings.TrimSpace(body)) < 120:
		return false, "too-short"
	default:
		return false, fmt.Sprintf("below-floor(conf=%.2f)", conf)
	}
}

// sanitize runs the canonical secret/$HOME redactor then scrubs session UUIDs.
// Returns the clean text and a count of spans changed.
func sanitize(text string) (string, int) {
	clean := llm.Redact(text) // canonical: secrets + current $HOME
	n := strings.Count(clean, "[REDACTED]") + strings.Count(clean, "/FIXTURE")
	// publish-grade layer: any user's home path + session UUIDs. Count actual
	// matches (not output markers) so the redaction stat never overclaims.
	n += len(goldHomePattern.FindAllString(clean, -1))
	clean = goldHomePattern.ReplaceAllString(clean, "~")
	n += len(uuidPattern.FindAllString(clean, -1))
	clean = uuidPattern.ReplaceAllString(clean, "[session]")
	return clean, n
}

// Compile runs the full bridge: mine -> sanitize -> emit OKF -> lint.
func (c *GoldCompiler) Compile(dryRun bool) (GoldStats, error) {
	stats := GoldStats{Rejections: map[string]int{}, ByType: map[string]int{}}
	codec := NewFrontmatterCodec()
	var docs []goldDoc

	for _, sub := range goldMineDirs {
		dir := filepath.Join(c.AgentsDir, sub)
		entries, err := collectMarkdown(dir)
		if err != nil {
			return stats, err
		}
		for _, path := range entries {
			raw, err := os.ReadFile(path) //nolint:gosec // corpus path, operator-owned
			if err != nil {
				continue
			}
			stats.Scanned++
			doc := codec.Decode(string(raw))
			durable, reason := c.isDurable(doc.Fields, doc.Body)
			if !durable {
				stats.Rejected++
				stats.Rejections[reason]++
				continue
			}
			clean, n := sanitize(doc.Body)
			stats.Redactions += n

			mtype := strings.ToLower(fieldStr(doc.Fields, "type"))
			if mtype == "" {
				mtype = strings.TrimSuffix(sub, "s")
			}
			ot := okfType[mtype]
			if ot == "" {
				ot = strings.Title(mtype) //nolint:staticcheck // ASCII tags only
			}
			maturity := strings.ToLower(fieldStr(doc.Fields, "maturity"))
			fmTitle, _ := sanitize(fieldStr(doc.Fields, "title")) // explicit title wins, sanitized
			title := flattenWikilinks(cleanTitle(firstNonEmpty(strings.TrimSpace(fmTitle), firstSentence(clean)), filepath.Base(path)))
			rawID := nullableID(fieldStr(doc.Fields, "id"))
			fileStem := nullableID(strings.TrimSuffix(filepath.Base(path), ".md"))
			// slug prefers a real id, then the title, then the filename — so a
			// sentinel id ("null") never becomes the slug when a title exists.
			slugSrc := firstNonEmpty(rawID, title, fileStem)
			digest := sha256.Sum256([]byte(firstNonEmpty(rawID, fileStem, title) + fieldStr(doc.Fields, "source_session")))
			status := okfStatus[maturity]
			if status == "" {
				if strings.HasPrefix(reason, "confidence") {
					status = "draft"
				} else {
					status = "authoritative"
				}
			}
			conf := goldConfidenceFloor
			if doc.Fields["confidence"] != nil {
				conf = ParseConfidence(doc.Fields["confidence"]).Value
			}
			// srcKeys: every identifier another entry might use to [[link]] here
			// — the source id, filename stem, frontmatter name, and title slug.
			srcKeys := dedupeStrings([]string{
				slugify(rawID), slugify(fileStem),
				slugify(fieldStr(doc.Fields, "name")), slugify(title),
			})
			// explicit cross-refs from frontmatter (comma/space-separated)
			var relatedRefs []string
			for _, k := range []string{"related", "related_learning", "related_finding", "parent_epic"} {
				relatedRefs = append(relatedRefs, splitRefs(fieldStr(doc.Fields, k))...)
			}
			gd := goldDoc{
				okfType:      ot,
				title:        truncateWords(title, 120),
				description:  truncateWords(flattenWikilinks(titlePrefix.ReplaceAllString(firstSentence(clean), "")), 200),
				resource:     filepath.Join(filepath.Base(c.AgentsDir), sub, filepath.Base(path)),
				tags:         harvestTags(doc.Fields, clean, mtype, strings.ToLower(fieldStr(doc.Fields, "tier"))),
				timestamp:    firstNonEmpty(fieldStr(doc.Fields, "date"), c.now().Format("2006-01-02")),
				status:       status,
				confidence:   conf,
				sourceDigest: hex.EncodeToString(digest[:])[:12],
				body:         dedupeSections(strings.TrimRight(clean, "\n"), title),
				category:     sub,
				slug:         slugify(slugSrc),
				srcKeys:      srcKeys,
				relatedRefs:  relatedRefs,
			}
			docs = append(docs, gd)
			stats.Promoted++
			stats.ByType[ot]++
		}
	}

	if dryRun {
		return stats, nil
	}

	// Finalize slugs (dedup within category) BEFORE link rewriting so relative
	// paths are stable, then weave the OKF cross-link graph.
	c.finalizeSlugs(docs)
	stats.Links = c.weaveLinks(docs)

	if err := c.emit(docs, stats); err != nil {
		return stats, err
	}
	stats.Lint = c.lint()
	return stats, nil
}

// finalizeSlugs assigns each doc a unique slug within its category.
func (c *GoldCompiler) finalizeSlugs(docs []goldDoc) {
	seen := map[string]bool{}
	for i := range docs {
		slug := docs[i].slug
		for n := 2; seen[docs[i].category+"/"+slug]; n++ {
			slug = fmt.Sprintf("%s-%d", docs[i].slug, n)
		}
		seen[docs[i].category+"/"+slug] = true
		docs[i].slug = slug
	}
}

// weaveLinks rewrites [[wikilink]] references into OKF relative-path markdown
// links when the target resolves to another promoted doc, and appends a
// "## Related" section listing the resolved edges. Unresolved references are
// flattened to plain text (no dead wiki-syntax in the OKF output). Returns the
// number of resolved edges.
func (c *GoldCompiler) weaveLinks(docs []goldDoc) int {
	index := map[string]int{} // slugified srcKey -> doc position
	for i := range docs {
		for _, k := range docs[i].srcKeys {
			if k != "" {
				if _, exists := index[k]; !exists {
					index[k] = i
				}
			}
		}
	}
	resolved := 0
	for i := range docs {
		var related []int
		seen := map[int]bool{i: true} // never self-link
		body := wikilinkPattern.ReplaceAllStringFunc(docs[i].body, func(m string) string {
			target := slugify(strings.Trim(m, "[]"))
			j, ok := index[target]
			if !ok {
				return strings.Trim(m, "[]") // flatten unresolved
			}
			rel := relLink(docs[i].category, docs[j].category, docs[j].slug)
			if !seen[j] {
				seen[j] = true
				related = append(related, j)
			}
			resolved++
			return fmt.Sprintf("[%s](%s)", docs[j].title, rel)
		})
		// explicit frontmatter cross-refs → Related edges (no body to rewrite)
		for _, ref := range docs[i].relatedRefs {
			j, ok := index[slugify(ref)]
			if !ok || seen[j] {
				continue
			}
			seen[j] = true
			related = append(related, j)
			resolved++
		}
		if len(related) > 0 {
			var sb strings.Builder
			sb.WriteString(body)
			sb.WriteString("\n\n## Related\n\n")
			for _, j := range related {
				rel := relLink(docs[i].category, docs[j].category, docs[j].slug)
				fmt.Fprintf(&sb, "- [%s](%s) — `%s`\n", docs[j].title, rel, docs[j].okfType)
			}
			body = sb.String()
		}
		docs[i].body = body
	}
	return resolved
}

// emit writes the gold docs plus the OKF reserved index.md (catalog) and
// log.md (history) files.
func (c *GoldCompiler) emit(docs []goldDoc, stats GoldStats) error {
	// Idempotent rebuild: gold is fully derived from .agents/, so clear the
	// tool's own output tree first. This keeps reruns deterministic and stops
	// stale slugs (renamed/removed source entries) from accumulating. Guarded
	// to the configured OutDir, which the compiler owns.
	if c.OutDir != "" && c.OutDir != "/" && c.OutDir != "." {
		if err := os.RemoveAll(c.OutDir); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(c.OutDir, 0o755); err != nil {
		return err
	}
	byCat := map[string][]goldDoc{}
	for i := range docs {
		catDir := filepath.Join(c.OutDir, docs[i].category)
		if err := os.MkdirAll(catDir, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(catDir, docs[i].slug+".md"), []byte(docs[i].render()), 0o644); err != nil { //nolint:gosec
			return err
		}
		byCat[docs[i].category] = append(byCat[docs[i].category], docs[i])
	}

	// per-category catalog — authoritative first, then newest; each entry
	// carries its description so the catalog supports OKF progressive
	// disclosure (scan the index, then open the doc).
	cats := make([]string, 0, len(byCat))
	for cat, items := range byCat {
		cats = append(cats, cat)
		sort.Slice(items, func(a, b int) bool {
			ai, aj := items[a].status == "authoritative", items[b].status == "authoritative"
			if ai != aj {
				return ai
			}
			return items[a].timestamp > items[b].timestamp
		})
		var b strings.Builder
		fmt.Fprintf(&b, "# %s\n\n_%d entries · %d authoritative. OKF catalog._\n\n", cat, len(items), countAuthoritative(items))
		for _, d := range items {
			fmt.Fprintf(&b, "- [%s](%s.md) — `%s` · %s · %s\n", d.title, d.slug, d.okfType, d.status, d.timestamp)
			if desc := strings.TrimSpace(d.description); desc != "" && desc != d.title {
				fmt.Fprintf(&b, "  %s\n", desc)
			}
		}
		if err := os.WriteFile(filepath.Join(c.OutDir, cat, "index.md"), []byte(b.String()), 0o644); err != nil { //nolint:gosec
			return err
		}
	}
	sort.Strings(cats)

	// root catalog (index.md)
	var root strings.Builder
	root.WriteString("# .ao/wiki — knowledge gold (OKF)\n\n")
	fmt.Fprintf(&root, "_Compiled from .agents/ on %s._\n", c.now().Format(time.RFC3339))
	fmt.Fprintf(&root, "_%d durable entries · %d redactions · %d raw entries gated out._\n\n## Catalog\n\n",
		stats.Promoted, stats.Redactions, stats.Rejected)
	for _, cat := range cats {
		fmt.Fprintf(&root, "- [%s/](%s/index.md) — %d entries (%d authoritative)\n", cat, cat, len(byCat[cat]), countAuthoritative(byCat[cat]))
	}
	if err := os.WriteFile(filepath.Join(c.OutDir, "index.md"), []byte(root.String()), 0o644); err != nil { //nolint:gosec
		return err
	}

	// change history (log.md)
	sort.Slice(docs, func(a, b int) bool { return docs[a].timestamp > docs[b].timestamp })
	var log strings.Builder
	log.WriteString("# log.md — change history (OKF reserved)\n\n")
	for _, d := range docs {
		fmt.Fprintf(&log, "- %s · `%s` · [%s](%s/%s.md) (%s)\n", d.timestamp, d.okfType, d.title, d.category, d.slug, d.status)
	}
	if err := os.WriteFile(filepath.Join(c.OutDir, "log.md"), []byte(log.String()), 0o644); err != nil { //nolint:gosec
		return err
	}

	return c.writeManifest(docs, stats)
}

// manifestEntry is one gold document's machine-readable record.
type manifestEntry struct {
	Type         string   `json:"type"`
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	Status       string   `json:"status"`
	Tags         []string `json:"tags"`
	Timestamp    string   `json:"timestamp"`
	Path         string   `json:"path"`
	Resource     string   `json:"resource,omitempty"`
	Confidence   float64  `json:"confidence"`
	SourceDigest string   `json:"source_digest"`
}

// writeManifest emits manifest.json — a single machine-readable catalog of the
// whole gold wiki so an agent or external OKF tool can consume it without
// parsing every markdown file. This is the catalog-export side of OKF interop.
func (c *GoldCompiler) writeManifest(docs []goldDoc, stats GoldStats) error {
	entries := make([]manifestEntry, 0, len(docs))
	for _, d := range docs {
		if d.tags == nil {
			d.tags = []string{}
		}
		entries = append(entries, manifestEntry{
			Type: d.okfType, Title: d.title, Description: d.description,
			Status: d.status, Tags: d.tags, Timestamp: d.timestamp,
			Path: d.category + "/" + d.slug + ".md", Resource: d.resource,
			Confidence: d.confidence, SourceDigest: d.sourceDigest,
		})
	}
	sort.Slice(entries, func(a, b int) bool { return entries[a].Path < entries[b].Path })
	manifest := map[string]any{
		"format":    "okf",
		"generator": "ao wiki gold",
		"generated": c.now().Format(time.RFC3339),
		"count":     len(entries),
		"documents": entries,
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	return os.WriteFile(filepath.Join(c.OutDir, "manifest.json"), append(data, '\n'), 0o644) //nolint:gosec
}

// lint is a minimal OKF conformance check over the emitted tree.
func (c *GoldCompiler) lint() []string {
	var problems []string
	if !fileExists(filepath.Join(c.OutDir, "index.md")) {
		problems = append(problems, "missing root index.md (OKF reserved)")
	}
	if !fileExists(filepath.Join(c.OutDir, "log.md")) {
		problems = append(problems, "missing root log.md (OKF reserved)")
	}
	entries, _ := os.ReadDir(c.OutDir)
	codec := NewFrontmatterCodec()
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if !fileExists(filepath.Join(c.OutDir, e.Name(), "index.md")) {
			problems = append(problems, e.Name()+"/: missing catalog index.md")
		}
		mds, _ := collectMarkdown(filepath.Join(c.OutDir, e.Name()))
		for _, md := range mds {
			if filepath.Base(md) == "index.md" {
				continue
			}
			raw, _ := os.ReadFile(md) //nolint:gosec
			doc := codec.Decode(string(raw))
			if fieldStr(doc.Fields, "type") == "" {
				rel, _ := filepath.Rel(c.OutDir, md)
				problems = append(problems, rel+": missing required `type`")
			}
		}
	}
	return problems
}

func (d goldDoc) render() string {
	tags := "[]"
	if len(d.tags) > 0 {
		tags = "[" + strings.Join(d.tags, ", ") + "]"
	}
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "type: %s\n", d.okfType) // OKF required field
	fmt.Fprintf(&b, "title: %s\n", yamlScalar(d.title))
	fmt.Fprintf(&b, "description: %s\n", yamlScalar(d.description))
	if d.resource != "" {
		fmt.Fprintf(&b, "resource: %s\n", d.resource) // OKF standard field
	}
	fmt.Fprintf(&b, "tags: %s\n", tags)
	fmt.Fprintf(&b, "timestamp: %s\n", d.timestamp)
	fmt.Fprintf(&b, "status: %s\n", d.status) // trust label (handbook ext)
	fmt.Fprintf(&b, "confidence: %.2f\n", d.confidence)
	fmt.Fprintf(&b, "source_digest: %s\n", d.sourceDigest) // sanitized provenance
	b.WriteString("---\n\n")
	b.WriteString(d.body)
	b.WriteString("\n")
	return b.String()
}

// --- small helpers -------------------------------------------------------

func collectMarkdown(dir string) ([]string, error) {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return nil, nil //nolint:nilerr // absent dir is not an error
	}
	var out []string
	err = filepath.Walk(dir, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil //nolint:nilerr
		}
		if !fi.IsDir() && strings.HasSuffix(p, ".md") {
			out = append(out, p)
		}
		return nil
	})
	sort.Strings(out)
	return out, err
}

func fieldStr(fields map[string]any, key string) string {
	v, ok := fields[key]
	if !ok || v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

func fieldFloat(fields map[string]any, key string) float64 {
	s := strings.TrimSpace(fieldStr(fields, key))
	if s == "" {
		return 0
	}
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// genericHeading are boilerplate section labels that make terrible titles
// ("Intent", "Summary"); firstSentence skips them to find real content.
var genericHeading = map[string]bool{
	"intent": true, "summary": true, "overview": true, "context": true,
	"tl;dr": true, "tldr": true, "background": true, "what we learned": true,
	"pattern": true, "detection question": true, "notes": true, "details": true,
	"description": true, "what": true, "why": true, "how": true,
}

// sentenceEnd returns the index of the first real sentence-ending punctuation
// in line, or -1. A boundary is `.`/`!`/`?` followed by whitespace or
// end-of-line — so periods inside tokens (SKILL.md, unique_by(.key), .service,
// 0.85) are NOT treated as sentence ends.
func sentenceEnd(line string) int {
	for i := 0; i < len(line); i++ {
		if c := line[i]; c == '.' || c == '!' || c == '?' {
			if i == len(line)-1 || line[i+1] == ' ' || line[i+1] == '\t' {
				return i
			}
		}
	}
	return -1
}

func firstSentence(body string) string {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(strings.TrimLeft(line, "#"))
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "**ID") ||
			strings.HasPrefix(line, "**Category") || strings.HasPrefix(line, "**Confidence") {
			continue
		}
		if genericHeading[strings.ToLower(strings.TrimRight(line, ":"))] {
			continue // a boilerplate section label is not a title
		}
		if i := sentenceEnd(line); i > 0 && i < len(line)-1 {
			return strings.TrimSpace(line[:i+1])
		}
		return line
	}
	return ""
}

// titlePrefix strips a redundant leading "<Type>: " label (e.g.
// "Finding: ...", "Learning: ...") so the title isn't doubled against the
// OKF type field.
var titlePrefix = regexp.MustCompile(`^(Learning|Finding|Pattern|Decision|Research|Design|Verdict):\s+`)

func cleanTitle(s, fallback string) string {
	s = titlePrefix.ReplaceAllString(strings.TrimSpace(s), "")
	s = strings.TrimSpace(s)
	if s == "" {
		return strings.TrimSuffix(fallback, ".md")
	}
	return s
}

// yamlScalar renders s as a YAML-safe double-quoted scalar so colons, hashes,
// and other special characters in titles/descriptions can't corrupt the
// frontmatter block (the corruption that made an OKF consumer unable to read
// the emitted `type` field).
func yamlScalar(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

// nullableID returns the id unless it is a YAML/JSON null sentinel, in which
// case it returns "" so slug selection falls back to the title.
func nullableID(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "null", "none", "nil", "<nil>", "~":
		return ""
	}
	return s
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// truncateWords trims s to at most n bytes at a word boundary, appending an
// ellipsis — so titles/descriptions never cut mid-word ("...starter-bund").
func truncateWords(s string, n int) string {
	if len(s) <= n {
		return s
	}
	cut := s[:n]
	if i := strings.LastIndexAny(cut, " \t-/,;:"); i > n/2 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, " \t-/,;:") + "…"
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// relLink builds a relative markdown link target from a doc in fromCat to a
// doc (toCat/slug). Same category -> "slug.md"; cross category -> "../toCat/slug.md".
func relLink(fromCat, toCat, slug string) string {
	if fromCat == toCat {
		return slug + ".md"
	}
	return "../" + toCat + "/" + slug + ".md"
}

// normalizeText lowercases and collapses whitespace for content comparison.
func normalizeText(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

// dedupeSections removes "## Heading" blocks whose body merely repeats the
// title or an earlier block — the Summary/Pattern echo that the
// finding-compiler emits — and drops the redundant "## Source" block, whose
// provenance now lives in the resource/source_digest frontmatter. Unique
// sections (Detection Question, Checklist, Applicability, ...) are preserved.
func dedupeSections(body, title string) string {
	lines := strings.Split(body, "\n")
	titleNorm := normalizeText(title)
	seen := map[string]bool{}
	var out []string
	i := 0
	for i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), "## ") {
		out = append(out, lines[i]) // H1 + any intro before the first section
		i++
	}
	for i < len(lines) {
		block := []string{lines[i]}
		name := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[i]), "##")))
		i++
		for i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), "## ") {
			block = append(block, lines[i])
			i++
		}
		contentNorm := normalizeText(strings.Join(block[1:], "\n"))
		if name == "source" {
			continue // provenance is in frontmatter (resource + source_digest)
		}
		if contentNorm != "" && (contentNorm == titleNorm || seen[contentNorm]) {
			continue // a section that merely echoes the title or a prior block
		}
		if contentNorm != "" {
			seen[contentNorm] = true
		}
		out = append(out, block...)
	}
	return strings.TrimRight(strings.Join(out, "\n"), "\n")
}

// flattenWikilinks turns [[x]] into x — for titles/descriptions, which are
// summaries and must not carry wiki-syntax (body links are rewritten to OKF
// markdown links separately in weaveLinks).
func flattenWikilinks(s string) string {
	return wikilinkPattern.ReplaceAllStringFunc(s, func(m string) string {
		return strings.Trim(m, "[]")
	})
}

// splitRefs splits a frontmatter cross-ref value (comma/space/bracket
// separated, possibly [[wikilink]] wrapped) into individual reference tokens.
func splitRefs(s string) []string {
	s = strings.NewReplacer("[", " ", "]", " ", ",", " ", "\"", " ", "'", " ").Replace(s)
	var out []string
	for _, tok := range strings.Fields(s) {
		if tok != "" {
			out = append(out, tok)
		}
	}
	return out
}

// harvestTags builds the OKF tags set: type + tier, the frontmatter tags
// field, and the comma-separated values from a finding's Applicability section
// (Work shapes / Scope tags / Languages). Tokens are slugified, deduped,
// noise-filtered ("n/a", "any", "none"), and capped so tags stay queryable.
func harvestTags(fields map[string]any, body, mtype, tier string) []string {
	const maxTags = 8
	noise := map[string]bool{"n-a": true, "na": true, "any": true, "none": true, "": true}
	var tags []string
	seen := map[string]bool{}
	add := func(raw string) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return // guard before slugify, which maps "" -> "untitled"
		}
		t := slugify(raw)
		// skip noise, dupes, and phrase-length tokens (not real tags)
		if noise[t] || seen[t] || len(t) > 30 {
			return
		}
		seen[t] = true
		tags = append(tags, t)
	}
	add(mtype)
	add(tier)
	for _, v := range strings.Split(fieldStr(fields, "tags"), ",") {
		add(v)
	}
	for _, line := range strings.Split(body, "\n") {
		if m := tagLinePattern.FindStringSubmatch(line); m != nil {
			for _, tok := range strings.Split(m[1], ",") {
				add(tok)
			}
		}
	}
	if len(tags) > maxTags {
		tags = tags[:maxTags]
	}
	return tags
}

func countAuthoritative(items []goldDoc) int {
	n := 0
	for _, d := range items {
		if d.status == "authoritative" {
			n++
		}
	}
	return n
}

func dedupeStrings(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
