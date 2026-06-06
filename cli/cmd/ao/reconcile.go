// practices: [sre, dora-metrics, hexagonal-architecture]
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const reconcileSchemaVersion = "1.0"

var (
	reconcileLimit     int
	reconcileSince     string
	reconcileRepo      string
	reconcileRun       reconcileRunner
	reconcileNow       = time.Now
	reconcileTimeout   = 12 * time.Second
	reconcileMaxRecent = 12
)

var reconcileCmd = &cobra.Command{
	Use:   "reconcile",
	Short: "Join git, CI, release, beads, and .agents truth into one report",
	Long: `Build a read-only reconciliation report for the current AgentOps repo.

The report joins the surfaces that often drift apart during agent work: local
git state, GitHub Actions, the latest release and tag Validate run, bd beads,
and recent .agents execution evidence. It is a decision surface, not a mutator:
missing gh/bd auth is reported as an unavailable surface instead of failing the
command.

Examples:
  ao reconcile
  ao reconcile --json
  ao reconcile --repo boshu2/agentops --limit 80`,
	Args: cobra.NoArgs,
	RunE: runReconcile,
}

type reconcileOptions struct {
	Cwd    string
	Output string
	Writer io.Writer
	Limit  int
	Since  time.Duration
	Repo   string
	Now    time.Time
	Run    reconcileRunner
}

type reconcileRunner func(context.Context, string, ...string) ([]byte, error)

type reconcileReport struct {
	SchemaVersion string                 `json:"schema_version"`
	GeneratedAt   string                 `json:"generated_at"`
	OverallStatus string                 `json:"overall_status"`
	Summary       reconcileSummary       `json:"summary"`
	Git           reconcileGitStatus     `json:"git"`
	CI            reconcileCIStatus      `json:"ci"`
	Release       reconcileReleaseStatus `json:"release"`
	Beads         reconcileBeadsStatus   `json:"beads"`
	Agents        reconcileAgentsStatus  `json:"agents"`
	Findings      []reconcileFinding     `json:"findings"`
	NextActions   []string               `json:"next_actions"`
}

type reconcileSummary struct {
	FindingCount int `json:"finding_count"`
	High         int `json:"high"`
	Medium       int `json:"medium"`
	Low          int `json:"low"`
}

type reconcileGitStatus struct {
	Available  bool   `json:"available"`
	Branch     string `json:"branch,omitempty"`
	Head       string `json:"head,omitempty"`
	Upstream   string `json:"upstream,omitempty"`
	RemoteMain string `json:"remote_main,omitempty"`
	Dirty      bool   `json:"dirty"`
	DirtyFiles int    `json:"dirty_files"`
	Ahead      int    `json:"ahead"`
	Behind     int    `json:"behind"`
	Error      string `json:"error,omitempty"`
}

type reconcileCIStatus struct {
	Available bool              `json:"available"`
	Runs      []reconcileCIRun  `json:"runs,omitempty"`
	Latest    *reconcileCIRun   `json:"latest_validate,omitempty"`
	Error     string            `json:"error,omitempty"`
	Counts    map[string]int    `json:"counts,omitempty"`
	ByFlow    map[string]string `json:"by_workflow,omitempty"`
}

type reconcileCIRun struct {
	DatabaseID   int64  `json:"database_id,omitempty"`
	WorkflowName string `json:"workflow_name,omitempty"`
	Status       string `json:"status,omitempty"`
	Conclusion   string `json:"conclusion,omitempty"`
	HeadSHA      string `json:"head_sha,omitempty"`
	DisplayTitle string `json:"display_title,omitempty"`
	URL          string `json:"url,omitempty"`
	CreatedAt    string `json:"created_at,omitempty"`
}

type reconcileReleaseStatus struct {
	Available       bool             `json:"available"`
	TagName         string           `json:"tag_name,omitempty"`
	Name            string           `json:"name,omitempty"`
	PublishedAt     string           `json:"published_at,omitempty"`
	URL             string           `json:"url,omitempty"`
	TargetCommitish string           `json:"target_commitish,omitempty"`
	TagValidateRuns []reconcileCIRun `json:"tag_validate_runs,omitempty"`
	Error           string           `json:"error,omitempty"`
}

