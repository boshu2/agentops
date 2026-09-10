package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/evidence"
	"github.com/boshu2/agentops/cli/internal/skilltrial"
)

type workFixture struct {
	args                                       []string
	root, proof, author, reviewer, receiptPath string
	receipt                                    evidence.JudgmentReceipt
	verdict                                    map[string]any
}

func workPut(t *testing.T, path string, raw []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func workJSON(t *testing.T, path string, value any) []byte {
	t.Helper()
	raw, err := evidence.Canonical(value)
	if err != nil {
		t.Fatal(err)
	}
	workPut(t, path, raw)
	return raw
}

func newWorkFixture(t *testing.T) *workFixture {
	t.Helper()
	f := &workFixture{root: t.TempDir(), proof: t.TempDir()}
	workPut(t, filepath.Join(f.root, "answer.go"), []byte("package answer\nconst Value = 42\n"))
	manifest, err := evidence.BuildManifest(f.root, []string{"answer.go"}, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath, intentPath, profilesPath := filepath.Join(f.proof, "manifest.json"), filepath.Join(f.proof, "intent"), filepath.Join(f.proof, "profiles.json")
	workJSON(t, manifestPath, manifest)
	intent := []byte("value: answer.Value is 42\nscope: changes are limited to answer.go\n")
	workPut(t, intentPath, intent)
	profile := evidence.JudgeProfile{ID: "review", Runtime: "codex", Model: "gpt-test", Family: "openai", Effort: "high"}
	workJSON(t, profilesPath, map[string]any{"profiles": []evidence.JudgeProfile{profile}})
	f.author, f.reviewer = filepath.Join(f.proof, "author.jsonl"), filepath.Join(f.proof, "reviewer.jsonl")
	workPut(t, f.author, []byte("{\"type\":\"session_meta\",\"payload\":{\"id\":\"author\"}}\n{\"type\":\"event_msg\",\"payload\":{\"type\":\"task_complete\"}}\n"))
	native := []byte("{\"type\":\"session_meta\",\"payload\":{\"id\":\"reviewer\",\"model\":\"gpt-test\",\"model_provider\":\"openai\",\"reasoning_effort\":\"high\"}}\n{\"type\":\"event_msg\",\"payload\":{\"type\":\"task_complete\"}}\n")
	workPut(t, f.reviewer, native)
	zero, yes, no := 0, true, false
	f.receipt = evidence.JudgmentReceipt{Version: "1", Requested: profile, SubjectManifestDigest: manifest.Digest, AcceptanceDigest: evidence.Hash(intent), AuthorContextID: "author", Transcript: evidence.TranscriptBinding{Path: f.reviewer, Start: 0, End: int64(len(native)), SHA256: evidence.Hash(native)}, ExitCode: &zero, TimedOut: &no, Truncated: &no, CleanupVerified: &yes, Omissions: []string{}}
	f.verdict = map[string]any{"schema_version": "verdict.v2", "subject_manifest_digest": manifest.Digest, "acceptance_digest": evidence.Hash(intent), "author_context_id": "author", "validator_context_id": "reviewer", "freshness_attestation": map[string]any{"source": "runtime", "attester_identity": "native-test"}, "verdict": "PASS", "criteria": []any{
		map[string]any{"id": "value", "result": "PASS", "evidence_refs": []string{"check:value-42"}},
		map[string]any{"id": "scope", "result": "PASS", "evidence_refs": []string{"check:scope"}}}, "findings": []any{}, "checked": []string{"answer.go"}, "not_checked": []string{}, "validated_at": "2026-09-10T19:00:00Z"}
	f.args = []string{"--session", f.author, "--root", f.root, "--manifest", manifestPath, "--intent", intentPath, "--evidence-root", f.proof, "--author-context-id", "author", "--required-profiles", profilesPath, "--allowed-provider", "openai", "--required-criterion", "value", "--required-criterion", "scope"}
	return f
}

func (f *workFixture) seal(t *testing.T) []string {
	t.Helper()
	f.receiptPath = filepath.Join(f.proof, "receipt.json")
	raw := workJSON(t, f.receiptPath, f.receipt)
	f.verdict["evidence_refs"] = []string{"judgment-receipt:" + f.receiptPath + "#sha256=" + evidence.Hash(raw)}
	delete(f.verdict, "artifact_digest")
	digest, err := evidence.Digest(f.verdict)
	if err != nil {
		t.Fatal(err)
	}
	f.verdict["artifact_digest"] = digest
	path := filepath.Join(f.proof, digest+".json")
	workJSON(t, path, f.verdict)
	return append(append([]string{}, f.args...), "--verdict", path)
}

func workRun(t *testing.T, args []string) map[string]any {
	t.Helper()
	var out, stderr bytes.Buffer
	if err := run(args, &out, &stderr); err != nil {
		t.Fatalf("report: %v; %s", err, stderr.String())
	}
	var report map[string]any
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	return report
}

func TestNativeWorkRequiresIndependentProof(t *testing.T) {
	f := newWorkFixture(t)
	without := workRun(t, []string{"--session", f.author})
	if work, _ := without["work"].(map[string]any); work != nil && work["status"] == "accepted" {
		t.Fatal("execution became acceptance")
	}
	with := workRun(t, f.seal(t))
	work, _ := with["work"].(map[string]any)
	if work == nil || work["status"] != "accepted" || work["execution"] != "completed" || work["association"] != "standalone_session" {
		t.Fatalf("work: %+v", work)
	}
	if with["sessions"].([]any)[0].(map[string]any)["copies"].([]any)[0].(map[string]any)["accounting"].(map[string]any)["usage"] != nil {
		t.Fatal("missing usage fabricated")
	}
}

func TestNativeWorkPreservesIndependentFailure(t *testing.T) {
	f := newWorkFixture(t)
	f.verdict["verdict"] = "FAIL"
	f.verdict["criteria"].([]any)[0].(map[string]any)["result"] = "FAIL"
	work := workRun(t, f.seal(t))["work"].(map[string]any)
	if work["status"] != "failed" || work["judgments"].(map[string]any)["satisfied"] != false {
		t.Fatalf("coverage replaced FAIL: %+v", work)
	}
}

func TestNativeWorkInvalidOrMissingProofNeverAccepts(t *testing.T) {
	cases := []struct {
		name   string
		before func(*testing.T, *workFixture)
		after  func(*testing.T, *workFixture)
		want   string
	}{
		{name: "wrong subject", before: func(_ *testing.T, f *workFixture) { f.receipt.SubjectManifestDigest = strings.Repeat("a", 64) }, want: "subject_mismatch"},
		{name: "wrong acceptance", before: func(_ *testing.T, f *workFixture) { f.receipt.AcceptanceDigest = strings.Repeat("a", 64) }, want: "acceptance_mismatch"},
		{name: "no freshness", before: func(_ *testing.T, f *workFixture) { f.verdict["freshness_attestation"] = nil }, want: "verdict_integrity"},
		{name: "self review", before: func(_ *testing.T, f *workFixture) { f.verdict["validator_context_id"] = "author" }, want: "distinct nonempty context"},
		{name: "missing criteria", before: func(_ *testing.T, f *workFixture) { f.verdict["criteria"] = []any{} }, want: "criteria must be nonempty"},
		{name: "missing criterion evidence", before: func(_ *testing.T, f *workFixture) {
			f.verdict["criteria"].([]any)[0].(map[string]any)["evidence_refs"] = []string{}
		}, want: "unproven criterion"},
		{name: "unchecked acceptance", before: func(_ *testing.T, f *workFixture) { f.verdict["not_checked"] = []string{"scope"} }, want: "verdict_integrity"},
		{name: "omitted evidence", before: func(_ *testing.T, f *workFixture) { f.receipt.Omissions = []string{"check log unavailable"} }, want: "omitted_evidence"},
		{name: "reviewer timeout", before: func(_ *testing.T, f *workFixture) { yes := true; f.receipt.TimedOut = &yes }, want: "incomplete_runtime"},
		{name: "missing reviewer", after: func(t *testing.T, f *workFixture) {
			if err := os.Remove(f.reviewer); err != nil {
				t.Fatal(err)
			}
		}, want: "no such file"},
		{name: "missing receipt", after: func(t *testing.T, f *workFixture) {
			if err := os.Remove(f.receiptPath); err != nil {
				t.Fatal(err)
			}
		}, want: "receipt_integrity"},
		{name: "tampered receipt", after: func(t *testing.T, f *workFixture) { workPut(t, f.receiptPath, []byte("{}")) }, want: "receipt_integrity"},
		{name: "tampered native span", after: func(t *testing.T, f *workFixture) { workPut(t, f.reviewer, []byte("{}\n")) }, want: "transcript"},
		{name: "changed content", after: func(t *testing.T, f *workFixture) {
			workPut(t, filepath.Join(f.root, "answer.go"), []byte("package answer\nconst Value = 0\n"))
		}, want: "subject content no longer matches"},
		{name: "unknown author", before: func(_ *testing.T, f *workFixture) {
			for i := range f.args {
				if f.args[i] == "--author-context-id" {
					f.args[i+1] = "different"
				}
			}
		}, want: "author_session_missing"},
		{name: "native NOT_PROVEN", before: func(_ *testing.T, f *workFixture) { f.verdict["verdict"] = "NOT_PROVEN" }, want: "verdict_not_pass"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newWorkFixture(t)
			if tc.before != nil {
				tc.before(t, f)
			}
			args := f.seal(t)
			if tc.after != nil {
				tc.after(t, f)
			}
			work := workRun(t, args)["work"].(map[string]any)
			raw, err := json.Marshal(work)
			if err != nil {
				t.Fatal(err)
			}
			if work["status"] != "not_proven" || !strings.Contains(string(raw), tc.want) {
				t.Fatalf("wanted %s: %s", tc.want, raw)
			}
		})
	}
}

func TestNativeWorkRequiresCompleteUnambiguousExecution(t *testing.T) {
	for _, tc := range []struct{ name, event, want string }{
		{"started", `{"type":"task_started"}`, "unfinished"},
		{"aborted", `{"type":"task_aborted"}`, "failed"},
		{"uninterpreted abort after completion", "{\"type\":\"task_complete\"}}\n{\"type\":\"event_msg\",\"payload\":{\"type\":\"turn_aborted\"}", "unknown"},
		{"error with completion marker", `{"type":"task_complete","error":{"message":"local execution failed"}}`, "failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newWorkFixture(t)
			workPut(t, f.author, []byte("{\"type\":\"session_meta\",\"payload\":{\"id\":\"author\"}}\n{\"type\":\"event_msg\",\"payload\":"+tc.event+"}\n"))
			work := workRun(t, f.seal(t))["work"].(map[string]any)
			if work["status"] != "not_proven" || work["execution"] != tc.want || work["judgments"].(map[string]any)["satisfied"] != true {
				t.Fatalf("execution and judgment conflated: %+v", work)
			}
		})
	}
	t.Run("different cumulative copies", func(t *testing.T) {
		f := newWorkFixture(t)
		copyPath := filepath.Join(f.proof, "other-copy.jsonl")
		raw, err := os.ReadFile(f.author)
		if err != nil {
			t.Fatal(err)
		}
		workPut(t, copyPath, append(raw, '\n'))
		args := append(f.seal(t), "--session", copyPath)
		work := workRun(t, args)["work"].(map[string]any)
		if work["status"] != "not_proven" || work["association"] != "ambiguous" || len(work["author_evidence"].([]any)) != 2 {
			t.Fatalf("copy selected: %+v", work)
		}
	})
}

