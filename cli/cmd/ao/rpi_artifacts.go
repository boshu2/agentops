package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/rpi"
)

const (
	phaseEvaluatorFileFmt    = "phase-%d-evaluator.json"
	artifactPreviewByteLimit = 64 * 1024
)

type rpiArtifactRef = rpi.ArtifactRef

type rpiArtifactContent = rpi.ArtifactContent

func pathClean(rel string) string {
	return rpi.PathClean(rel)
}

func isSafeArtifactRelPath(rel string) bool {
	return rpi.IsSafeArtifactRelPath(rel)
}

func collectRunArtifacts(root, runID string) []rpiArtifactRef {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil
	}

	refs := make(map[string]rpiArtifactRef)
	add := func(rel string) {
		rel = pathClean(rel)
		if !isSafeArtifactRelPath(rel) {
			return
		}
		full := filepath.Join(root, filepath.FromSlash(rel))
		info, err := os.Stat(full)
		if err != nil || info.IsDir() {
			return
		}
		kind, label, phase := classifyRPIArtifact(rel)
		refs[rel] = rpiArtifactRef{
			Path:      rel,
			Label:     label,
			Kind:      kind,
			Phase:     phase,
			UpdatedAt: info.ModTime().UTC().Format(time.RFC3339),
			SizeBytes: info.Size(),
		}
	}

	add(filepath.Join(".agents", "rpi", "execution-packet.json"))
	add(filepath.Join(".agents", "rpi", phasedStateFile))
	for i := 1; i <= 3; i++ {
		add(filepath.Join(".agents", "rpi", fmt.Sprintf(phaseResultFileFmt, i)))
		add(filepath.Join(".agents", "rpi", fmt.Sprintf("phase-%d-handoff.json", i)))
		add(filepath.Join(".agents", "rpi", fmt.Sprintf(phaseEvaluatorFileFmt, i)))

		summaryPattern := filepath.Join(root, ".agents", "rpi", fmt.Sprintf("phase-%d-summary*.md", i))
		if matches, err := filepath.Glob(summaryPattern); err == nil {
			for _, match := range matches {
				if rel, err := filepath.Rel(root, match); err == nil {
					add(rel)
				}
			}
		}
	}

	if runID = strings.TrimSpace(runID); runID != "" {
		add(filepath.Join(".agents", "rpi", "runs", runID, phasedStateFile))
		add(filepath.Join(".agents", "rpi", "runs", runID, rpiC2EventsFileName))
		add(filepath.Join(".agents", "rpi", "runs", runID, "heartbeat.txt"))
	}

	for _, rel := range executionPacketReferencedPaths(root) {
		add(rel)
	}

	out := make([]rpiArtifactRef, 0, len(refs))
	for _, ref := range refs {
		out = append(out, ref)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].UpdatedAt != out[j].UpdatedAt {
			return out[i].UpdatedAt > out[j].UpdatedAt
		}
		return out[i].Path < out[j].Path
	})
	return out
}

