// practices: [agile-manifesto, adr]
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	daemonpkg "github.com/boshu2/agentops/cli/internal/daemon"
	plansPkg "github.com/boshu2/agentops/cli/internal/plans"
	"github.com/boshu2/agentops/cli/internal/types"
)

const (
	// ManifestFileName is the name of the plan manifest file.
	ManifestFileName = plansPkg.ManifestFileName

	// PlansDir is the subdirectory under .agents for plan manifests.
	PlansDir = plansPkg.PlansDir
)

var (
	planProjectPath string
	planBeadsID     string
	planStatus      string
	planName        string
	// plansSyncViaDaemon (atom-3 / soc-fmgx) routes `ao plans sync` through the
	// daemon's plans.projection job-type instead of the legacy direct shellout
	// to bd. Default false in atom-3; flipped to true in atom-5 (soc-4sal)
	// after the foundation §9 release-N+1 soak window passes.
	plansSyncViaDaemon bool
)

var plansCmd = &cobra.Command{
	Use:   "plans",
	Short: "Manage plan manifest for robust plan discovery",
	Long: `Plans manages the plan manifest at .agents/plans/manifest.jsonl.

This command group provides robust plan discovery, fixing:
  - G2: Fragile discovery of ~/.claude/plans/ files
  - G4: Transcript parsing issues
  - G5: Hardcoded path assumptions

The manifest tracks all plans with metadata for filtering and traceability.`,
}

var plansRegisterCmd = &cobra.Command{
	Use:   "register <plan-path>",
	Short: "Register a plan in the manifest",
	Long: `Register adds a plan to the manifest.jsonl for discovery.

Called automatically when Claude exits plan mode, or manually for existing plans.

Examples:
  ao plans register ~/.claude/plans/peaceful-stirring-tome.md
  ao plans register ~/.claude/plans/my-plan.md --beads-id ol-a46.2
  ao plans register ./docs/plan.md --project /path/to/project`,
	Args: cobra.ExactArgs(1),
	RunE: runPlansRegister,
}

var plansListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all registered plans",
	Long: `List shows all plans in the manifest.

Use --project to filter by project path.
Use --status to filter by plan status.`,
	RunE: runPlansList,
}

var plansSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search plans by name or project",
	Args:  cobra.ExactArgs(1),
	RunE:  runPlansSearch,
}

var plansUpdateCmd = &cobra.Command{
	Use:   "update <plan-path>",
	Short: "Update a plan's status or metadata",
	Args:  cobra.ExactArgs(1),
	RunE:  runPlansUpdate,
}

var plansSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync manifest with beads (beads is source of truth)",
	Long: `Sync pulls plan metadata from beads to prevent drift.

F6: Beads is the source of truth. The manifest syncs FROM beads:
  1. Find all epics with linked plans in beads
  2. Update manifest status to match beads status
  3. Add missing plans that beads references
  4. Report drift (manifest entries without beads linkage)

This ensures manifest and beads stay consistent.`,
	RunE: runPlansSync,
}

var plansDiffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Show drift between manifest and beads",
	Long: `Diff compares the plan manifest against beads reality.

F6: Shows discrepancies without modifying anything:
  - Status mismatches (manifest says active, beads says closed)
  - Orphaned plans (in manifest but no beads ID)
  - Missing plans (beads epic references plan not in manifest)

Use 'ao plans sync' to fix the drift.`,
	RunE: runPlansDiff,
}

