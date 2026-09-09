package sourceread

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// This fixture builds the actual composition root and invokes its production
// policy/reader boundary. The native BD adapter reads synthetic native responses;
// source bytes, source policy, config and output are real files/process streams.
func TestSourceBuiltCommandBoundary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX native BD fixture")
	}
	f := newFixture(t, []byte("CLI_BYTE_CANARY\x00\xff€\n"))
	f.policy.OutputProfile.MaxSerializedBytes = 2048
	if ref := os.Getenv("AO_SOURCE_READ_OBSERVATION"); ref != "" {
		data, err := os.ReadFile(ref)
		if err != nil {
			t.Fatal(err)
		}
		f.policy.OutputProfile.ID = "codex-exec-command-2048-v1"
		f.policy.OutputProfile.ObservationRef = ref
		f.policy.OutputProfile.ObservationSHA256 = digest(data)
	}
	f.savePolicy()
	bin := filepath.Join(f.root, "ao")
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	build := exec.CommandContext(ctx, "go", "build", "-o", bin, "./cmd/ao")
	build.Dir = filepath.Join("..", "..")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("source build: %v %s", err, out)
	}
	nativePath := filepath.Join(f.root, "native-context.json")
	commentsPath := filepath.Join(f.root, "comments.json")
	writeJSONFixture(t, nativePath, f.native.native)
	writeJSONFixture(t, commentsPath, f.native.comments)
	bd := filepath.Join(f.root, "bd")
	script := `#!/bin/sh
case "$5" in
 context) cat "$AO_TEST_NATIVE_CONTEXT" ;;
 show) printf '[{"id":"anchor"}]\n' ;;
 comments) cat "$AO_TEST_NATIVE_COMMENTS" ;;
 *) exit 42 ;;
esac
`
	if err := os.WriteFile(bd, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", f.root+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("AO_TEST_NATIVE_CONTEXT", nativePath)
	t.Setenv("AO_TEST_NATIVE_COMMENTS", commentsPath)
	args := []string{"session", "read-source", "--file", f.options.File, "--access-policy-ref", f.route.AccessPolicyRef, "--source-id", f.options.Context.SourceID, "--project-id", "project", "--owner-scope", "owner", "--task-ref", "task", "--model-ref", "model", "--destination-ref", "destination", "--consumer-root", f.options.Context.ConsumerRoot, "--native-directory", f.options.Context.NativeDirectory, "--start-byte", "0", "--max-bytes", "1", "--json"}
	run := func(args []string) ([]byte, string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, bin, args...)
		var out, diagnostic bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &diagnostic
		err := cmd.Run()
		if ctx.Err() != nil {
			t.Fatalf("reader boundary did not finish; may have opened denied FIFO: %v", ctx.Err())
		}
		return out.Bytes(), diagnostic.String(), err
	}
	out, diagnostic, err := run(args)
	if err != nil {
		t.Fatalf("command fixture: %v %s", err, diagnostic)
	}
	var result Result
	if err = json.Unmarshal(out, &result); err != nil {
		t.Fatal(err)
	}
	raw, err := base64.StdEncoding.DecodeString(result.BytesBase64)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "C" || len(out) > 2048 || result.SerializedBytes != int64(len(out)) || result.AccessEnforcement != "not_attested" || result.CompleteReading || result.HostDelivery != "host-delivery-unverified" {
		t.Fatalf("bad command facts: %s", out)
	}
	t.Logf("source-built CLI emitted %d bytes within selected 2048-byte profile; host delivery remains unverified", len(out))
	if evidence := os.Getenv("AO_SOURCE_READ_FIXTURE_OUTPUT"); evidence != "" {
		if err := os.WriteFile(evidence, out, 0600); err != nil {
			t.Fatal(err)
		}
	}
	// A premature source open would block on this FIFO. Both absent caller
	// context and mismatched task fail promptly without opening it or emitting.
	fifo := filepath.Join(f.root, "denied-fifo")
	if output, err := exec.Command("mkfifo", fifo).CombinedOutput(); err != nil {
		t.Fatalf("FIFO fixture: %v %s", err, output)
	}
	denied := append([]string{}, args...)
	denied[3] = fifo
	for _, tc := range []struct {
		name string
		args []string
	}{{"missing-policy", []string{"session", "read-source", "--file", fifo, "--start-byte", "0", "--max-bytes", "1", "--json"}}, {"unlisted-source", denied}} {
		out, diagnostic, err := run(tc.args)
		if err == nil || len(out) != 0 || strings.Contains(diagnostic, "CLI_BYTE_CANARY") {
			t.Fatalf("%s denial: %v stdout=%s stderr=%s", tc.name, err, out, diagnostic)
		}
	}
	wrongTask := append([]string{}, denied...)
	for i := range wrongTask {
		if wrongTask[i] == "--task-ref" {
			wrongTask[i+1] = "wrong"
		}
	}
	out, diagnostic, err = run(wrongTask)
	if err == nil || len(out) != 0 {
		t.Fatalf("wrong task: %v %s %s", err, out, diagnostic)
	}
}