func TestNativeWorkJoinsOneTrialAndKeepsAllAttempts(t *testing.T) {
	f := newWorkFixture(t)
	job := t.TempDir()
	workPut(t, filepath.Join(job, "result.json"), []byte(`{"n_total_trials":3}`))
	terminal := `{"started_at":"2026-09-10T14:00:00Z","finished_at":"2026-09-10T14:00:05Z","verifier_result":{"rewards":{"reward":1}}}`
	workPut(t, filepath.Join(job, "passing", "result.json"), []byte(terminal))
	workPut(t, filepath.Join(job, "failed", "result.json"), []byte(strings.ReplaceAll(terminal, `"reward":1`, `"reward":0`)))
	workPut(t, filepath.Join(job, "unfinished", "config.json"), []byte(`{}`))
	raw, err := os.ReadFile(f.author)
	if err != nil {
		t.Fatal(err)
	}
	workPut(t, filepath.Join(job, "passing", "agent", "sessions", "native.jsonl"), raw)
	args := append(f.seal(t), "--job", "native="+job)
	first := workRun(t, args)
	work := first["work"].(map[string]any)
	if work["status"] != "accepted" || work["association"] != "trial" || filepath.Base(work["trial_directory"].(string)) != "passing" {
		t.Fatalf("trial association: %+v", work)
	}
	counts := first["jobs"].([]any)[0].(map[string]any)["counts"].(map[string]any)
	if counts["success"] != float64(1) || counts["failed"] != float64(1) || counts["unfinished"] != float64(1) {
		t.Fatalf("attempts lost: %+v", counts)
	}
	a, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(workRun(t, args))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("replay changed")
	}
	workPut(t, filepath.Join(job, "failed", "agent", "sessions", "native.jsonl"), raw)
	work = workRun(t, args)["work"].(map[string]any)
	if work["status"] != "not_proven" || work["association"] != "ambiguous" {
		t.Fatalf("retry chosen: %+v", work)
	}
}

