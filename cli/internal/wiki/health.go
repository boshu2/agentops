package wiki

// OpenKB-style structural health checks (age-port-openkb-into-agentops-go-5qw.4).
//
// CheckWiki walks a compiled OpenKB workspace's wiki/ tree and reports
// deterministic STRUCTURAL defects on the artifacts the compile stage emits
// (cli/internal/llmwiki/compile.go): frontmatter-bearing pages under
// wiki/{summaries,concepts,entities,...} plus wiki/index.md and wiki/log.md,
// cross-linked with [[dir/slug]] wikilinks.
//
// The checks are pure (filesystem-read-only) and free of cobra / global state,
// so the CLI layer (cmd/ao/wiki_health.go) is a thin renderer over them. This
// is the lint/list/status logic; --fix repairs live in repair.go.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DefectKind classifies a structural defect CheckWiki found.
type DefectKind string

const (
	// DefectBrokenLink is a [[dir/slug]] wikilink whose target page does not
	// exist on disk. Blocking — it is a dangling reference.
	DefectBrokenLink DefectKind = "broken-link"
	// DefectInvalidFrontmatter is a page whose leading frontmatter block is
	// missing or fails to parse. Blocking.
	DefectInvalidFrontmatter DefectKind = "invalid-frontmatter"
	// DefectMissingField is a page whose frontmatter parses but lacks a
	// required key. Blocking.
	DefectMissingField DefectKind = "missing-field"
	// DefectOrphan is a valid page that no other page links to (excluding the
	// index/log). Non-blocking — a hygiene warning, not a structural break.
	DefectOrphan DefectKind = "orphan"
)

// requiredFrontmatterFields is the minimal frontmatter key set every compiled
// wiki page must carry. The compile stage always emits `type` (the page kind)
// and `attempt` (the idempotency gate, see llmwiki.hasValidArtifact), so those
// two are the deterministic required set. We do NOT require `description`: the
// compile artifacts do not emit one, so requiring it would flag every real
// page (a false positive the bead's "confirm the actual required set"
// instruction guards against).
var requiredFrontmatterFields = []string{"type", "attempt"}

// compiledPageDirs are the subdirectories under wiki/ whose .md files are
// COMPILED pages — frontmatter-bearing artifacts subject to the frontmatter and
// orphan checks. index.md and log.md (wiki/ root) are handled separately.
var compiledPageDirs = []string{"summaries", "concepts", "entities", "explorations", "reports"}

// linkTargetDirs are the subdirectories whose .md files are valid wikilink
// TARGETS. This is compiledPageDirs plus "sources" — the raw distilled source
// pages (wiki/sources/<slug>.md) are real link destinations ([[sources/<slug>]]
// appears in every summary/entity) but are NOT compiled artifacts, so they are
// link-resolvable yet exempt from the frontmatter/orphan checks.
var linkTargetDirs = append([]string{"sources"}, compiledPageDirs...)

// WikiDefect is one structural problem found on a page.
type WikiDefect struct {
	// Kind classifies the defect.
	Kind DefectKind `json:"kind"`
	// Page is the wiki-relative page key (e.g. "concepts/foo-<hash>" or
	// "index") the defect was found ON.
	Page string `json:"page"`
	// Detail carries the kind-specific subject: the dangling target for a
	// broken link, the missing key for a missing field, empty otherwise.
	Detail string `json:"detail,omitempty"`
	// Blocking reports whether this defect should fail a gate (non-zero exit).
	Blocking bool `json:"blocking"`
	// Message is a human-readable one-line description.
	Message string `json:"message"`
}

// WikiHealthReport is the result of CheckWiki.
type WikiHealthReport struct {
	// Workspace is the resolved workspace root the check ran against.
	Workspace string `json:"workspace"`
	// PageCount is the number of wiki pages discovered (incl. index/log).
	PageCount int `json:"page_count"`
	// Defects is every defect found, ordered deterministically by (page, kind).
	Defects []WikiDefect `json:"defects"`
}