func init() {
	plansCmd.GroupID = "config"
	rootCmd.AddCommand(plansCmd)
	plansCmd.AddCommand(plansRegisterCmd)
	plansCmd.AddCommand(plansListCmd)
	plansCmd.AddCommand(plansSearchCmd)
	plansCmd.AddCommand(plansUpdateCmd)
	plansCmd.AddCommand(plansSyncCmd)
	plansCmd.AddCommand(plansDiffCmd)

	// Register flags
	plansRegisterCmd.Flags().StringVar(&planProjectPath, "project", "", "Project path this plan applies to")
	plansRegisterCmd.Flags().StringVar(&planBeadsID, "beads-id", "", "Beads issue/epic ID this plan implements")
	plansRegisterCmd.Flags().StringVar(&planName, "name", "", "Human-readable plan name")

	// List flags
	plansListCmd.Flags().StringVar(&planProjectPath, "project", "", "Filter by project path")
	plansListCmd.Flags().StringVar(&planStatus, "status", "", "Filter by status (active, completed, abandoned, superseded)")
	_ = plansListCmd.RegisterFlagCompletionFunc("status", staticCompletionFunc("active", "completed", "abandoned", "superseded"))

	// Update flags
	plansUpdateCmd.Flags().StringVar(&planStatus, "status", "", "New status for the plan")
	plansUpdateCmd.Flags().StringVar(&planBeadsID, "beads-id", "", "Update beads ID")
	_ = plansUpdateCmd.RegisterFlagCompletionFunc("status", staticCompletionFunc("active", "completed", "abandoned", "superseded"))

	// Sync flags (atom-3 / soc-fmgx).
	plansSyncCmd.Flags().BoolVar(&plansSyncViaDaemon, "via-daemon", false,
		"Route plans sync through the agentopsd plans.projection job-type. "+
			"Default false in this release; flipped to true in a later release after the soak window. "+
			"With --via-daemon=false the command takes the legacy direct path (file-locked).")
}

// computePlanChecksum returns first 8 bytes of SHA256 as hex
func computePlanChecksum(path string) (string, error) { return plansPkg.ComputePlanChecksum(path) }

// createPlanEntry builds a manifest entry from path and metadata
func createPlanEntry(absPath string, modTime time.Time, projectPath, name, beadsID, checksum string) types.PlanManifestEntry {
	return plansPkg.CreatePlanEntry(absPath, modTime, projectPath, name, beadsID, checksum)
}

// appendManifestEntry appends an entry to the manifest file
func appendManifestEntry(manifestPath string, entry types.PlanManifestEntry) error {
	return plansPkg.AppendManifestEntry(manifestPath, entry)
}

// resolveProjectPath returns the explicit project path or detects it from the plan file.
func resolveProjectPath(explicit, planPath string) string {
	if explicit != "" {
		return explicit
	}
	return detectProjectPath(planPath)
}

// resolvePlanName returns the explicit name or derives one from the file path.
func resolvePlanName(explicit, planPath string) string {
	return plansPkg.ResolvePlanName(explicit, planPath)
}

// upsertManifestEntry updates an existing entry or appends a new one.
// Returns true if an existing entry was updated.
func upsertManifestEntry(manifestPath string, existing []types.PlanManifestEntry, entry types.PlanManifestEntry) (bool, error) {
	return plansPkg.UpsertEntry(manifestPath, existing, entry)
}

// printRegistrationSummary prints details after a new plan registration.
func printRegistrationSummary(entry types.PlanManifestEntry) {
	fmt.Printf("✓ Registered plan: %s\n", entry.PlanName)
	if entry.BeadsID != "" {
		fmt.Printf("  Beads ID: %s\n", entry.BeadsID)
	}
	if entry.ProjectPath != "" {
		fmt.Printf("  Project: %s\n", entry.ProjectPath)
	}
}

