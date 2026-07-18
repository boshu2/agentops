package doctor

// Workspace subsystem: fm-ws-undeclared-writer + fm-ws-drift-ref (both
// report-only). This file ports the 2026-07-18 `.agents/` writer-matrix audit
// into standing detectors, so the audit's cross-join runs at detect time
// forever instead of living in a hand-regenerated markdown table.
//
// The shared scan walks the two source corpora the audit used —
// skills/** (*.md, *.sh, *.py, *.yaml) and cli/** non-test *.go — and extracts
// every `.agents/<segment>` reference, where <segment> is the FIRST path
// segment after the workspace root (the audit's own extraction rule). From one
// pass it derives both products:
//
//   - fm-ws-undeclared-writer (P3): each live top-level workspace directory
//     with NO reference anywhere in the corpus is an orphan — some writer
//     produces it by convention only, with no source contract. Orphans are
//     classified in the finding title as ACTIVE-ORPHAN (newest write within
//     the workspace GC TTL — a convention-only writer is still alive) vs
//     DEAD-ORPHAN (stale, promotion-or-death candidate).
//   - fm-ws-drift-ref (P2): each corpus file referencing an ALIAS spelling
//     from the workspaceCanonicalAliases registry (the source-side twin of
//     fm-ws-naming-drift, which flags alias DIRECTORIES). The fix is always
//     in the source: reference the canonical name. This is the
//     capabilities.go / config.go / finding_compiler_adapter.go class the
//     audit caught by hand.
//
// Both detectors are PURE reads and deliberately have NO fixer: adopting an
// orphan dir in source, promoting or staling its contents, and rewriting a
// drifted source reference are human/author calls, not mechanical renames.
// The alias registry is consumed as data — never copy its contents here.
//
// Scan bounds: the walk skips .git, node_modules, testdata, _fixtures,
// archive(d), and nested .agents trees, never follows symlinks, and refuses
// files larger than workspaceRefMaxFileBytes. When NEITHER corpus root exists
// (a consumer repo without this repo's skills/ + cli/ layout) both detectors
// stay silent: with no source corpus there is no writer-contract surface to
// join against, and flagging every workspace dir as an orphan would be noise.

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Detector IDs for the writer-matrix failure modes.
const (
	fmWorkspaceUndeclaredWriterID = "fm-ws-undeclared-writer"
	fmWorkspaceDriftRefID         = "fm-ws-drift-ref"
)

func init() {
	RegisterDetector(workspaceUndeclaredWriterDetector{})
	RegisterDetector(workspaceDriftRefDetector{})
	// Report-only by design: no fixers are registered for either ID.
}

// workspaceRefSegmentRe extracts the first path segment of a workspace
// reference in source text. The segment must start alphanumeric — this drops
// prose artifacts (a bare trailing slash, punctuation runs) and dot-leading
// internal names, which are runtime state, not writer-matrix surface.
var workspaceRefSegmentRe = regexp.MustCompile(`\.agents/([A-Za-z0-9][A-Za-z0-9._-]*)`)

// workspaceRefMaxFileBytes caps how large a corpus file the scan will read.
const workspaceRefMaxFileBytes = 1 << 20 // 1 MiB

// workspaceRefSkipDirNames are directory basenames the corpus walk never
// descends into: VCS internals, vendored deps, fixture corpora, archives, and
// nested workspace trees. The same set doubles as the path-element exemption
// for drift-ref findings (see workspaceDriftRefExemptFile).
var workspaceRefSkipDirNames = map[string]bool{
	".git":         true,
	"node_modules": true,
	"testdata":     true,
	"_fixtures":    true,
	"archive":      true,
	"archived":     true,
	".agents":      true,
}

// workspaceRefSkillExts are the file extensions scanned under skills/.
var workspaceRefSkillExts = map[string]bool{
	".md":   true,
	".sh":   true,
	".py":   true,
	".yaml": true,
}

// workspaceAliasHit is one corpus file referencing one alias spelling, with
// the 1-based line numbers of every occurrence.
type workspaceAliasHit struct {
	// File is the repo-relative, slash-separated source path.
	File string
	// Alias is the drifted spelling found (a workspaceCanonicalAliases key).
	Alias string
	// Lines holds the 1-based line numbers of each occurrence, ascending.
	Lines []int
}

