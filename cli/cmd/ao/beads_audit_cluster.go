// practices: [dora-metrics, lean-startup]
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
)

var (
	beadsAuditJSON      bool
	beadsAuditStrict    bool
	beadsAuditAutoClose bool
	beadsClusterJSON    bool
	beadsClusterApply   bool
)

var beadsAuditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Audit open beads for likely-fixed, stale, or consolidatable work",
	Long: `Audits open and in-progress beads for backlog hygiene issues.

The audit checks for:
- bead IDs already referenced by git commits
- bead descriptions whose cited files changed since the bead was created
- bead descriptions whose referenced patterns no longer appear in the repo
- multiple beads that mention the same file path

This is the native Go equivalent of scripts/bd-audit.sh. The shell script is
kept as a compatibility entrypoint for existing hooks and skill guidance.`,
	RunE: runBeadsAudit,
}

var beadsClusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "Suggest consolidation clusters for overlapping open beads",
	Long: `Analyzes open beads for domain overlap and suggests consolidation groups.

The scorer compares title/body keywords, mentioned file paths, and labels. It
prefers an existing epic as the cluster representative when one exists, falling
back to the lexicographically smallest bead ID.

This is the native Go equivalent of scripts/bd-cluster.sh. The shell script is
kept as a compatibility entrypoint for existing hooks and skill guidance.`,
	RunE: runBeadsCluster,
}

func init() {
	beadsCmd.AddCommand(beadsAuditCmd)
	beadsCmd.AddCommand(beadsClusterCmd)

	beadsAuditCmd.Flags().BoolVar(&beadsAuditJSON, "json", false,
		"Emit audit report as JSON")
	beadsAuditCmd.Flags().BoolVar(&beadsAuditStrict, "strict", false,
		"Exit 1 when any likely-fixed, likely-stale, or consolidatable bead is found")
	beadsAuditCmd.Flags().BoolVar(&beadsAuditAutoClose, "auto-close", false,
		"Close likely-fixed beads when commit or file-change evidence is found")

	beadsClusterCmd.Flags().BoolVar(&beadsClusterJSON, "json", false,
		"Emit cluster report as JSON")
	beadsClusterCmd.Flags().BoolVar(&beadsClusterApply, "apply", false,
		"Reparent non-representative beads under the cluster representative")
}

type beadRecord struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Subject     string   `json:"subject"`
	Description string   `json:"description"`
	Body        string   `json:"body"`
	Status      string   `json:"status"`
	IssueType   string   `json:"issue_type"`
	Type        string   `json:"type"`
	Kind        string   `json:"kind"`
	CreatedAt   string   `json:"created_at"`
	Labels      []string `json:"labels"`
	Children    []any    `json:"children"`
}

func (b beadRecord) displayTitle() string {
	if b.Title != "" {
		return b.Title
	}
	return b.Subject
}

func (b beadRecord) textBody() string {
	if b.Body != "" {
		return b.Body
	}
	return b.Description
}

func (b beadRecord) isEpic() bool {
	for _, v := range []string{b.IssueType, b.Type, b.Kind} {
		if strings.EqualFold(v, "epic") {
			return true
		}
	}
	return len(b.Children) > 0
}

// execGitLog shells out to git. Tests override it so audit logic does not
// depend on the live repository history.
var execGitLog = func(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...) // #nosec G204 -- fixed git binary; args are read-only log queries built from local bead metadata.
	out, err := cmd.Output()
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	return string(out), err
}

// repoPatternExists searches the worktree for a literal pattern, reusing a
// content index so the tree is walked once per audit instead of once per
// pattern (soc-2grz, PERF-C2). Tests override it to keep audit classification
// deterministic.
var repoPatternExists = func(pattern string, cache *repoContentCache) bool {
	return patternExistsInIndex(pattern, cache.index())
}

// repoContentCache walks the scoped audit roots at most once and memoizes a
// path -> file-content map. recordAuditStaleFinding probes up to 10 patterns
// per bead; without the cache that was up to 10N full-repo walks for N beads.
type repoContentCache struct {
	once sync.Once
	data map[string]string
}

// index returns the memoized worktree content map, building it on first use.
func (c *repoContentCache) index() map[string]string {
	c.once.Do(func() { c.data = buildRepoContentIndex() })
	return c.data
}