// loadOrCreateManifest returns the manifest path and its current entries,
// creating the directory if needed.
func loadOrCreateManifest() (string, []types.PlanManifestEntry, error) {
	manifestPath, err := getManifestPath()
	if err != nil {
		return "", nil, fmt.Errorf("get manifest path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0750); err != nil {
		return "", nil, fmt.Errorf("create manifest dir: %w", err)
	}
	existing, err := loadManifest(manifestPath)
	if err != nil && !os.IsNotExist(err) {
		return "", nil, fmt.Errorf("load manifest: %w", err)
	}
	return manifestPath, existing, nil
}

// buildRegisterEntry validates the plan path, computes checksum, and builds the entry.
func buildRegisterEntry(planPath, projectFlag, nameFlag, beadsID string) (types.PlanManifestEntry, error) {
	info, err := os.Stat(planPath)
	if err != nil {
		return types.PlanManifestEntry{}, fmt.Errorf("plan not found: %w", err)
	}
	checksum, err := computePlanChecksum(planPath)
	if err != nil {
		return types.PlanManifestEntry{}, fmt.Errorf("checksum: %w", err)
	}
	return createPlanEntry(
		planPath, info.ModTime(),
		resolveProjectPath(projectFlag, planPath),
		resolvePlanName(nameFlag, planPath),
		beadsID, checksum,
	), nil
}

func runPlansRegister(cmd *cobra.Command, args []string) error {
	absPath, err := filepath.Abs(args[0])
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}

	if GetDryRun() {
		if _, statErr := os.Stat(absPath); statErr != nil {
			return fmt.Errorf("plan not found: %w", statErr)
		}
		fmt.Printf("[dry-run] Would register plan: %s\n", absPath)
		return nil
	}

	entry, err := buildRegisterEntry(absPath, planProjectPath, planName, planBeadsID)
	if err != nil {
		return err
	}

	manifestPath, existing, err := loadOrCreateManifest()
	if err != nil {
		return err
	}

	updated, err := upsertManifestEntry(manifestPath, existing, entry)
	if err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	if updated {
		fmt.Printf("✓ Updated plan in manifest: %s\n", absPath)
		return nil
	}

	printRegistrationSummary(entry)
	return nil
}

// planStatusSymbols maps plan status to a display symbol; unknown statuses fall through to string form.
var planStatusSymbols = map[types.PlanStatus]string{
	types.PlanStatusActive:    "○",
	types.PlanStatusCompleted: "✓",
}

// filterPlans returns entries matching the project and status filters.
func filterPlans(entries []types.PlanManifestEntry, project, status string) []types.PlanManifestEntry {
	return plansPkg.FilterPlans(entries, project, status)
}

// printPlanEntry prints a single plan entry with optional verbose detail.
func printPlanEntry(e types.PlanManifestEntry, verbose bool) {
	sym, ok := planStatusSymbols[e.Status]
	if !ok {
		sym = string(e.Status)
	}
	fmt.Printf("%s %s", sym, e.PlanName)
	if e.BeadsID != "" {
		fmt.Printf(" [%s]", e.BeadsID)
	}
	fmt.Println()

	if verbose {
		fmt.Printf("    Path: %s\n", e.Path)
		fmt.Printf("    Project: %s\n", e.ProjectPath)
		fmt.Printf("    Created: %s\n", e.CreatedAt.Format("2006-01-02"))
	}
}

func runPlansList(cmd *cobra.Command, args []string) error {
	manifestPath, err := getManifestPath()
	if err != nil {
		return fmt.Errorf("get manifest path: %w", err)
	}

	entries, err := loadManifest(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No plans registered. Use 'ao plans register <path>' to add plans.")
			return nil
		}
		return fmt.Errorf("load manifest: %w", err)
	}

	filtered := filterPlans(entries, planProjectPath, planStatus)
	if len(filtered) == 0 {
		fmt.Println("No plans match the filter criteria.")
		return nil
	}

	verbose := GetVerbose()
	for _, e := range filtered {
		printPlanEntry(e, verbose)
	}

	return nil
}

func runPlansSearch(cmd *cobra.Command, args []string) error {
	query := args[0]

	manifestPath, err := getManifestPath()
	if err != nil {
		return fmt.Errorf("get manifest path: %w", err)
	}

	entries, err := loadManifest(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No plans registered.")
			return nil
		}
		return fmt.Errorf("load manifest: %w", err)
	}

	matches := plansPkg.SearchPlans(entries, query)

	if len(matches) == 0 {
		fmt.Printf("No plans matching '%s'\n", query)
		return nil
	}

	fmt.Printf("Found %d plan(s) matching '%s':\n\n", len(matches), query)
	for _, e := range matches {
		fmt.Printf("  %s\n", e.PlanName)
		fmt.Printf("    Path: %s\n", e.Path)
		if e.BeadsID != "" {
			fmt.Printf("    Beads: %s\n", e.BeadsID)
		}
	}

	return nil
}