func TestNativeWorkMissingRequiredLegAndPartialFlags(t *testing.T) {
	f := newWorkFixture(t)
	work := workRun(t, f.args)["work"].(map[string]any)
	if work["status"] != "not_proven" {
		t.Fatalf("missing required verdict accepted: %+v", work)
	}
	var out, stderr bytes.Buffer
	if err := run([]string{"--session", f.author, "--root", f.root}, &out, &stderr); err == nil || !strings.Contains(err.Error(), "requires --manifest") {
		t.Fatalf("partial judgment flags: %v", err)
	}
}

func TestNativeWorkRequiresCompleteCallerCriterionSet(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*workFixture)
		problem string
	}{
		{"partial omission", func(f *workFixture) { f.verdict["criteria"] = f.verdict["criteria"].([]any)[:1] }, "missing_required_criterion: scope"},
		{"duplicate verdict ID", func(f *workFixture) { f.verdict["criteria"].([]any)[1].(map[string]any)["id"] = "value" }, "duplicate_criterion: value"},
		{"unknown verdict ID", func(f *workFixture) { f.verdict["criteria"].([]any)[1].(map[string]any)["id"] = "other" }, "unexpected_criterion: other"},
		{"unknown expected ID", func(f *workFixture) { f.args[len(f.args)-1] = "other" }, "missing_required_criterion: other"},
		{"duplicate expected ID", func(f *workFixture) { f.args[len(f.args)-1] = "value" }, "criterion IDs must be nonempty, unpadded and unique"},
		{"empty expected ID", func(f *workFixture) { f.args[len(f.args)-1] = "" }, "criterion IDs must be nonempty, unpadded and unique"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newWorkFixture(t)
			tc.mutate(f)
			work := workRun(t, f.seal(t))["work"].(map[string]any)
			raw, err := json.Marshal(work)
			if err != nil {
				t.Fatal(err)
			}
			if work["status"] != "not_proven" || !strings.Contains(string(raw), tc.problem) {
				t.Fatalf("incomplete or ambiguous criteria accepted: %s", raw)
			}
			if strings.Contains(tc.problem, "_criterion:") {
				legs := work["judgments"].(map[string]any)["legs"].([]any)
				if len(legs) != 1 || legs[0].(map[string]any)["verdict"] != "PASS" {
					t.Fatalf("original invalid leg lost: %s", raw)
				}
			}
		})
	}
}