type reconcileBeadsStatus struct {
	Available       bool                   `json:"available"`
	Limit           int                    `json:"limit"`
	TotalSampled    int                    `json:"total_sampled"`
	Open            int                    `json:"open"`
	InProgress      int                    `json:"in_progress"`
	Blocked         int                    `json:"blocked"`
	Closed          int                    `json:"closed"`
	ReadyP0         []reconcileBeadSummary `json:"ready_p0,omitempty"`
	OpenReleaseLike []reconcileBeadSummary `json:"open_release_like,omitempty"`
	Error           string                 `json:"error,omitempty"`
	ReadyError      string                 `json:"ready_error,omitempty"`
}

type reconcileBeadSummary struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Status   string `json:"status,omitempty"`
	Priority int    `json:"priority,omitempty"`
	Parent   string `json:"parent,omitempty"`
}

type reconcileAgentsStatus struct {
	Available      bool                      `json:"available"`
	Root           string                    `json:"root"`
	Since          string                    `json:"since"`
	RecentFiles    int                       `json:"recent_files"`
	LatestFiles    []reconcileAgentsEvidence `json:"latest_files,omitempty"`
	ByDirectory    map[string]int            `json:"by_directory,omitempty"`
	MissingTracked []string                  `json:"missing_tracked,omitempty"`
}

type reconcileAgentsEvidence struct {
	Path       string `json:"path"`
	Directory  string `json:"directory"`
	ModifiedAt string `json:"modified_at"`
}

type reconcileFinding struct {
	ID         string `json:"id"`
	Severity   string `json:"severity"`
	Surface    string `json:"surface"`
	Title      string `json:"title"`
	Detail     string `json:"detail"`
	Evidence   string `json:"evidence,omitempty"`
	NextAction string `json:"next_action,omitempty"`
}

type bdIssueForReconcile struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Status   string `json:"status"`
	Priority int    `json:"priority"`
	Parent   string `json:"parent"`
}

func init() {
	reconcileCmd.GroupID = "core"
	reconcileCmd.Flags().IntVar(&reconcileLimit, "limit", 80, "maximum bead and run records to sample")
	reconcileCmd.Flags().StringVar(&reconcileSince, "since", "48h", "recent .agents evidence window")
	reconcileCmd.Flags().StringVar(&reconcileRepo, "repo", "", "GitHub repo override for gh calls (owner/name)")
	rootCmd.AddCommand(reconcileCmd)
}

func runReconcile(cmd *cobra.Command, _ []string) error {
	since, err := time.ParseDuration(reconcileSince)
	if err != nil {
		return fmt.Errorf("parse --since: %w", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	cwd = detectReconcileGitRoot(cmd.Context(), cwd)
	return runReconcileWithOptions(cmd.Context(), reconcileOptions{
		Cwd:    cwd,
		Output: GetOutput(),
		Writer: cmd.OutOrStdout(),
		Limit:  reconcileLimit,
		Since:  since,
		Repo:   reconcileRepo,
		Now:    reconcileNow(),
		Run:    reconcileRun,
	})
}

func runReconcileWithOptions(ctx context.Context, opts reconcileOptions) error {
	opts = normalizeReconcileOptions(opts)
	report := buildReconcileReport(ctx, opts)
	if report.OverallStatus == "" {
		report.OverallStatus = classifyReconcileOverall(report.Findings)
	}
	if opts.Output == "json" {
		enc := json.NewEncoder(opts.Writer)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}
	writeReconcileHuman(opts.Writer, report)
	return nil
}

func normalizeReconcileOptions(opts reconcileOptions) reconcileOptions {
	if opts.Cwd == "" {
		if cwd, err := os.Getwd(); err == nil {
			opts.Cwd = cwd
		}
	}
	if opts.Output == "" {
		opts.Output = "table"
	}
	if opts.Writer == nil {
		opts.Writer = os.Stdout
	}
	if opts.Limit <= 0 {
		opts.Limit = 80
	}
	if opts.Since <= 0 {
		opts.Since = 48 * time.Hour
	}
	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}
	if opts.Run == nil {
		opts.Run = newDefaultReconcileRunner(opts.Cwd)
	}
	return opts
}

func buildReconcileReport(ctx context.Context, opts reconcileOptions) reconcileReport {
	opts = normalizeReconcileOptions(opts)
	report := reconcileReport{
		SchemaVersion: reconcileSchemaVersion,
		GeneratedAt:   opts.Now.UTC().Format(time.RFC3339),
	}
	report.Git = collectReconcileGit(ctx, opts)
	report.CI = collectReconcileCI(ctx, opts)
	report.Release = collectReconcileRelease(ctx, opts)
	report.Beads = collectReconcileBeads(ctx, opts, report.Release.TagName)
	report.Agents = collectReconcileAgents(ctx, opts)
	report.Findings = buildReconcileFindings(report)
	report.Summary = summarizeReconcileFindings(report.Findings)
	report.OverallStatus = classifyReconcileOverall(report.Findings)
	report.NextActions = buildReconcileNextActions(report.Findings)
	return report
}

func defaultReconcileRun(ctx context.Context, name string, args ...string) ([]byte, error) {
	return defaultReconcileRunInDir(ctx, "", name, args...)
}

func newDefaultReconcileRunner(cwd string) reconcileRunner {
	return func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return defaultReconcileRunInDir(ctx, cwd, name, args...)
	}
}