// patternExistsInIndex reports whether any indexed file contains the literal
// pattern.
func patternExistsInIndex(pattern string, index map[string]string) bool {
	if pattern == "" {
		return false
	}
	for _, content := range index {
		if strings.Contains(content, pattern) {
			return true
		}
	}
	return false
}

type AuditFinding struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Reason   string `json:"reason"`
	Evidence string `json:"evidence,omitempty"`
}

type AuditConsolidation struct {
	File    string   `json:"file"`
	BeadIDs []string `json:"bead_ids"`
}

type AuditSummary struct {
	LikelyFixed    int `json:"likely_fixed"`
	LikelyStale    int `json:"likely_stale"`
	Consolidatable int `json:"consolidatable"`
	Total          int `json:"total"`
	FlaggedPct     int `json:"flagged_pct,omitempty"`
}

type AuditReport struct {
	LikelyFixed    []AuditFinding       `json:"likely_fixed"`
	LikelyStale    []AuditFinding       `json:"likely_stale"`
	Consolidatable []AuditConsolidation `json:"consolidatable"`
	Summary        AuditSummary         `json:"summary"`
	BDAvailable    bool                 `json:"bd_available"`
	Error          string               `json:"error,omitempty"`
}

func runBeadsAudit(cmd *cobra.Command, args []string) error {
	report, err := auditBeads(beadsAuditAutoClose)
	if err != nil {
		return err
	}
	if !report.BDAvailable {
		if beadsAuditJSON {
			return emitJSON(os.Stdout, report)
		}
		fmt.Fprintln(os.Stderr, "WARN: bd not on PATH — skipping audit (graceful degradation)")
		return nil
	}
	if beadsAuditJSON {
		if err := emitJSON(os.Stdout, report); err != nil {
			return err
		}
	} else {
		emitAuditHuman(os.Stdout, report)
	}
	if beadsAuditStrict && auditFlaggedCount(report) > 0 {
		os.Exit(1)
	}
	return nil
}

func auditBeads(autoClose bool) (*AuditReport, error) {
	report := &AuditReport{
		LikelyFixed:    []AuditFinding{},
		LikelyStale:    []AuditFinding{},
		Consolidatable: []AuditConsolidation{},
		BDAvailable:    bdAvailable(),
	}
	if !report.BDAvailable {
		report.Error = "bd CLI not found"
		return report, nil
	}

	beads, err := collectAuditBeads()
	if err != nil {
		return nil, err
	}
	report.Summary.Total = len(beads)
	if len(beads) == 0 {
		return report, nil
	}

	fileToBeads := make(map[string]map[string]bool)
	consolidatableIDs := make(map[string]bool)

	// PERF-C2 (soc-2grz): capture git history once and share a lazily-built
	// worktree content index, rather than re-shelling/re-walking per bead.
	commits := captureAuditCommits()
	cache := &repoContentCache{}

	for _, bead := range beads {
		if bead.ID == "" {
			continue
		}
		if recordAuditBeadFlow(report, bead, autoClose, fileToBeads, commits) {
			continue
		}
		recordAuditBeadPaths(fileToBeads, bead.ID, extractAuditFilePaths(bead.textBody(), 10))
		recordAuditStaleFinding(report, bead, cache)
	}

	finalizeAuditReport(report, fileToBeads, consolidatableIDs)
	return report, nil
}

func recordAuditBeadFlow(report *AuditReport, bead beadRecord, autoClose bool, fileToBeads map[string]map[string]bool, commits []auditCommit) bool {
	desc := bead.textBody()
	if recordLikelyFixedAuditFinding(report, bead, desc, autoClose, commits) {
		return true
	}
	recordAuditBeadPaths(fileToBeads, bead.ID, extractAuditFilePaths(desc, 10))
	return false
}