// HasBlocking reports whether any defect is blocking (gate should fail).
func (r WikiHealthReport) HasBlocking() bool {
	for _, d := range r.Defects {
		if d.Blocking {
			return true
		}
	}
	return false
}

// BlockingCount returns how many defects are blocking.
func (r WikiHealthReport) BlockingCount() int {
	n := 0
	for _, d := range r.Defects {
		if d.Blocking {
			n++
		}
	}
	return n
}

// wikiPage is a discovered page plus its parsed metadata and outbound links.
type wikiPage struct {
	// key is the wiki-relative page key without the .md suffix:
	// "summaries/alpha", "index", "log".
	key string
	// path is the absolute file path.
	path string
	// pageType is the frontmatter `type`, "" when absent.
	pageType string
	// hasFrontmatter reports whether a well-formed frontmatter block parsed.
	hasFrontmatter bool
	// fields is the parsed frontmatter map (nil when none).
	fields map[string]any
	// links are the outbound [[dir/slug]] targets (page keys) this page emits.
	links []string
	// isRoot reports whether this is wiki/index.md or wiki/log.md (excluded
	// from the orphan check — nothing links to the index by design).
	isRoot bool
}

// CheckWiki runs every structural health check over the workspace's wiki/ tree
// and returns a deterministic report. It is read-only.
func CheckWiki(workspace string) (WikiHealthReport, error) {
	pages, err := loadWikiPages(workspace)
	if err != nil {
		return WikiHealthReport{}, err
	}
	existing, err := linkTargetSet(workspace)
	if err != nil {
		return WikiHealthReport{}, err
	}
	report := WikiHealthReport{Workspace: workspace, PageCount: len(pages)}

	// Track inbound links so we can find orphans: a page nothing else links to.
	linkedTo := make(map[string]bool, len(pages))

	for _, p := range pages {
		// Root index/log are generated structural files with no frontmatter by
		// design — exempt them from the frontmatter checks (they are still
		// scanned for outbound broken links below).
		if !p.isRoot {
			report.Defects = append(report.Defects, frontmatterDefects(p)...)
		}
		for _, target := range p.links {
			if !existing[target] {
				report.Defects = append(report.Defects, WikiDefect{
					Kind:     DefectBrokenLink,
					Page:     p.key,
					Detail:   target,
					Blocking: true,
					Message:  fmt.Sprintf("broken wikilink [[%s]] (target page does not exist)", target),
				})
				continue
			}
			if target != p.key { // a self-link is not inbound coverage
				linkedTo[target] = true
			}
		}
	}

	// Orphan check: a non-root page with valid frontmatter that nothing links
	// to. We only flag well-formed pages — a page that already has a
	// frontmatter defect is reported under that defect, not double-counted as
	// an orphan.
	for _, p := range pages {
		if p.isRoot || linkedTo[p.key] {
			continue
		}
		if !p.hasFrontmatter {
			continue
		}
		report.Defects = append(report.Defects, WikiDefect{
			Kind:     DefectOrphan,
			Page:     p.key,
			Blocking: false,
			Message:  fmt.Sprintf("orphan page %q — no other page links to it", p.key),
		})
	}

	sortDefects(report.Defects)
	return report, nil
}

// frontmatterDefects returns the frontmatter defects for a single page: at most
// one invalid-frontmatter defect, or (when the block is well-formed) one
// missing-field defect per absent required key.
func frontmatterDefects(p wikiPage) []WikiDefect {
	if !p.hasFrontmatter {
		return []WikiDefect{{
			Kind:     DefectInvalidFrontmatter,
			Page:     p.key,
			Blocking: true,
			Message:  fmt.Sprintf("page %q has missing or invalid frontmatter", p.key),
		}}
	}
	var out []WikiDefect
	for _, field := range requiredFrontmatterFields {
		if !hasNonEmptyField(p.fields, field) {
			out = append(out, WikiDefect{
				Kind:     DefectMissingField,
				Page:     p.key,
				Detail:   field,
				Blocking: true,
				Message:  fmt.Sprintf("page %q is missing required frontmatter field %q", p.key, field),
			})
		}
	}
	return out
}

