package evidence

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func ptr[T any](v T) *T { return &v }

type judgmentFixture struct {
	options                JudgmentOptions
	receipt                JudgmentReceipt
	out, transcript, draft string
}

func newJudgmentFixture(t *testing.T, runtime string) *judgmentFixture {
	t.Helper()
	store, manifest := fixture(t)
	profile := JudgeProfile{ID: "primary", Runtime: runtime, Model: "gpt-test", Family: "openai", Effort: "high"}
	native := `{"type":"session_meta","payload":{"id":"judge","model":"gpt-test","model_provider":"openai","reasoning_effort":"high"}}
{"type":"event_msg","payload":{"type":"task_complete"}}
`
	if runtime == "claude" {
		profile.Model, profile.Family, profile.Effort = "claude-test", "anthropic", ""
		native = `{"type":"system","subtype":"init","session_id":"judge","model":"requested-echo"}
{"type":"assistant","session_id":"judge","message":{"role":"assistant","model":"claude-test","content":[{"type":"text","text":"PASS"}]}}
{"type":"result","subtype":"success","is_error":false,"session_id":"judge"}
`
	}
	transcript := filepath.Join(store.EvidenceRoot, "native.jsonl")
	write(t, transcript, []byte(native))
	receipt := JudgmentReceipt{
		Version: "1", Requested: profile, SubjectManifestDigest: manifest.Digest, AcceptanceDigest: Hash([]byte("independent source-support acceptance\n")), AuthorContextID: "author",
		Transcript: TranscriptBinding{transcript, 0, int64(len(native)), Hash([]byte(native))}, ExitCode: ptr(0), TimedOut: ptr(false), Truncated: ptr(false), CleanupVerified: ptr(true), Omissions: []string{},
	}
	return &judgmentFixture{JudgmentOptions{Root: store.Root, EvidenceRoot: store.EvidenceRoot, Manifest: store.SubjectManifest, Intent: store.IntentSource, AuthorContextID: "author", Required: []JudgeProfile{profile}, AllowedProviders: []string{profile.Family}}, receipt, store.EvidenceRoot, transcript, store.Draft}
}

func (f *judgmentFixture) publish(t *testing.T, mutate func(map[string]any)) {
	t.Helper()
	receiptBytes, err := Canonical(f.receipt)
	if err != nil {
		t.Fatal(err)
	}
	receiptPath := filepath.Join(f.out, "receipt-"+Hash(receiptBytes)+".json")
	write(t, receiptPath, receiptBytes)
	ref := "judgment-receipt:" + receiptPath + "#sha256=" + Hash(receiptBytes)
	v := draft()
	v["schema_version"] = "verdict.v2"
	v["acceptance_digest"] = f.receipt.AcceptanceDigest
	v["subject_manifest_digest"] = f.receipt.SubjectManifestDigest
	v["author_context_id"] = "author"
	v["validator_context_id"] = "judge"
	v["freshness_attestation"] = map[string]any{"source": "runtime", "attester_identity": "native-launch"}
	v["evidence_refs"] = []string{ref}
	if mutate != nil {
		mutate(v)
	}
	digest, err := Digest(v)
	if err != nil {
		t.Fatal(err)
	}
	v["artifact_digest"] = digest
	path := filepath.Join(f.out, digest+".json")
	writeJSON(t, path, v)
	f.options.Verdicts = []string{path}
}

func TestJudgmentsNativeRequiredLegs(t *testing.T) {
	for _, runtime := range []string{"codex", "claude"} {
		t.Run(runtime, func(t *testing.T) {
			f := newJudgmentFixture(t, runtime)
			f.publish(t, nil)
			result, err := VerifyJudgments(f.options)
			if err != nil || !result.Satisfied || len(result.Legs) != 1 || result.Legs[0].Native.Model != f.receipt.Requested.Model {
				t.Fatalf("result %+v error %v", result, err)
			}
		})
	}
}