// applyPlanUpdates applies status and beadsID updates to the manifest entry matching absPath.
func applyPlanUpdates(entries []types.PlanManifestEntry, absPath, status, beadsID string) bool {
	return plansPkg.ApplyPlanUpdates(entries, absPath, status, beadsID)
}

func runPlansUpdate(cmd *cobra.Command, args []string) error {
	absPath, err := filepath.Abs(args[0])
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}

	if GetDryRun() {
		fmt.Printf("[dry-run] Would update plan: %s\n", absPath)
		return nil
	}

	manifestPath, err := getManifestPath()
	if err != nil {
		return fmt.Errorf("get manifest path: %w", err)
	}

	entries, err := loadManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("load manifest: %w", err)
	}

	if !applyPlanUpdates(entries, absPath, planStatus, planBeadsID) {
		return fmt.Errorf("plan not found in manifest: %s", absPath)
	}

	if err := saveManifest(manifestPath, entries); err != nil {
		return fmt.Errorf("save manifest: %w", err)
	}

	fmt.Printf("✓ Updated plan: %s\n", absPath)
	return nil
}

// getManifestPath returns the path to the manifest file.
func getManifestPath() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Look for .agents directory
	agentsDir := filepath.Join(cwd, ".agents")
	if _, err := os.Stat(agentsDir); os.IsNotExist(err) {
		// Try to find rig root
		agentsDir = findAgentsDir(cwd)
		if agentsDir == "" {
			// Default to cwd
			agentsDir = filepath.Join(cwd, ".agents")
		}
	}

	return filepath.Join(agentsDir, PlansDir, ManifestFileName), nil
}

// findAgentsDir looks for .agents directory walking up to rig root.
func findAgentsDir(startDir string) string { return plansPkg.FindAgentsDir(startDir) }

// loadManifest reads all entries from the manifest file.
func loadManifest(path string) ([]types.PlanManifestEntry, error) {
	return plansPkg.LoadManifest(path)
}

// saveManifest writes all entries to the manifest file.
func saveManifest(path string, entries []types.PlanManifestEntry) error {
	return plansPkg.SaveManifest(path, entries)
}

// detectProjectPath attempts to find the project path for a plan file.
func detectProjectPath(planPath string) string { return plansPkg.DetectProjectPath(planPath) }

// buildBeadsIDIndex creates a map of beadsID -> slice index
func buildBeadsIDIndex(entries []types.PlanManifestEntry) map[string]int {
	return plansPkg.BuildBeadsIDIndex(entries)
}

// syncEpicStatus syncs a single epic status and returns true if changed
func syncEpicStatus(entries []types.PlanManifestEntry, idx int, beadsStatus string) bool {
	return plansPkg.SyncEpicStatus(entries, idx, beadsStatus)
}

// countUnlinkedEntries counts entries without beads linkage
func countUnlinkedEntries(entries []types.PlanManifestEntry) int {
	count, names := plansPkg.CountUnlinkedEntries(entries)
	for _, name := range names {
		VerbosePrintf("Drift: %s has no beads linkage\n", name)
	}
	return count
}

// syncEpicsToManifest syncs beads epic statuses into the manifest entries.
// Returns the count of entries that were updated.
func syncEpicsToManifest(entries []types.PlanManifestEntry, epics []beadsEpic, byBeadsID map[string]int) int {
	synced := 0
	for _, epic := range epics {
		if idx, ok := byBeadsID[epic.ID]; ok {
			if syncEpicStatus(entries, idx, epic.Status) {
				synced++
				VerbosePrintf("Synced %s: -> %s\n", epic.ID, entries[idx].Status)
			}
		}
	}
	return synced
}

