package doctor

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Exit codes for the doctor surface. See CLI-SURFACE.md § Exit codes.
const (
	ExitHealthy        = 0
	ExitFindings       = 1
	ExitFixPartial     = 2
	ExitFixFailed      = 3
	ExitRefusedUnsafe  = 4
	ExitConcurrency    = 5
	ExitOnlineRequired = 6
	ExitUsage          = 64
	ExitNoInput        = 66
	ExitCantCreate     = 73
	ExitIOError        = 74
)

// Options configures a Diagnose or Fix invocation.
type Options struct {
	RepoRoot    string
	CWD         string
	HomeDir     string
	ToolVersion string
	Only        []string
	Skip        []string
	Quick       bool
	Online      bool
	Severity    string // minimum severity: P0|P1|P2|P3
	DryRun      bool
	JSON        bool
	Now         time.Time
}

// Report is the structured result of a Diagnose or Fix run, matching the
// CLI-SURFACE.md diagnose --json schema (Fix adds the optional fields).
type Report struct {
	SchemaVersion string        `json:"schema_version"`
	Tool          string        `json:"tool"`
	ToolVersion   string        `json:"tool_version"`
	DoctorVersion string        `json:"doctor_version"`
	RunID         string        `json:"run_id"`
	RunDir        string        `json:"run_dir"`
	StartedAt     string        `json:"started_at"`
	FinishedAt    string        `json:"finished_at"`
	DurationMS    int64         `json:"duration_ms"`
	TargetSHA     string        `json:"target_sha"`
	OK            bool          `json:"ok"`
	Summary       ReportSummary `json:"summary"`
	Findings      []Finding     `json:"findings"`
	ExitCode      int           `json:"exit_code"`
	NextSteps     []string      `json:"next_steps"`
	ActionsTaken  int           `json:"actions_taken,omitempty"`
	ActionsPath   string        `json:"actions_jsonl_path,omitempty"`
	BackupsDir    string        `json:"backups_dir,omitempty"`
	UndoCommand   string        `json:"undo_command,omitempty"`
}

// ReportSummary is the summary block of a Report.
type ReportSummary struct {
	TotalFindings  int            `json:"total_findings"`
	BySeverity     map[string]int `json:"by_severity"`
	AutoFixable    int            `json:"auto_fixable"`
	OnlineRequired int            `json:"online_required"`
}

// severityRank maps a severity string to a numeric rank (lower = more severe).
func severityRank(s string) int {
	switch strings.ToUpper(s) {
	case "P0":
		return 0
	case "P1":
		return 1
	case "P2":
		return 2
	case "P3":
		return 3
	default:
		return 3
	}
}