func TestJudgmentsRequiredFailures(t *testing.T) {
	tests := []struct {
		name     string
		mutate   func(*testing.T, *judgmentFixture)
		verdict  func(map[string]any)
		contains string
	}{
		{name: "missing leg", mutate: func(_ *testing.T, f *judgmentFixture) {
			p := f.options.Required[0]
			p.ID = "second"
			f.options.Required = append(f.options.Required, p)
		}, contains: "missing_required_leg"},
		{name: "requested model substitution", mutate: func(_ *testing.T, f *judgmentFixture) { f.receipt.Requested.Model = "other" }, contains: "requested_profile_mismatch"},
		{name: "actual model substitution", mutate: func(t *testing.T, f *judgmentFixture) { f.changeTranscript(t, "gpt-test", "other") }, contains: "runtime_model_mismatch"},
		{name: "wrong family", mutate: func(t *testing.T, f *judgmentFixture) { f.changeTranscript(t, "openai", "other") }, contains: "runtime_family_mismatch"},
		{name: "wrong effort", mutate: func(t *testing.T, f *judgmentFixture) { f.changeTranscript(t, "high", "low") }, contains: "runtime_effort_mismatch"},
		{name: "unknown model", mutate: func(t *testing.T, f *judgmentFixture) { f.changeTranscript(t, `"model":"gpt-test",`, "") }, contains: "identity_unverified"},
		{name: "reused author", mutate: func(t *testing.T, f *judgmentFixture) { f.changeTranscript(t, `"id":"judge"`, `"id":"author"`) }, contains: "reused_author_context"},
		{name: "native context substitution", mutate: func(t *testing.T, f *judgmentFixture) { f.changeTranscript(t, `"id":"judge"`, `"id":"different"`) }, contains: "validator_context_mismatch"},
		{name: "wrong expected author", mutate: func(_ *testing.T, f *judgmentFixture) { f.options.AuthorContextID = "different" }, contains: "author_context_mismatch"},
		{name: "wrong acceptance", mutate: func(_ *testing.T, f *judgmentFixture) { f.receipt.AcceptanceDigest = strings.Repeat("a", 64) }, contains: "acceptance_mismatch"},
		{name: "stale subject", mutate: func(_ *testing.T, f *judgmentFixture) { f.receipt.SubjectManifestDigest = strings.Repeat("b", 64) }, contains: "subject_mismatch"},
		{name: "timed out", mutate: func(_ *testing.T, f *judgmentFixture) { f.receipt.TimedOut = ptr(true) }, contains: "incomplete_runtime"},
		{name: "missing exit", mutate: func(_ *testing.T, f *judgmentFixture) { f.receipt.ExitCode = nil }, contains: "incomplete_runtime"},
		{name: "truncated", mutate: func(_ *testing.T, f *judgmentFixture) { f.receipt.Truncated = ptr(true) }, contains: "incomplete_runtime"},
		{name: "cleanup unverified", mutate: func(_ *testing.T, f *judgmentFixture) { f.receipt.CleanupVerified = ptr(false) }, contains: "incomplete_runtime"},
		{name: "omitted input", mutate: func(_ *testing.T, f *judgmentFixture) { f.receipt.Omissions = []string{"subject unread"} }, contains: "omitted_evidence"},
		{name: "no completion", mutate: func(t *testing.T, f *judgmentFixture) { f.changeTranscript(t, "task_complete", "turn_aborted") }, contains: "native_termination_unverified"},
		{name: "wrong transcript digest", mutate: func(_ *testing.T, f *judgmentFixture) { f.receipt.Transcript.SHA256 = strings.Repeat("a", 64) }, contains: "transcript_digest_mismatch"},
		{name: "wrong span", mutate: func(_ *testing.T, f *judgmentFixture) { f.receipt.Transcript.End++ }, contains: "transcript_span"},
		{name: "missing receipt", verdict: func(v map[string]any) { v["evidence_refs"] = []string{"unresolvable"} }, contains: "missing_required_leg"},
		{name: "unchanged failure", verdict: func(v map[string]any) { v["verdict"] = "FAIL" }, contains: "verdict_not_pass"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newJudgmentFixture(t, "codex")
			if tt.mutate != nil {
				tt.mutate(t, f)
			}
			f.publish(t, tt.verdict)
			before, err := os.ReadFile(f.options.Verdicts[0])
			if err != nil {
				t.Fatal(err)
			}
			result, err := VerifyJudgments(f.options)
			if err != nil {
				t.Fatal(err)
			}
			raw, marshalErr := json.Marshal(result)
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			if result.Satisfied || !strings.Contains(string(raw), tt.contains) {
				t.Fatalf("wanted %s, got %s", tt.contains, raw)
			}
			after, err := os.ReadFile(f.options.Verdicts[0])
			if err != nil || string(before) != string(after) {
				t.Fatal("verdict was changed")
			}
		})
	}
}

