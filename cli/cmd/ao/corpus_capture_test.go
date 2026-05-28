// practices: [tdd]
package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/ports"
)

func TestCorpusCapture_EmptyPathRejected(t *testing.T) {
	err := corpusCaptureRun(context.Background(), corpusCaptureOptions{})
	if err == nil {
		t.Fatal("expected error on empty path")
	}
	if !strings.Contains(err.Error(), "--path required") {
		t.Fatalf("error not informative: %v", err)
	}
}

func TestCorpusCapture_BodyRequired(t *testing.T) {
	err := corpusCaptureRun(context.Background(), corpusCaptureOptions{
		path: "x.md",
	})
	if err == nil {
		t.Fatal("expected error on no body source")
	}
	if !strings.Contains(err.Error(), "body source required") {
		t.Fatalf("error not informative: %v", err)
	}
}

func TestCorpusCapture_MultipleBodySourcesRejected(t *testing.T) {
	err := corpusCaptureRun(context.Background(), corpusCaptureOptions{
		path:     "x.md",
		body:     "inline",
		bodyFile: "f.md",
	})
	if err == nil {
		t.Fatal("expected error on multiple body sources")
	}
}

func TestCorpusCapture_InlineBodyToStub(t *testing.T) {
	var gotPath string
	var gotBody []byte
	stub := func(_ context.Context, opts corpusCaptureOptions, body []byte, _ map[string]string) (ports.CorpusWriteResult, error) {
		gotPath = opts.path
		gotBody = body
		return ports.CorpusWriteResult{ResolvedPath: "/r/" + opts.path, Created: true}, nil
	}
	var buf bytes.Buffer
	err := corpusCaptureRun(context.Background(), corpusCaptureOptions{
		path:      "notes/x.md",
		body:      "hello world",
		writer:    &buf,
		captureFn: stub,
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "notes/x.md" || string(gotBody) != "hello world" {
		t.Fatalf("stub got %q / %q", gotPath, string(gotBody))
	}
	if !strings.Contains(buf.String(), "created /r/notes/x.md") {
		t.Fatalf("missing created confirmation: %q", buf.String())
	}
}

func TestCorpusCapture_StdinBody(t *testing.T) {
	stub := func(_ context.Context, _ corpusCaptureOptions, body []byte, _ map[string]string) (ports.CorpusWriteResult, error) {
		if string(body) != "from stdin" {
			t.Fatalf("body = %q, want 'from stdin'", body)
		}
		return ports.CorpusWriteResult{ResolvedPath: "ok", Created: false}, nil
	}
	var buf bytes.Buffer
	err := corpusCaptureRun(context.Background(), corpusCaptureOptions{
		path:      "x.md",
		bodyStdin: true,
		stdin:     strings.NewReader("from stdin"),
		writer:    &buf,
		captureFn: stub,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "updated ok") {
		t.Fatalf("missing updated label: %q", buf.String())
	}
}

func TestCorpusCapture_MetaParsed(t *testing.T) {
	var gotMeta map[string]string
	stub := func(_ context.Context, _ corpusCaptureOptions, _ []byte, meta map[string]string) (ports.CorpusWriteResult, error) {
		gotMeta = meta
		return ports.CorpusWriteResult{ResolvedPath: "ok", Created: true}, nil
	}
	err := corpusCaptureRun(context.Background(), corpusCaptureOptions{
		path:      "x.md",
		body:      "body",
		meta:      []string{"tag=evolve", "date=2026-05-13"},
		captureFn: stub,
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotMeta["tag"] != "evolve" || gotMeta["date"] != "2026-05-13" {
		t.Fatalf("meta wrong: %+v", gotMeta)
	}
}

func TestCorpusCapture_MalformedMetaRejected(t *testing.T) {
	stub := func(_ context.Context, _ corpusCaptureOptions, _ []byte, _ map[string]string) (ports.CorpusWriteResult, error) {
		return ports.CorpusWriteResult{}, nil
	}
	err := corpusCaptureRun(context.Background(), corpusCaptureOptions{
		path:      "x.md",
		body:      "body",
		meta:      []string{"notakeyvalue"},
		captureFn: stub,
	})
	if err == nil {
		t.Fatal("expected error on malformed meta")
	}
	if !strings.Contains(err.Error(), "expected key=value") {
		t.Fatalf("error not informative: %v", err)
	}
}

func TestCorpusCapture_BodyFileReadsContent(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "in.md")
	_ = os.WriteFile(fp, []byte("file body content"), 0o644)

	var gotBody []byte
	stub := func(_ context.Context, _ corpusCaptureOptions, body []byte, _ map[string]string) (ports.CorpusWriteResult, error) {
		gotBody = body
		return ports.CorpusWriteResult{ResolvedPath: "ok", Created: true}, nil
	}
	err := corpusCaptureRun(context.Background(), corpusCaptureOptions{
		path:      "x.md",
		bodyFile:  fp,
		captureFn: stub,
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(gotBody) != "file body content" {
		t.Fatalf("body = %q", gotBody)
	}
}

func TestCorpusCapture_StubErrorWrapped(t *testing.T) {
	stub := func(_ context.Context, _ corpusCaptureOptions, _ []byte, _ map[string]string) (ports.CorpusWriteResult, error) {
		return ports.CorpusWriteResult{}, errors.New("disk full")
	}
	err := corpusCaptureRun(context.Background(), corpusCaptureOptions{
		path:      "x.md",
		body:      "body",
		captureFn: stub,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "corpus capture:") {
		t.Fatalf("error not wrapped: %v", err)
	}
}