// printSyncSummary prints the result of a plans sync operation.
func printSyncSummary(synced, drift int) {
	fmt.Printf("✓ Sync complete: %d synced, %d drift\n", synced, drift)
	if drift > 0 {
		fmt.Printf("  Hint: Run 'ao plans list' to see entries without beads linkage\n")
	}
}

// runPlansSync syncs manifest with beads (F6: beads is source of truth).
//
// 3-case fallback decision (atom-3 / soc-fmgx; G3 dual-write safety):
//
//	case A: --via-daemon=false (default in atom-3) → legacy direct path,
//	        guarded by a manifest.lock LOCK_EX file lock so concurrent
//	        writers can't race against each other or against the daemon
//	        executor during the dual-write window.
//	case B: --via-daemon=true AND daemon-up → POST /v1/jobs with a
//	        plans.projection spec, await terminal event via /v1/events,
//	        then re-load the snapshot the daemon wrote.
//	case C: --via-daemon=true AND daemon-down → fall back to the legacy
//	        direct path with file lock and emit a warning. This is the
//	        "stop the bleeding" knob noted in the atom-5 rollback section.
//
// atom-5 (soc-4sal) flips the default to true and removes the legacy
// direct path entirely; the file lock retires with it.
func runPlansSync(cmd *cobra.Command, args []string) error {
	if GetDryRun() {
		fmt.Println("[dry-run] Would sync manifest with beads")
		return nil
	}

	if plansSyncViaDaemon {
		err := plansSyncViaDaemonPath(cmd)
		if err == nil {
			return nil
		}
		// case C: daemon path failed. Surface a warning and fall through
		// to the direct path. The daemon-unreachable error is the only
		// fall-through case; payload-level failures already emit their
		// own diagnostic via plansSyncViaDaemonPath.
		if !errors.Is(err, errDaemonUnavailable) {
			return err
		}
		VerbosePrintf("Warning: daemon unreachable (%v); falling back to direct path\n", err)
		fmt.Println("Daemon unreachable; using direct path with file lock.")
	}

	return plansSyncDirectPath()
}

// plansSyncDirectPath is the legacy "shell out to bd, append to manifest"
// implementation, wrapped in a manifest.lock LOCK_EX file lock so concurrent
// writers (e.g., a parallel `ao plans sync` invocation, or the daemon
// executor during the dual-write window) cannot collide. atom-5 deletes
// this function and the lock helper together.
func plansSyncDirectPath() error {
	manifestPath, err := getManifestPath()
	if err != nil {
		return fmt.Errorf("get manifest path: %w", err)
	}

	unlock, err := acquireManifestLock(manifestPath)
	if err != nil {
		return fmt.Errorf("acquire manifest lock: %w", err)
	}
	defer unlock()

	entries, err := loadManifest(manifestPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("load manifest: %w", err)
	}

	beadsOutput, err := queryBeadsEpics()
	if err != nil {
		VerbosePrintf("Warning: could not query beads: %v\n", err)
		fmt.Println("Beads not available. Checking manifest for drift...")
	}

	synced := syncEpicsToManifest(entries, beadsOutput, buildBeadsIDIndex(entries))
	drift := countUnlinkedEntries(entries)

	if synced > 0 {
		if err := saveManifest(manifestPath, entries); err != nil {
			return fmt.Errorf("save manifest: %w", err)
		}
	}

	printSyncSummary(synced, drift)
	return nil
}

// errDaemonUnavailable is the sentinel returned by plansSyncViaDaemonPath
// when the daemon is not reachable. runPlansSync uses errors.Is to decide
// whether to fall through to the direct path or surface the error to the
// caller.
var errDaemonUnavailable = errors.New("agentopsd daemon is not reachable")