func defaultReconcileRunInDir(ctx context.Context, cwd, name string, args ...string) ([]byte, error) {
	runCtx, cancel := context.WithTimeout(ctx, reconcileTimeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, name, args...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	cmd.Env = os.Environ()
	if name == "bd" {
		cmd.Env = withEnvOverride(cmd.Env, "BEADS_DOLT_AUTO_START", "0")
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return stdout.Bytes(), fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, detail)
		}
		return stdout.Bytes(), fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return stdout.Bytes(), nil
}

func withEnvOverride(env []string, key, value string) []string {
	prefix := key + "="
	next := env[:0]
	for _, item := range env {
		if !strings.HasPrefix(item, prefix) {
			next = append(next, item)
		}
	}
	return append(next, prefix+value)
}

func detectReconcileGitRoot(ctx context.Context, cwd string) string {
	runCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(runCtx, "git", "-C", cwd, "rev-parse", "--show-toplevel")
	raw, err := cmd.Output()
	if err != nil {
		return cwd
	}
	root := strings.TrimSpace(string(raw))
	if root == "" {
		return cwd
	}
	return root
}

func collectReconcileGit(ctx context.Context, opts reconcileOptions) reconcileGitStatus {
	status := reconcileGitStatus{}
	head, err := runTrimmed(ctx, opts, "git", "rev-parse", "HEAD")
	if err != nil {
		status.Error = err.Error()
		return status
	}
	status.Available = true
	status.Head = head
	status.Branch, _ = runTrimmed(ctx, opts, "git", "rev-parse", "--abbrev-ref", "HEAD")
	status.Upstream, _ = runTrimmed(ctx, opts, "git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	status.RemoteMain, _ = runTrimmed(ctx, opts, "git", "ls-remote", "origin", "refs/heads/main")
	if status.RemoteMain != "" {
		status.RemoteMain = strings.Fields(status.RemoteMain)[0]
	}
	if porcelain, err := runTrimmed(ctx, opts, "git", "status", "--porcelain"); err == nil {
		status.DirtyFiles = countNonEmptyLines(porcelain)
		status.Dirty = status.DirtyFiles > 0
	}
	compareTarget := status.Upstream
	if compareTarget == "" {
		compareTarget = "origin/main"
	}
	if counts, err := runTrimmed(ctx, opts, "git", "rev-list", "--left-right", "--count", "HEAD..."+compareTarget); err == nil {
		fields := strings.Fields(counts)
		if len(fields) >= 2 {
			status.Ahead, _ = strconv.Atoi(fields[0])
			status.Behind, _ = strconv.Atoi(fields[1])
		}
	}
	return status
}

func collectReconcileCI(ctx context.Context, opts reconcileOptions) reconcileCIStatus {
	args := []string{"run", "list", "--branch", "main", "--limit", strconv.Itoa(reconcileMinInt(opts.Limit, reconcileMaxRecent)), "--json", "databaseId,workflowName,status,conclusion,headSha,displayTitle,url,createdAt"}
	args = appendRepoArgs(args, opts.Repo)
	raw, err := opts.Run(ctx, "gh", args...)
	if err != nil {
		return reconcileCIStatus{Available: false, Error: err.Error()}
	}
	runs, err := parseReconcileGHRuns(raw)
	if err != nil {
		return reconcileCIStatus{Available: false, Error: fmt.Sprintf("parse gh runs: %v", err)}
	}
	status := reconcileCIStatus{
		Available: true,
		Runs:      runs,
		Counts:    map[string]int{},
		ByFlow:    map[string]string{},
	}
	for i := range runs {
		r := runs[i]
		key := r.Status
		if r.Conclusion != "" {
			key = r.Conclusion
		}
		status.Counts[key]++
		if _, ok := status.ByFlow[r.WorkflowName]; !ok {
			status.ByFlow[r.WorkflowName] = statusLabel(r.Status, r.Conclusion)
		}
		if status.Latest == nil && strings.EqualFold(r.WorkflowName, "Validate") {
			runCopy := r
			status.Latest = &runCopy
		}
	}
	return status
}

func collectReconcileRelease(ctx context.Context, opts reconcileOptions) reconcileReleaseStatus {
	args := []string{"release", "view", "--json", "tagName,name,publishedAt,url,targetCommitish"}
	args = appendRepoArgs(args, opts.Repo)
	raw, err := opts.Run(ctx, "gh", args...)
	if err != nil {
		return reconcileReleaseStatus{Available: false, Error: err.Error()}
	}
	var release ghReleaseForReconcile
	if err := json.Unmarshal(raw, &release); err != nil {
		return reconcileReleaseStatus{Available: false, Error: fmt.Sprintf("parse gh release: %v", err)}
	}
	status := reconcileReleaseStatus{
		Available:       release.TagName != "",
		TagName:         release.TagName,
		Name:            release.Name,
		PublishedAt:     release.PublishedAt,
		URL:             release.URL,
		TargetCommitish: release.TargetCommitish,
	}
	status.Available = status.TagName != ""
	if status.TagName == "" {
		return status
	}
	tagArgs := []string{"run", "list", "--branch", status.TagName, "--workflow", "Validate", "--limit", "5", "--json", "databaseId,workflowName,status,conclusion,headSha,displayTitle,url,createdAt"}
	tagArgs = appendRepoArgs(tagArgs, opts.Repo)
	if raw, err := opts.Run(ctx, "gh", tagArgs...); err == nil {
		runs, parseErr := parseReconcileGHRuns(raw)
		if parseErr != nil {
			status.Error = fmt.Sprintf("parse tag validate runs: %v", parseErr)
		} else {
			status.TagValidateRuns = runs
		}
	} else {
		status.Error = strings.TrimSpace(err.Error())
	}
	return status
}

func collectReconcileBeads(ctx context.Context, opts reconcileOptions, releaseTag string) reconcileBeadsStatus {
	status := reconcileBeadsStatus{Limit: opts.Limit}
	raw, err := opts.Run(ctx, "bd", "list", "--limit", strconv.Itoa(opts.Limit), "--json")
	if err != nil {
		status.Error = err.Error()
		return status
	}
	var issues []bdIssueForReconcile
	if err := json.Unmarshal(raw, &issues); err != nil {
		status.Error = fmt.Sprintf("parse bd list: %v", err)
		return status
	}
	status.Available = true
	status.TotalSampled = len(issues)
	for _, issue := range issues {
		switch strings.ToLower(issue.Status) {
		case "open":
			status.Open++
		case "in_progress":
			status.InProgress++
		case "blocked":
			status.Blocked++
		case "closed":
			status.Closed++
		}
		if isOpenish(issue.Status) && isReleaseLike(issue.Title, releaseTag) {
			status.OpenReleaseLike = append(status.OpenReleaseLike, summarizeBead(issue))
		}
	}
	readyRaw, err := opts.Run(ctx, "bd", "ready", "-n", "20", "--json")
	if err != nil {
		status.ReadyError = err.Error()
		return status
	}
	var ready []bdIssueForReconcile
	if err := json.Unmarshal(readyRaw, &ready); err != nil {
		status.ReadyError = fmt.Sprintf("parse bd ready: %v", err)
		return status
	}
	for _, issue := range ready {
		if issue.Priority == 0 {
			status.ReadyP0 = append(status.ReadyP0, summarizeBead(issue))
		}
	}
	return status
}

func collectReconcileAgents(ctx context.Context, opts reconcileOptions) reconcileAgentsStatus {
	rootDir := opts.Cwd
	if gitRoot, err := runTrimmed(ctx, opts, "git", "rev-parse", "--show-toplevel"); err == nil && gitRoot != "" {
		rootDir = gitRoot
	}
	root := filepath.Join(rootDir, ".agents")
	status := reconcileAgentsStatus{
		Root:        root,
		Since:       opts.Now.Add(-opts.Since).UTC().Format(time.RFC3339),
		ByDirectory: map[string]int{},
	}
	if _, err := os.Stat(root); err != nil {
		status.Available = false
		return status
	}
	status.Available = true
	tracked := []string{"rpi", "tests", "evals", "learnings", "findings", "plans", "validation", "releases", "handoff"}
	cutoff := opts.Now.Add(-opts.Since)
	for _, dir := range tracked {
		full := filepath.Join(root, dir)
		if _, err := os.Stat(full); err != nil {
			status.MissingTracked = append(status.MissingTracked, dir)
			continue
		}
		_ = filepath.WalkDir(full, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			info, err := d.Info()
			if err != nil || info.ModTime().Before(cutoff) {
				return nil
			}
			rel := relativePathOrOriginal(rootDir, path)
			status.RecentFiles++
			status.ByDirectory[dir]++
			status.LatestFiles = append(status.LatestFiles, reconcileAgentsEvidence{
				Path:       rel,
				Directory:  dir,
				ModifiedAt: info.ModTime().UTC().Format(time.RFC3339),
			})
			return nil
		})
	}
	sort.Slice(status.LatestFiles, func(i, j int) bool {
		return status.LatestFiles[i].ModifiedAt > status.LatestFiles[j].ModifiedAt
	})
	if len(status.LatestFiles) > 10 {
		status.LatestFiles = status.LatestFiles[:10]
	}
	return status
}

