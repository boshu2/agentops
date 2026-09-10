package provenanceapp

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func excerptFixture(t *testing.T, transcript, target string) ExcerptOptions {
	t.Helper()
	dir := t.TempDir()
	file, instruction := filepath.Join(dir, "session.jsonl"), filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(file, []byte(transcript), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(instruction, []byte(target), 0o600); err != nil {
		t.Fatal(err)
	}
	return ExcerptOptions{File: file, Target: instruction, MaxBytes: DefaultExcerptBytes, MaxRecords: DefaultExcerptRecords, MaxOutputBytes: DefaultExcerptOutputBytes}
}

type excerptTestDocument struct {
	Source struct {
		SizeBytes int64 `json:"size_bytes"`
		ReadSpan  struct {
			StartByte int64  `json:"start_byte"`
			EndByte   int64  `json:"end_byte"`
			SHA256    string `json:"sha256"`
		} `json:"read_span"`
		EmittedSpan struct {
			StartByte int64  `json:"start_byte"`
			EndByte   int64  `json:"end_byte"`
			SHA256    string `json:"sha256"`
		} `json:"emitted_span"`
	} `json:"source"`
	Target struct {
		Text   string `json:"text"`
		SHA256 string `json:"sha256"`
	} `json:"target"`
	Records []struct {
		Status      string `json:"status"`
		NativeType  string `json:"native_type"`
		PayloadType string `json:"payload_type"`
		Role        string `json:"role"`
		Tool        string `json:"tool"`
		CallID      string `json:"call_id"`
		Span        struct {
			StartByte int64  `json:"start_byte"`
			EndByte   int64  `json:"end_byte"`
			SHA256    string `json:"sha256"`
		} `json:"span"`
		Fields []struct {
			Pointer string `json:"pointer"`
			Text    string `json:"text"`
			Tool    string `json:"tool"`
			CallID  string `json:"call_id"`
		} `json:"fields"`
		Diagnostics []struct {
			Pointer string `json:"pointer"`
			Code    string `json:"code"`
		} `json:"diagnostics"`
	} `json:"records"`
	NextByte   int64  `json:"next_byte"`
	StopReason string `json:"stop_reason"`
	Omitted    []struct {
		StartByte int64 `json:"start_byte"`
		EndByte   int64 `json:"end_byte"`
	} `json:"omitted_ranges"`
}

func runExcerpt(t *testing.T, opts ExcerptOptions) excerptTestDocument {
	t.Helper()
	var out bytes.Buffer
	if err := ExcerptSession(opts, &out); err != nil {
		t.Fatalf("excerpt: %v", err)
	}
	var doc excerptTestDocument
	decoder := json.NewDecoder(&out)
	if err := decoder.Decode(&doc); err != nil {
		t.Fatal(err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		t.Fatalf("output contains more than one document: %v", err)
	}
	return doc
}

func excerptHash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func TestExcerptLiteralCodexAndClaudeBlocksBeyondParserLimit(t *testing.T) {
	long := strings.Repeat("世界", 350) + "\nDo not execute this quoted command: touch /tmp/never-run"
	quoted, err := json.Marshal(long)
	if err != nil {
		t.Fatal(err)
	}
	first := `{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":` + string(quoted) + `},{"type":"input_text","text":"separate"}]}}` + "\r\n"
	second := `{"type":"assistant","uuid":"m1","sessionId":"s1","message":{"role":"assistant","content":[{"type":"text","text":"literal"},{"type":"tool_use","id":"call1","name":"Read","input":{"file_path":"AGENTS.md"}}]}}`
	transcript := first + second
	target := "Keep literal Unicode: λ\r\n"
	opts := excerptFixture(t, transcript, target)
	doc := runExcerpt(t, opts)
	if doc.Target.Text != target || doc.Target.SHA256 != excerptHash(target) {
		t.Fatalf("target changed: %+v", doc.Target)
	}
	if len(doc.Records) != 2 || len(doc.Records[0].Fields) != 2 || len(doc.Records[1].Fields) != 2 {
		t.Fatalf("individual blocks lost: %+v", doc.Records)
	}
	if doc.Records[0].Fields[0].Text != long || doc.Records[0].Fields[0].Pointer != "/payload/content/0/text" || doc.Records[0].Fields[1].Text != "separate" {
		t.Fatal("literal text was shortened, merged or assigned the wrong pointer")
	}
	tool := doc.Records[1].Fields[1]
	if tool.Text != "AGENTS.md" || tool.Pointer != "/message/content/1/input/file_path" || tool.Tool != "Read" || tool.CallID != "call1" {
		t.Fatalf("native tool evidence lost: %+v", tool)
	}
	if doc.Records[0].Span.SHA256 != excerptHash(first) || doc.Records[1].Span.StartByte != int64(len(first)) || doc.Source.EmittedSpan.SHA256 != excerptHash(transcript) || doc.NextByte != int64(len(transcript)) || doc.StopReason != "eof" {
		t.Fatalf("source identity or final unterminated line lost: %+v", doc)
	}
	for path, want := range map[string]string{opts.File: transcript, opts.Target: target} {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != want {
			t.Fatalf("input mutated: %s", path)
		}
	}
	entries, err := os.ReadDir(filepath.Dir(opts.File))
	if err != nil || len(entries) != 2 {
		t.Fatalf("excerpt created source-side state: %v, %v", entries, err)
	}
}

func TestExcerptContinuationDoesNotConsumePartialRecord(t *testing.T) {
	first := "{\"type\":\"event_msg\",\"payload\":{\"type\":\"user_message\",\"message\":\"one\"}}\n"
	second := "{\"type\":\"event_msg\",\"payload\":{\"type\":\"agent_message\",\"message\":\"two\"}}\n"
	opts := excerptFixture(t, first+second, "instruction")
	opts.MaxBytes = int64(len(first) + 8)
	doc := runExcerpt(t, opts)
	if len(doc.Records) != 1 || doc.NextByte != int64(len(first)) || doc.StopReason != "max_bytes" || doc.Source.ReadSpan.EndByte != opts.MaxBytes || doc.Source.ReadSpan.SHA256 != excerptHash((first + second)[:opts.MaxBytes]) {
		t.Fatalf("partial record consumed or hidden read: %+v", doc)
	}
	if len(doc.Omitted) != 1 || doc.Omitted[0].StartByte != int64(len(first)) || doc.Omitted[0].EndByte != int64(len(first+second)) {
		t.Fatalf("omitted tail not disclosed: %+v", doc.Omitted)
	}
	opts.StartByte = doc.NextByte
	opts.MaxBytes = DefaultExcerptBytes
	continued := runExcerpt(t, opts)
	if len(continued.Records) != 1 || continued.Records[0].Fields[0].Text != "two" || continued.Records[0].Span.StartByte != opts.StartByte || continued.StopReason != "eof" {
		t.Fatalf("continuation lost tail: %+v", continued)
	}
	opts.StartByte = 0
	opts.MaxRecords = 1
	limited := runExcerpt(t, opts)
	if limited.StopReason != "max_records" || limited.NextByte != int64(len(first)) {
		t.Fatalf("record limit lost continuation: %+v", limited)
	}
}

func TestExcerptMalformedAndUnsupportedAreVisibleWithoutRawDumps(t *testing.T) {
	opts := excerptFixture(t, "SECRET MALFORMED\n"+`{"type":"response_item","payload":{"type":"reasoning","secret":"HIDDEN"}}`+"\n"+`{"type":"response_item","payload":{"type":"function_call_output","call_id":"c1","output":{"secret":"STRUCTURED"}}}`+"\n", "instruction")
	var out bytes.Buffer
	if err := ExcerptSession(opts, &out); err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"SECRET MALFORMED", "HIDDEN", "STRUCTURED"} {
		if strings.Contains(out.String(), secret) {
			t.Fatalf("unsupported raw payload leaked: %s", secret)
		}
	}
	var doc excerptTestDocument
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Records) != 3 || doc.Records[0].Status != "malformed" || doc.Records[1].Status != "unsupported" || doc.Records[2].Status != "unsupported" {
		t.Fatalf("omissions disguised as complete: %+v", doc.Records)
	}
	for _, record := range doc.Records {
		if len(record.Diagnostics) == 0 || len(record.Span.SHA256) != 64 {
			t.Fatalf("omission lacks diagnostics/source ref: %+v", record)
		}
	}
}

