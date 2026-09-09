package sourceread

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/config"
)

type nativeFixture struct {
	native   config.NativeContext
	comments []config.AnchorComment
	err      error
	reads    int
}

func (n *nativeFixture) NativeContext(context.Context, string) (config.NativeContext, error) {
	return n.native, n.err
}
func (n *nativeFixture) AnchorComments(context.Context, string, string) ([]config.AnchorComment, error) {
	n.reads++
	return n.comments, n.err
}

type fixture struct {
	t       *testing.T
	service Service
	options Options
	policy  Policy
	route   config.ContextConfig
	native  *nativeFixture
	root    string
}

func newFixture(t *testing.T, content []byte) *fixture {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dir := func(name string) string {
		p := filepath.Join(root, name)
		if err := os.Mkdir(p, 0700); err != nil {
			t.Fatal(err)
		}
		return p
	}
	source, consumer, bundle, evidence, stage := dir("native"), dir("consumer"), dir("bundle"), dir("evidence"), dir("stage")
	r := config.ContextConfig{SourceID: source, ProjectID: "project", OwnerScope: "owner", BundleID: "bundle", BundleRoot: bundle, EvidenceRoot: evidence, StagingRoot: stage, AccessPolicyRef: filepath.Join(root, "access.json"), OwnerPolicyRef: filepath.Join(root, "owner.json"), TaskPolicyRef: filepath.Join(root, "task.json"), ModelPolicyRef: filepath.Join(root, "model.json"), DestinationPolicyRef: filepath.Join(root, "destination.json"), MaintenanceWorkRef: "anchor"}
	for _, p := range []string{r.OwnerPolicyRef, r.ModelPolicyRef, r.DestinationPolicyRef} {
		if err := os.WriteFile(p, []byte("{}"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	file := filepath.Join(root, "source.jsonl")
	if err := os.WriteFile(file, content, 0600); err != nil {
		t.Fatal(err)
	}
	observation := filepath.Join(root, "observation.json")
	observed := []byte(`{"fixture_only":true,"serialized_output_observation":"synthetic test"}`)
	if err := os.WriteFile(observation, observed, 0600); err != nil {
		t.Fatal(err)
	}
	p := Policy{SchemaVersion: "source-read-policy.v1", SourceID: source, ProjectID: "project", OwnerScope: "owner", TaskRef: "task", ModelRef: "model", DestinationRef: "destination", Files: []SourcePermission{{Path: file, ContentScope: "synthetic"}}, OutputProfile: OutputProfile{ID: "synthetic-fixture-only", ModelRef: "model", DestinationRef: "destination", MaxSerializedBytes: 4096, ObservationRef: observation, ObservationSHA256: digest(observed)}}
	access := config.ContextPolicy{SchemaVersion: 1, SourceID: source, ProjectID: "project", OwnerScope: "owner", TaskRef: "task", ModelRef: "model", DestinationRef: "destination", BundleID: r.BundleID, BundleRoot: r.BundleRoot, EvidenceRoot: r.EvidenceRoot, StagingRoot: r.StagingRoot, OwnerPolicyRef: r.OwnerPolicyRef, TaskPolicyRef: r.TaskPolicyRef, ModelPolicyRef: r.ModelPolicyRef, DestinationPolicyRef: r.DestinationPolicyRef, MaintenanceWorkRef: r.MaintenanceWorkRef}
	writeJSONFixture(t, r.AccessPolicyRef, access)
	configPath := filepath.Join(root, "config.yaml")
	writeJSONFixture(t, configPath, map[string]any{"context": r})
	t.Setenv("AGENTOPS_CONFIG", configPath)
	for _, k := range []string{"SOURCE_ID", "PROJECT_ID", "OWNER_SCOPE", "BUNDLE_ID", "BUNDLE_ROOT", "EVIDENCE_ROOT", "STAGING_ROOT", "ACCESS_POLICY_REF", "OWNER_POLICY_REF", "TASK_POLICY_REF", "MODEL_POLICY_REF", "DESTINATION_POLICY_REF", "MAINTENANCE_WORK_REF"} {
		t.Setenv("AGENTOPS_CONTEXT_"+k, "")
	}
	fact, _ := json.Marshal(config.ContextFact{Type: "context.route.v1", FactID: "route", Route: &r})
	native := &nativeFixture{native: config.NativeContext{BDVersion: "1.2.2", SchemaVersion: 1, Backend: "dolt", ProjectID: "project", BeadsDir: source}, comments: []config.AnchorComment{{ID: "1", IssueID: "anchor", Text: string(fact)}}}
	f := &fixture{t: t, root: root, route: r, policy: p, native: native, service: Service{Gateway: native}, options: Options{File: file, StartByte: 0, MaxBytes: 16, Context: config.ContextRequest{SourceID: source, ProjectID: "project", OwnerScope: "owner", TaskRef: "task", ModelRef: "model", DestinationRef: "destination", ConsumerRoot: consumer, NativeDirectory: consumer, Overrides: config.ContextConfig{AccessPolicyRef: r.AccessPolicyRef}}}}
	f.savePolicy()
	return f
}
func writeJSONFixture(t *testing.T, p string, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(p, b, 0600); err != nil {
		t.Fatal(err)
	}
}
func (f *fixture) savePolicy() { writeJSONFixture(f.t, f.route.TaskPolicyRef, f.policy) }
func (f *fixture) run() (Result, []byte, error) {
	var out bytes.Buffer
	err := f.service.Run(context.Background(), f.options, &out)
	var r Result
	if err == nil {
		if decErr := json.Unmarshal(out.Bytes(), &r); decErr != nil {
			f.t.Fatal(decErr)
		}
	}
	return r, out.Bytes(), err
}
func (f *fixture) success() Result {
	f.t.Helper()
	r, raw, err := f.run()
	if err != nil {
		f.t.Fatal(err)
	}
	if r.SerializedBytes != int64(len(raw)) {
		f.t.Fatalf("serialized size %d != %d", r.SerializedBytes, len(raw))
	}
	if r.HostDelivery != "host-delivery-unverified" || r.CompleteReading || r.SemanticProcessing != "not-established" {
		f.t.Fatalf("unsupported coverage: %+v", r)
	}
	return r
}
func TestRawSpanAndFrozenAppend(t *testing.T) {
	original := []byte("operator correction\n{\"tool\":\"same\"}\n\u2028\u2029\n")
	f := newFixture(t, original)
	f.options.StartByte = 3
	f.options.MaxBytes = 11
	r := f.success()
	raw, err := base64.StdEncoding.DecodeString(r.BytesBase64)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw, original[3:14]) || r.StartByte != 3 || r.EndByte != 14 || r.NextByte != 14 || r.CapturedThrough != int64(len(original)) || r.PrefixSHA256 != digest(original) || r.SpanSHA256 != digest(original[3:14]) {
		t.Fatalf("incorrect raw facts: %+v", r)
	}
	f.options.ThroughByte = &r.CapturedThrough
	f.options.ExpectPrefixSHA256 = r.PrefixSHA256
	f.options.ExpectFileIdentity = r.Before.Identity
	file, err := os.OpenFile(f.options.File, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.WriteString("new tail"); err != nil {
		t.Fatal(err)
	}
	file.Close()
	appended := f.success()
	if appended.PrefixSHA256 != r.PrefixSHA256 || appended.CapturedThrough != r.CapturedThrough || appended.After.Size <= r.After.Size {
		t.Fatal("append changed frozen prefix")
	}
}
func TestOperatorCorrectionBetweenIdenticalToolCalls(t *testing.T) {
	a := []byte("{\"tool\":\"same\"}\n{\"user\":\"correct A\"}\n{\"tool\":\"same\"}\n")
	f := newFixture(t, a)
	r := f.success()
	f.options.ThroughByte = &r.CapturedThrough
	f.options.ExpectPrefixSHA256 = r.PrefixSHA256
	b := bytes.Replace(a, []byte("correct A"), []byte("correct B"), 1)
	if err := os.WriteFile(f.options.File, b, 0600); err != nil {
		t.Fatal(err)
	}
	_, raw, err := f.run()
	if err == nil || !strings.Contains(err.Error(), "prefix changed") || len(raw) != 0 {
		t.Fatalf("correction accepted: %v %s", err, raw)
	}
}
func TestInvalidAndSplitUTF8RemainReversible(t *testing.T) {
	for _, tc := range []struct {
		name       string
		data       []byte
		start, max int64
	}{{"invalid", []byte{0xff, 0x00, 'x', 0xfe}, 0, 4}, {"split", []byte("A€B"), 2, 1}, {"separators", []byte("x\u2028y\u2029z\n"), 0, 20}} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, tc.data)
			f.options.StartByte = tc.start
			f.options.MaxBytes = tc.max
			r := f.success()
			raw, err := base64.StdEncoding.DecodeString(r.BytesBase64)
			if err != nil {
				t.Fatal(err)
			}
			end := min(int64(len(tc.data)), tc.start+tc.max)
			if !bytes.Equal(raw, tc.data[tc.start:end]) {
				t.Fatal("bytes repaired or lost")
			}
			if tc.name != "separators" && !strings.Contains(r.TextViewEncoding, "replacement-view") {
				t.Fatal("unlabelled lossy text")
			}
		})
	}
}
func TestBounds(t *testing.T) {
	for _, tc := range []struct {
		name       string
		start, max int64
		hash       string
	}{{"zero", 0, 0, ""}, {"negative-max", 0, -1, ""}, {"negative-start", -1, 1, ""}, {"huge", 0, math.MaxInt64, ""}, {"unpaired-digest", 0, 1, strings.Repeat("0", 64)}, {"beyond-source", 99, 1, ""}} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, []byte("abc"))
			f.options.StartByte = tc.start
			f.options.MaxBytes = tc.max
			f.options.ExpectPrefixSHA256 = tc.hash
			_, out, err := f.run()
			if err == nil || len(out) > 0 {
				t.Fatalf("invalid bounds emitted %s, %v", out, err)
			}
		})
	}
	t.Run("empty-source", func(t *testing.T) {
		f := newFixture(t, nil)
		r := f.success()
		if r.CapturedThrough != 0 || r.EndByte != 0 || r.PrefixSHA256 != digest(nil) {
			t.Fatal("empty source facts invalid")
		}
	})
	t.Run("end-of-prefix", func(t *testing.T) {
		f := newFixture(t, []byte("abc"))
		r := f.success()
		f.options.StartByte = 3
		f.options.ThroughByte = &r.CapturedThrough
		f.options.ExpectPrefixSHA256 = r.PrefixSHA256
		if f.success().BytesBase64 != "" {
			t.Fatal("nonempty terminal span")
		}
	})
}
func TestSerializedProfileAndOversize(t *testing.T) {
	f := newFixture(t, bytes.Repeat([]byte("\x00"), 1000))
	f.options.MaxBytes = 1000
	_, out, err := f.run()
	if err == nil || !strings.Contains(err.Error(), "serialized output exceeds") || len(out) != 0 {
		t.Fatalf("serialized overflow missed: %v (%d bytes)", err, len(out))
	}
	f.options.AllowOversize = true
	r := f.success()
	if r.ProfileBoundSatisfied || !r.OversizeOverride {
		t.Fatal("oversize not disclosed")
	}
	f.options.MaxBytes = math.MaxInt64
	r = f.success()
	if r.EndByte != 1000 {
		t.Fatal("oversize arithmetic overflow")
	}
	f.options.MaxBytes = 1
	r = f.success()
	if !r.OversizeOverride {
		t.Fatal("explicit override lost")
	}
}
func TestDeniedPolicyNeverOpensSource(t *testing.T) {
	cases := map[string]func(*fixture){
		"missing-access":       func(f *fixture) { f.options.Context.Overrides.AccessPolicyRef = "" },
		"missing-owner":        func(f *fixture) { f.options.Context.OwnerScope = "" },
		"wrong-source":         func(f *fixture) { f.options.Context.SourceID = "/other-native" },
		"wrong-project":        func(f *fixture) { f.options.Context.ProjectID = "wrong" },
		"wrong-task":           func(f *fixture) { f.options.Context.TaskRef = "wrong" },
		"wrong-model":          func(f *fixture) { f.options.Context.ModelRef = "wrong" },
		"wrong-destination":    func(f *fixture) { f.options.Context.DestinationRef = "wrong" },
		"unlisted-file":        func(f *fixture) { f.policy.Files = nil; f.savePolicy() },
		"restricted":           func(f *fixture) { f.policy.Files[0].ContentScope = "restricted"; f.savePolicy() },
		"bad-profile":          func(f *fixture) { f.policy.OutputProfile.MaxSerializedBytes = 0; f.savePolicy() },
		"observation-tampered": func(f *fixture) { os.WriteFile(f.policy.OutputProfile.ObservationRef, []byte("changed"), 0600) },
		"policy-wrong-model":   func(f *fixture) { f.policy.ModelRef = "wrong"; f.savePolicy() },
		"anchor-unavailable":   func(f *fixture) { f.native.err = fmt.Errorf("unavailable") },
		"native-mismatch":      func(f *fixture) { f.native.native.ProjectID = "other" },
		"bad-json": func(f *fixture) {
			os.WriteFile(f.route.TaskPolicyRef, []byte(`{"schema_version":"source-read-policy.v1","schema_version":"source-read-policy.v1"}`), 0600)
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t, []byte("DENIED_SOURCE_CANARY"))
			opened := false
			f.service.openSource = func(p string) (*os.File, error) { opened = true; return os.Open(p) }
			change(f)
			_, out, err := f.run()
			if err == nil || opened || len(out) != 0 || strings.Contains(err.Error(), "DENIED_SOURCE_CANARY") {
				t.Fatalf("denial leaked or opened: opened=%v err=%v out=%s", opened, err, out)
			}
		})
	}
}
func TestReplacementAndShortReads(t *testing.T) {
	t.Run("same-content-new-inode", func(t *testing.T) {
		f := newFixture(t, []byte("abc"))
		r := f.success()
		replacement := filepath.Join(f.root, "replacement")
		os.WriteFile(replacement, []byte("abc"), 0600)
		os.Rename(replacement, f.options.File)
		f.options.ExpectFileIdentity = r.Before.Identity
		_, out, err := f.run()
		if err == nil || len(out) > 0 {
			t.Fatal("replacement accepted")
		}
	})
	t.Run("replace-at-open", func(t *testing.T) {
		f := newFixture(t, []byte("abc"))
		f.service.openSource = func(p string) (*os.File, error) {
			replacement := filepath.Join(f.root, "replacement")
			os.WriteFile(replacement, []byte("abc"), 0600)
			os.Rename(replacement, p)
			return os.Open(p)
		}
		_, out, err := f.run()
		if err == nil || len(out) > 0 {
			t.Fatal("open replacement accepted")
		}
	})
	t.Run("replace-during-read", func(t *testing.T) {
		f := newFixture(t, []byte("abc"))
		f.service.afterRead = func() {
			p := filepath.Join(f.root, "replacement")
			os.WriteFile(p, []byte("abc"), 0600)
			os.Rename(p, f.options.File)
		}
		_, out, err := f.run()
		if err == nil || len(out) > 0 {
			t.Fatal("read replacement accepted")
		}
	})
	t.Run("short-read", func(t *testing.T) {
		f := newFixture(t, []byte("abcdef"))
		r := f.success()
		f.options.ThroughByte = &r.CapturedThrough
		f.options.ExpectPrefixSHA256 = r.PrefixSHA256
		os.Truncate(f.options.File, 2)
		_, out, err := f.run()
		if err == nil || len(out) > 0 {
			t.Fatal("truncated frozen source accepted")
		}
	})
	t.Run("truncate-during-read", func(t *testing.T) {
		f := newFixture(t, []byte("abcdef"))
		f.service.afterRead = func() { os.Truncate(f.options.File, 2) }
		_, out, err := f.run()
		if err == nil || len(out) > 0 {
			t.Fatal("concurrent truncate accepted")
		}
	})
	t.Run("rewrite-during-read", func(t *testing.T) {
		f := newFixture(t, []byte("abcdef"))
		f.service.afterRead = func() { os.WriteFile(f.options.File, []byte("abcxef"), 0600) }
		_, out, err := f.run()
		if err == nil || len(out) > 0 {
			t.Fatal("concurrent correction accepted")
		}
	})
	t.Run("append-during-read", func(t *testing.T) {
		f := newFixture(t, []byte("abc"))
		f.service.afterRead = func() {
			h, _ := os.OpenFile(f.options.File, os.O_WRONLY|os.O_APPEND, 0600)
			h.WriteString("new tail")
			h.Close()
		}
		r := f.success()
		if r.CapturedThrough != 3 {
			t.Fatal("append enlarged denominator")
		}
	})
	t.Run("symlink", func(t *testing.T) {
		f := newFixture(t, []byte("abc"))
		p := filepath.Join(f.root, "link")
		os.Symlink(f.options.File, p)
		f.options.File = p
		f.policy.Files[0].Path = p
		f.savePolicy()
		_, out, err := f.run()
		if err == nil || len(out) > 0 {
			t.Fatal("source alias accepted")
		}
	})
}

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) { return len(p) / 2, nil }
func TestOutputShortWriteIsUnverified(t *testing.T) {
	f := newFixture(t, []byte("abc"))
	err := f.service.Run(context.Background(), f.options, shortWriter{})
	if err == nil || !strings.Contains(err.Error(), "delivery unverified") {
		t.Fatalf("short write: %v", err)
	}
}
func TestCanceledReadEmitsNothing(t *testing.T) {
	f := newFixture(t, []byte("abc"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out bytes.Buffer
	err := f.service.Run(ctx, f.options, &out)
	if err == nil || out.Len() > 0 {
		t.Fatal("canceled read emitted source")
	}
}

func TestFrozenBoundArguments(t *testing.T) {
	for _, tc := range []struct {
		name     string
		through  int64
		expected string
		start    int64
	}{{"negative", -1, strings.Repeat("0", 64), 0}, {"missing-digest", 1, "", 0}, {"invalid-digest", 1, "NOT_A_SHA256", 0}, {"start-after-boundary", 1, strings.Repeat("0", 64), 2}, {"truncated", 4, digest([]byte("abcd")), 0}} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, []byte("abc"))
			f.options.ThroughByte = &tc.through
			f.options.ExpectPrefixSHA256 = tc.expected
			f.options.StartByte = tc.start
			_, out, err := f.run()
			if err == nil || len(out) > 0 {
				t.Fatalf("frozen argument admitted: %v %s", err, out)
			}
		})
	}
	t.Run("zero-boundary", func(t *testing.T) {
		f := newFixture(t, []byte("abc"))
		zero := int64(0)
		f.options.ThroughByte = &zero
		f.options.ExpectPrefixSHA256 = digest(nil)
		r := f.success()
		if r.EndByte != 0 || r.PrefixSHA256 != digest(nil) {
			t.Fatal("zero frozen boundary changed")
		}
	})
}