// hasNonEmptyField reports whether fields holds key with a non-empty value.
func hasNonEmptyField(fields map[string]any, key string) bool {
	if fields == nil {
		return false
	}
	v, ok := fields[key]
	if !ok || v == nil {
		return false
	}
	return strings.TrimSpace(fmt.Sprint(v)) != ""
}

// loadWikiPages discovers and parses every COMPILED wiki page under
// workspace/wiki/ (root index/log plus the compiledPageDirs). The raw
// wiki/sources/ pages are deliberately excluded — they are link targets, not
// checked artifacts (see linkTargetSet).
func loadWikiPages(workspace string) ([]wikiPage, error) {
	wikiRoot := filepath.Join(workspace, "wiki")
	var pages []wikiPage

	// Root pages: index.md, log.md.
	for _, name := range []string{"index", "log"} {
		path := filepath.Join(wikiRoot, name+".md")
		if !regularFileExists(path) {
			continue
		}
		page, err := loadWikiPage(path, name, true)
		if err != nil {
			return nil, err
		}
		pages = append(pages, page)
	}

	// Compiled subdirectory pages.
	for _, dir := range compiledPageDirs {
		dirPages, err := loadDirPages(wikiRoot, dir)
		if err != nil {
			return nil, err
		}
		pages = append(pages, dirPages...)
	}

	sort.Slice(pages, func(i, j int) bool { return pages[i].key < pages[j].key })
	return pages, nil
}

// loadDirPages parses every .md page in wikiRoot/dir into wikiPages keyed
// "dir/slug". A missing directory yields no pages (not an error).
func loadDirPages(wikiRoot, dir string) ([]wikiPage, error) {
	dirPath := filepath.Join(wikiRoot, dir)
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read wiki dir %s: %w", dir, err)
	}
	var pages []wikiPage
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			continue
		}
		slug := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		page, err := loadWikiPage(filepath.Join(dirPath, e.Name()), dir+"/"+slug, false)
		if err != nil {
			return nil, err
		}
		pages = append(pages, page)
	}
	return pages, nil
}

// linkTargetSet returns the set of every page key a wikilink may resolve to:
// the root index/log, plus every .md file under linkTargetDirs (compiled pages
// AND raw sources). A [[target]] is a broken link iff its key is absent here.
func linkTargetSet(workspace string) (map[string]bool, error) {
	wikiRoot := filepath.Join(workspace, "wiki")
	set := map[string]bool{}
	for _, name := range []string{"index", "log"} {
		if regularFileExists(filepath.Join(wikiRoot, name+".md")) {
			set[name] = true
		}
	}
	for _, dir := range linkTargetDirs {
		entries, err := os.ReadDir(filepath.Join(wikiRoot, dir))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read wiki dir %s: %w", dir, err)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
				continue
			}
			slug := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
			set[dir+"/"+slug] = true
		}
	}
	return set, nil
}

// loadWikiPage reads and parses one page file into a wikiPage.
func loadWikiPage(path, key string, isRoot bool) (wikiPage, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is wiki-tree-bounded
	if err != nil {
		return wikiPage{}, fmt.Errorf("read wiki page %s: %w", key, err)
	}
	text := string(data)
	doc := FrontmatterCodec{}.Decode(text)
	page := wikiPage{
		key:            key,
		path:           path,
		hasFrontmatter: doc.HasFrontmatter,
		fields:         doc.Fields,
		links:          ExtractWikilinks(text),
		isRoot:         isRoot,
	}
	if doc.HasFrontmatter {
		page.pageType = strings.TrimSpace(fmt.Sprint(doc.Fields["type"]))
		if page.pageType == "<nil>" {
			page.pageType = ""
		}
	}
	return page, nil
}

