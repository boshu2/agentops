package main

import (
	"bufio"
	"cmp"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	rpilib "github.com/boshu2/agentops/cli/internal/rpi"
)

const worktreeTimeout = 30 * time.Second

// lookFn is the function signature for file content lookup.
type lookFn func(file string) (string, error)

// getCurrentBranch returns the current git branch for a repo root.
func getCurrentBranch(repoRoot string) (string, error) {
	return rpilib.GetCurrentBranch(repoRoot, worktreeTimeout)
}

// repoAffinityRank delegates to internal/rpi.
func repoAffinityRank(item nextWorkItem, repoFilter string) int {
	return rpilib.RepoAffinityRank(item, repoFilter)
}

// severityRank delegates to internal/rpi.
func severityRank(s string) int { return rpilib.SeverityRank(s) }

// freshnessRank delegates to internal/rpi.
func freshnessRank(item nextWorkItem) int { return rpilib.FreshnessRank(item) }

// uniqueStringsPreserveOrder delegates to internal/rpi.
func uniqueStringsPreserveOrder(items []string) []string {
	return rpilib.UniqueStringsPreserveOrder(items)
}

// compiledChecklistSummary reads a markdown file and extracts a single-line
// summary for use in context compilation.
func compiledChecklistSummary(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	id := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return rpilib.CompiledChecklistSummaryFromContent(id, string(data))
}

// compiledSummariesForFindings gathers checklist summaries for a set of finding IDs.
func compiledSummariesForFindings(cwd, subdir string, findingIDs []string) []string {
	summaries := make([]string, 0, len(findingIDs))
	for _, id := range uniqueStringsPreserveOrder(findingIDs) {
		path := filepath.Join(cwd, ".agents", subdir, id+".md")
		if summary := compiledChecklistSummary(path); summary != "" {
			summaries = append(summaries, summary)
		}
	}
	return uniqueStringsPreserveOrder(summaries)
}

// countCronHistoryRows counts non-empty lines in a JSONL history file.
func countCronHistoryRows(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	n := 0
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) != "" {
			n++
		}
	}
	return n
}

type (
	evidenceOnlyClosureProof  = rpilib.EvidenceOnlyClosureProof
	evidenceOnlyClosurePacket = rpilib.EvidenceOnlyClosurePacket
	queueSelection            = rpilib.QueueSelection
)

func classifyNextWorkCompletionProof(cwd string, sourceEpic string, item nextWorkItem) nextWorkProofDecision {
	if item.ProofRef != nil {
		switch item.ProofRef.Kind {
		case "completed_run":
			if run := findCompletedRunByID(cwd, item.ProofRef.RunID); run != nil {
				return nextWorkProofDecision{Complete: true, Source: "completed_run", Detail: run.RunID}
			}
		case "execution_packet":
			if packetPath := strings.TrimSpace(item.ProofRef.Path); packetPath != "" {
				absPath := packetPath
				if !filepath.IsAbs(absPath) {
					absPath = filepath.Join(cwd, absPath)
				}
				if executionPacketPathIsValid(absPath) {
					detail := packetPath
					if item.ProofRef.RunID != "" {
						detail = item.ProofRef.RunID
					}
					return nextWorkProofDecision{Complete: true, Source: "execution_packet", Detail: detail}
				}
			}
			if run := findCompletedRunByID(cwd, item.ProofRef.RunID); run != nil {
				return nextWorkProofDecision{Complete: true, Source: "execution_packet", Detail: run.RunID}
			}
		case "evidence_only_closure":
			if proof := findEvidenceOnlyClosureProofByTarget(cwd, item.ProofRef.TargetID); proof != nil {
				return nextWorkProofDecision{
					Complete: true,
					Source:   "evidence_only_closure",
					Detail:   fmt.Sprintf("%s (%s)", proof.TargetID, proof.PacketPath),
				}
			}
		}
	}

	if run := findCompletedRunForQueueSelection(cwd, &queueSelection{
		Item:       item,
		SourceEpic: sourceEpic,
	}); run != nil {
		return nextWorkProofDecision{Complete: true, Source: "completed_run", Detail: run.RunID}
	}
	if proof := findEvidenceOnlyClosureProofForQueueSelection(cwd, &queueSelection{
		Item:       item,
		SourceEpic: sourceEpic,
	}); proof != nil {
		return nextWorkProofDecision{
			Complete: true,
			Source:   "evidence_only_closure",
			Detail:   fmt.Sprintf("%s (%s)", proof.TargetID, proof.PacketPath),
		}
	}

	return nextWorkProofDecision{}
}

func findCompletedRunByID(cwd, runID string) *rpiRunInfo {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil
	}
	_, historical := discoverRPIRunsRegistryFirst(cwd)
	for i := range historical {
		run := &historical[i]
		if run.Status != "completed" {
			continue
		}
		if strings.TrimSpace(run.RunID) == runID {
			return run
		}
	}
	return nil
}

func findCompletedRunForQueueSelection(cwd string, sel *queueSelection) *rpiRunInfo {
	if sel == nil {
		return nil
	}
	goal := strings.TrimSpace(sel.Item.Title)
	sourceEpic := strings.TrimSpace(sel.SourceEpic)
	if goal == "" || sourceEpic == "" {
		return nil
	}
	_, historical := discoverRPIRunsRegistryFirst(cwd)
	var best *rpiRunInfo
	for i := range historical {
		run := &historical[i]
		if run.Status != "completed" {
			continue
		}
		if strings.TrimSpace(run.Goal) != goal {
			continue
		}
		runEpic := strings.TrimSpace(run.EpicID)
		runIDTrimmed := strings.TrimSpace(run.RunID)
		if sourceEpic != runEpic && sourceEpic != runIDTrimmed {
			continue
		}
		if best == nil || run.StartedAt > best.StartedAt {
			best = run
		}
	}
	return best
}