func TestNativeWorkCannotAcceptWithoutCriterionExpectation(t *testing.T) {
	f := newWorkFixture(t)
	f.args = f.args[:len(f.args)-4] // The complete required-criterion input is absent.
	args := f.seal(t)
	var out, stderr bytes.Buffer
	if err := run(args, &out, &stderr); err == nil || !strings.Contains(err.Error(), "requires --required-criterion") {
		t.Fatalf("missing CLI criterion policy: %v", err)
	}
	profiles, err := evidence.LoadJudgeProfiles(filepath.Join(f.proof, "profiles.json"))
	if err != nil {
		t.Fatal(err)
	}
	options := evidence.JudgmentOptions{Root: f.root, Manifest: filepath.Join(f.proof, "manifest.json"), Intent: filepath.Join(f.proof, "intent"), EvidenceRoot: f.proof, AuthorContextID: "author", Required: profiles, AllowedProviders: []string{"openai"}, Verdicts: []string{args[len(args)-1]}}
	report, err := skilltrial.Build(nil, []string{f.author}, "reward")
	if err != nil {
		t.Fatal(err)
	}
	work := skilltrial.InspectWork(report, options)
	if work.Status != "not_proven" || work.Judgments == nil || !work.Judgments.Satisfied || !strings.Contains(strings.Join(work.Problems, ";"), "required_criteria_missing") {
		t.Fatalf("legacy coverage without expected criteria became accepted: %+v", work)
	}
}