// plansSyncViaDaemonPath submits a plans.projection job to the agentopsd
// daemon and waits for it to terminate, then prints the manifest snapshot
// path. Returns errDaemonUnavailable when the daemon health probe fails;
// other errors (job submission, terminal failure) bubble up as-is.
func plansSyncViaDaemonPath(cmd *cobra.Command) error {
	cwd, err := resolveProjectDir()
	if err != nil {
		return fmt.Errorf("resolve project dir: %w", err)
	}
	baseURL, err := resolveDaemonURL(cwd, daemonURL)
	if err != nil {
		return fmt.Errorf("resolve daemon URL: %w", err)
	}
	ctx, cancel := context.WithTimeout(cobraContext(cmd), 60*time.Second)
	defer cancel()

	if _, err := fetchDaemonStatus(ctx, baseURL); err != nil {
		return fmt.Errorf("%w: %v", errDaemonUnavailable, err)
	}

	projectID, issuePrefix, manifestDir := plansDaemonRequestParams()
	spec := daemonpkg.NewPlansProjectionJobSpec(projectID, issuePrefix, manifestDir)

	specRaw, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("marshal plans.projection spec: %w", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(specRaw, &payload); err != nil {
		return fmt.Errorf("unmarshal plans.projection spec: %w", err)
	}

	token, err := resolveAgentOpsDaemonClientMutationToken(cwd, daemonToken, daemonTokenFile)
	if err != nil {
		return fmt.Errorf("resolve daemon mutation token: %w", err)
	}

	request := daemonpkg.SubmitJobRequest{
		JobType:        daemonpkg.JobTypePlansProjection,
		IdempotencyKey: spec.IdempotencyKey(),
		Payload:        payload,
	}
	var response daemonpkg.SubmitJobResponse
	if err := postDaemonJSON(ctx, baseURL+"/v1/jobs", token, request, &response); err != nil {
		return fmt.Errorf("submit plans.projection job: %w", err)
	}
	if !response.Accepted {
		return fmt.Errorf("daemon rejected plans.projection job (status=%s, degraded=%v)",
			response.Status, response.DegradedReasons)
	}

	job, err := waitForDaemonJobStatus(ctx, baseURL, response.JobID, 30*time.Second)
	if err != nil {
		return fmt.Errorf("await plans.projection job %s: %w", response.JobID, err)
	}
	if job.Status != daemonpkg.JobStatusCompleted {
		return fmt.Errorf("plans.projection job %s terminated with status=%s", response.JobID, job.Status)
	}
	manifestPath := job.Artifacts["manifest_jsonl"]
	manifestCount := job.Artifacts["manifest_count"]
	fmt.Fprintf(cmd.OutOrStdout(),
		"Daemon synced plans.projection: job=%s manifest=%s entries=%s\n",
		response.JobID, manifestPath, manifestCount)
	return nil
}

// plansDaemonRequestParams returns the (project_id, issue_prefix, output_dir)
// triple used by plansSyncViaDaemonPath. The defaults match the shared
// bushido bd workspace; tests override via the package-level seam below.
func plansDaemonRequestParams() (projectID, issuePrefix, outputDir string) {
	if plansDaemonParamsFn != nil {
		return plansDaemonParamsFn()
	}
	manifestPath, _ := getManifestPath()
	dir := filepath.Dir(manifestPath)
	return "default", "", dir
}

// plansDaemonParamsFn is a test seam — set in unit tests to inject
// deterministic project/prefix/output values without touching .beads/.
var plansDaemonParamsFn func() (string, string, string)

// acquireManifestLock opens (creating if needed) <manifest>.lock and
// acquires an exclusive flock. Returns an unlock function that releases the
// lock and closes the lock file. The lock is intentionally a separate file
// (not the manifest itself) so partial-write recovery on the manifest
// doesn't break the lock contract. atom-5 deletes this helper together
// with the legacy direct path.
func acquireManifestLock(manifestPath string) (func(), error) {
	lockPath := manifestPath + ".lock"
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return nil, fmt.Errorf("manifest.lock mkdir: %w", err)
	}
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("manifest.lock open %s: %w", lockPath, err)
	}
	if err := flockLock(f); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("manifest.lock flock %s: %w", lockPath, err)
	}
	unlock := func() {
		_ = flockUnlock(f)
		_ = f.Close()
	}
	return unlock, nil
}

// beadsEpic represents a beads epic for sync.
type beadsEpic = plansPkg.BeadsEpic