// workspaceRefScan is the product of one corpus pass.
type workspaceRefScan struct {
	// Referenced is the set of first path segments referenced anywhere in the
	// corpus. A reference to an alias spelling also marks its canonical name
	// as referenced: the writer contract is for the FAMILY, so a drifted
	// reference must not additionally orphan the canonical directory (the
	// drift is fm-ws-drift-ref's finding, not fm-ws-undeclared-writer's).
	Referenced map[string]bool
	// AliasHits are the drift references, sorted by file then alias. Files
	// exempt under workspaceDriftRefExemptFile are excluded here but still
	// contribute to Referenced.
	AliasHits []workspaceAliasHit
	// FilesScanned counts corpus files actually read (evidence context).
	FilesScanned int
}

// workspaceDriftRefExemptFile reports whether a repo-relative source path is
// exempt from drift-ref findings: the alias registry's own defining file, the
// hygiene doc that documents the aliases, ADRs (immutable decision records may
// cite historical spellings), anything under a workspace tree, and any path
// routed through a skipped corpus element (testdata, archives, fixtures, ...).
func workspaceDriftRefExemptFile(rel string) bool {
	rel = filepath.ToSlash(rel)
	switch rel {
	case "cli/internal/doctor/fix_workspace.go", "docs/agents-dir-hygiene.md":
		return true
	}
	if strings.HasPrefix(rel, "docs/adr/") || strings.HasPrefix(rel, ".agents/") {
		return true
	}
	for _, el := range strings.Split(rel, "/") {
		if workspaceRefSkipDirNames[el] {
			return true
		}
	}
	return false
}

// workspaceScanSourceRefs walks the source corpora under repoRoot and returns
// the reference scan plus whether ANY corpus root existed. Unreadable entries
// and oversized files are skipped, never fatal: a partial corpus yields a
// smaller Referenced set, which can only over-report orphans on a broken
// tree — and both consumers are report-only.
func workspaceScanSourceRefs(repoRoot string) (workspaceRefScan, bool) {
	scan := workspaceRefScan{Referenced: make(map[string]bool)}
	roots := []struct {
		dir     string
		include func(name string) bool
	}{
		{"skills", func(name string) bool {
			return workspaceRefSkillExts[strings.ToLower(filepath.Ext(name))]
		}},
		{"cli", func(name string) bool {
			return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
		}},
	}
	// aliasLines accumulates file -> alias -> occurrence lines for flattening.
	aliasLines := make(map[string]map[string][]int)
	corpusFound := false
	for _, root := range roots {
		rootPath := filepath.Join(repoRoot, root.dir)
		// Symlink-safe root gate, same rationale as workspaceRealDir's use on
		// the workspace root: never walk through a symlinked corpus root.
		if !workspaceRealDir(rootPath) {
			continue
		}
		corpusFound = true
		workspaceWalkRefRoot(repoRoot, rootPath, root.include, &scan, aliasLines)
	}
	for file, perFile := range aliasLines {
		for alias, lines := range perFile {
			scan.AliasHits = append(scan.AliasHits, workspaceAliasHit{File: file, Alias: alias, Lines: lines})
		}
	}
	sort.Slice(scan.AliasHits, func(i, j int) bool {
		if scan.AliasHits[i].File != scan.AliasHits[j].File {
			return scan.AliasHits[i].File < scan.AliasHits[j].File
		}
		return scan.AliasHits[i].Alias < scan.AliasHits[j].Alias
	})
	return scan, corpusFound
}