func recordLikelyFixedAuditFinding(report *AuditReport, bead beadRecord, desc string, autoClose bool, commits []auditCommit) bool {
	if evidence := grepCommitsForID(commits, bead.ID); evidence != "" {
		report.LikelyFixed = append(report.LikelyFixed, AuditFinding{
			ID:       bead.ID,
			Title:    bead.displayTitle(),
			Reason:   "commit_match",
			Evidence: evidence,
		})
		if autoClose {
			autoCloseLikelyFixed(bead.ID, "Auto-closed by ao beads audit: commit evidence found: "+evidence)
		}
		return true
	}
	paths := extractAuditFilePaths(desc, 10)
	if bead.CreatedAt != "" && len(paths) > 0 {
		if evidence := fileChangesSinceCommits(commits, bead.CreatedAt, paths); evidence != "" {
			report.LikelyFixed = append(report.LikelyFixed, AuditFinding{
				ID:       bead.ID,
				Title:    bead.displayTitle(),
				Reason:   "file_modified_since_creation",
				Evidence: evidence,
			})
			if autoClose {
				autoCloseLikelyFixed(bead.ID, "Auto-closed by ao beads audit: mentioned files modified since creation.")
			}
			return true
		}
	}
	return false
}

func recordAuditStaleFinding(report *AuditReport, bead beadRecord, cache *repoContentCache) {
	patterns := extractAuditPatterns(bead.textBody(), 10)
	if len(patterns) > 0 && !anyPatternExists(patterns, cache) {
		report.LikelyStale = append(report.LikelyStale, AuditFinding{
			ID:     bead.ID,
			Title:  bead.displayTitle(),
			Reason: "referenced_patterns_not_found",
		})
	}
}

func recordAuditBeadPaths(fileToBeads map[string]map[string]bool, beadID string, paths []string) {
	for _, path := range paths {
		if fileToBeads[path] == nil {
			fileToBeads[path] = make(map[string]bool)
		}
		fileToBeads[path][beadID] = true
	}
}

func finalizeAuditReport(report *AuditReport, fileToBeads map[string]map[string]bool, consolidatableIDs map[string]bool) {
	for path, ids := range fileToBeads {
		if len(ids) < 2 {
			continue
		}
		idList := sortedMapKeys(ids)
		for _, id := range idList {
			consolidatableIDs[id] = true
		}
		report.Consolidatable = append(report.Consolidatable, AuditConsolidation{
			File:    path,
			BeadIDs: idList,
		})
	}
	sort.Slice(report.Consolidatable, func(i, j int) bool {
		return report.Consolidatable[i].File < report.Consolidatable[j].File
	})

	report.Summary.LikelyFixed = len(report.LikelyFixed)
	report.Summary.LikelyStale = len(report.LikelyStale)
	report.Summary.Consolidatable = len(consolidatableIDs)
	flagged := auditFlaggedCount(report)
	if report.Summary.Total > 0 {
		report.Summary.FlaggedPct = flagged * 100 / report.Summary.Total
	}
}

func collectAuditBeads() ([]beadRecord, error) {
	openBeads, err := listBDRecordsByStatus("open")
	if err != nil {
		return nil, err
	}
	inProgress, err := listBDRecordsByStatus("in_progress")
	if err != nil {
		return nil, err
	}
	return append(openBeads, inProgress...), nil
}

func listBDRecordsByStatus(status string) ([]beadRecord, error) {
	raw, err := execBD("list", "--status", status, "--json")
	if err != nil {
		return nil, fmt.Errorf("bd list --status %s --json: %w", status, err)
	}
	return parseBDRecordList(raw)
}

func parseBDRecordList(raw []byte) ([]beadRecord, error) {
	raw = []byte(strings.TrimSpace(string(raw)))
	if len(raw) == 0 {
		return nil, nil
	}
	var records []beadRecord
	if err := json.Unmarshal(raw, &records); err != nil {
		return nil, err
	}
	return records, nil
}

func parseBDRecord(raw []byte) (beadRecord, error) {
	raw = []byte(strings.TrimSpace(string(raw)))
	if len(raw) == 0 {
		return beadRecord{}, nil
	}
	var records []beadRecord
	if err := json.Unmarshal(raw, &records); err == nil {
		if len(records) == 0 {
			return beadRecord{}, nil
		}
		return records[0], nil
	}
	var record beadRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return beadRecord{}, err
	}
	return record, nil
}

// auditCommit is one reachable commit's metadata, captured once for the whole
// audit instead of re-shelling to git per bead.
type auditCommit struct {
	shortSHA string
	subject  string
	body     string
	commitAt time.Time
	files    map[string]struct{}
}