func findEvidenceOnlyClosureProofByTarget(cwd, targetID string) *evidenceOnlyClosureProof {
	if packetPath, ok := findValidEvidenceOnlyClosurePacket(cwd, targetID); ok {
		return &evidenceOnlyClosureProof{
			TargetID:   targetID,
			PacketPath: packetPath,
		}
	}
	return nil
}

func findEvidenceOnlyClosureProofForQueueSelection(cwd string, sel *queueSelection) *evidenceOnlyClosureProof {
	if sel == nil {
		return nil
	}
	for _, targetID := range rpilib.QueueProofTargetIDs(sel) {
		if proof := findEvidenceOnlyClosureProofByTarget(cwd, targetID); proof != nil {
			return proof
		}
	}
	return nil
}

func findValidEvidenceOnlyClosurePacket(cwd, targetID string) (string, bool) {
	if strings.TrimSpace(targetID) == "" {
		return "", false
	}
	safeTargetID := strings.ReplaceAll(strings.TrimSpace(targetID), "/", "_")
	roots := collectSearchRoots(cwd)
	for _, root := range roots {
		for _, relPath := range []string{
			filepath.Join(".agents", "releases", "evidence-only-closures", safeTargetID+".json"),
			filepath.Join(".agents", "council", "evidence-only-closures", safeTargetID+".json"),
		} {
			packetPath := filepath.Join(root, relPath)
			if packetIsValidForTarget(packetPath, targetID) {
				return packetPath, true
			}
		}
	}
	return "", false
}

func packetIsValidForTarget(packetPath, targetID string) bool {
	data, err := os.ReadFile(packetPath)
	if err != nil {
		return false
	}
	var packet evidenceOnlyClosurePacket
	if err := json.Unmarshal(data, &packet); err != nil {
		return false
	}
	if strings.TrimSpace(packet.TargetID) != strings.TrimSpace(targetID) {
		return false
	}
	switch packet.EvidenceMode {
	case "commit", "staged", "worktree":
	default:
		return false
	}
	return len(packet.Evidence.Artifacts) > 0
}

func executionPacketPathIsValid(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var packet struct {
		Objective string `json:"objective"`
		RunID     string `json:"run_id"`
	}
	if err := json.Unmarshal(data, &packet); err != nil {
		return false
	}
	return strings.TrimSpace(packet.Objective) != "" || strings.TrimSpace(packet.RunID) != ""
}

func discoverRPIRunsRegistryFirst(cwd string) (active, historical []rpiRunInfo) {
	roots := collectSearchRoots(cwd)
	seen := make(map[string]struct{})
	for _, root := range roots {
		runs := scanRegistryRunsForProof(root)
		for _, r := range runs {
			if _, ok := seen[r.RunID]; ok {
				continue
			}
			seen[r.RunID] = struct{}{}
			historical = append(historical, r)
		}
	}
	return nil, historical
}

func scanRegistryRunsForProof(root string) []rpiRunInfo {
	runsDir := filepath.Join(root, ".agents", "rpi", "runs")
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		return nil
	}
	runs := make([]rpiRunInfo, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		runID := entry.Name()
		statePath := filepath.Join(runsDir, runID, phasedStateFile)
		data, err := os.ReadFile(statePath)
		if err != nil {
			continue
		}
		var state struct {
			RunID          string `json:"run_id"`
			Goal           string `json:"goal"`
			EpicID         string `json:"epic_id"`
			Phase          int    `json:"phase"`
			SchemaVersion  int    `json:"schema_version"`
			StartedAt      string `json:"started_at"`
			WorktreePath   string `json:"worktree_path"`
			TerminalStatus string `json:"terminal_status"`
		}
		if json.Unmarshal(data, &state) != nil || state.RunID == "" {
			continue
		}
		worktreeExists := true
		if state.WorktreePath != "" {
			if _, err := os.Stat(state.WorktreePath); err != nil {
				worktreeExists = false
			}
		}
		status := rpilib.ClassifyRunStatus(state.TerminalStatus, false, state.Phase, state.SchemaVersion, worktreeExists)
		runs = append(runs, rpiRunInfo{
			RunID:     state.RunID,
			Goal:      state.Goal,
			EpicID:    state.EpicID,
			StartedAt: state.StartedAt,
			Worktree:  state.WorktreePath,
			Status:    status,
			IsActive:  false,
		})
	}
	return runs
}

// workTypeRank delegates to internal/rpi.
func workTypeRank(item nextWorkItem) int { return rpilib.WorkTypeRank(item) }

func effectiveBDCommand(command string) string {
	return cmp.Or(strings.TrimSpace(command), "bd")
}

// splitRuntimeCommand delegates to internal/rpi.
func splitRuntimeCommand(command string) (string, []string) {
	return rpilib.SplitRuntimeCommand(command)
}

func defaultLookPath(fn lookFn) lookFn {
	if fn != nil {
		return fn
	}
	return exec.LookPath
}

func cleanEnvNoClaude() []string {
	var env []string
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "CLAUDECODE=") || strings.HasPrefix(e, "CLAUDE_CODE_") {
			continue
		}
		env = append(env, e)
	}
	return env
}