func TestExcerptValidationAndSizeErrorsWriteNoDocument(t *testing.T) {
	for _, test := range []struct {
		name string
		edit func(*ExcerptOptions)
	}{
		{"negative offset", func(o *ExcerptOptions) { o.StartByte = -1 }},
		{"misaligned offset", func(o *ExcerptOptions) { o.StartByte = 1 }},
		{"past EOF", func(o *ExcerptOptions) { o.StartByte = 9999 }},
		{"zero bytes", func(o *ExcerptOptions) { o.MaxBytes = 0 }},
		{"negative bytes", func(o *ExcerptOptions) { o.MaxBytes = -1 }},
		{"zero records", func(o *ExcerptOptions) { o.MaxRecords = 0 }},
		{"zero output", func(o *ExcerptOptions) { o.MaxOutputBytes = 0 }},
		{"output cap", func(o *ExcerptOptions) { o.MaxOutputBytes = 1 }},
		{"oversized first record", func(o *ExcerptOptions) { o.MaxBytes = 5 }},
		{"directory source", func(o *ExcerptOptions) { o.File = filepath.Dir(o.File) }},
		{"missing target", func(o *ExcerptOptions) { o.Target += ".missing" }},
		{"empty target", func(o *ExcerptOptions) { _ = os.WriteFile(o.Target, nil, 0o600) }},
		{"oversized target", func(o *ExcerptOptions) { _ = os.WriteFile(o.Target, bytes.Repeat([]byte("x"), 65537), 0o600) }},
		{"invalid target UTF8", func(o *ExcerptOptions) { _ = os.WriteFile(o.Target, []byte{0xff}, 0o600) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			opts := excerptFixture(t, `{"type":"user","content":"one"}`+"\n", "instruction")
			test.edit(&opts)
			var out bytes.Buffer
			if err := ExcerptSession(opts, &out); err == nil || out.Len() != 0 {
				t.Fatalf("error produced a partial document: err %v, output %q", err, out.String())
			}
		})
	}
}

