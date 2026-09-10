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

func TestCodexAccountingInheritedParentHeader(t *testing.T) {
	const child = `{"type":"session_meta","payload":{"id":"child","session_id":"parent","parent_thread_id":"parent","forked_from_id":"parent","timestamp":"2026-09-10T15:51:27.355Z","source":{"subagent":{"thread_spawn":{"parent_thread_id":"parent"}}}}}`
	const parent = `{"type":"session_meta","payload":{"id":"parent","session_id":"parent","timestamp":"2026-09-10T15:49:48.395Z","source":"exec","future_field":"retained"}}`
	a, err := ParseCodexAccounting(strings.NewReader(child + "\n" + parent + `
{"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":1000,"cached_input_tokens":500,"output_tokens":20}}}}
{"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":1100,"cached_input_tokens":550,"output_tokens":30}}}}
`))
	if err != nil {
		t.Fatal(err)
	}
	if a.ID != "child" || a.ParentID != "parent" || len(a.Diagnostics) != 0 {
		t.Fatalf("corroborated inherited header changed child identity or was diagnosed as conflict: %+v", a)
	}
	if !strings.Contains(string(a.Metadata), `"id":"child"`) || a.Usage == nil || *a.Usage.Input != 1100 {
		t.Fatalf("primary metadata or native cumulative usage changed: %+v", a)
	}
	if len(a.InheritedParentHeaders) != 1 || a.InheritedParentHeaders[0].Line != 2 || a.InheritedParentHeaders[0].InheritedUsage != nil || !strings.Contains(string(a.InheritedParentHeaders[0].Metadata), "future_field") {
		t.Fatalf("inherited evidence lost or token attribution invented: %+v", a.InheritedParentHeaders)
	}
}

func TestCodexAccountingUncorroboratedParentHeadersRemainConflicts(t *testing.T) {
	const child = `{"type":"session_meta","payload":{"id":"child","session_id":"parent","parent_thread_id":"parent","forked_from_id":"parent","timestamp":"2026-09-10T15:51:27.355Z","source":{"subagent":{"thread_spawn":{"parent_thread_id":"parent"}}}}}`
	const parent = `{"type":"session_meta","payload":{"id":"parent","session_id":"parent","timestamp":"2026-09-10T15:49:48.395Z","source":"exec"}}`
	for _, tc := range []struct{ name, first, second string }{
		{"unrelated identity", child, strings.ReplaceAll(parent, "parent", "unrelated")},
		{"conflicting spawn parent", strings.Replace(child, `"thread_spawn":{"parent_thread_id":"parent"}`, `"thread_spawn":{"parent_thread_id":"other"}`, 1), parent},
		{"conflicting fork parent", strings.Replace(child, `"forked_from_id":"parent"`, `"forked_from_id":"other"`, 1), parent},
		{"newer alleged parent", child, strings.Replace(parent, "15:49:48.395Z", "15:52:48.395Z", 1)},
		{"unknown parent source", child, strings.Replace(parent, `"source":"exec"`, `"source":"unknown"`, 1)},
		{"root identity switch", `{"type":"session_meta","payload":{"id":"child","session_id":"child","source":"exec"}}`, parent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, err := ParseCodexAccounting(strings.NewReader(tc.first + "\n" + tc.second))
			if err != nil {
				t.Fatal(err)
			}
			if a.ID != "child" || len(a.InheritedParentHeaders) != 0 || len(a.Diagnostics) != 1 || !strings.Contains(a.Diagnostics[0].Message, "conflicting session_meta") {
				t.Fatalf("uncorroborated header accepted: %+v", a)
			}
		})
	}
}