// captureAuditCommits runs a single `git log --all` and parses every reachable
// commit's metadata and touched files. The audit previously forked git once
// per bead for --grep matching and once per (bead, path) pair for --since file
// history; this collapses all of it to one subprocess (soc-2grz, PERF-C2).
func captureAuditCommits() []auditCommit {
	const recSep, fldSep = "\x1e", "\x1f"
	out, err := execGitLog("log", "--all", "--name-only",
		"--pretty=format:"+recSep+"%h"+fldSep+"%cI"+fldSep+"%s"+fldSep+"%b"+fldSep)
	if err != nil || strings.TrimSpace(out) == "" {
		return nil
	}
	var commits []auditCommit
	for _, record := range strings.Split(out, recSep) {
		if record = strings.TrimLeft(record, "\n"); record == "" {
			continue
		}
		// parts: short-sha, committer-date, subject, body, "\n<files>".
		parts := strings.SplitN(record, fldSep, 5)
		if len(parts) < 5 {
			continue
		}
		commit := auditCommit{
			shortSHA: strings.TrimSpace(parts[0]),
			commitAt: parseGitTime(parts[1]),
			subject:  strings.TrimSpace(parts[2]),
			body:     parts[3],
			files:    map[string]struct{}{},
		}
		for _, line := range strings.Split(parts[4], "\n") {
			if f := strings.TrimSpace(line); f != "" {
				commit.files[f] = struct{}{}
			}
		}
		commits = append(commits, commit)
	}
	return commits
}

// parseGitTime parses a strict-ISO git date, returning the zero time on error.
func parseGitTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(s))
	if err != nil {
		return time.Time{}
	}
	return t
}

// grepCommitsForID returns up to 3 "<short-sha> <subject>" lines for commits
// whose message references the bead ID, mirroring `git log --oneline --grep`.
func grepCommitsForID(commits []auditCommit, id string) string {
	if id == "" {
		return ""
	}
	var lines []string
	for _, c := range commits {
		if strings.Contains(c.subject, id) || strings.Contains(c.body, id) {
			lines = append(lines, c.shortSHA+" "+c.subject)
			if len(lines) == 3 {
				break
			}
		}
	}
	return strings.Join(lines, "\n")
}

// fileChangesSinceCommits returns "<short-sha> <subject>" evidence for commits
// after createdAt that touched one of the given paths, mirroring the prior
// per-path `git log --oneline --since=<createdAt> -- <path>` calls.
func fileChangesSinceCommits(commits []auditCommit, createdAt string, paths []string) string {
	since := parseGitTime(createdAt)
	var chunks []string
	for _, p := range paths {
		var lines []string
		for _, c := range commits {
			if !since.IsZero() && !c.commitAt.After(since) {
				continue
			}
			if _, ok := c.files[p]; ok {
				lines = append(lines, c.shortSHA+" "+c.subject)
				if len(lines) == 3 {
					break
				}
			}
		}
		if len(lines) > 0 {
			chunks = append(chunks, strings.Join(lines, "\n"))
		}
	}
	return strings.Join(chunks, "\n")
}

func autoCloseLikelyFixed(id, note string) {
	_, _ = execBD("update", id, "--status", "closed", "--append-notes", note)
}

