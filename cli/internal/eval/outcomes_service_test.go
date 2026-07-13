package eval

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/evalsubstrate"
)

type outcomesRuntimeSpy struct {
	files    map[string][]byte
	saves    int
	ledger   evalsubstrate.HoldoutBurnLedger
	manifest *RunRecord
}

func (runtime *outcomesRuntimeSpy) ReadFile(path string) ([]byte, error) {
	return runtime.files[path], nil
}
func (runtime *outcomesRuntimeSpy) LoadBurnLedger(string) (evalsubstrate.HoldoutBurnLedger, error) {
	return runtime.ledger, nil
}
func (runtime *outcomesRuntimeSpy) SaveBurnLedger(string, evalsubstrate.HoldoutBurnLedger) error {
	runtime.saves++
	return nil
}
func (runtime *outcomesRuntimeSpy) WriteOutcomesManifest(_ string, _ string, record RunRecord) (string, error) {
	runtime.manifest = &record
	return "manifest.json", nil
}

func TestOutcomesServiceIngestPersistsHoldoutBurnAndManifest(t *testing.T) {
	runtime := &outcomesRuntimeSpy{files: map[string][]byte{"score.json": []byte(`{"source_task_id":"task-1","judge_content_hash":"hash","aggregate":0.9,"threshold":0.8,"criterion_scores":{"correctness":0.9},"split":"holdout","suite_ref":"suite-1","ground_truth_version":"gt-1","run_id":"run-1"}`)}, ledger: evalsubstrate.HoldoutBurnLedger{Budget: 2}}
	result, err := (OutcomesService{Runtime: runtime}).Ingest(context.Background(), OutcomesIngestRequest{ScorePath: "score.json", BurnLedgerPath: "burn.json", ManifestDir: "runs"})
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if result.Verdict.Verdict != "PASS" || result.Warning == "" || result.ManifestPath != "manifest.json" {
		t.Fatalf("result = %#v", result)
	}
	if runtime.saves != 1 || runtime.manifest == nil || runtime.manifest.Suite.Visibility != VisibilityPrivateHoldout {
		t.Fatalf("saves=%d manifest=%#v", runtime.saves, runtime.manifest)
	}
}

func TestOutcomesServiceIngestRefusesExhaustedHoldout(t *testing.T) {
	ledger := evalsubstrate.HoldoutBurnLedger{Budget: 1, Records: []evalsubstrate.BurnRecord{{SuiteRef: "suite-1", GTVersion: "gt-1", RunID: "old"}}}
	runtime := &outcomesRuntimeSpy{files: map[string][]byte{"score.json": []byte(`{"aggregate":1,"threshold":1,"split":"holdout","suite_ref":"suite-1","ground_truth_version":"gt-1","run_id":"new"}`)}, ledger: ledger}
	_, err := (OutcomesService{Runtime: runtime}).Ingest(context.Background(), OutcomesIngestRequest{ScorePath: "score.json", BurnLedgerPath: "burn.json"})
	if err == nil || !strings.Contains(err.Error(), "quota") {
		t.Fatalf("Ingest error = %v", err)
	}
	if runtime.saves != 0 {
		t.Fatalf("saves = %d", runtime.saves)
	}
}
func (*outcomesRuntimeSpy) Now() time.Time { return time.Unix(1_000, 0).UTC() }

func TestOutcomesServiceCompileRefusesHoldoutLeak(t *testing.T) {
	runtime := &outcomesRuntimeSpy{files: map[string][]byte{"input.json": []byte(`{"task":{"id":"task-1","stats":{"min_n_samples":3}},"criteria":[{"id":"c1","description":"secret answer","weight":1}],"holdout_values":["secret answer"]}`)}}
	_, err := (OutcomesService{Runtime: runtime}).Compile(context.Background(), "input.json")
	if err == nil || !strings.Contains(err.Error(), "would leak") {
		t.Fatalf("Compile error = %v", err)
	}
}

func TestOutcomesServiceIngestRefusesJudgeHashMismatchBeforeBurn(t *testing.T) {
	runtime := &outcomesRuntimeSpy{files: map[string][]byte{"score.json": []byte(`{"source_task_id":"task-1","judge_content_hash":"old","aggregate":1,"threshold":1,"split":"holdout"}`)}}
	_, err := (OutcomesService{Runtime: runtime}).Ingest(context.Background(), OutcomesIngestRequest{ScorePath: "score.json", ExpectedJudgeHash: "new", BurnLedgerPath: "burn.json"})
	if err == nil || !strings.Contains(err.Error(), "judge_content_hash mismatch") {
		t.Fatalf("Ingest error = %v", err)
	}
	if runtime.saves != 0 {
		t.Fatalf("burn saves = %d, want 0", runtime.saves)
	}
}