func buildReconcileFindings(report reconcileReport) []reconcileFinding {
	var findings []reconcileFinding
	add := func(id, severity, surface, title, detail, evidence, next string) {
		findings = append(findings, reconcileFinding{
			ID: id, Severity: severity, Surface: surface, Title: title,
			Detail: detail, Evidence: evidence, NextAction: next,
		})
	}
	if !report.Git.Available {
		add("git-unavailable", "medium", "git", "Local git state unavailable", report.Git.Error, "", "Run inside a git checkout or inspect git manually.")
	} else {
		if report.Git.Dirty {
			add("git-dirty", "medium", "git", "Local checkout has uncommitted changes", fmt.Sprintf("%d changed files are visible to git.", report.Git.DirtyFiles), report.Git.Branch, "Separate user work from implementation branches before reconciling shipped state.")
		}
		if report.Git.Behind > 0 {
			add("git-behind", "medium", "git", "Local branch is behind its upstream", fmt.Sprintf("behind=%d ahead=%d", report.Git.Behind, report.Git.Ahead), report.Git.Upstream, "Prefer GitHub/origin truth or update the local checkout before acting.")
		}
	}
	if !report.CI.Available {
		add("ci-unavailable", "medium", "ci", "GitHub Actions state unavailable", report.CI.Error, "", "Run gh auth status or inspect GitHub Actions in browser.")
	} else if report.CI.Latest == nil {
		add("ci-main-validate-missing", "high", "ci", "No recent main Validate run found", "Could not identify a Validate workflow run on main.", "", "Inspect GitHub Actions before treating main as green.")
	} else if report.CI.Latest.Status != "completed" || report.CI.Latest.Conclusion != "success" {
		add("ci-main-validate-not-green", "high", "ci", "Main Validate is not green", statusLabel(report.CI.Latest.Status, report.CI.Latest.Conclusion), report.CI.Latest.URL, "Fix or classify the latest main Validate run before backlog work.")
	}
	if !report.Release.Available {
		add("release-unavailable", "medium", "release", "Latest release state unavailable", report.Release.Error, "", "Inspect releases before making release-health claims.")
	} else if len(report.Release.TagValidateRuns) == 0 {
		add("release-tag-validate-missing", "medium", "release", "Latest release has no visible tag Validate run", report.Release.TagName, report.Release.URL, "Rerun or inspect tag Validate before closing release-health work.")
	} else {
		latest := report.Release.TagValidateRuns[0]
		if latest.Status != "completed" || latest.Conclusion != "success" {
			add("release-tag-validate-not-green", "high", "release", "Latest release tag Validate is not green", statusLabel(latest.Status, latest.Conclusion)+" for "+report.Release.TagName, latest.URL, "Rerun tag Validate when appropriate, or cut a newer release tag whose Validate run is green before claiming release health.")
		}
	}
	if !report.Beads.Available {
		add("beads-unavailable", "medium", "beads", "Bead ledger unavailable", report.Beads.Error, "", "Use live bd on the shared ledger before planning tracked work.")
	} else {
		if len(report.Beads.OpenReleaseLike) > 0 {
			add("beads-release-stale", "medium", "beads", "Open release-like beads may misrepresent shipped reality", fmt.Sprintf("%d sampled open/in-progress release-like beads remain.", len(report.Beads.OpenReleaseLike)), firstBeadEvidence(report.Beads.OpenReleaseLike), "Reconcile release beads against PRs, tags, and release assets.")
		}
		if len(report.Beads.ReadyP0) > 0 {
			add("beads-ready-p0", "medium", "beads", "Ready P0 work exists", fmt.Sprintf("%d ready P0 beads found in the sampled ready queue.", len(report.Beads.ReadyP0)), firstBeadEvidence(report.Beads.ReadyP0), "Confirm whether product/legal/customer gates outrank code work today.")
		}
	}
	if !report.Agents.Available {
		add("agents-unavailable", "low", ".agents", ".agents evidence directory missing", report.Agents.Root, "", "Do not infer execution evidence from missing local artifacts.")
	} else if report.Agents.RecentFiles == 0 {
		add("agents-stale", "low", ".agents", "No recent .agents evidence in tracked dirs", "No files changed inside tracked evidence dirs during the configured window.", report.Agents.Since, "Treat beads/GitHub as stronger evidence until fresh .agents artifacts exist.")
	}
	return findings
}