func TestNativeWorkKeepsCounterUncertaintySeparateFromExecution(t *testing.T) {
	for _, tc := range []struct{ event, execution, status string }{
		{"task_complete", "completed", "accepted"},
		{"task_started", "unfinished", "not_proven"},
	} {
		t.Run(tc.event, func(t *testing.T) {
			f := newWorkFixture(t)
			native := "{\"type\":\"session_meta\",\"payload\":{\"id\":\"author\"}}\n" +
				"{\"type\":\"event_msg\",\"payload\":{\"type\":\"token_count\",\"info\":{\"total_token_usage\":{\"input_tokens\":100,\"output_tokens\":10}}}}\n" +
				"{\"type\":\"event_msg\",\"payload\":{\"type\":\"token_count\",\"info\":{\"total_token_usage\":{\"input_tokens\":50,\"output_tokens\":5}}}}\n" +
				"{\"type\":\"event_msg\",\"payload\":{\"type\":\"" + tc.event + "\"}}\n"
			workPut(t, f.author, []byte(native))
			report := workRun(t, f.seal(t))
			work := report["work"].(map[string]any)
			if work["status"] != tc.status || work["execution"] != tc.execution || strings.Contains(strings.Join(anyStrings(work["problems"].([]any)), ";"), "author_accounting_unverified") {
				t.Fatalf("counter uncertainty blocked native execution evidence: %+v", work)
			}
			accounting := report["sessions"].([]any)[0].(map[string]any)["copies"].([]any)[0].(map[string]any)["accounting"].(map[string]any)
			if len(accounting["diagnostics"].([]any)) != 1 || accounting["usage"].(map[string]any)["input_tokens"] != float64(50) {
				t.Fatalf("unknown aggregate accounting hidden or added: %+v", accounting)
			}
		})
	}
}

func anyStrings(values []any) []string {
	result := make([]string, len(values))
	for i, value := range values {
		result[i] = value.(string)
	}
	return result
}

func TestNativeWorkMalformedAuthorStillFailsClosed(t *testing.T) {
	f := newWorkFixture(t)
	raw, err := os.ReadFile(f.author)
	if err != nil {
		t.Fatal(err)
	}
	workPut(t, f.author, append([]byte("{invalid\n"), raw...))
	work := workRun(t, f.seal(t))["work"].(map[string]any)
	if work["status"] != "not_proven" || !strings.Contains(strings.Join(anyStrings(work["problems"].([]any)), ";"), "author_accounting_unverified") {
		t.Fatalf("parse uncertainty was treated as usage-only: %+v", work)
	}
}

func TestNativeWorkPassingCodeCannotCompleteInterruptedTrial(t *testing.T) {
	for _, tc := range []struct{ name, result, outcome, execution string }{
		{"timeout with passing code", `{"started_at":"2026-09-10T14:00:00Z","finished_at":"2026-09-10T14:00:05Z","exception_info":{"exception_type":"AgentTimeoutError"},"verifier_result":{"rewards":{"reward":1}}}`, "execution_error", "failed"},
		{"unfinished native trial", `{"started_at":"2026-09-10T14:00:00Z","finished_at":null,"verifier_result":{"rewards":{"reward":1}}}`, "unfinished", "unfinished"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newWorkFixture(t)
			job := t.TempDir()
			workPut(t, filepath.Join(job, "trial", "result.json"), []byte(tc.result))
			raw, err := os.ReadFile(f.author)
			if err != nil {
				t.Fatal(err)
			}
			workPut(t, filepath.Join(job, "trial", "agent", "sessions", "native.jsonl"), raw)
			report := workRun(t, append(f.seal(t), "--job", job))
			work := report["work"].(map[string]any)
			trial := report["jobs"].([]any)[0].(map[string]any)["trials"].([]any)[0].(map[string]any)
			if work["status"] != "not_proven" || work["execution"] != tc.execution || work["judgments"].(map[string]any)["satisfied"] != true || trial["outcome"] != tc.outcome {
				t.Fatalf("passing review/code rehabilitated execution: work=%+v trial=%+v", work, trial)
			}
		})
	}
}
