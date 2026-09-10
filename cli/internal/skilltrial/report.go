// Package skilltrial reads explicitly selected Harbor job outputs for repository
// development. It does not launch trials, grade work, or write lifecycle state.
package skilltrial

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/parser"
)

type JobInput struct {
	Arm       string
	Directory string
}

type Evidence struct {
	Path   string          `json:"path"`
	SHA256 string          `json:"sha256"`
	Data   json.RawMessage `json:"data,omitempty"`
}

type Counts struct {
	Expected        *int `json:"expected"`
	Observed        int  `json:"observed"`
	Started         int  `json:"started"`
	Completed       int  `json:"completed"`
	NotStarted      *int `json:"not_started"`
	Created         int  `json:"created"`
	Unfinished      int  `json:"unfinished"`
	Success         int  `json:"success"`
	Failed          int  `json:"failed"`
	Infrastructure  int  `json:"infrastructure_error"`
	ExecutionErrors int  `json:"execution_error"`
	Unknown         int  `json:"unknown_outcome"`
}

type Timing struct {
	StartedAt      string   `json:"started_at"`
	FinishedAt     string   `json:"finished_at"`
	ElapsedSeconds *float64 `json:"elapsed_seconds"`
}

type Trial struct {
	Directory    string              `json:"directory"`
	ID           string              `json:"id"`
	Name         string              `json:"name"`
	TaskName     string              `json:"task_name"`
	TaskChecksum string              `json:"task_checksum"`
	Arm          string              `json:"arm"`
	Started      bool                `json:"started"`
	Completed    bool                `json:"completed"`
	Outcome      string              `json:"outcome"`
	Timing       Timing              `json:"timing"`
	Documents    map[string]Evidence `json:"documents"`
	SessionIDs   []string            `json:"session_ids"`
	Diagnostics  []string            `json:"diagnostics"`
}

type Job struct {
	Directory   string              `json:"directory"`
	ID          string              `json:"id"`
	Arm         string              `json:"arm"`
	Counts      Counts              `json:"counts"`
	Timing      Timing              `json:"timing"`
	Documents   map[string]Evidence `json:"documents"`
	Trials      []Trial             `json:"trials"`
	Diagnostics []string            `json:"diagnostics"`
}

// SessionCopy retains every supplied copy. No copy's cumulative usage is added
// to another, and no parent/child aggregate is inferred from native identity.
type SessionCopy struct {
	Evidence   Evidence                `json:"evidence"`
	Trial      string                  `json:"trial"`
	Accounting *parser.CodexAccounting `json:"accounting"`
}

type Session struct {
	ID          string        `json:"id"`
	Copies      []SessionCopy `json:"copies"`
	Diagnostics []string      `json:"diagnostics"`
}

type Report struct {
	RewardKey string    `json:"reward_key"`
	Jobs      []Job     `json:"jobs"`
	Sessions  []Session `json:"sessions"`
	Limits    []string  `json:"limits"`
}

type builder struct {
	reward   string
	sessions map[string]*Session
}

// Build produces deterministic JSON-ready data from the current source bytes.
// Unknown fields remain in raw native documents; missing facts remain nullable.
func Build(inputs []JobInput, sessionFiles []string, rewardKey string) (*Report, error) {
	if len(inputs) == 0 && len(sessionFiles) == 0 {
		return nil, fmt.Errorf("supply at least one job directory or native session file")
	}
	if rewardKey == "" {
		return nil, fmt.Errorf("reward key must not be empty")
	}
	b := builder{reward: rewardKey, sessions: map[string]*Session{}}
	report := &Report{RewardKey: rewardKey, Jobs: []Job{}, Sessions: []Session{}, Limits: []string{
		"Outcome uses only the selected native verifier reward: 1 success, 0 failed; other or missing values are unknown. This is endpoint grading, not semantic acceptance or comparison validity.",
		"Expected is native n_total_trials or lock.trials length. Started requires trial config, result timestamp, or native sessions; unstarted and unfinished attempts stay visible. Harbor may overwrite retries on resume; unavailable historical attempts cannot be reconstructed.",
		"Native cumulative usage is reported per identity and per evidence copy, never summed across copies, parents, children, or Harbor agent_result. Parent counters may include inherited history; billing and phase allocation are unknown.",
		"Harbor documents retain their native metrics as source data; they are not trusted as all-session totals. Worker-authored claims are not extracted as measurements.",
		"Timing uses recorded endpoints only; absent timezone remains unspecified. No current clock, file modification time, or missing usage is substituted. Setup and grader phase times remain in native documents.",
	}}
	seen := map[string]string{}
	for _, input := range inputs {
		dir, err := canonical(input.Directory)
		if err != nil {
			return nil, err
		}
		if arm, ok := seen[dir]; ok {
			if arm != input.Arm {
				return nil, fmt.Errorf("job %s supplied under conflicting arms", dir)
			}
			continue
		}
		seen[dir] = input.Arm
		job, err := b.readJob(dir, input.Arm)
		if err != nil {
			return nil, err
		}
		report.Jobs = append(report.Jobs, job)
	}
	for _, path := range sessionFiles {
		if _, err := b.addSession(path, ""); err != nil {
			return nil, err
		}
	}
	for _, session := range b.sessions {
		sort.Slice(session.Copies, func(i, j int) bool { return session.Copies[i].Evidence.Path < session.Copies[j].Evidence.Path })
		report.Sessions = append(report.Sessions, *session)
	}
	sort.Slice(report.Jobs, func(i, j int) bool { return report.Jobs[i].Directory < report.Jobs[j].Directory })
	sort.Slice(report.Sessions, func(i, j int) bool { return report.Sessions[i].ID < report.Sessions[j].ID })
	return report, nil
}