// ExtractWikilinks returns the distinct [[dir/slug]] link targets (page keys)
// in text, in first-seen order. The slug-anchored pattern (shared with gold.go)
// ignores bash double-bracket tests like [[ -f x ]]. A bare [[slug]] without a
// directory segment is kept verbatim as its own key.
func ExtractWikilinks(text string) []string {
	matches := wikilinkPattern.FindAllString(text, -1)
	seen := make(map[string]bool, len(matches))
	var out []string
	for _, m := range matches {
		target := strings.TrimSuffix(strings.TrimPrefix(m, "[["), "]]")
		target = strings.TrimSpace(target)
		if target == "" || seen[target] {
			continue
		}
		seen[target] = true
		out = append(out, target)
	}
	return out
}

// sortDefects orders defects deterministically by (page, kind, detail).
func sortDefects(defects []WikiDefect) {
	sort.SliceStable(defects, func(i, j int) bool {
		if defects[i].Page != defects[j].Page {
			return defects[i].Page < defects[j].Page
		}
		if defects[i].Kind != defects[j].Kind {
			return defects[i].Kind < defects[j].Kind
		}
		return defects[i].Detail < defects[j].Detail
	})
}

// regularFileExists reports whether path is an existing regular file (not a
// directory). The package's fileExists (gold.go) is true for dirs too, which is
// the wrong test for a wiki page file.
func regularFileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// --- list / status ---

// WikiPageInfo is one enumerated page for `ao wiki list`.
type WikiPageInfo struct {
	// Key is the wiki-relative page key (e.g. "concepts/foo-<hash>").
	Key string `json:"key"`
	// Type is the frontmatter `type` ("summary"/"concept"/"entity"/...), or
	// "" when the page has no parseable type.
	Type string `json:"type"`
	// Path is the absolute file path.
	Path string `json:"path"`
}

// ListPages enumerates every wiki page, optionally filtered to a single
// frontmatter type. Root pages (index/log) report their key with an empty type.
// Results are ordered by key.
func ListPages(workspace, typeFilter string) ([]WikiPageInfo, error) {
	pages, err := loadWikiPages(workspace)
	if err != nil {
		return nil, err
	}
	filter := strings.TrimSpace(typeFilter)
	out := make([]WikiPageInfo, 0, len(pages))
	for _, p := range pages {
		if filter != "" && p.pageType != filter {
			continue
		}
		out = append(out, WikiPageInfo{Key: p.key, Type: p.pageType, Path: p.path})
	}
	return out, nil
}

// WikiStatus is the health/counts summary for `ao wiki status`.
type WikiStatus struct {
	// Workspace is the resolved workspace root.
	Workspace string `json:"workspace"`
	// TotalPages is the number of pages discovered (incl. index/log).
	TotalPages int `json:"total_pages"`
	// ByType counts pages per frontmatter type. Pages with no type are counted
	// under the "untyped" key.
	ByType map[string]int `json:"by_type"`
	// DefectCount is the total number of structural defects.
	DefectCount int `json:"defect_count"`
	// BlockingDefects is how many defects are blocking.
	BlockingDefects int `json:"blocking_defects"`
	// DefectsByKind counts defects per kind.
	DefectsByKind map[string]int `json:"defects_by_kind"`
}

// Status returns counts and a health summary for the workspace's wiki.
func Status(workspace string) (WikiStatus, error) {
	pages, err := loadWikiPages(workspace)
	if err != nil {
		return WikiStatus{}, err
	}
	report, err := CheckWiki(workspace)
	if err != nil {
		return WikiStatus{}, err
	}
	st := WikiStatus{
		Workspace:     workspace,
		TotalPages:    len(pages),
		ByType:        map[string]int{},
		DefectsByKind: map[string]int{},
	}
	for _, p := range pages {
		t := p.pageType
		if t == "" {
			t = "untyped"
		}
		st.ByType[t]++
	}
	st.DefectCount = len(report.Defects)
	st.BlockingDefects = report.BlockingCount()
	for _, d := range report.Defects {
		st.DefectsByKind[string(d.Kind)]++
	}
	return st, nil
}
