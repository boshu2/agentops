package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/yieldledger"
)

func TestDeriveTranscriptTokens_SumsRealUsage(t *testing.T) {
	// E4.2 bronze->silver bridge (age-membrane-memory-arch-tz2s.3.2): the real
	// tokens for a bead-tied yield-usage event are derived from the session
	// transcript, not the env default of 0. Fixture parsed by the production
	// reader (fixture fidelity).
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	jsonl := `{"type":"assistant","timestamp":"2026-04-11T12:00:00Z","message":{"role":"assistant","content":"a","usage":{"input_tokens":100,"cache_read_input_tokens":400,"output_tokens":30}}}
{"type":"assistant","timestamp":"2026-04-11T12:00:05Z","message":{"role":"assistant","content":"b","usage":{"input_tokens":50,"cache_creation_input_tokens":200,"output_tokens":20}}}
{"type":"user","timestamp":"2026-04-11T12:00:10Z","content":"thanks"}
`
	if err := os.WriteFile(path, []byte(jsonl), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	in, out, err := deriveTranscriptTokens(path)
	if err != nil {
		t.Fatalf("deriveTranscriptTokens: %v", err)
	}
	if in != 750 { // (100+400) + (50+200)
		t.Errorf("tokens_in = %d, want 750", in)
	}
	if out != 50 { // 30 + 20
		t.Errorf("tokens_out = %d, want 50", out)
	}
}

func TestDeriveTranscriptTokens_MissingFile(t *testing.T) {
	if _, _, err := deriveTranscriptTokens("/nonexistent/transcript.jsonl"); err == nil {
		t.Fatal("expected error for missing transcript, got nil")
	}
}

func TestDeriveTranscriptTokens_MalformedErrors(t *testing.T) {
	// A non-JSONL file parses to zero messages; that must ERROR so reconcile-pr.sh
	// takes a visible fail-open path, not silently emit a derived 0. (REFUTE fix)
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.jsonl")
	if err := os.WriteFile(bad, []byte("not json\nstill not json\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, _, err := deriveTranscriptTokens(bad); err == nil {
		t.Fatal("expected error for unparseable transcript, got nil")
	}
}

func TestDeriveTranscriptTokens_RealFixtureDedups(t *testing.T) {
	// Regression for the cross-family REFUTE: real Claude Code transcripts
	// repeat the same usage block across multiple rows of one response. The
	// committed real fixture's naive per-row sum is 26781956/559; the correct
	// per-response-deduped total is 10946330/228. Locking the deduped value
	// guards against the overcount returning. Fixture parsed by the production reader.
	const fixture = "testdata/transcripts/real-2.4mb.jsonl"
	if _, err := os.Stat(fixture); err != nil {
		t.Skipf("real fixture not present: %v", err)
	}
	in, out, err := deriveTranscriptTokens(fixture)
	if err != nil {
		t.Fatalf("deriveTranscriptTokens: %v", err)
	}
	if in != 10946330 {
		t.Errorf("tokens_in = %d, want 10946330 (deduped; naive overcount was 26781956)", in)
	}
	if out != 228 {
		t.Errorf("tokens_out = %d, want 228 (deduped; naive overcount was 559)", out)
	}
}

// TestYieldTokensCmd_JSON exercises the `ao yield tokens` command end-to-end
// (cobra invocation), asserting the --json output shape. (reconcile-pr.sh
// consumes the --pair form; --json is for other machine consumers.)
func TestYieldTokensCmd_JSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	jsonl := `{"type":"assistant","message":{"role":"assistant","content":"a","usage":{"input_tokens":100,"cache_read_input_tokens":900,"output_tokens":50}}}
`
	if err := os.WriteFile(path, []byte(jsonl), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	var buf bytes.Buffer
	yieldTokensCmd.SetOut(&buf)
	t.Cleanup(func() { yieldTokensCmd.SetOut(nil) })
	if err := yieldTokensCmd.Flags().Set("transcript", path); err != nil {
		t.Fatalf("set transcript: %v", err)
	}
	if err := yieldTokensCmd.Flags().Set("json", "true"); err != nil {
		t.Fatalf("set json: %v", err)
	}
	t.Cleanup(func() {
		_ = yieldTokensCmd.Flags().Set("transcript", "")
		_ = yieldTokensCmd.Flags().Set("json", "false")
	})

	if err := runYieldTokens(yieldTokensCmd, nil); err != nil {
		t.Fatalf("runYieldTokens: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	want := `{"tokens_in":1000,"tokens_out":50}`
	if got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestEmitYieldEvent_GateVerdictCarriesDomainAndReason is the regression for the
// cross-family refute (age-membrane-memory-j9c6): the emit path parsed domain+
// reason into the body but DROPPED them when building GateVerdictInput, so the
// membrane's new memory fields were a no-op in production. They must survive
// emit -> load end-to-end.
func TestEmitYieldEvent_GateVerdictCarriesDomainAndReason(t *testing.T) {
	root := t.TempDir()
	ts := time.Date(2026, 6, 19, 14, 0, 0, 0, time.UTC)
	body := `{"difficulty":1,"pawl_verdict_ref":{"bead_id":"ag-r","head_sha":"abc1234"},"disposition":"REFUTED","head_sha":"abc1234","attempt":2,"author_context_id":"ctx","refuter_families":["codex"],"author_family":"claude","cross_family":true,"author_ne_reviewer":true,"evidence_present":true,"domain":"concurrency","reason":"data race on a shared counter"}`
	if err := emitYieldEvent(root, yieldledger.EventGateVerdict, "ag-r", "r1", ts, body); err != nil {
		t.Fatalf("emit: %v", err)
	}
	ledger, err := yieldledger.Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	gvs := ledger.GateVerdictsFor("ag-r")
	if len(gvs) != 1 || gvs[0].GateVerdict == nil {
		t.Fatalf("want 1 gate-verdict with a body, got %d", len(gvs))
	}
	if gvs[0].GateVerdict.Domain != "concurrency" {
		t.Errorf("Domain = %q, want concurrency (emit path dropped it)", gvs[0].GateVerdict.Domain)
	}
	if gvs[0].GateVerdict.Reason != "data race on a shared counter" {
		t.Errorf("Reason = %q, want the missed reason (emit path dropped it)", gvs[0].GateVerdict.Reason)
	}
}

// TestEmitYieldEvent_AllKinds verifies each event kind decodes its JSON body and
// appends through the ledger, and that the bead-keyed projections see it.
func TestEmitYieldEvent_AllKinds(t *testing.T) {
	root := t.TempDir()
	ts := time.Date(2026, 6, 14, 18, 0, 0, 0, time.UTC)

	gvBody := `{"difficulty":3,"pawl_verdict_ref":{"bead_id":"ag-x","head_sha":"abc1234"},"disposition":"CONFIRMED","head_sha":"abc1234","attempt":1,"author_context_id":"ctx-1","refuter_families":["claude","gpt"],"author_family":"claude","cross_family":true,"author_ne_reviewer":true,"evidence_present":true}`
	if err := emitYieldEvent(root, yieldledger.EventGateVerdict, "ag-x", "r1", ts, gvBody); err != nil {
		t.Fatalf("emit gate-verdict: %v", err)
	}

	usageBody := `{"tokens_in":100,"tokens_out":20,"cost_usd":0.3,"wall_clock_s":60,"model":"claude-opus-4-8","phase":"implement","category_hint":"productive"}`
	if err := emitYieldEvent(root, yieldledger.EventUsage, "ag-x", "r1", ts, usageBody); err != nil {
		t.Fatalf("emit usage: %v", err)
	}

	acceptBody := `{"merge_sha":"def5678","merged_by":"orch","gate_verdict_ref":{"bead_id":"ag-x","head_sha":"abc1234"}}`
	if err := emitYieldEvent(root, yieldledger.EventAccept, "ag-x", "r1", ts, acceptBody); err != nil {
		t.Fatalf("emit accept: %v", err)
	}

	ledger, err := yieldledger.Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := len(ledger.EventsFor("ag-x")); got != 3 {
		t.Fatalf("EventsFor(ag-x) = %d, want 3", got)
	}
	if got := len(ledger.AcceptsFor("ag-x")); got != 1 {
		t.Errorf("AcceptsFor(ag-x) = %d, want 1", got)
	}
	if got := len(ledger.GateVerdictsFor("ag-x")); got != 1 {
		t.Errorf("GateVerdictsFor(ag-x) = %d, want 1", got)
	}
	if got := len(ledger.UsageFor("ag-x")); got != 1 {
		t.Errorf("UsageFor(ag-x) = %d, want 1", got)
	}
}

// TestFmtCatchRate covers the membrane catch-rate rendering: a real value formats
// to 3 dp; a nil (no adjudicated verdicts) renders n/a with the divide-guard note.
func TestFmtCatchRate(t *testing.T) {
	v := 0.6666666666666666
	if got := fmtCatchRate(&v, ""); got != "0.667" {
		t.Errorf("fmtCatchRate(0.667) = %q, want %q", got, "0.667")
	}
	zero := 0.0
	if got := fmtCatchRate(&zero, ""); got != "0.000" {
		t.Errorf("fmtCatchRate(0.0) = %q, want %q", got, "0.000")
	}
	if got := fmtCatchRate(nil, "no confirmed+refuted gate-verdicts"); got != "n/a (no confirmed+refuted gate-verdicts)" {
		t.Errorf("fmtCatchRate(nil, note) = %q", got)
	}
	if got := fmtCatchRate(nil, ""); got != "n/a (0 denominator)" {
		t.Errorf("fmtCatchRate(nil, \"\") = %q, want default note", got)
	}
}

// TestEmitYieldEvent_Rejects verifies envelope/body validation surfaces errors
// (so a malformed emit fails loudly even though callers swallow it with || true).
func TestEmitYieldEvent_Rejects(t *testing.T) {
	root := t.TempDir()
	ts := time.Now().UTC()
	cases := []struct {
		name                  string
		kind, bead, run, body string
	}{
		{"missing bead", yieldledger.EventUsage, "", "r1", `{"tokens_in":1,"tokens_out":1,"cost_usd":0.1,"wall_clock_s":1,"model":"m","phase":"implement"}`},
		{"missing run", yieldledger.EventUsage, "ag-x", "", `{"tokens_in":1,"tokens_out":1,"cost_usd":0.1,"wall_clock_s":1,"model":"m","phase":"implement"}`},
		{"missing body", yieldledger.EventUsage, "ag-x", "r1", ``},
		{"bad json", yieldledger.EventUsage, "ag-x", "r1", `{not json}`},
		{"invalid phase", yieldledger.EventUsage, "ag-x", "r1", `{"tokens_in":1,"tokens_out":1,"cost_usd":0.1,"wall_clock_s":1,"model":"m","phase":"bogus"}`},
		{"unknown kind", "bogus", "ag-x", "r1", `{}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := emitYieldEvent(root, tc.kind, tc.bead, tc.run, ts, tc.body); err == nil {
				t.Error("emitYieldEvent accepted a bad input, want error")
			}
		})
	}
}

// TestWriteGaugeReport_EscapeRateRow verifies the human gauge report surfaces
// the escape_rate row (age-6ty) with the escapes/confirmed counts and the
// rubber-stamp framing — the v2 quality gauge must be visible, not just in JSON.
func TestWriteGaugeReport_EscapeRateRow(t *testing.T) {
	er := 0.25
	g := yieldledger.Gauges{
		RunID:        "r-test",
		SpendMeasure: yieldledger.SpendMeasure,
		Confirmed:    4,
		Escapes:      1,
		EscapeRate:   &er,
	}
	var buf bytes.Buffer
	if err := writeGaugeReport(&buf, g, false); err != nil {
		t.Fatalf("writeGaugeReport: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "escape_rate") {
		t.Errorf("report missing escape_rate row:\n%s", out)
	}
	if !strings.Contains(out, "0.250") {
		t.Errorf("report missing escape_rate value 0.250:\n%s", out)
	}
	if !strings.Contains(out, "1 escapes / 4 confirmed") {
		t.Errorf("report missing escape/confirmed counts:\n%s", out)
	}
	if !strings.Contains(out, "rubber-stamp") {
		t.Errorf("report missing the rubber-stamp framing:\n%s", out)
	}
}
