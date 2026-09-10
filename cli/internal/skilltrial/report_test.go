package skilltrial

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func put(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestReportIncludesFailedAndInterruptedAttempts(t *testing.T) {
	dir := t.TempDir()
	put(t, filepath.Join(dir, "result.json"), `{"id":"job","n_total_trials":8,"started_at":"2026-09-10T10:00:00","finished_at":null,"future_job_field":17,"stats":{"n_completed_trials":99}}`)
	base := `"started_at":"2026-09-10T14:00:00Z","finished_at":"2026-09-10T14:00:05Z","task_name":"repair","task_checksum":"frozen-task","agent_result":{"n_input_tokens":999999},`
	cases := map[string]string{
		"success":   base + `"id":"s","verifier_result":{"rewards":{"reward":1}}`,
		"failed":    base + `"id":"f","verifier_result":{"rewards":{"reward":0}}`,
		"infra":     base + `"id":"i","exception_info":{"exception_type":"RuntimeError"},"environment_setup":{"started_at":"2026-09-10T14:00:00Z"},"agent_execution":null,"verifier_result":null`,
		"execution": base + `"id":"e","exception_info":{"exception_type":"AgentTimeoutError"},"agent_execution":{"started_at":"2026-09-10T14:00:01Z"},"verifier_result":null`,
		"unknown":   base + `"id":"u","verifier_result":null`,
	}
	for name, content := range cases {
		put(t, filepath.Join(dir, name, "result.json"), "{"+content+"}")
	}
	put(t, filepath.Join(dir, "interrupted", "config.json"), `{"task":{"path":"/tasks/repair"},"agent":{"model_name":"native-model"},"future_config":"retained"}`)
	put(t, filepath.Join(dir, "created", "trial.log"), "")
	put(t, filepath.Join(dir, "irrelevant", "notes.txt"), "not a trial")
	r, err := Build([]JobInput{{Arm: "treatment", Directory: dir}}, nil, "reward")
	if err != nil {
		t.Fatal(err)
	}
	c := r.Jobs[0].Counts
	if c.Expected == nil || *c.Expected != 8 || c.Observed != 7 || c.Started != 6 || c.Completed != 5 || c.NotStarted == nil || *c.NotStarted != 2 || c.Success != 1 || c.Failed != 1 || c.Infrastructure != 1 || c.ExecutionErrors != 1 || c.Unknown != 1 || c.Unfinished != 1 || c.Created != 1 {
		t.Fatalf("counts: %+v", c)
	}
	for _, trial := range r.Jobs[0].Trials {
		if trial.Arm != "treatment" {
			t.Fatal("arm missing")
		}
		if trial.Completed && (trial.Timing.ElapsedSeconds == nil || *trial.Timing.ElapsedSeconds != 5) {
			t.Fatalf("timing: %+v", trial.Timing)
		}
	}
	if !strings.Contains(string(r.Jobs[0].Documents["result.json"].Data), "future_job_field") {
		t.Fatal("unknown field lost")
	}
	if len(r.Sessions) != 0 {
		t.Fatal("fabricated native usage")
	}
}

func TestReportNativeIdentityAndStableReplay(t *testing.T) {
	dir := t.TempDir()
	put(t, filepath.Join(dir, "lock.json"), `{"trials":[{},{}],"harbor":{"version":"0.22.0"}}`)
	put(t, filepath.Join(dir, "one", "config.json"), `{"task":{"path":"/task/one"}}`)
	put(t, filepath.Join(dir, "two", "config.json"), `{"task":{"path":"/task/two"}}`)
	rollout := `{"type":"session_meta","payload":{"id":"author","source":"cli"}}
{"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":100,"cached_input_tokens":10,"output_tokens":5}}}}
{"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":100,"cached_input_tokens":10,"output_tokens":5}}}}
`
	p1 := filepath.Join(dir, "one", "agent", "sessions", "2026", "rollout.jsonl")
	put(t, p1, rollout)
	put(t, filepath.Join(dir, "two", "steps", "review", "agent", "sessions", "review.jsonl"), strings.ReplaceAll(rollout, `"id":"author","source":"cli"`, `"id":"reviewer","parent_thread_id":"author"`))
	put(t, filepath.Join(dir, "two", "agent", "sessions", "copy.jsonl"), rollout)
	put(t, filepath.Join(dir, "two", "artifacts", "sessions", "fake.jsonl"), strings.ReplaceAll(rollout, "author", "fake"))
	r1, err := Build([]JobInput{{Directory: dir}, {Directory: dir}}, []string{p1}, "reward")
	if err != nil {
		t.Fatal(err)
	}
	r2, err := Build([]JobInput{{Directory: dir}}, nil, "reward")
	if err != nil {
		t.Fatal(err)
	}
	b1, e1 := json.Marshal(r1)
	b2, e2 := json.Marshal(r2)
	if e1 != nil || e2 != nil || !bytes.Equal(b1, b2) {
		t.Fatalf("replay differs: %v %v", e1, e2)
	}
	if len(r1.Jobs) != 1 || len(r1.Sessions) != 2 || len(r1.Sessions[0].Copies) != 2 || len(r1.Sessions[1].Copies) != 1 {
		t.Fatalf("identities: %+v", r1.Sessions)
	}
	for _, session := range r1.Sessions {
		for _, copy := range session.Copies {
			if *copy.Accounting.Usage.Input != 100 || *copy.Accounting.Usage.Fresh != 90 || len(copy.Evidence.SHA256) != 64 {
				t.Fatalf("usage/evidence: %+v", copy)
			}
		}
	}
	if r1.Sessions[1].Copies[0].Accounting.ParentID != "author" {
		t.Fatal("reviewer parent lost")
	}
}

func TestReportMalformedAndMissingEvidence(t *testing.T) {
	dir := t.TempDir()
	put(t, filepath.Join(dir, "config.json"), `{"tasks":[{"path":"unknown"}]}`)
	put(t, filepath.Join(dir, "broken", "config.json"), `{`)
	put(t, filepath.Join(dir, "broken", "result.json"), `{"finished_at":true}`)
	put(t, filepath.Join(dir, "broken", "agent", "sessions", "partial.jsonl"), `{"type":"session_meta","payload":{"id":"broken"}}`+"\n{")
	r, err := Build([]JobInput{{Directory: dir}}, nil, "reward")
	if err != nil {
		t.Fatal(err)
	}
	j := r.Jobs[0]
	if j.Counts.Expected != nil || j.Counts.NotStarted != nil || j.Counts.Unfinished != 1 || len(j.Trials[0].Diagnostics) < 2 {
		t.Fatalf("missing accounting: %+v", j)
	}
	if r.Sessions[0].Copies[0].Accounting.Usage != nil || len(r.Sessions[0].Copies[0].Accounting.Diagnostics) != 1 {
		t.Fatal("malformed session hidden or missing usage set to zero")
	}
	if _, err := Build([]JobInput{{Arm: "a", Directory: dir}, {Arm: "b", Directory: dir}}, nil, "reward"); err == nil {
		t.Fatal("conflicting arms accepted")
	}
	if _, err := Build(nil, nil, "reward"); err == nil {
		t.Fatal("unscoped scan accepted")
	}
}

func TestNativeProviderRejectionIsNotAZeroQualityScore(t *testing.T) {
	dir := t.TempDir()
	put(t, filepath.Join(dir, "result.json"), `{"n_total_trials":1}`)
	put(t, filepath.Join(dir, "trial", "result.json"), `{"started_at":"2026-09-10T15:32:23Z","finished_at":"2026-09-10T15:32:26Z","agent_execution":{"started_at":"2026-09-10T15:32:23Z"},"exception_info":{"exception_type":"NonZeroAgentExitCodeError"},"verifier_result":{"rewards":{"reward":0}}}`)
	put(t, filepath.Join(dir, "trial", "agent", "sessions", "rollout.jsonl"), `{"type":"session_meta","payload":{"id":"root","source":"cli"}}
{"type":"event_msg","payload":{"type":"task_complete","last_agent_message":null,"error":{"message":"{\"type\":\"error\",\"status\":400,\"error\":{\"type\":\"invalid_request_error\",\"message\":\"The selected model requires a newer version of Codex.\"}}","codex_error_info":"other"}}}
`)
	r, err := Build([]JobInput{{Directory: dir}}, nil, "reward")
	if err != nil {
		t.Fatal(err)
	}
	if r.Jobs[0].Counts.Infrastructure != 1 || r.Jobs[0].Counts.Failed != 0 {
		t.Fatalf("counts: %+v", r.Jobs[0].Counts)
	}
	a := r.Sessions[0].Copies[0].Accounting
	if a.Usage != nil || a.ProviderError == nil || a.ProviderError.Status != 400 || !strings.Contains(string(a.LatestTurnPayload), "newer version") {
		t.Fatalf("native error: %+v", a)
	}
}