func summarizeReconcileFindings(findings []reconcileFinding) reconcileSummary {
	summary := reconcileSummary{FindingCount: len(findings)}
	for _, finding := range findings {
		switch finding.Severity {
		case "high":
			summary.High++
		case "medium":
			summary.Medium++
		default:
			summary.Low++
		}
	}
	return summary
}

type ghRunForReconcile struct {
	DatabaseID   int64  `json:"databaseId"`
	WorkflowName string `json:"workflowName"`
	Status       string `json:"status"`
	Conclusion   string `json:"conclusion"`
	HeadSHA      string `json:"headSha"`
	DisplayTitle string `json:"displayTitle"`
	URL          string `json:"url"`
	CreatedAt    string `json:"createdAt"`
}

type ghReleaseForReconcile struct {
	TagName         string `json:"tagName"`
	Name            string `json:"name"`
	PublishedAt     string `json:"publishedAt"`
	URL             string `json:"url"`
	TargetCommitish string `json:"targetCommitish"`
}

func parseReconcileGHRuns(raw []byte) ([]reconcileCIRun, error) {
	var ghRuns []ghRunForReconcile
	if err := json.Unmarshal(raw, &ghRuns); err != nil {
		return nil, err
	}
	runs := make([]reconcileCIRun, 0, len(ghRuns))
	for _, run := range ghRuns {
		runs = append(runs, reconcileCIRun(run))
	}
	return runs, nil
}