// targetSHA returns the repo HEAD SHA, or "unknown" if git is unavailable.
func targetSHA(repoRoot string) string {
	cmd := exec.Command("git", "-C", repoRoot, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

// selectDetectors applies --only / --skip / --quick / --severity filters.
func selectDetectors(opts Options) []Detector {
	only := toSet(opts.Only)
	skip := toSet(opts.Skip)
	minRank := severityRank(strings.ToUpper(opts.Severity))
	if opts.Severity == "" {
		minRank = severityRank("P3")
	}
	var out []Detector
	for _, d := range Detectors() {
		if len(only) > 0 && !only[d.ID()] && !only[d.Subsystem()] {
			continue
		}
		if skip[d.ID()] || skip[d.Subsystem()] {
			continue
		}
		if opts.Quick && !d.QuickPath() {
			continue
		}
		if severityRank(d.Severity()) > minRank {
			continue
		}
		out = append(out, d)
	}
	return out
}

func toSet(items []string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, it := range items {
		it = strings.TrimSpace(it)
		if it != "" {
			m[it] = true
		}
	}
	return m
}

// runDetectors runs the selected detectors and returns sorted findings plus a
// flag indicating an online probe was needed but --online was not passed.
func runDetectors(env *DetectEnv, dets []Detector, online bool) ([]Finding, bool, error) {
	var findings []Finding
	onlineNeeded := false
	for _, d := range dets {
		if d.OnlineRequired() && !online {
			onlineNeeded = true
			continue
		}
		fs, err := d.Detect(env)
		if err != nil {
			return nil, onlineNeeded, fmt.Errorf("doctor: detector %s failed: %w", d.ID(), err)
		}
		findings = append(findings, fs...)
	}
	sort.Slice(findings, func(i, j int) bool {
		if severityRank(findings[i].Severity) != severityRank(findings[j].Severity) {
			return severityRank(findings[i].Severity) < severityRank(findings[j].Severity)
		}
		return findings[i].ID < findings[j].ID
	})
	return findings, onlineNeeded, nil
}

// summarize builds the ReportSummary from a finding set.
func summarize(findings []Finding) ReportSummary {
	s := ReportSummary{BySeverity: map[string]int{"P0": 0, "P1": 0, "P2": 0, "P3": 0}}
	s.TotalFindings = len(findings)
	for _, f := range findings {
		key := strings.ToUpper(f.Severity)
		s.BySeverity[key]++
		if f.Remediation.AutoFixable {
			s.AutoFixable++
		}
	}
	return s
}

// Diagnose runs all selected detectors, writes the run artifacts, and returns
// the report with the correct exit code (0 healthy / 1 findings / 4 refused /
// 6 online-required).
func Diagnose(opts Options) (*Report, error) {
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	sha := targetSHA(opts.RepoRoot)
	ra, err := NewRunArtifact(opts.RepoRoot, sha, now)
	if err != nil {
		return nil, fmt.Errorf("doctor: %w", err)
	}
	env := &DetectEnv{
		RepoRoot:  opts.RepoRoot,
		CWD:       opts.CWD,
		HomeDir:   opts.HomeDir,
		TargetSHA: sha,
		Online:    opts.Online,
		Logger:    os.Stderr,
	}
	dets := selectDetectors(opts)
	findings, onlineNeeded, err := runDetectors(env, dets, opts.Online)
	if err != nil {
		return nil, err
	}

	rep := buildReport(ra, opts.ToolVersion, sha, now, findings)
	switch {
	case onlineNeeded:
		rep.ExitCode = ExitOnlineRequired
	case len(findings) > 0:
		rep.ExitCode = ExitFindings
	default:
		rep.ExitCode = ExitHealthy
	}
	rep.OK = rep.ExitCode == ExitHealthy
	rep.NextSteps = diagnoseNextSteps(findings)
	if err := persistRun(ra, opts, rep, 0); err != nil {
		return rep, err
	}
	return rep, nil
}

// buildReport assembles a Report shell from a finding set.
func buildReport(ra *RunArtifact, toolVersion, sha string, now time.Time, findings []Finding) *Report {
	finished := time.Now()
	return &Report{
		SchemaVersion: SchemaVersion,
		Tool:          ToolName,
		ToolVersion:   toolVersion,
		DoctorVersion: DoctorVersion,
		RunID:         ra.RunID,
		RunDir:        filepath.Join(".doctor", "runs", filepath.Base(ra.RunDir)),
		StartedAt:     ra.StartedAt.Format(time.RFC3339),
		FinishedAt:    finished.UTC().Format(time.RFC3339Nano),
		DurationMS:    finished.Sub(now).Milliseconds(),
		TargetSHA:     sha,
		Summary:       summarize(findings),
		Findings:      findings,
	}
}

// diagnoseNextSteps produces the next_steps hints for a diagnose run.
func diagnoseNextSteps(findings []Finding) []string {
	if len(findings) == 0 {
		return []string{"Workspace healthy. No action needed."}
	}
	steps := []string{"Run: ao doctor --fix"}
	if len(findings) > 0 {
		steps = append(steps,
			"Or scope: ao doctor --fix --only "+findings[0].ID,
			"Inspect: ao doctor explain "+findings[0].ID,
		)
	}
	return steps
}

// persistRun writes all run artifacts (report.json, report.md, stderr.log,
// undo.sh, scorecard history) for a completed run.
func persistRun(ra *RunArtifact, opts Options, rep *Report, actions int) error {
	if rep.Findings == nil {
		rep.Findings = []Finding{}
	}
	if err := ra.WriteReportJSON(rep); err != nil {
		return err
	}
	if err := ra.WriteReportMD(renderReportMD(rep)); err != nil {
		return err
	}
	if err := ra.WriteStderrLog(fmt.Sprintf("doctor run %s completed exit=%d\n", ra.RunID, rep.ExitCode)); err != nil {
		return err
	}
	if err := ra.WriteUndoScript(); err != nil {
		return err
	}
	return ra.AppendScorecardHistory(opts.ToolVersion, rep.OK, rep.Summary.TotalFindings, rep.Summary.BySeverity, actions, time.Duration(rep.DurationMS)*time.Millisecond)
}

// Fix runs detectors, then runs the auto-fixable fixer for each finding through
// Mutate, honoring the dependency order from analysis/dependency_graph.json.
// It maps results to exit 0 (all fixed) / 2 (partial) / 3 (failed).
func Fix(opts Options) (*Report, error) {
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	sha := targetSHA(opts.RepoRoot)
	ra, err := NewRunArtifact(opts.RepoRoot, sha, now)
	if err != nil {
		return nil, fmt.Errorf("doctor: %w", err)
	}
	env := &DetectEnv{
		RepoRoot: opts.RepoRoot, CWD: opts.CWD, HomeDir: opts.HomeDir,
		TargetSHA: sha, Online: opts.Online, Logger: os.Stderr,
	}
	dets := selectDetectors(opts)
	findings, onlineNeeded, err := runDetectors(env, dets, opts.Online)
	if err != nil {
		return nil, err
	}
	rep := buildReport(ra, opts.ToolVersion, sha, now, findings)

	if onlineNeeded {
		rep.ExitCode = ExitOnlineRequired
		rep.NextSteps = []string{"Re-run with --online to enable network probes."}
		return rep, persistRun(ra, opts, rep, 0)
	}
	if len(findings) == 0 {
		rep.ExitCode = ExitHealthy
		rep.OK = true
		rep.NextSteps = []string{"No findings. Nothing to fix."}
		return rep, persistRun(ra, opts, rep, 0)
	}

	actionsFile, err := ra.OpenActionsFile()
	if err != nil {
		return rep, err
	}
	defer func() { _ = actionsFile.Close() }()

	caps := NewCapabilities(opts.ToolVersion)
	locks := NewLockManager(filepath.Join(opts.RepoRoot, ".doctor", "locks"))
	mctx := NewMutateContext(ra, caps, opts.HomeDir, locks, actionsFile, opts.DryRun)

	totalActions, fixedCount, failed := applyFixers(opts.RepoRoot, mctx, env, findings)

	rep.ActionsTaken = totalActions
	rep.ActionsPath = filepath.Join(rep.RunDir, "actions.jsonl")
	rep.BackupsDir = filepath.Join(rep.RunDir, "backups")
	rep.UndoCommand = fmt.Sprintf("ao doctor undo %s", filepath.Base(ra.RunDir))
	rep.ExitCode = fixExitCode(len(findings), fixedCount, failed)
	rep.OK = rep.ExitCode == ExitHealthy
	rep.NextSteps = []string{rep.UndoCommand + "  # if --fix went wrong"}
	return rep, persistRun(ra, opts, rep, totalActions)
}

// applyFixers runs the registered fixer for each finding in dependency order.
func applyFixers(repoRoot string, mctx *MutateContext, env *DetectEnv, findings []Finding) (actions, fixed int, failed bool) {
	order := loadFixerOrder(repoRoot)
	byFixer := groupFindingsByFixer(findings)
	for _, fixerID := range orderFixers(order, byFixer) {
		fx := FixerByID(fixerID)
		fs := byFixer[fixerID]
		if fx == nil || !fx.AutoFixable() {
			continue
		}
		res, err := fx.Fix(mctx.WithFixer(fixerID), env, fs)
		actions += res.ActionsTaken
		if err != nil || !res.Fixed {
			failed = true
			continue
		}
		fixed += len(fs)
	}
	return actions, fixed, failed
}

// groupFindingsByFixer buckets findings by the fixer that handles them. A fixer
// is matched by ID equality with the finding ID (the convention from the specs).
func groupFindingsByFixer(findings []Finding) map[string][]Finding {
	out := make(map[string][]Finding)
	for _, f := range findings {
		if FixerByID(f.ID) != nil {
			out[f.ID] = append(out[f.ID], f)
		}
	}
	return out
}

// fixExitCode maps fix outcomes to an exit code.
func fixExitCode(total, fixed int, failed bool) int {
	switch {
	case failed && fixed == 0:
		return ExitFixFailed
	case fixed < total:
		return ExitFixPartial
	default:
		return ExitHealthy
	}
}

// loadFixerOrder loads the topological fixer order from
// analysis/dependency_graph.json. If absent, it returns nil and callers fall
// back to ID sort.
func loadFixerOrder(repoRoot string) []string {
	for _, candidate := range []string{
		filepath.Join(repoRoot, "..", "analysis", "dependency_graph.json"),
		filepath.Join(repoRoot, ".doctor", "dependency_graph.json"),
	} {
		data, err := os.ReadFile(candidate)
		if err != nil {
			continue
		}
		order, err := topoSortGraph(data)
		if err == nil {
			return order
		}
	}
	return nil
}

// depGraph is the JSON shape of analysis/dependency_graph.json.
type depGraph struct {
	Nodes []string `json:"nodes"`
	Edges []struct {
		From string `json:"from"`
		To   string `json:"to"`
	} `json:"edges"`
}

// topoSortGraph topologically sorts a dependency graph (from -> to means "from
// must run before to"). Ties break by ID for determinism. Cycles fall back to
// node order.
func topoSortGraph(data []byte) ([]string, error) {
	var g depGraph
	if err := json.Unmarshal(data, &g); err != nil {
		return nil, err
	}
	indeg := make(map[string]int)
	adj := make(map[string][]string)
	for _, n := range g.Nodes {
		indeg[n] = 0
	}
	for _, e := range g.Edges {
		adj[e.From] = append(adj[e.From], e.To)
		indeg[e.To]++
	}
	var ready []string
	for n, d := range indeg {
		if d == 0 {
			ready = append(ready, n)
		}
	}
	sort.Strings(ready)
	var out []string
	for len(ready) > 0 {
		n := ready[0]
		ready = ready[1:]
		out = append(out, n)
		next := append([]string(nil), adj[n]...)
		sort.Strings(next)
		for _, m := range next {
			indeg[m]--
			if indeg[m] == 0 {
				ready = append(ready, m)
				sort.Strings(ready)
			}
		}
	}
	if len(out) != len(g.Nodes) {
		// cycle: fall back to node order
		return g.Nodes, nil
	}
	return out, nil
}

// orderFixers returns the fixer ids that have findings, in dependency order.
func orderFixers(order []string, byFixer map[string][]Finding) []string {
	var out []string
	seen := make(map[string]bool)
	for _, id := range order {
		if _, ok := byFixer[id]; ok {
			out = append(out, id)
			seen[id] = true
		}
	}
	var rest []string
	for id := range byFixer {
		if !seen[id] {
			rest = append(rest, id)
		}
	}
	sort.Strings(rest)
	return append(out, rest...)
}

// renderReportMD renders the human-readable report.md narrative.
func renderReportMD(rep *Report) string {
	var b strings.Builder
	status := "healthy"
	if rep.ExitCode != ExitHealthy {
		status = fmt.Sprintf("findings present (exit %d)", rep.ExitCode)
	}
	fmt.Fprintf(&b, "# `ao doctor` — %s (run %s)\n\n", rep.StartedAt, rep.RunID)
	fmt.Fprintf(&b, "**Status:** %s\n", status)
	fmt.Fprintf(&b, "**Duration:** %d ms\n", rep.DurationMS)
	fmt.Fprintf(&b, "**Target SHA:** %s\n\n", rep.TargetSHA)
	fmt.Fprintf(&b, "## Summary\n\n%d findings (P0=%d P1=%d P2=%d P3=%d).\n\n",
		rep.Summary.TotalFindings,
		rep.Summary.BySeverity["P0"], rep.Summary.BySeverity["P1"],
		rep.Summary.BySeverity["P2"], rep.Summary.BySeverity["P3"])
	if len(rep.Findings) == 0 {
		b.WriteString("No findings.\n")
		return b.String()
	}
	b.WriteString("## Findings\n\n")
	for _, f := range rep.Findings {
		fmt.Fprintf(&b, "### %s — %s (%s)\n\n%s\n\n- Remediation: `%s`\n- Auto-fixable: %t\n\n",
			f.Severity, f.ID, f.Subsystem, f.Title, f.Remediation.Command, f.Remediation.AutoFixable)
	}
	return b.String()
}

// runsDir returns <repo>/.doctor/runs.
func runsDir(repoRoot string) string {
	return filepath.Join(repoRoot, ".doctor", "runs")
}

// resolveRunDir resolves a run-id (or "latest") to an absolute run directory.
func resolveRunDir(repoRoot, runID string) (string, error) {
	if runID == "latest" {
		link := filepath.Join(repoRoot, ".doctor", "latest")
		target, err := os.Readlink(link)
		if err != nil {
			return "", fmt.Errorf("doctor: no latest run: %w", err)
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(repoRoot, ".doctor", target)
		}
		return target, nil
	}
	// Accept either a bare 6-char id or a full <ISO>__<id> dir name.
	entries, err := os.ReadDir(runsDir(repoRoot))
	if err != nil {
		return "", fmt.Errorf("doctor: read runs dir: %w", err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if e.Name() == runID || strings.HasSuffix(e.Name(), "__"+runID) {
			return filepath.Join(runsDir(repoRoot), e.Name()), nil
		}
	}
	return "", fmt.Errorf("doctor: run %q not found", runID)
}

// UndoResult is the outcome of an Undo invocation.
type UndoResult struct {
	RunID       string `json:"run_id"`
	Restored    int    `json:"restored"`
	Skipped     int    `json:"skipped"`
	ExitCode    int    `json:"exit_code"`
	StrictError string `json:"strict_error,omitempty"`
}

// Undo reads a run's actions.jsonl in reverse and restores each mutated file
// from backups/. Under strict mode (default) it fails if a backup is missing
// or the restored hash does not match the recorded before_hash.
func Undo(repoRoot, runID string, strict, dryRun bool) (*UndoResult, error) {
	runDir, err := resolveRunDir(repoRoot, runID)
	if err != nil {
		return &UndoResult{RunID: runID, ExitCode: ExitFixFailed}, err
	}
	records, err := readActions(filepath.Join(runDir, "actions.jsonl"))
	if err != nil {
		return &UndoResult{RunID: runID, ExitCode: ExitFixFailed}, err
	}
	res := &UndoResult{RunID: filepath.Base(runDir), ExitCode: ExitHealthy}
	for i := len(records) - 1; i >= 0; i-- {
		rec := records[i]
		if err := undoOne(repoRoot, runDir, rec, strict, dryRun, res); err != nil {
			res.ExitCode = ExitFixFailed
			res.StrictError = err.Error()
			return res, err
		}
	}
	return res, nil
}

// undoOne reverses a single action record.
func undoOne(repoRoot, runDir string, rec ActionRecord, strict, dryRun bool, res *UndoResult) error {
	target := filepath.Join(repoRoot, rec.Path)
	if rec.Op == "Rename" && rec.RenameTo != "" {
		// Reverse the move: rename the quarantined file back.
		if dryRun {
			fmt.Fprintf(os.Stderr, "[dry-run] would restore (un-rename) %s\n", target)
			res.Skipped++
			return nil
		}
		if err := os.Rename(rec.RenameTo, target); err != nil {
			if strict {
				return fmt.Errorf("doctor: un-rename %s: %w", target, err)
			}
			res.Skipped++
			return nil
		}
		res.Restored++
		return nil
	}
	backup := filepath.Join(runDir, "backups", rec.Path)
	if _, err := os.Stat(backup); err != nil {
		if !rec.Existed {
			// The file did not exist before; undo leaves it (created files are
			// the user's to inspect; we never delete).
			res.Skipped++
			return nil
		}
		if strict {
			return fmt.Errorf("doctor: missing backup for %s", rec.Path)
		}
		res.Skipped++
		return nil
	}
	if dryRun {
		fmt.Fprintf(os.Stderr, "[dry-run] would restore %s from backup\n", target)
		res.Skipped++
		return nil
	}
	if err := copyVerbatim(backup, target); err != nil {
		return fmt.Errorf("doctor: restore %s: %w", target, err)
	}
	restored, err := os.ReadFile(target)
	if err != nil {
		return fmt.Errorf("doctor: read restored %s: %w", target, err)
	}
	if strict && sha256Hex(restored) != rec.BeforeHash {
		return fmt.Errorf("doctor: restored hash mismatch for %s", rec.Path)
	}
	res.Restored++
	return nil
}

// readActions reads and parses a run's actions.jsonl.
func readActions(path string) ([]ActionRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("doctor: open actions.jsonl: %w", err)
	}
	defer func() { _ = f.Close() }()
	var recs []ActionRecord
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 256*1024), 4*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var rec ActionRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			return nil, fmt.Errorf("doctor: parse actions.jsonl line: %w", err)
		}
		recs = append(recs, rec)
	}
	return recs, sc.Err()
}