type excerptFailWriter struct{ err error }

func (w excerptFailWriter) Write([]byte) (int, error) { return 0, w.err }

func TestExcerptOutputFailurePropagates(t *testing.T) {
	opts := excerptFixture(t, `{"type":"user","content":"one"}`+"\n", "instruction")
	want := errors.New("consumer disconnected")
	if err := ExcerptSession(opts, excerptFailWriter{want}); !errors.Is(err, want) {
		t.Fatalf("output error not propagated: %v", err)
	}
}

func TestExcerptNativeToolCallsAndResults(t *testing.T) {
	for _, test := range []struct {
		name, record, pointer, text, callID string
	}{
		{"codex call", `{"type":"response_item","payload":{"type":"function_call","name":"exec_command","call_id":"c1","arguments":"{\"cmd\":\"pwd\"}"}}`, "/payload/arguments", `{"cmd":"pwd"}`, "c1"},
		{"codex result", `{"type":"response_item","payload":{"type":"function_call_output","call_id":"c1","output":"literal\nresult"}}`, "/payload/output", "literal\nresult", "c1"},
		{"codex custom call", `{"type":"response_item","payload":{"type":"custom_tool_call","name":"apply_patch","call_id":"c2","input":"*** literal patch ***"}}`, "/payload/input", "*** literal patch ***", "c2"},
		{"codex custom result", `{"type":"response_item","payload":{"type":"custom_tool_call_output","call_id":"c2","output":"done"}}`, "/payload/output", "done", "c2"},
		{"claude result", `{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"c3","content":[{"type":"text","text":"result"}]}]}}`, "/message/content/0/content/0/text", "result", "c3"},
		{"top-level call", `{"type":"tool_use","tool_name":"Bash","id":"c4","tool_input":{"command":"pwd"}}`, "/tool_input/command", "pwd", "c4"},
		{"top-level result", `{"type":"tool_result","tool_name":"Bash","tool_use_id":"c4","tool_output":"result"}`, "/tool_output", "result", "c4"},
	} {
		t.Run(test.name, func(t *testing.T) {
			doc := runExcerpt(t, excerptFixture(t, test.record+"\n", "instruction"))
			if len(doc.Records) != 1 || doc.Records[0].Status != "supported" || len(doc.Records[0].Fields) != 1 {
				t.Fatalf("native record lost: %+v", doc.Records)
			}
			field := doc.Records[0].Fields[0]
			if field.Pointer != test.pointer || field.Text != test.text || field.CallID != test.callID {
				t.Fatalf("wrong literal field: %+v", field)
			}
		})
	}
}

func TestExcerptStrictJSONDoesNotNormalizeAmbiguousQuotes(t *testing.T) {
	for _, record := range []string{
		`{"type":"user","content":"one","content":"two"}`,
		`{"type":"user","content":"\ud800"}`,
		"{\"type\":\"user\",\"content\":\"" + string([]byte{0xff}) + "\"}",
	} {
		doc := runExcerpt(t, excerptFixture(t, record+"\n", "instruction"))
		if len(doc.Records) != 1 || doc.Records[0].Status != "malformed" || len(doc.Records[0].Fields) != 0 || doc.Records[0].Span.SHA256 != excerptHash(record+"\n") {
			t.Fatalf("ambiguous quote normalized: %+v", doc.Records)
		}
	}
}