func classifyReconcileOverall(findings []reconcileFinding) string {
	summary := summarizeReconcileFindings(findings)
	switch {
	case summary.High > 0:
		return "needs_attention"
	case summary.Medium > 0:
		return "needs_reconciliation"
	case summary.Low > 0:
		return "green_with_warnings"
	default:
		return "green"
	}
}

func buildReconcileNextActions(findings []reconcileFinding) []string {
	seen := map[string]bool{}
	var actions []string
	for _, finding := range findings {
		if finding.NextAction == "" || seen[finding.NextAction] {
			continue
		}
		seen[finding.NextAction] = true
		actions = append(actions, finding.NextAction)
	}
	if len(actions) > 5 {
		return actions[:5]
	}
	return actions
}

func writeReconcileHuman(w io.Writer, report reconcileReport) {
	fmt.Fprintln(w, "AgentOps Reconciliation")
	fmt.Fprintln(w, "=======================")
	fmt.Fprintf(w, "Status: %s\n", report.OverallStatus)
	if report.Git.Available {
		fmt.Fprintf(w, "Git: %s %s", report.Git.Branch, shortSHA(report.Git.Head))
		if report.Git.Dirty {
			fmt.Fprintf(w, " dirty=%d", report.Git.DirtyFiles)
		}
		if report.Git.Ahead > 0 || report.Git.Behind > 0 {
			fmt.Fprintf(w, " ahead=%d behind=%d", report.Git.Ahead, report.Git.Behind)
		}
		fmt.Fprintln(w)
	}
	if report.CI.Latest != nil {
		fmt.Fprintf(w, "Main Validate: %s (%s)\n", statusLabel(report.CI.Latest.Status, report.CI.Latest.Conclusion), report.CI.Latest.URL)
	}
	if report.Release.Available {
		fmt.Fprintf(w, "Release: %s (%s)\n", report.Release.TagName, report.Release.URL)
	}
	if report.Beads.Available {
		fmt.Fprintf(w, "Beads: %d sampled, %d open, %d in_progress, %d blocked\n",
			report.Beads.TotalSampled, report.Beads.Open, report.Beads.InProgress, report.Beads.Blocked)
	}
	if report.Agents.Available {
		fmt.Fprintf(w, ".agents: %d recent files since %s\n", report.Agents.RecentFiles, report.Agents.Since)
	}
	if len(report.Findings) > 0 {
		fmt.Fprintln(w, "\nFindings:")
		for _, finding := range report.Findings {
			fmt.Fprintf(w, "- [%s] %s: %s\n", strings.ToUpper(finding.Severity), finding.Surface, finding.Title)
			if finding.Evidence != "" {
				fmt.Fprintf(w, "  Evidence: %s\n", finding.Evidence)
			}
			if finding.NextAction != "" {
				fmt.Fprintf(w, "  Next: %s\n", finding.NextAction)
			}
		}
	}
	if len(report.NextActions) > 0 {
		fmt.Fprintln(w, "\nNext actions:")
		for i, action := range report.NextActions {
			fmt.Fprintf(w, "%d. %s\n", i+1, action)
		}
	}
}