// HealthResult is the structured result of a Health check.
type HealthResult struct {
	Status   string `json:"status"`
	Findings int    `json:"findings"`
	LastRun  string `json:"last_run"`
	RunID    string `json:"run_id"`
	ExitCode int    `json:"exit_code"`
}

// Health returns a cheap one-line liveness summary based on the latest run.
func Health(repoRoot, toolVersion string) (string, *HealthResult, error) {
	hr := &HealthResult{Status: "ok", LastRun: "none", ExitCode: ExitHealthy}
	runDir, err := resolveRunDir(repoRoot, "latest")
	if err != nil {
		line := fmt.Sprintf("ok  ao=%s doctor=%s findings=0 last_run=none", toolVersion, DoctorVersion)
		return line, hr, nil
	}
	data, err := os.ReadFile(filepath.Join(runDir, "report.json"))
	if err != nil {
		return fmt.Sprintf("io  ao=%s doctor=%s reason=report_unreadable", toolVersion, DoctorVersion),
			&HealthResult{Status: "io", ExitCode: ExitIOError}, nil
	}
	var rep Report
	if err := json.Unmarshal(data, &rep); err != nil {
		return fmt.Sprintf("io  ao=%s doctor=%s reason=report_corrupt", toolVersion, DoctorVersion),
			&HealthResult{Status: "io", ExitCode: ExitIOError}, nil
	}
	hr.Findings = rep.Summary.TotalFindings
	hr.LastRun = rep.StartedAt
	hr.RunID = rep.RunID
	if rep.Summary.TotalFindings > 0 {
		hr.Status = "findings"
		hr.ExitCode = ExitFindings
		line := fmt.Sprintf("findings  ao=%s doctor=%s findings=%d P0=%d P2=%d last_run=%s run_id=%s",
			toolVersion, DoctorVersion, rep.Summary.TotalFindings,
			rep.Summary.BySeverity["P0"], rep.Summary.BySeverity["P2"], rep.StartedAt, rep.RunID)
		return line, hr, nil
	}
	line := fmt.Sprintf("ok  ao=%s doctor=%s findings=0 last_run=%s run_id=%s",
		toolVersion, DoctorVersion, rep.StartedAt, rep.RunID)
	return line, hr, nil
}

