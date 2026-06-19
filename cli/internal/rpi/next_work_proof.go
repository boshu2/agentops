package rpi

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// next_work_proof.go — next-work completion-proof classification, migrated out
// of cmd/ao (rpi_loop.go) so the keeper context/codex commands stay
// self-contained after the ao rpi command surface is removed (ADR-0009
// teardown, age-3pdt / age-uco1 layer 3). Builds on the run-discovery and
// search-root helpers migrated in layers 1-2. Logic is preserved from the
// cmd/ao originals.

// ClassifyNextWorkCompletionProof decides whether a queued next-work item has
// already been completed, by correlating its proof_ref (or, failing that, the
// goal/epic) against the run registry and recorded evidence-only closures.
func ClassifyNextWorkCompletionProof(cwd string, sourceEpic string, item NextWorkItem) NextWorkProofDecision {
	if item.ProofRef != nil {
		switch item.ProofRef.Kind {
		case "completed_run":
			if run := findCompletedRunByID(cwd, item.ProofRef.RunID); run != nil {
				return NextWorkProofDecision{Complete: true, Source: "completed_run", Detail: run.RunID}
			}
		case "execution_packet":
			// Prefer proof_ref.path: if the artifact file exists and is a
			// non-empty JSON object, that is sufficient proof regardless of
			// whether we can correlate a run ID in the registry.
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
					return NextWorkProofDecision{Complete: true, Source: "execution_packet", Detail: detail}
				}
			}
			// Fall back to run-registry lookup when no path is set or the
			// file does not yet exist.
			if run := findCompletedRunByID(cwd, item.ProofRef.RunID); run != nil {
				return NextWorkProofDecision{Complete: true, Source: "execution_packet", Detail: run.RunID}
			}
		case "evidence_only_closure":
			if proof := findEvidenceOnlyClosureProofByTarget(cwd, item.ProofRef.TargetID); proof != nil {
				return NextWorkProofDecision{
					Complete: true,
					Source:   "evidence_only_closure",
					Detail:   fmt.Sprintf("%s (%s)", proof.TargetID, proof.PacketPath),
				}
			}
		}
	}

	if run := findCompletedRunForQueueSelection(cwd, &QueueSelection{
		Item:       item,
		SourceEpic: sourceEpic,
	}); run != nil {
		return NextWorkProofDecision{Complete: true, Source: "completed_run", Detail: run.RunID}
	}
	if proof := findEvidenceOnlyClosureProofForQueueSelection(cwd, &QueueSelection{
		Item:       item,
		SourceEpic: sourceEpic,
	}); proof != nil {
		return NextWorkProofDecision{
			Complete: true,
			Source:   "evidence_only_closure",
			Detail:   fmt.Sprintf("%s (%s)", proof.TargetID, proof.PacketPath),
		}
	}

	return NextWorkProofDecision{}
}

func findCompletedRunByID(cwd, runID string) *RPIRunInfo {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil
	}

	_, historical := DiscoverRunsRegistryFirst(cwd)
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

func findCompletedRunForQueueSelection(cwd string, sel *QueueSelection) *RPIRunInfo {
	if sel == nil {
		return nil
	}

	goal := strings.TrimSpace(sel.Item.Title)
	sourceEpic := strings.TrimSpace(sel.SourceEpic)
	if goal == "" || sourceEpic == "" {
		return nil
	}

	_, historical := DiscoverRunsRegistryFirst(cwd)
	var best *RPIRunInfo
	for i := range historical {
		run := &historical[i]
		if run.Status != "completed" {
			continue
		}
		if strings.TrimSpace(run.Goal) != goal {
			continue
		}
		runEpic := strings.TrimSpace(run.EpicID)
		runID := strings.TrimSpace(run.RunID)
		if sourceEpic != runEpic && sourceEpic != runID {
			continue
		}
		if best == nil || run.StartedAt > best.StartedAt {
			best = run
		}
	}
	return best
}

func findEvidenceOnlyClosureProofForQueueSelection(cwd string, sel *QueueSelection) *EvidenceOnlyClosureProof {
	if sel == nil {
		return nil
	}

	for _, targetID := range QueueProofTargetIDs(sel) {
		if proof := findEvidenceOnlyClosureProofByTarget(cwd, targetID); proof != nil {
			return proof
		}
	}
	return nil
}

func findEvidenceOnlyClosureProofByTarget(cwd, targetID string) *EvidenceOnlyClosureProof {
	if packetPath, ok := findValidEvidenceOnlyClosurePacket(cwd, targetID); ok {
		return &EvidenceOnlyClosureProof{
			TargetID:   targetID,
			PacketPath: packetPath,
		}
	}
	return nil
}

func findValidEvidenceOnlyClosurePacket(cwd, targetID string) (string, bool) {
	if strings.TrimSpace(targetID) == "" {
		return "", false
	}

	safeTargetID := strings.ReplaceAll(strings.TrimSpace(targetID), "/", "_")
	roots := CollectSearchRoots(cwd)
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

// executionPacketPathIsValid returns true when the given path resolves to a
// readable, non-empty JSON object that carries an "objective" or "run_id"
// field; the minimum proof that a real execution packet was written there. It
// intentionally avoids full schema validation so it stays tolerant of minor
// version drift.
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

func packetIsValidForTarget(packetPath, targetID string) bool {
	data, err := os.ReadFile(packetPath)
	if err != nil {
		return false
	}

	var packet EvidenceOnlyClosurePacket
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