// workspaceWalkRefRoot walks one corpus root, accumulating referenced segments
// into scan and alias occurrences into aliasLines. Root-scoped reads: os.Root
// confines every ReadFile to rootPath and refuses symlink escapes, closing the
// walk-then-read TOCTOU window (gosec G122) the dirent checks alone cannot
// close. WalkDir never follows symlinks; walk errors skip the subtree.
func workspaceWalkRefRoot(repoRoot, rootPath string, include func(string) bool, scan *workspaceRefScan, aliasLines map[string]map[string][]int) {
	rootHandle, rootErr := os.OpenRoot(rootPath)
	if rootErr != nil {
		return
	}
	defer func() { _ = rootHandle.Close() }()
	_ = filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if workspaceRefSkipDirNames[d.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || !include(d.Name()) {
			return nil
		}
		fi, err := d.Info()
		if err != nil || fi.Size() > workspaceRefMaxFileBytes {
			return nil
		}
		relInRoot, relInRootErr := filepath.Rel(rootPath, path)
		if relInRootErr != nil {
			return nil
		}
		data, err := rootHandle.ReadFile(relInRoot)
		if err != nil {
			return nil
		}
		rel, relErr := filepath.Rel(repoRoot, path)
		if relErr != nil {
			rel = path
		}
		rel = filepath.ToSlash(rel)
		scan.FilesScanned++
		exempt := workspaceDriftRefExemptFile(rel)
		for i, line := range strings.Split(string(data), "\n") {
			for _, m := range workspaceRefSegmentRe.FindAllStringSubmatch(line, -1) {
				// Trailing dots are sentence punctuation, not path chars.
				seg := strings.TrimRight(m[1], ".")
				if seg == "" {
					continue
				}
				scan.Referenced[seg] = true
				canonical, isAlias := workspaceCanonicalAliases[seg]
				if !isAlias {
					continue
				}
				scan.Referenced[canonical] = true
				if exempt {
					continue
				}
				perFile := aliasLines[rel]
				if perFile == nil {
					perFile = make(map[string][]int)
					aliasLines[rel] = perFile
				}
				perFile[seg] = append(perFile[seg], i+1)
			}
		}
		return nil
	})
}

// ---------------------------------------------------------------------------
// FM: fm-ws-undeclared-writer (report-only)
// ---------------------------------------------------------------------------

// workspaceUndeclaredWriterDetector flags each live top-level workspace
// directory that no corpus source file references.
type workspaceUndeclaredWriterDetector struct{}

func (workspaceUndeclaredWriterDetector) ID() string        { return fmWorkspaceUndeclaredWriterID }
func (workspaceUndeclaredWriterDetector) Subsystem() string { return subsystemWorkspace }
func (workspaceUndeclaredWriterDetector) Severity() string  { return "P3" }

// EstimatedCostMS is honest about the corpus walk: this reads every skills/
// and cli/ source file AND inventories the workspace tree (measured ~340 ms
// live on this repo), not a handful of stats.
func (workspaceUndeclaredWriterDetector) EstimatedCostMS() int { return 350 }
func (workspaceUndeclaredWriterDetector) OnlineRequired() bool { return false }
func (workspaceUndeclaredWriterDetector) QuickPath() bool      { return false }
func (workspaceUndeclaredWriterDetector) Describe() string {
	return "top-level .agents directory has no declared writer in skills or cli source"
}