// RunSummary is one entry in the `ls` listing.
type RunSummary struct {
	RunID       string `json:"run_id"`
	StartedAt   string `json:"started_at"`
	ExitCode    int    `json:"exit_code"`
	ActionCount int    `json:"action_count"`
}

// Ls lists the runs under .doctor/runs/.
func Ls(repoRoot string) ([]RunSummary, error) {
	entries, err := os.ReadDir(runsDir(repoRoot))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("doctor: read runs dir: %w", err)
	}
	var out []RunSummary
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(runsDir(repoRoot), e.Name())
		rs := RunSummary{RunID: e.Name()}
		if data, err := os.ReadFile(filepath.Join(dir, "report.json")); err == nil {
			var rep Report
			if json.Unmarshal(data, &rep) == nil {
				rs.StartedAt = rep.StartedAt
				rs.ExitCode = rep.ExitCode
			}
		}
		if recs, err := readActions(filepath.Join(dir, "actions.jsonl")); err == nil {
			rs.ActionCount = len(recs)
		}
		out = append(out, rs)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RunID < out[j].RunID })
	return out, nil
}

// Explain returns the finding with the given ID from the latest run, expanded.
func Explain(repoRoot, findingID string) (*Finding, error) {
	runDir, err := resolveRunDir(repoRoot, "latest")
	if err != nil {
		return nil, fmt.Errorf("doctor: no runs to explain from: %w", err)
	}
	data, err := os.ReadFile(filepath.Join(runDir, "report.json"))
	if err != nil {
		return nil, fmt.Errorf("doctor: read report.json: %w", err)
	}
	var rep Report
	if err := json.Unmarshal(data, &rep); err != nil {
		return nil, fmt.Errorf("doctor: parse report.json: %w", err)
	}
	for i := range rep.Findings {
		if rep.Findings[i].ID == findingID {
			return &rep.Findings[i], nil
		}
	}
	return nil, fmt.Errorf("doctor: finding %q not found in latest run", findingID)
}