func canonical(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

func readEvidence(path string, document bool) (Evidence, []byte, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Evidence{}, nil, err
	}
	e := Evidence{Path: path, SHA256: fmt.Sprintf("%x", sha256.Sum256(content))}
	if document && json.Valid(content) {
		e.Data = content
	}
	return e, content, nil
}

func readDocuments(dir string) (map[string]Evidence, []string) {
	docs, diagnostics := map[string]Evidence{}, []string{}
	for _, name := range []string{"config.json", "lock.json", "result.json"} {
		path := filepath.Join(dir, name)
		e, raw, err := readEvidence(path, true)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			diagnostics = append(diagnostics, path+": "+err.Error())
			continue
		}
		docs[name] = e
		var object map[string]json.RawMessage
		if err := json.Unmarshal(raw, &object); err != nil || object == nil {
			diagnostics = append(diagnostics, path+": malformed native JSON object; raw evidence retained by path and hash")
		}
	}
	return docs, diagnostics
}

func decodeDocument(docs map[string]Evidence, name string, target any, diagnostics *[]string) bool {
	doc, ok := docs[name]
	if !ok || len(doc.Data) == 0 || bytes.Equal(bytes.TrimSpace(doc.Data), []byte("null")) {
		return false
	}
	if err := json.Unmarshal(doc.Data, target); err != nil {
		*diagnostics = append(*diagnostics, doc.Path+": unsupported native fields: "+err.Error())
		return false
	}
	return true
}

func (b *builder) readJob(dir, arm string) (Job, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return Job{}, err
	}
	j := Job{Directory: dir, Arm: arm, Trials: []Trial{}}
	j.Documents, j.Diagnostics = readDocuments(dir)
	var result struct {
		ID         string `json:"id"`
		Expected   *int   `json:"n_total_trials"`
		StartedAt  string `json:"started_at"`
		FinishedAt string `json:"finished_at"`
	}
	if decodeDocument(j.Documents, "result.json", &result, &j.Diagnostics) {
		j.ID, j.Counts.Expected = result.ID, result.Expected
		j.Timing = timing(result.StartedAt, result.FinishedAt, &j.Diagnostics)
	}
	var lock struct {
		Trials []json.RawMessage `json:"trials"`
	}
	if decodeDocument(j.Documents, "lock.json", &lock, &j.Diagnostics) && lock.Trials != nil {
		expected := len(lock.Trials)
		if j.Counts.Expected == nil {
			j.Counts.Expected = &expected
		} else if *j.Counts.Expected != expected {
			j.Diagnostics = append(j.Diagnostics, "native result and lock disagree on expected trials")
		}
	}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		trialDir := filepath.Join(dir, entry.Name())
		if !looksLikeTrial(trialDir) {
			j.Diagnostics = append(j.Diagnostics, "unrecognized directory: "+trialDir)
			continue
		}
		trial := b.readTrial(trialDir, arm)
		j.Trials = append(j.Trials, trial)
		j.Counts.count(trial)
	}
	j.Counts.finish(&j.Diagnostics)
	if j.Counts.Expected == nil {
		j.Diagnostics = append(j.Diagnostics, "expected trial count unknown: no native total or resolved lock")
	}
	return j, nil
}

func looksLikeTrial(dir string) bool {
	for _, name := range []string{"config.json", "result.json", "lock.json", "trial.log", "agent", "steps", "exception.txt"} {
		if _, err := os.Lstat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}
	return false
}

func timing(start, finish string, diagnostics *[]string) Timing {
	t := Timing{StartedAt: start, FinishedAt: finish}
	if start == "" || finish == "" {
		return t
	}
	for _, format := range []string{time.RFC3339Nano, "2006-01-02T15:04:05.999999999"} {
		s, e1 := time.Parse(format, start)
		f, e2 := time.Parse(format, finish)
		if e1 == nil && e2 == nil && !f.Before(s) {
			duration := f.Sub(s).Seconds()
			t.ElapsedSeconds = &duration
			return t
		}
	}
	*diagnostics = append(*diagnostics, "invalid, reversed, or incompatible timing endpoints")
	return t
}

func (c *Counts) count(t Trial) {
	c.Observed++
	if t.Started {
		c.Started++
	}
	if t.Completed {
		c.Completed++
	}
	switch t.Outcome {
	case "created":
		c.Created++
	case "unfinished":
		c.Unfinished++
	case "success":
		c.Success++
	case "failed":
		c.Failed++
	case "infrastructure_error":
		c.Infrastructure++
	case "execution_error":
		c.ExecutionErrors++
	default:
		c.Unknown++
	}
}

func (c *Counts) finish(diagnostics *[]string) {
	if c.Expected == nil {
		return
	}
	if *c.Expected < 0 {
		*diagnostics = append(*diagnostics, "invalid negative native expected count")
		c.Expected = nil
		return
	}
	if c.Started > *c.Expected {
		*diagnostics = append(*diagnostics, "started exceeds expected; retries or conflicting evidence require review")
		return
	}
	n := *c.Expected - c.Started
	c.NotStarted = &n
}