func (f *judgmentFixture) changeTranscript(t *testing.T, old, new string) {
	t.Helper()
	b, err := os.ReadFile(f.transcript)
	if err != nil {
		t.Fatal(err)
	}
	b = []byte(strings.ReplaceAll(string(b), old, new))
	write(t, f.transcript, b)
	f.receipt.Transcript.End = int64(len(b))
	f.receipt.Transcript.SHA256 = Hash(b)
}

func TestJudgmentsDeniedBeforeCandidateReads(t *testing.T) {
	f := newJudgmentFixture(t, "claude")
	f.options.AllowedProviders = []string{"openai"}
	f.options.Verdicts = []string{"nonexistent-private-verdict"}
	f.options.Manifest = "nonexistent-private-manifest"
	_, err := VerifyJudgments(f.options)
	if err == nil || !strings.Contains(err.Error(), "provider_denied") {
		t.Fatalf("expected policy denial before reading candidate: %v", err)
	}
}

func TestJudgmentsReceiptsStrictAndBound(t *testing.T) {
	for _, kind := range []string{"digest", "unknown-field", "duplicate-field", "unsafe-path"} {
		t.Run(kind, func(t *testing.T) {
			f := newJudgmentFixture(t, "codex")
			f.publish(t, nil)
			receiptBytes, err := Canonical(f.receipt)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(f.out, "receipt-"+Hash(receiptBytes)+".json")
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "digest":
				write(t, path, append(b, ' '))
			case "unknown-field":
				f.publish(t, func(v map[string]any) {
					write(t, path, []byte(`{"unknown":true}`))
					v["evidence_refs"] = []string{"judgment-receipt:" + path + "#sha256=" + Hash([]byte(`{"unknown":true}`))}
				})
			case "duplicate-field":
				f.publish(t, func(v map[string]any) {
					b = []byte(`{"version":"1","version":"2"}`)
					write(t, path, b)
					v["evidence_refs"] = []string{"judgment-receipt:" + path + "#sha256=" + Hash(b)}
				})
			case "unsafe-path":
				f.publish(t, func(v map[string]any) {
					v["evidence_refs"] = []string{"judgment-receipt:relative.json#sha256=" + Hash(b)}
				})
			}
			result, err := VerifyJudgments(f.options)
			if err != nil {
				t.Fatal(err)
			}
			if result.Satisfied {
				t.Fatalf("accepted %s: %+v", kind, result)
			}
		})
	}
}

func TestJudgmentsReusedPeerContext(t *testing.T) {
	f := newJudgmentFixture(t, "codex")
	f.publish(t, nil)
	first := f.options.Verdicts[0]
	p := f.options.Required[0]
	p.ID = "second"
	f.options.Required = append(f.options.Required, p)
	f.receipt.Requested = p
	f.publish(t, nil)
	f.options.Verdicts = append(f.options.Verdicts, first)
	result, err := VerifyJudgments(f.options)
	if err != nil {
		t.Fatal(err)
	}
	b, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	if result.Satisfied || !strings.Contains(string(b), "reused_peer_context") {
		t.Fatalf("%s", b)
	}
}