// Diff computes what --fix would change (read-only). With the FOUNDATION wave's
// empty registry it always reports a clean diff.
func Diff(opts Options) (*Report, error) {
	opts.DryRun = true
	return Diagnose(opts)
}

// RobotTriageResult is the mega-command JSON for --robot-triage.
type RobotTriageResult struct {
	SchemaVersion      string        `json:"schema_version"`
	Summary            ReportSummary `json:"summary"`
	QuickRef           []string      `json:"quick_ref"`
	Findings           []Finding     `json:"findings"`
	RecommendedCommand string        `json:"recommended_command"`
	CapabilitiesURL    string        `json:"capabilities_url"`
	RobotDocsCommand   string        `json:"robot_docs_command"`
}

// RobotTriage runs a diagnose and returns the mega-command triage payload.
func RobotTriage(opts Options) (*RobotTriageResult, *Report, error) {
	rep, err := Diagnose(opts)
	if err != nil {
		return nil, rep, err
	}
	quick := []string{}
	for sev, n := range rep.Summary.BySeverity {
		if n > 0 {
			quick = append(quick, fmt.Sprintf("%s: %d finding(s)", sev, n))
		}
	}
	sort.Strings(quick)
	recommended := "ao doctor (healthy)"
	if len(rep.Findings) > 0 {
		recommended = "ao doctor --fix"
	}
	return &RobotTriageResult{
		SchemaVersion:      SchemaVersion,
		Summary:            rep.Summary,
		QuickRef:           quick,
		Findings:           rep.Findings,
		RecommendedCommand: recommended,
		CapabilitiesURL:    "ao doctor capabilities --json",
		RobotDocsCommand:   "ao doctor robot-docs",
	}, rep, nil
}