func executionPacketReferencedPaths(root string) []string {
	path := filepath.Join(root, ".agents", "rpi", "execution-packet.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var packet map[string]any
	if err := json.Unmarshal(data, &packet); err != nil {
		return nil
	}

	var refs []string
	for key, raw := range packet {
		switch v := raw.(type) {
		case string:
			if strings.HasSuffix(key, "_path") {
				refs = append(refs, v)
			}
		case []any:
			if key == "proof_artifacts" {
				for _, item := range v {
					if s, ok := item.(string); ok {
						refs = append(refs, s)
					}
				}
			}
		case map[string]any:
			if key == "evaluator_artifacts" {
				for _, item := range v {
					if s, ok := item.(string); ok {
						refs = append(refs, s)
					}
				}
			}
		}
	}
	return uniqueStringsPreserveOrder(refs)
}

func classifyRPIArtifact(rel string) (kind, label string, phase int) {
	return rpi.ClassifyRPIArtifact(rel, phasedStateFile, rpiC2EventsFileName)
}

func artifactPhaseNumber(name string) int {
	return rpi.ArtifactPhaseNumber(name)
}

func artifactContentType(rel string) string {
	return rpi.ArtifactContentType(rel)
}

func readRunArtifactContent(root string, ref rpiArtifactRef, limit int64) (*rpiArtifactContent, error) {
	if limit <= 0 {
		limit = artifactPreviewByteLimit
	}

	full := filepath.Join(root, filepath.FromSlash(ref.Path))
	f, err := os.Open(full)
	if err != nil {
		return nil, fmt.Errorf("open artifact %s: %w", ref.Path, err)
	}
	defer func() { _ = f.Close() }()

	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, fmt.Errorf("read artifact %s: %w", ref.Path, err)
	}
	truncated := int64(len(data)) > limit
	if truncated {
		data = data[:limit]
	}

	return &rpiArtifactContent{
		Path:        ref.Path,
		Label:       ref.Label,
		Kind:        ref.Kind,
		ContentType: artifactContentType(ref.Path),
		UpdatedAt:   ref.UpdatedAt,
		SizeBytes:   ref.SizeBytes,
		Body:        string(data),
		Truncated:   truncated,
	}, nil
}

func latestRelativeArtifact(cwd string, patterns ...string) string {
	match := latestMatchingFile(cwd, patterns...)
	if match == "" {
		return ""
	}
	rel, err := filepath.Rel(cwd, match)
	if err != nil {
		return ""
	}
	return pathClean(rel)
}

func updatePhaseResultArtifacts(cwd string, state *phasedState, phaseNum int, extras map[string]string) error {
	resultPath := filepath.Join(cwd, ".agents", "rpi", fmt.Sprintf(phaseResultFileFmt, phaseNum))
	data, err := os.ReadFile(resultPath)
	if err != nil {
		return fmt.Errorf("read phase result: %w", err)
	}

	var result phaseResult
	if err := json.Unmarshal(data, &result); err != nil {
		return fmt.Errorf("parse phase result: %w", err)
	}
	if result.Artifacts == nil {
		result.Artifacts = make(map[string]string)
	}

	if rel := latestRelativeArtifact(cwd, filepath.Join(".agents", "rpi", fmt.Sprintf("phase-%d-summary*.md", phaseNum))); rel != "" {
		result.Artifacts["summary"] = rel
	}
	if rel := latestRelativeArtifact(cwd, filepath.Join(".agents", "rpi", fmt.Sprintf("phase-%d-handoff.json", phaseNum))); rel != "" {
		result.Artifacts["handoff"] = rel
	}
	if rel := latestRelativeArtifact(cwd, filepath.Join(".agents", "rpi", fmt.Sprintf(phaseEvaluatorFileFmt, phaseNum))); rel != "" {
		result.Artifacts["evaluator"] = rel
	}
	if rel := latestRelativeArtifact(cwd, filepath.Join(".agents", "rpi", "execution-packet.json")); rel != "" {
		result.Artifacts["execution_packet"] = rel
	}
	if rel := latestRelativeArtifact(cwd, filepath.Join(".agents", "rpi", phasedStateFile)); rel != "" {
		result.Artifacts["phased_state"] = rel
	}
	if state != nil && strings.TrimSpace(state.RunID) != "" {
		runID := strings.TrimSpace(state.RunID)
		if rel := latestRelativeArtifact(cwd, filepath.Join(".agents", "rpi", "runs", runID, phasedStateFile)); rel != "" {
			result.Artifacts["registry_state"] = rel
		}
		if rel := latestRelativeArtifact(cwd, filepath.Join(".agents", "rpi", "runs", runID, rpiC2EventsFileName)); rel != "" {
			result.Artifacts["events"] = rel
		}
	}

	for key, rel := range extras {
		rel = pathClean(rel)
		if !isSafeArtifactRelPath(rel) {
			continue
		}
		full := filepath.Join(cwd, filepath.FromSlash(rel))
		if info, err := os.Stat(full); err == nil && !info.IsDir() {
			result.Artifacts[key] = rel
			_ = info
		}
	}
	if state != nil {
		result.Verdicts = state.Verdicts
	}
	return writePhaseResult(cwd, &result)
}