func ExampleJudgmentReceipt() {
	fmt.Println("judgment-receipt:/private/evidence/receipt.json#sha256=<exact-receipt-byte-hash>")
	// Output: judgment-receipt:/private/evidence/receipt.json#sha256=<exact-receipt-byte-hash>
}

func TestJudgmentsClaudeNativeIdentityNotRequestedEcho(t *testing.T) {
	tests := []struct{ name, old, replacement, want string }{
		{"actual model differs from request", `"model":"claude-test"`, `"model":"different-model"`, "runtime_model_mismatch"},
		{"unknown actual with fake text", `"model":"claude-test",`, `"requested_model":"claude-test",`, "identity_unverified"},
		{"native reused author", `"session_id":"judge"`, `"session_id":"author"`, "reused_author_context"},
		{"failed result despite successful exit", `"subtype":"success"`, `"subtype":"error_max_turns"`, "native_termination_unverified"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newJudgmentFixture(t, "claude")
			f.changeTranscript(t, tt.old, tt.replacement)
			f.publish(t, nil)
			got, err := VerifyJudgments(f.options)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			if got.Satisfied || !strings.Contains(string(raw), tt.want) {
				t.Fatalf("%s", raw)
			}
		})
	}
}

func TestJudgmentsRejectsPartialLineAndOutsideRoot(t *testing.T) {
	f := newJudgmentFixture(t, "codex")
	native, err := os.ReadFile(f.transcript)
	if err != nil {
		t.Fatal(err)
	}
	f.receipt.Transcript.Start = 1
	f.receipt.Transcript.SHA256 = Hash(native[1:])
	f.publish(t, nil)
	got, err := VerifyJudgments(f.options)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if got.Satisfied || !strings.Contains(string(raw), "native line boundary") {
		t.Fatalf("accepted interior JSON: %s", raw)
	}
	f.receipt.Transcript.Start = 0
	f.receipt.Transcript.SHA256 = Hash(native)
	outside := filepath.Join(t.TempDir(), "private.jsonl")
	write(t, outside, native)
	f.receipt.Transcript.Path = outside
	f.publish(t, nil)
	got, err = VerifyJudgments(f.options)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if got.Satisfied || !strings.Contains(string(raw), "authorized root") {
		t.Fatalf("accepted outside read: %s", raw)
	}
}

func TestJudgmentsSymlinkRootAndCanonicalReceipt(t *testing.T) {
	f := newJudgmentFixture(t, "codex")
	f.publish(t, nil)
	alias := filepath.Join(t.TempDir(), "evidence-alias")
	if err := os.Symlink(f.out, alias); err != nil {
		t.Fatal(err)
	}
	f.options.EvidenceRoot = alias
	got, err := VerifyJudgments(f.options)
	if err != nil || !got.Satisfied {
		t.Fatalf("caller root alias lost file identity: %+v %v", got, err)
	}
}

func TestJudgmentsUnknownIdentityIsNeverDiversity(t *testing.T) {
	f := newJudgmentFixture(t, "codex")
	f.changeTranscript(t, "gpt-test", "unknown")
	f.publish(t, nil)
	got, err := VerifyJudgments(f.options)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if got.Satisfied || !strings.Contains(string(raw), "identity_unverified") {
		t.Fatalf("unknown reported model: %s", raw)
	}
	f.options.Required[0].Model = "unknown"
	if _, err = VerifyJudgments(f.options); err == nil {
		t.Fatal("caller unknown model satisfied identity")
	}
}

func TestJudgmentsRequiredEffortMissingFromNativeReporting(t *testing.T) {
	f := newJudgmentFixture(t, "claude")
	f.options.Required[0].Effort = "low"
	f.receipt.Requested.Effort = "low"
	f.publish(t, nil)
	got, err := VerifyJudgments(f.options)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if got.Satisfied || !strings.Contains(string(raw), "runtime_effort_mismatch") || got.Legs[0].Native.Model != "claude-test" {
		t.Fatalf("requested effort was treated as observed: %s", raw)
	}
}
