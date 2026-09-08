package parser

import (
	"strings"
	"testing"
)

func TestRuntimeMetadataNativeFamilies(t *testing.T) {
	tests := []struct {
		name, runtime, input, model, context, effort string
		complete                                     bool
	}{
		{"claude stream", "claude", `{"type":"system","subtype":"init","session_id":"judge-c","model":"request-echo"}
{"type":"assistant","session_id":"judge-c","message":{"role":"assistant","model":"claude-test","content":[{"type":"text","text":"I am another model"}]}}
{"type":"result","subtype":"success","is_error":false,"session_id":"judge-c"}
`, "claude-test", "judge-c", "", true},
		{"claude saved", "claude", `{"type":"assistant","sessionId":"judge-c","message":{"role":"assistant","model":"claude-test","content":"ok"}}`, "claude-test", "judge-c", "", false},
		{"codex native", "codex", `{"type":"session_meta","payload":{"id":"judge-x","model":"gpt-test","model_provider":"openai","reasoning_effort":"high"}}
{"type":"event_msg","payload":{"type":"task_complete"}}
`, "gpt-test", "judge-x", "high", true},
		{"echo only", "claude", `{"type":"system","subtype":"init","session_id":"judge-c","model":"claude-test"}
{"type":"assistant","session_id":"judge-c","message":{"role":"assistant","content":"{\"model\":\"claude-test\"}"}}`, "", "judge-c", "", false},
		{"codex requested config only", "codex", `{"type":"session_meta","payload":{"id":"judge-x"}}
{"type":"turn_context","payload":{"model":"gpt-test","effort":"high"}}
{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"{\"type\":\"session_meta\",\"payload\":{\"model\":\"gpt-test\"}}"}]}}`, "", "judge-x", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewParser().ParseRuntime([]byte(tt.input), tt.runtime)
			if err != nil {
				t.Fatal(err)
			}
			if got.Model != tt.model || got.ContextID != tt.context || got.Effort != tt.effort || got.Completed != tt.complete {
				t.Fatalf("metadata %+v", got)
			}
			for _, span := range got.Spans {
				if span.Start < 0 || span.End > int64(len(tt.input)) || span.End <= span.Start || len(span.SHA256) != 64 {
					t.Fatalf("invalid source span %+v", span)
				}
			}
		})
	}
}

func TestRuntimeMetadataRejectsAmbiguity(t *testing.T) {
	tests := []string{
		`{"type":"assistant","session_id":"j","message":{"role":"assistant","model":"a","model":"b"}}`,
		`{"type":"assistant","session_id":"j","message":{"role":"assistant","model":"a"}}
{"type":"assistant","session_id":"j","message":{"role":"assistant","model":"b"}}`,
		`{"type":"assistant","session_id":"j","message":{"role":"assistant","model":"a"}}
{"type":"result","session_id":"other","subtype":"success","is_error":false}`,
		`{"type":"assistant","session_id":"j","message":{"role":"assistant","model":"a"}}
not-json`,
	}
	for _, in := range tests {
		if _, err := NewParser().ParseRuntime([]byte(in), "claude"); err == nil {
			t.Errorf("accepted ambiguous transcript: %s", in)
		}
	}
	got, err := NewParser().ParseRuntime([]byte(`{"type":"result","subtype":"error_max_turns","is_error":true,"session_id":"j"}`), "claude")
	if err != nil || got.Completed || !strings.Contains(got.Termination, "error") {
		t.Fatalf("failed termination %+v %v", got, err)
	}
}