func extractAuditFilePaths(desc string, limit int) []string {
	pathRe := regexp.MustCompile(`[a-zA-Z0-9_./-]+\.[a-zA-Z]{1,6}`)
	seen := make(map[string]bool)
	var out []string
	for _, match := range pathRe.FindAllString(desc, -1) {
		if !strings.Contains(match, "/") || seen[match] {
			continue
		}
		seen[match] = true
		out = append(out, match)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func extractAuditPatterns(desc string, limit int) []string {
	seen := make(map[string]bool)
	var out []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			return
		}
		seen[s] = true
		out = append(out, s)
	}

	backtickRe := regexp.MustCompile("`([^`]+)`")
	for _, m := range backtickRe.FindAllStringSubmatch(desc, -1) {
		add(m[1])
		if limit > 0 && len(out) >= limit/2 {
			break
		}
	}
	identRe := regexp.MustCompile(`\b[a-z][a-zA-Z0-9_]{5,}\b`)
	for _, m := range identRe.FindAllString(desc, -1) {
		add(m)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func anyPatternExists(patterns []string, cache *repoContentCache) bool {
	for _, pattern := range patterns {
		if repoPatternExists(pattern, cache) {
			return true
		}
	}
	return false
}

// patternExistsInRepo reports whether a literal pattern occurs anywhere in the
// scoped audit roots. It builds a fresh index per call; the audit pipeline
// instead shares one repoContentCache across all patterns.
func patternExistsInRepo(pattern string) bool {
	return patternExistsInIndex(pattern, buildRepoContentIndex())
}

// buildRepoContentIndex walks the scoped audit roots once and returns a
// root-prefixed path -> content map of searchable files under the size cap.
func buildRepoContentIndex() map[string]string {
	index := map[string]string{}
	roots := []string{"cli", "skills", "skills-codex", "scripts", "docs", "tests"}
	for _, root := range roots {
		openRoot, err := os.OpenRoot(root)
		if err != nil {
			continue
		}
		_ = fs.WalkDir(openRoot.FS(), ".", func(walkPath string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				switch path.Base(walkPath) {
				case ".git", ".beads", ".agents", "node_modules", "vendor", "testdata":
					return fs.SkipDir
				}
				return nil
			}
			if !isAuditSearchFile(walkPath) {
				return nil
			}
			info, statErr := d.Info()
			if statErr != nil || info.Size() > 1_000_000 {
				return nil
			}
			if content, readErr := openRoot.ReadFile(walkPath); readErr == nil {
				index[path.Join(root, walkPath)] = string(content)
			}
			return nil
		})
		_ = openRoot.Close()
	}
	return index
}

func isAuditSearchFile(path string) bool {
	switch filepath.Ext(path) {
	case ".go", ".py", ".sh", ".ts", ".js", ".md":
		return true
	default:
		return false
	}
}

func auditFlaggedCount(report *AuditReport) int {
	return report.Summary.LikelyFixed + report.Summary.LikelyStale + report.Summary.Consolidatable
}

func emitAuditHuman(w *os.File, r *AuditReport) {
	fmt.Fprintln(w, "=== ao beads audit results ===")
	fmt.Fprintf(w, "Total open/in-progress beads: %d\n", r.Summary.Total)
	fmt.Fprintf(w, "likely-fixed:              %d\n", r.Summary.LikelyFixed)
	fmt.Fprintf(w, "likely-stale:              %d\n", r.Summary.LikelyStale)
	fmt.Fprintf(w, "consolidatable:            %d\n", r.Summary.Consolidatable)
	if len(r.LikelyFixed) > 0 {
		fmt.Fprintf(w, "\nLikely fixed: %s\n", auditFindingIDs(r.LikelyFixed))
	}
	if len(r.LikelyStale) > 0 {
		fmt.Fprintf(w, "\nLikely stale: %s\n", auditFindingIDs(r.LikelyStale))
	}
	if len(r.Consolidatable) > 0 {
		fmt.Fprintln(w, "\nConsolidatable:")
		for _, c := range r.Consolidatable {
			fmt.Fprintf(w, "  %s: %s\n", c.File, strings.Join(c.BeadIDs, " "))
		}
	}
}

func auditFindingIDs(findings []AuditFinding) string {
	ids := make([]string, 0, len(findings))
	for _, finding := range findings {
		ids = append(ids, finding.ID)
	}
	sort.Strings(ids)
	return strings.Join(ids, " ")
}

type ClusterBead struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	IsEpic bool   `json:"is_epic"`
}

type BeadCluster struct {
	Representative string        `json:"representative"`
	SharedKeywords []string      `json:"shared_keywords"`
	Beads          []ClusterBead `json:"beads"`
}

type ClusterReport struct {
	Clusters    []BeadCluster `json:"clusters"`
	Unclustered []ClusterBead `json:"unclustered"`
	Message     string        `json:"message,omitempty"`
	BDAvailable bool          `json:"bd_available"`
	Applied     int           `json:"applied,omitempty"`
	ApplyErrors []string      `json:"apply_errors,omitempty"`
	Error       string        `json:"error,omitempty"`
}

func runBeadsCluster(cmd *cobra.Command, args []string) error {
	report, err := clusterBeads(beadsClusterApply)
	if err != nil {
		return err
	}
	if !report.BDAvailable {
		if beadsClusterJSON {
			return emitJSON(os.Stdout, report)
		}
		fmt.Fprintln(os.Stderr, "WARN: bd not on PATH — skipping cluster analysis (graceful degradation)")
		return nil
	}
	if beadsClusterJSON {
		return emitJSON(os.Stdout, report)
	}
	emitClusterHuman(os.Stdout, report)
	return nil
}

