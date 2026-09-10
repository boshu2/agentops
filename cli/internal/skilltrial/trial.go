package skilltrial

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/boshu2/agentops/cli/internal/parser"
)

type nativeTrial struct {
	ID               string          `json:"id"`
	Name             string          `json:"trial_name"`
	TaskName         string          `json:"task_name"`
	TaskChecksum     string          `json:"task_checksum"`
	StartedAt        string          `json:"started_at"`
	FinishedAt       string          `json:"finished_at"`
	EnvironmentSetup *nativePhase    `json:"environment_setup"`
	AgentSetup       *nativePhase    `json:"agent_setup"`
	AgentExecution   *nativePhase    `json:"agent_execution"`
	StepResults      json.RawMessage `json:"step_results"`
	Exception        *struct {
		Type string `json:"exception_type"`
	} `json:"exception_info"`
	Verifier *struct {
		Rewards map[string]*float64 `json:"rewards"`
	} `json:"verifier_result"`
}

type nativePhase struct {
	StartedAt string `json:"started_at"`
}

func (b *builder) readTrial(dir, arm string) Trial {
	t := Trial{Directory: dir, Arm: arm, Name: filepath.Base(dir), Outcome: "created", SessionIDs: []string{}}
	t.Documents, t.Diagnostics = readDocuments(dir)
	_, t.Started = t.Documents["config.json"]
	var result nativeTrial
	if decodeDocument(t.Documents, "result.json", &result, &t.Diagnostics) {
		t.ID, t.TaskName, t.TaskChecksum = result.ID, result.TaskName, result.TaskChecksum
		if result.Name != "" {
			t.Name = result.Name
		}
		t.Started = t.Started || result.StartedAt != ""
		t.Timing = timing(result.StartedAt, result.FinishedAt, &t.Diagnostics)
		t.Completed = t.Timing.ElapsedSeconds != nil
		if t.Completed {
			t.Outcome = classifyOutcome(result, b.reward)
		}
	}
	if t.TaskName == "" {
		var config struct {
			Task struct {
				Path string `json:"path"`
				Name string `json:"name"`
			} `json:"task"`
		}
		if decodeDocument(t.Documents, "config.json", &config, &t.Diagnostics) {
			t.TaskName = config.Task.Name
			if t.TaskName == "" && config.Task.Path != "" {
				t.TaskName = filepath.Base(config.Task.Path)
			}
		}
	}
	for _, root := range sessionRoots(dir, &t.Diagnostics) {
		b.scanSessions(root, &t)
	}
	if t.Completed && result.Exception != nil && result.Exception.Type == "NonZeroAgentExitCodeError" && b.rootProviderRejected(t) {
		t.Outcome = "infrastructure_error"
		t.Diagnostics = append(t.Diagnostics, "nonzero agent exit accompanied by native root-session provider rejection; see session provider_error and latest_turn_payload")
	}
	if len(t.SessionIDs) > 0 {
		t.Started = true
	}
	if !t.Completed && t.Started {
		t.Outcome = "unfinished"
	}
	if _, ok := t.Documents["result.json"]; !ok {
		t.Diagnostics = append(t.Diagnostics, "native trial result missing; terminal outcome is unknown")
	}
	sort.Strings(t.SessionIDs)
	return t
}

func (b *builder) rootProviderRejected(trial Trial) bool {
	for _, id := range trial.SessionIDs {
		for _, copy := range b.sessions[id].Copies {
			a := copy.Accounting
			if copy.Trial == trial.Directory && a.ID != "" && a.ParentID == "" && a.ProviderError != nil {
				return true
			}
		}
	}
	return false
}

func classifyOutcome(result nativeTrial, rewardKey string) string {
	if result.Exception != nil {
		// A completed setup exception before any agent execution is an
		// infrastructure failure even when the exception class is RuntimeError.
		noSteps := len(result.StepResults) == 0 || string(result.StepResults) == "null" || string(result.StepResults) == "[]"
		if result.AgentExecution == nil && noSteps && (result.EnvironmentSetup != nil || result.AgentSetup != nil) {
			return "infrastructure_error"
		}
		switch result.Exception.Type {
		case "AgentAuthenticationError", "ModelNotFoundError", "ApiUsageLimitError", "EnvironmentStartError", "EnvironmentBuildError",
			"RewardFileNotFoundError", "RewardFileEmptyError", "VerifierOutputParseError", "VerifierTimeoutError":
			return "infrastructure_error"
		default:
			return "execution_error"
		}
	}
	if result.Verifier == nil || result.Verifier.Rewards[rewardKey] == nil {
		return "unknown_outcome"
	}
	switch *result.Verifier.Rewards[rewardKey] {
	case 1:
		return "success"
	case 0:
		return "failed"
	default:
		return "unknown_outcome"
	}
}

func sessionRoots(dir string, diagnostics *[]string) []string {
	roots := []string{filepath.Join(dir, "agent")}
	steps, err := os.ReadDir(filepath.Join(dir, "steps"))
	if err != nil && !os.IsNotExist(err) {
		*diagnostics = append(*diagnostics, "read step directories: "+err.Error())
	}
	for _, step := range steps {
		if step.IsDir() {
			roots = append(roots, filepath.Join(dir, "steps", step.Name(), "agent"))
		}
	}
	return roots
}

func (b *builder) scanSessions(root string, trial *Trial) {
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			trial.Diagnostics = append(trial.Diagnostics, "native session symlink not followed: "+path)
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		// Search only the captured native sessions tree. Worker artifacts and
		// transcripts elsewhere are not accounting evidence for this adapter.
		if !strings.Contains(string(filepath.Separator)+rel, string(filepath.Separator)+"sessions"+string(filepath.Separator)) {
			return nil
		}
		id, err := b.addSession(path, trial.Directory)
		if err != nil {
			trial.Diagnostics = append(trial.Diagnostics, path+": "+err.Error())
			return nil
		}
		for _, existing := range trial.SessionIDs {
			if existing == id {
				return nil
			}
		}
		trial.SessionIDs = append(trial.SessionIDs, id)
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		trial.Diagnostics = append(trial.Diagnostics, "scan native sessions: "+err.Error())
	}
}

func (b *builder) addSession(path, trial string) (string, error) {
	path, err := canonical(path)
	if err != nil {
		return "", err
	}
	e, content, err := readEvidence(path, false)
	if err != nil {
		return "", err
	}
	accounting, parseErr := parser.ParseCodexAccounting(strings.NewReader(string(content)))
	if parseErr != nil {
		accounting.Diagnostics = append(accounting.Diagnostics, parser.AccountingDiagnostic{Line: accounting.Lines + 1, Message: parseErr.Error()})
	}
	id := accounting.ID
	if id == "" {
		id = "unknown:" + path
	}
	session := b.sessions[id]
	if session == nil {
		session = &Session{ID: id, Copies: []SessionCopy{}, Diagnostics: []string{}}
		b.sessions[id] = session
	}
	for _, copy := range session.Copies {
		if copy.Evidence.Path == path {
			if copy.Evidence.SHA256 != e.SHA256 {
				return "", fmt.Errorf("native session changed during report read")
			}
			return id, nil
		}
		if copy.Accounting.ParentID != accounting.ParentID {
			session.Diagnostics = append(session.Diagnostics, "conflicting parent identity between native copies")
		}
	}
	session.Copies = append(session.Copies, SessionCopy{Evidence: e, Trial: trial, Accounting: accounting})
	return id, nil
}