func updateExecutionPacketProof(cwd string, state *phasedState) error {
	packet, err := loadExecutionPacketForProof(cwd, state)
	if err != nil {
		return err
	}

	applyExecutionPacketProofState(packet, state)
	runID := executionPacketProofRunID(packet, state)
	attachExecutionPacketProofArtifacts(packet, collectRunArtifacts(cwd, runID))

	updated, err := json.MarshalIndent(packet, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal execution packet proof: %w", err)
	}
	updated = append(updated, '\n')
	if err := writeExecutionPacketData(cwd, state, runID, updated); err != nil {
		return fmt.Errorf("write execution packet proof: %w", err)
	}
	return nil
}

func loadExecutionPacketForProof(cwd string, state *phasedState) (map[string]any, error) {
	data, err := readExecutionPacketForProof(cwd, state)
	if err != nil {
		return nil, err
	}
	packet := make(map[string]any)
	if err := json.Unmarshal(data, &packet); err != nil {
		return nil, fmt.Errorf("parse execution packet: %w", err)
	}
	return packet, nil
}

func readExecutionPacketForProof(cwd string, state *phasedState) ([]byte, error) {
	packetPath := filepath.Join(cwd, ".agents", "rpi", executionPacketFile)
	data, err := os.ReadFile(packetPath)
	if err == nil {
		return data, nil
	}
	if state == nil {
		return nil, fmt.Errorf("read execution packet: %w", err)
	}
	if err := writeExecutionPacketSeed(cwd, state); err != nil {
		return nil, err
	}
	data, err = os.ReadFile(packetPath)
	if err != nil {
		return nil, fmt.Errorf("read execution packet after seed write: %w", err)
	}
	return data, nil
}

func applyExecutionPacketProofState(packet map[string]any, state *phasedState) {
	if _, ok := packet["schema_version"]; !ok {
		packet["schema_version"] = 1
	}
	if state == nil {
		return
	}
	if strings.TrimSpace(state.Goal) != "" && packet["objective"] == nil {
		packet["objective"] = state.Goal
	}
	if strings.TrimSpace(state.EpicID) != "" {
		packet["epic_id"] = state.EpicID
	}
	if strings.TrimSpace(state.RunID) != "" {
		packet["run_id"] = state.RunID
	}
	if strings.TrimSpace(state.TrackerMode) != "" {
		packet["tracker_mode"] = state.TrackerMode
	}
	if strings.TrimSpace(string(state.Complexity)) != "" {
		packet["complexity"] = string(state.Complexity)
	}
}

func executionPacketProofRunID(packet map[string]any, state *phasedState) string {
	if state != nil {
		return state.RunID
	}
	if raw, ok := packet["run_id"].(string); ok {
		return raw
	}
	return ""
}

func attachExecutionPacketProofArtifacts(packet map[string]any, artifacts []rpiArtifactRef) {
	if len(artifacts) == 0 {
		return
	}
	paths, evaluatorArtifacts := executionPacketProofArtifactMaps(artifacts)
	packet["proof_artifacts"] = paths
	if len(evaluatorArtifacts) > 0 {
		packet["evaluator_artifacts"] = evaluatorArtifacts
	}
	packet["proof_updated_at"] = time.Now().UTC().Format(time.RFC3339)
}

func executionPacketProofArtifactMaps(artifacts []rpiArtifactRef) ([]string, map[string]string) {
	paths := make([]string, 0, len(artifacts))
	evaluatorArtifacts := make(map[string]string)
	for _, ref := range artifacts {
		paths = append(paths, ref.Path)
		if ref.Kind == "phase_evaluator" && ref.Phase > 0 {
			evaluatorArtifacts[fmt.Sprintf("phase_%d", ref.Phase)] = ref.Path
		}
	}
	return paths, evaluatorArtifacts
}