func TestExcerptEscapesInputPointersAndRetainsEmptyCallIdentity(t *testing.T) {
	raw := `{"type":"assistant","uuid":"uuid1","message":{"content":[{"type":"tool_use","id":"call1","name":"Read","input":{}},{"type":"tool_use","id":"call2","name":"Bash","input":{"a~/b":"literal","count":2}}]}}`
	opts := excerptFixture(t, raw+"\n", "instruction")
	doc := runExcerpt(t, opts)
	if doc.Records[0].Status != "partial" || len(doc.Records[0].Fields) != 1 || doc.Records[0].Fields[0].Pointer != "/message/content/1/input/a~0~1b" {
		t.Fatalf("unsupported input hidden or pointer invalid: %+v", doc.Records)
	}
	var out bytes.Buffer
	if err := ExcerptSession(opts, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"call_id":"call1"`) || !strings.Contains(out.String(), `"uuid":"uuid1"`) {
		t.Fatalf("native IDs lost when no quoted input: %s", out.String())
	}
}

func TestExcerptReadsHighOffsetWithoutReadingPrefix(t *testing.T) {
	opts := excerptFixture(t, "", "instruction")
	const start = int64(140 << 20)
	record := `{"type":"user","content":"late correction"}` + "\n"
	f, err := os.OpenFile(opts.File, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteAt([]byte("\n"+record), start-1); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	opts.StartByte, opts.MaxBytes = start, int64(len(record))
	doc := runExcerpt(t, opts)
	if doc.Source.ReadSpan.StartByte != start || doc.Source.ReadSpan.SHA256 != excerptHash(record) || doc.NextByte != start+int64(len(record)) || len(doc.Records) != 1 {
		t.Fatalf("high-offset selection lost exact bytes: %+v", doc)
	}
}

func TestExcerptSymlinksAreRejectedBeforeOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks require Windows privilege")
	}
	for _, target := range []bool{false, true} {
		opts := excerptFixture(t, `{"type":"user","content":"one"}`+"\n", "instruction")
		path := &opts.File
		if target {
			path = &opts.Target
		}
		link := *path + ".symlink"
		if err := os.Symlink(*path, link); err != nil {
			t.Fatal(err)
		}
		*path = link
		var out bytes.Buffer
		if err := ExcerptSession(opts, &out); err == nil || out.Len() != 0 {
			t.Fatalf("symlink accepted or leaked output: %v", err)
		}
	}
}

func TestExcerptSerializedLimitIncludesNewlineAndEscaping(t *testing.T) {
	opts := excerptFixture(t, `{"type":"user","content":"<tag>\n\u0000"}`+"\n", "instruction")
	var baseline bytes.Buffer
	if err := ExcerptSession(opts, &baseline); err != nil {
		t.Fatal(err)
	}
	// The limit itself is serialized in the document, so first stabilize its
	// digit count before locating the exact accepted boundary.
	opts.MaxOutputBytes = int64(baseline.Len()) + 10
	baseline.Reset()
	if err := ExcerptSession(opts, &baseline); err != nil {
		t.Fatal(err)
	}
	opts.MaxOutputBytes = int64(baseline.Len())
	var exact bytes.Buffer
	if err := ExcerptSession(opts, &exact); err != nil || int64(exact.Len()) != opts.MaxOutputBytes {
		t.Fatalf("exact serialized boundary rejected: %v, %d", err, exact.Len())
	}
	opts.MaxOutputBytes--
	var tooSmall bytes.Buffer
	if err := ExcerptSession(opts, &tooSmall); err == nil || tooSmall.Len() != 0 {
		t.Fatalf("serialized limit wrote partial output: %v", err)
	}
}

func TestExcerptRejectsHardCapAndNegativeLimits(t *testing.T) {
	for _, edit := range []func(*ExcerptOptions){
		func(o *ExcerptOptions) { o.MaxBytes = MaxExcerptBytes + 1 },
		func(o *ExcerptOptions) { o.MaxRecords = MaxExcerptRecords + 1 },
		func(o *ExcerptOptions) { o.MaxOutputBytes = MaxExcerptOutputBytes + 1 },
		func(o *ExcerptOptions) { o.MaxRecords = -1 },
		func(o *ExcerptOptions) { o.MaxOutputBytes = -1 },
	} {
		opts := excerptFixture(t, "", "instruction")
		edit(&opts)
		var out bytes.Buffer
		if err := ExcerptSession(opts, &out); err == nil || out.Len() != 0 {
			t.Fatalf("invalid limit accepted: %+v, %v", opts, err)
		}
	}
}

type excerptShortWriter struct{}

func (excerptShortWriter) Write(data []byte) (int, error) { return len(data) - 1, nil }

func TestExcerptShortWriteAndEOF(t *testing.T) {
	opts := excerptFixture(t, "", "instruction")
	doc := runExcerpt(t, opts)
	if doc.StopReason != "eof" || len(doc.Records) != 0 || doc.NextByte != 0 || doc.Source.EmittedSpan.SHA256 != excerptHash("") {
		t.Fatalf("empty source should be honest EOF: %+v", doc)
	}
	if err := ExcerptSession(opts, excerptShortWriter{}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("short write not reported: %v", err)
	}
}