func (d workspaceUndeclaredWriterDetector) Detect(env *DetectEnv) ([]Finding, error) {
	base := workspaceAgentsDir(env)
	if !workspaceRealDir(base) {
		return nil, nil
	}
	scan, corpusFound := workspaceScanSourceRefs(env.RepoRoot)
	if !corpusFound {
		return nil, nil // no source corpus, no writer contracts to join against
	}
	inv, err := workspaceDirInventory(base)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("doctor: inventory workspace: %w", err)
	}
	ttl := workspaceGCTTL()
	now := time.Now()
	var findings []Finding
	// Inventory order is name-sorted, so findings are deterministic.
	for _, info := range inv {
		name := info.Name
		// Exemptions: "ao" is the CLI-owned proof store (never an orphan even
		// if a corpus subset misses it); stale/retry names are the stale-GC
		// detector's claim; dot-leading names are runtime state, invisible to
		// the segment regex by design and out of writer-matrix scope.
		if name == "ao" || strings.HasPrefix(name, ".") || isWorkspaceStaleDirName(name) {
			continue
		}
		if scan.Referenced[name] {
			continue
		}
		// ACTIVE vs DEAD: newest write within the GC TTL means some
		// convention-only writer is still alive. WalkErrs>0 means the
		// inventory is a lower bound (unknown is never provably dead), so it
		// classifies ACTIVE conservatively.
		class := "DEAD-ORPHAN"
		age := "no regular files"
		if !info.NewestMTime.IsZero() {
			age = fmt.Sprintf("newest write %dd ago", int(now.Sub(info.NewestMTime)/day))
		}
		if info.WalkErrs > 0 || (!info.NewestMTime.IsZero() && now.Sub(info.NewestMTime) <= ttl) {
			class = "ACTIVE-ORPHAN"
		}
		findings = append(findings, Finding{
			ID:        d.ID(),
			Severity:  d.Severity(),
			Subsystem: d.Subsystem(),
			Title:     fmt.Sprintf("workspace undeclared writer: .agents/%s (%s)", name, class),
			// Unreferenced-in-corpus is factual; "orphan" is inferred — a
			// session/skill may write the dir by convention with no source
			// contract, which is exactly what this finding surfaces.
			Confidence: 0.9,
			Evidence: Evidence{
				File: filepath.Join(".agents", name),
				Query: fmt.Sprintf("%s; %d transitive regular file(s); no .agents/%s reference in %d scanned source file(s) (skills *.md/*.sh/*.py/*.yaml + cli non-test *.go)",
					age, info.FileCount, name, scan.FilesScanned),
			},
			Remediation: Remediation{
				Command: "Adopt .agents/" + name + " in source (declare the writer), promote its " +
					"contents, or stale-mark it for GC per docs/agents-dir-hygiene.md. Then re-run: ao doctor",
				ExplainCommand:   "ao doctor explain " + d.ID(),
				AutoFixable:      false,
				EstimatedActions: 0,
			},
		})
	}
	return findings, nil
}

// ---------------------------------------------------------------------------
// FM: fm-ws-drift-ref (report-only)
// ---------------------------------------------------------------------------

// workspaceDriftRefDetector flags each corpus source file that references an
// alias spelling from the workspaceCanonicalAliases registry.
type workspaceDriftRefDetector struct{}

func (workspaceDriftRefDetector) ID() string        { return fmWorkspaceDriftRefID }
func (workspaceDriftRefDetector) Subsystem() string { return subsystemWorkspace }
func (workspaceDriftRefDetector) Severity() string  { return "P2" }

// EstimatedCostMS covers the same corpus walk as fm-ws-undeclared-writer,
// minus the workspace inventory (measured ~200 ms live on this repo).
func (workspaceDriftRefDetector) EstimatedCostMS() int { return 250 }
func (workspaceDriftRefDetector) OnlineRequired() bool { return false }
func (workspaceDriftRefDetector) QuickPath() bool      { return false }
func (workspaceDriftRefDetector) Describe() string {
	return "source file references a drifted alias spelling of a canonical .agents directory"
}

func (d workspaceDriftRefDetector) Detect(env *DetectEnv) ([]Finding, error) {
	// No workspace-root gate: a drifted reference is a source bug whether or
	// not the alias directory (or any .agents tree) exists yet — the reference
	// is what RECREATES the drift after a merge.
	scan, corpusFound := workspaceScanSourceRefs(env.RepoRoot)
	if !corpusFound {
		return nil, nil
	}
	// One finding per (file, alias) pair, already sorted deterministically.
	var findings []Finding
	for _, hit := range scan.AliasHits {
		canonical := workspaceCanonicalAliases[hit.Alias]
		findings = append(findings, Finding{
			ID:         d.ID(),
			Severity:   d.Severity(),
			Subsystem:  d.Subsystem(),
			Title:      fmt.Sprintf("workspace drift reference: %s references .agents/%s (canonical: .agents/%s)", hit.File, hit.Alias, canonical),
			Confidence: 1.0,
			Evidence: Evidence{
				File:  hit.File,
				Lines: hit.Lines,
				Query: fmt.Sprintf("%d occurrence(s) of .agents/%s; canonical directory is .agents/%s", len(hit.Lines), hit.Alias, canonical),
			},
			Remediation: Remediation{
				Command: fmt.Sprintf("Edit %s: replace .agents/%s with .agents/%s. Then re-run: ao doctor",
					hit.File, hit.Alias, canonical),
				ExplainCommand:   "ao doctor explain " + d.ID(),
				AutoFixable:      false,
				EstimatedActions: 0,
			},
		})
	}
	return findings, nil
}
