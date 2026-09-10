package parser

import (
	"strings"
	"testing"
)

func TestCodexAccountingPreservesCumulativeAndUnknownFacts(t *testing.T) {
	input := `{"type":"session_meta","payload":{"id":"worker","source":{"subagent":{"thread_spawn":{"parent_thread_id":"author"}}},"future_field":"retain"}}
{"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":100,"cached_input_tokens":20,"output_tokens":10}}}}
{"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":300,"cached_input_tokens":90,"output_tokens":20,"reasoning_output_tokens":10,"total_tokens":320,"future_counter":42}}}}
{"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":300,"cached_input_tokens":90,"output_tokens":20,"reasoning_output_tokens":10,"total_tokens":320,"future_counter":42}}}}
{"type":"event_msg","payload":{"type":"token_count","info":null}}
{"type":"new_native_event","payload":{}}
{"type":"event_msg"
`
	a, err := ParseCodexAccounting(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if a.ID != "worker" || a.ParentID != "author" {
		t.Fatalf("identity: %+v", a)
	}
	if a.Usage == nil || *a.Usage.Input != 300 || *a.Usage.Cached != 90 || *a.Usage.Fresh != 210 || *a.Usage.Output != 20 || *a.Usage.Total != 320 {
		t.Fatalf("usage: %+v", a.Usage)
	}
	if a.UsageEvents != 4 || a.UsageLine != 4 {
		t.Fatalf("usage source: %+v", a)
	}
	if len(a.Diagnostics) != 1 || a.Diagnostics[0].Line != 7 {
		t.Fatalf("diagnostics: %+v", a.Diagnostics)
	}
	if a.Uninterpreted["new_native_event"] != 1 || !strings.Contains(string(a.Metadata), "future_field") || !strings.Contains(string(a.UsagePayload), "future_counter") {
		t.Fatalf("unknown evidence dropped: %+v", a)
	}
}

func TestCodexAccountingMissingAndInvalidUsage(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		wantNil     bool
		diagnostic  string
	}{
		{"missing", ``, true, ""},
		{"null info", `{"type":"event_msg","payload":{"type":"token_count","info":null}}`, true, ""},
		{"missing cache", `{"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":12,"output_tokens":0}}}}`, false, ""},
		{"bad type", `{"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":"12"}}}}`, true, "malformed cumulative"},
		{"negative", `{"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":-1}}}}`, true, "negative"},
		{"impossible cache", `{"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":1,"cached_input_tokens":2}}}}`, true, "exceeds"},
		{"double reasoning", `{"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":1,"output_tokens":2,"reasoning_output_tokens":3}}}}`, true, "exceed"},
		{"wrong total", `{"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":1,"output_tokens":2,"total_tokens":9}}}}`, true, "disagree"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, err := ParseCodexAccounting(strings.NewReader("{\"type\":\"session_meta\",\"payload\":{\"id\":\"session\",\"source\":\"cli\"}}\n" + tc.input))
			if err != nil {
				t.Fatal(err)
			}
			if (a.Usage == nil) != tc.wantNil {
				t.Fatalf("usage = %+v", a.Usage)
			}
			if !tc.wantNil && (a.Usage.Cached != nil || a.Usage.Fresh != nil || a.Usage.Output == nil || *a.Usage.Output != 0) {
				t.Fatalf("missing/zero conflation: %+v", a.Usage)
			}
			if tc.diagnostic != "" && (len(a.Diagnostics) == 0 || !strings.Contains(a.Diagnostics[0].Message, tc.diagnostic)) {
				t.Fatalf("diagnostic: %+v", a.Diagnostics)
			}
		})
	}
}

func TestCodexAccountingRegressionAndIdentityConflict(t *testing.T) {
	a, err := ParseCodexAccounting(strings.NewReader(`{"type":"session_meta","payload":{"session_id":"one","source":"cli"}}
{"type":"session_meta","payload":{"id":"two"}}
{"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":100}}}}
{"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":20}}}}
`))
	if err != nil {
		t.Fatal(err)
	}
	if a.ID != "one" || a.Usage == nil || *a.Usage.Input != 20 || len(a.Diagnostics) != 2 {
		t.Fatalf("result: %+v", a)
	}
}