func clusterBeads(apply bool) (*ClusterReport, error) {
	report := &ClusterReport{
		Clusters:    []BeadCluster{},
		Unclustered: []ClusterBead{},
		BDAvailable: bdAvailable(),
	}
	if !report.BDAvailable {
		report.Error = "bd CLI not found"
		return report, nil
	}

	records, err := listBDRecordsByStatus("open")
	if err != nil {
		return nil, err
	}
	enriched := make([]beadRecord, 0, len(records))
	for _, record := range records {
		enriched = append(enriched, enrichBeadRecord(record))
	}
	if len(enriched) < 2 {
		report.Message = "fewer than 2 open beads — nothing to cluster"
		return report, nil
	}
	report.Clusters, report.Unclustered = clusterBeadRecords(enriched)

	if apply {
		for _, cluster := range report.Clusters {
			for _, bead := range cluster.Beads {
				if bead.ID == cluster.Representative {
					continue
				}
				if _, err := execBD("update", bead.ID, "--parent", cluster.Representative); err != nil {
					report.ApplyErrors = append(report.ApplyErrors,
						fmt.Sprintf("%s -> %s: %v", bead.ID, cluster.Representative, err))
					continue
				}
				report.Applied++
			}
		}
	}
	return report, nil
}

func enrichBeadRecord(record beadRecord) beadRecord {
	if record.ID == "" {
		return record
	}
	raw, err := execBD("show", record.ID, "--json")
	if err != nil {
		return record
	}
	enriched, err := parseBDRecord(raw)
	if err != nil || enriched.ID == "" {
		return record
	}
	if enriched.Title == "" {
		enriched.Title = record.Title
	}
	if len(enriched.Labels) == 0 {
		enriched.Labels = record.Labels
	}
	return enriched
}

func clusterBeadRecords(records []beadRecord) ([]BeadCluster, []ClusterBead) {
	if len(records) == 0 {
		return []BeadCluster{}, []ClusterBead{}
	}

	parent := make([]int, len(records))
	for i := range parent {
		parent[i] = i
	}
	find := func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]]
			i = parent[i]
		}
		return i
	}
	union := func(a, b int) {
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[rb] = ra
		}
	}

	for i := range records {
		for j := i + 1; j < len(records); j++ {
			if scoreBeadOverlap(records[i], records[j]) >= 2 {
				union(i, j)
			}
		}
	}

	groups := make(map[int][]beadRecord)
	for i, record := range records {
		groups[find(i)] = append(groups[find(i)], record)
	}

	var clusters []BeadCluster
	var unclustered []ClusterBead
	for _, group := range groups {
		sort.Slice(group, func(i, j int) bool { return group[i].ID < group[j].ID })
		if len(group) < 2 {
			unclustered = append(unclustered, clusterBead(group[0]))
			continue
		}
		clusters = append(clusters, BeadCluster{
			Representative: clusterRepresentative(group),
			SharedKeywords: sharedClusterKeywords(group),
			Beads:          clusterBeadsFromRecords(group),
		})
	}
	sort.Slice(clusters, func(i, j int) bool { return clusters[i].Representative < clusters[j].Representative })
	sort.Slice(unclustered, func(i, j int) bool { return unclustered[i].ID < unclustered[j].ID })
	return clusters, unclustered
}

func clusterBeadsFromRecords(records []beadRecord) []ClusterBead {
	out := make([]ClusterBead, 0, len(records))
	for _, record := range records {
		out = append(out, clusterBead(record))
	}
	return out
}

func clusterBead(record beadRecord) ClusterBead {
	return ClusterBead{ID: record.ID, Title: record.displayTitle(), IsEpic: record.isEpic()}
}

func clusterRepresentative(records []beadRecord) string {
	for _, record := range records {
		if record.isEpic() {
			return record.ID
		}
	}
	if len(records) == 0 {
		return ""
	}
	return records[0].ID
}

func scoreBeadOverlap(a, b beadRecord) int {
	score := intersectionCount(tokenSet(beadClusterText(a)), tokenSet(beadClusterText(b)))
	score += 2 * intersectionCount(pathSet(beadClusterText(a)), pathSet(beadClusterText(b)))
	score += 3 * intersectionCount(stringSet(a.Labels), stringSet(b.Labels))
	return score
}