// queryBeadsEpics queries beads for epic statuses.
func queryBeadsEpics() ([]beadsEpic, error) {
	cmd := exec.Command("bd", "list", "--type", "epic", "--json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("bd list: %w", err)
	}
	return plansPkg.ParseBeadsEpicJSONL(output), nil
}

// driftEntry represents a single drift detection
type driftEntry struct {
	Type     string
	PlanName string
	BeadsID  string
	Manifest string
	Beads    string
}

// buildBeadsStatusIndex creates a map of epic ID -> status from beads
func buildBeadsStatusIndex(epics []beadsEpic) map[string]string {
	return plansPkg.BuildBeadsStatusIndex(epics)
}

// detectStatusDrifts finds status mismatches between manifest and beads
func detectStatusDrifts(byBeadsID map[string]*types.PlanManifestEntry, beadsIndex map[string]string) []driftEntry {
	raw := plansPkg.DetectStatusDrifts(byBeadsID, beadsIndex)
	return convertDriftEntries(raw)
}

// detectOrphanedEntries finds manifest entries without beads linkage
func detectOrphanedEntries(entries []types.PlanManifestEntry) []driftEntry {
	raw := plansPkg.DetectOrphanedEntries(entries)
	return convertDriftEntries(raw)
}

// convertDriftEntries converts internal drift entries to the local type.
func convertDriftEntries(raw []plansPkg.DriftEntry) []driftEntry {
	out := make([]driftEntry, len(raw))
	for i, r := range raw {
		out[i] = driftEntry{Type: r.Type, PlanName: r.PlanName, BeadsID: r.BeadsID, Manifest: r.Manifest, Beads: r.Beads}
	}
	return out
}

// printDrifts outputs drift entries in a formatted way
func printDrifts(drifts []driftEntry) {
	fmt.Printf("Found %d drift(s):\n\n", len(drifts))
	for _, d := range drifts {
		switch d.Type {
		case "status_mismatch":
			fmt.Printf("  ⚠ Status mismatch: %s [%s]\n", d.PlanName, d.BeadsID)
			fmt.Printf("    Manifest: %s, Beads: %s\n", d.Manifest, d.Beads)
		case "orphaned":
			fmt.Printf("  ○ Orphaned plan: %s\n", d.PlanName)
			fmt.Printf("    No beads ID linked\n")
		case "missing_beads":
			fmt.Printf("  ✗ Missing in beads: %s [%s]\n", d.PlanName, d.BeadsID)
			fmt.Printf("    Epic not found in beads\n")
		}
	}
}

// runPlansDiff shows drift between manifest and beads (F6).
func runPlansDiff(cmd *cobra.Command, args []string) error {
	manifestPath, err := getManifestPath()
	if err != nil {
		return fmt.Errorf("get manifest path: %w", err)
	}

	entries, err := loadManifest(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No manifest found. Run 'ao plans register' to create one.")
			return nil
		}
		return fmt.Errorf("load manifest: %w", err)
	}

	// Build manifest index by beads ID
	byBeadsID := make(map[string]*types.PlanManifestEntry)
	for i := range entries {
		if entries[i].BeadsID != "" {
			byBeadsID[entries[i].BeadsID] = &entries[i]
		}
	}

	beadsOutput, err := queryBeadsEpics()
	if err != nil {
		return fmt.Errorf("query beads: %w", err)
	}

	beadsIndex := buildBeadsStatusIndex(beadsOutput)

	// Collect all drifts
	drifts := detectStatusDrifts(byBeadsID, beadsIndex)
	drifts = append(drifts, detectOrphanedEntries(entries)...)

	if len(drifts) == 0 {
		fmt.Println("✓ No drift detected. Manifest and beads are in sync.")
		return nil
	}

	printDrifts(drifts)

	fmt.Printf("\nRun 'ao plans sync' to fix status mismatches.\n")
	fmt.Printf("Run 'ao plans update <path> --beads-id <id>' to link orphaned plans.\n")

	return nil
}