func runTrimmed(ctx context.Context, opts reconcileOptions, name string, args ...string) (string, error) {
	raw, err := opts.Run(ctx, name, args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(raw)), nil
}

func appendRepoArgs(args []string, repo string) []string {
	if strings.TrimSpace(repo) == "" {
		return args
	}
	return append(args, "--repo", repo)
}

func countNonEmptyLines(s string) int {
	count := 0
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}

func isOpenish(status string) bool {
	switch strings.ToLower(status) {
	case "open", "in_progress", "blocked":
		return true
	default:
		return false
	}
}

func isReleaseLike(title, releaseTag string) bool {
	lower := strings.ToLower(title)
	if strings.Contains(lower, "release") || strings.Contains(lower, "changelog") || strings.Contains(lower, "migration") {
		return true
	}
	if releaseTag != "" && strings.Contains(lower, strings.TrimPrefix(strings.ToLower(releaseTag), "v")) {
		return true
	}
	return strings.Contains(lower, "3.0") || strings.Contains(lower, "version sync")
}

func summarizeBead(issue bdIssueForReconcile) reconcileBeadSummary {
	return reconcileBeadSummary(issue)
}

func firstBeadEvidence(beads []reconcileBeadSummary) string {
	if len(beads) == 0 {
		return ""
	}
	if len(beads) == 1 {
		return beads[0].ID
	}
	return fmt.Sprintf("%s (+%d more)", beads[0].ID, len(beads)-1)
}

func statusLabel(status, conclusion string) string {
	if status == "" && conclusion == "" {
		return "unknown"
	}
	if conclusion == "" {
		return status
	}
	if status == "" || status == "completed" {
		return conclusion
	}
	return status + "/" + conclusion
}

func shortSHA(sha string) string {
	if len(sha) <= 12 {
		return sha
	}
	return sha[:12]
}

func reconcileMinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