func beadClusterText(record beadRecord) string {
	return record.displayTitle() + " " + record.textBody()
}

var clusterStopWords = map[string]bool{
	"the": true, "a": true, "an": true, "in": true, "to": true, "for": true,
	"of": true, "and": true, "or": true, "with": true, "is": true, "are": true,
	"be": true, "was": true, "were": true, "by": true, "on": true, "at": true,
	"from": true, "as": true, "this": true, "that": true, "it": true, "its": true,
	"into": true,
}

func tokenSet(input string) map[string]bool {
	out := make(map[string]bool)
	for _, tok := range regexp.MustCompile(`[^a-z0-9/]+`).Split(strings.ToLower(input), -1) {
		if len(tok) < 3 || clusterStopWords[tok] {
			continue
		}
		out[tok] = true
	}
	return out
}

func pathSet(input string) map[string]bool {
	pathRe := regexp.MustCompile(`[a-zA-Z0-9_./-]+/[a-zA-Z0-9_./-]+`)
	out := make(map[string]bool)
	for _, match := range pathRe.FindAllString(input, -1) {
		if strings.Contains(match, ".") {
			out[match] = true
		}
	}
	return out
}

func stringSet(values []string) map[string]bool {
	out := make(map[string]bool)
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out[value] = true
		}
	}
	return out
}

func intersectionCount(a, b map[string]bool) int {
	count := 0
	for value := range a {
		if b[value] {
			count++
		}
	}
	return count
}

func sharedClusterKeywords(records []beadRecord) []string {
	if len(records) == 0 {
		return nil
	}
	shared := tokenSet(beadClusterText(records[0]))
	for _, record := range records[1:] {
		current := tokenSet(beadClusterText(record))
		for keyword := range shared {
			if !current[keyword] {
				delete(shared, keyword)
			}
		}
	}
	return sortedMapKeys(shared)
}

func emitClusterHuman(w *os.File, r *ClusterReport) {
	if r.Message != "" {
		fmt.Fprintln(w, r.Message)
		return
	}
	if len(r.Clusters) == 0 {
		fmt.Fprintf(w, "No clusters found across %d open bead(s).\n", len(r.Unclustered))
		return
	}
	for i, cluster := range r.Clusters {
		label := "overlapping beads"
		if len(cluster.SharedKeywords) > 0 {
			label = strings.Join(cluster.SharedKeywords[:beadMinInt(3, len(cluster.SharedKeywords))], " ")
		}
		fmt.Fprintf(w, "Cluster %d: %q (%d beads)\n", i+1, label, len(cluster.Beads))
		for _, bead := range cluster.Beads {
			epicMarker := ""
			if bead.IsEpic {
				epicMarker = " [epic]"
			}
			fmt.Fprintf(w, "  %s%s: %s\n", bead.ID, epicMarker, bead.Title)
		}
		if len(cluster.SharedKeywords) == 0 {
			fmt.Fprintln(w, "  Shared keywords: none")
		} else {
			fmt.Fprintf(w, "  Shared keywords: %s\n", strings.Join(cluster.SharedKeywords, " "))
		}
		fmt.Fprintf(w, "  Suggestion: Consolidate under %s", cluster.Representative)
		if representativeIsEpic(cluster) {
			fmt.Fprint(w, " (existing epic)")
		}
		fmt.Fprintln(w)
		fmt.Fprintln(w)
	}
	fmt.Fprintf(w, "No clusters found for %d remaining bead(s).\n", len(r.Unclustered))
	if r.Applied > 0 || len(r.ApplyErrors) > 0 {
		fmt.Fprintf(w, "Applied %d reparenting operation(s).\n", r.Applied)
		for _, err := range r.ApplyErrors {
			fmt.Fprintf(w, "WARN: %s\n", err)
		}
	}
}

func representativeIsEpic(cluster BeadCluster) bool {
	for _, bead := range cluster.Beads {
		if bead.ID == cluster.Representative {
			return bead.IsEpic
		}
	}
	return false
}

func firstNNonEmptyLines(input string, n int) string {
	var lines []string
	for _, line := range strings.Split(input, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lines = append(lines, line)
		if len(lines) >= n {
			break
		}
	}
	return strings.Join(lines, "\n")
}

func sortedMapKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