// GC prunes run directories whose started_at is before the cutoff. It refuses
// unless yes is true and cutoff is non-zero (never deletes silently).
func GC(repoRoot string, cutoff time.Time, yes bool) (int, error) {
	if !yes || cutoff.IsZero() {
		return 0, fmt.Errorf("doctor: gc requires --yes and --before <date>")
	}
	entries, err := os.ReadDir(runsDir(repoRoot))
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("doctor: read runs dir: %w", err)
	}
	pruned := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(runsDir(repoRoot), e.Name())
		started := runStartedAt(dir)
		if started.IsZero() || !started.Before(cutoff) {
			continue
		}
		if err := os.RemoveAll(dir); err != nil {
			return pruned, fmt.Errorf("doctor: prune %s: %w", e.Name(), err)
		}
		pruned++
	}
	return pruned, nil
}

// runStartedAt reads a run's started_at, falling back to dir mtime.
func runStartedAt(dir string) time.Time {
	if data, err := os.ReadFile(filepath.Join(dir, "report.json")); err == nil {
		var rep Report
		if json.Unmarshal(data, &rep) == nil {
			if t, err := time.Parse(time.RFC3339, rep.StartedAt); err == nil {
				return t
			}
		}
	}
	if info, err := os.Stat(dir); err == nil {
		return info.ModTime()
	}
	return time.Time{}
}
