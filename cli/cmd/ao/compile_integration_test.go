package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestRunCompile_EndToEnd_FixtureCorpus exercises `ao compile --full`
// against a tiny .agents/ fixture with a PATH-shimmed `claude` binary
// that echoes a canned wiki response. This catches the 2026-04-15
// regression class (embed miss, runtime preflight silently skipped,
// single-giant-prompt on large corpora) at integration level. It would
// have caught the original bug where the embedded compile.sh was missing.
//
// The test creates a self-contained environment:
//   - fresh tmp dir with .agents/{learnings,patterns,research}
//   - 8 source .md files spread across the three dirs
//   - a shell "claude" shim on PATH that returns a canned article response
//   - runtime=claude-cli so the shim is invoked
//   - batch-size=3 so the 8 files split across multiple batches
//
// Assertions:
//   - ao compile --full succeeds
//   - claude shim was invoked (batches > 0)
//   - .agents/compiled/ contains >=1 compiled article
//   - lint-report.md is generated
//   - no "file does not exist" errors
func TestRunCompile_EndToEnd_FixtureCorpus(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test skipped in -short mode")
	}
	if _, err := os.Stat("/bin/bash"); err != nil {
		t.Skip("bash not available; skipping")
	}

	tmp := t.TempDir()

	// Build a tiny corpus across three source dirs.
	sources := map[string]string{
		".agents/learnings/2026-01-01-auth-flow.md":      learningContent("auth-flow", "JWT validation"),
		".agents/learnings/2026-01-02-rate-limits.md":    learningContent("rate-limits", "Token bucket"),
		".agents/learnings/2026-01-03-cache-eviction.md": learningContent("cache-eviction", "LRU"),
		".agents/patterns/pattern-retry-backoff.md":      patternContent("retry-backoff"),
		".agents/patterns/pattern-idempotent-writes.md":  patternContent("idempotent-writes"),
		".agents/research/research-2026-01-observer.md":  researchContent("observer pattern"),
		".agents/research/research-2026-01-saga.md":      researchContent("saga pattern"),
		".agents/research/research-2026-01-cqrs.md":      researchContent("cqrs pattern"),
	}
	for rel, content := range sources {
		full := filepath.Join(tmp, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	// Create a PATH-shim claude binary in a bin/ dir under tmp. The shim
	// reads stdin and prints a canned response in the = ARTICLE = format
	// compile.sh expects, regardless of input. This proves the runtime
	// preflight passes, the script resolves, the batching logic fires,
	// and the parser can round-trip the output.
	shimDir := filepath.Join(tmp, "bin")
	if err := os.MkdirAll(shimDir, 0o755); err != nil {
		t.Fatal(err)
	}
	shimPath := filepath.Join(shimDir, "claude")
	shimScript := `#!/usr/bin/env bash
# Consume stdin so the pipe closes cleanly.
cat >/dev/null
cat <<'CANNED'
=== ARTICLE: auth-and-rate-limits.md ===
---
title: Auth and Rate Limits
tags: [security, rate-limit]
---

# Auth and Rate Limits

Synthesis of [[auth-flow]] and [[rate-limits]].

## Sources

- 2026-01-01-auth-flow.md
- 2026-01-02-rate-limits.md

=== ARTICLE: patterns-overview.md ===
---
title: Patterns Overview
tags: [patterns]
---

# Patterns Overview

Covers [[retry-backoff]] and [[idempotent-writes]].

=== INDEX ===
---
title: Index
---

# Compiled Wiki Index

- [[auth-and-rate-limits]]
- [[patterns-overview]]
CANNED
`
	if err := os.WriteFile(shimPath, []byte(shimScript), 0o755); err != nil {
		t.Fatalf("write claude shim: %v", err)
	}

	// Put the shim first on PATH and restore after test.
	t.Setenv("PATH", shimDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	// Make sure we don't accidentally pick up an API key that would
	// route the bash script down the HTTP path.
	t.Setenv("AGENTOPS_COMPILE_RUNTIME", "claude-cli")
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")

	resetCommandState(t)
	testProjectDir = tmp
	t.Cleanup(func() { testProjectDir = "" })

	// Use the real runCompileScript (no stub) so we exercise the full
	// materialize → exec bash → compile.sh → claude shim path. Keep mine
	// and defrag stubbed so they don't touch real git or lifecycle code.
	origMine := runCompileMineFn
	origDefrag := runCompileDefragFn
	runCompileMineFn = func(string, string, bool) error { return nil }
	runCompileDefragFn = func(string, bool) error { return nil }
	t.Cleanup(func() {
		runCompileMineFn = origMine
		runCompileDefragFn = origDefrag
	})

	compileFull = true
	compileBatchSize = 3
	compileQuiet = true

	cmd := &cobra.Command{}
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetContext(context.Background())

	if err := runCompile(cmd, nil); err != nil {
		t.Fatalf("runCompile: %v\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}

	// Check the compile output landed on disk.
	compiledDir := filepath.Join(tmp, ".agents", "compiled")
	entries, err := os.ReadDir(compiledDir)
	if err != nil {
		t.Fatalf("read compiled dir: %v\nstderr: %s", err, stderr.String())
	}
	foundArticle := false
	foundLint := false
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".md") && e.Name() != "log.md" && e.Name() != "lint-report.md" {
			foundArticle = true
		}
		if e.Name() == "lint-report.md" {
			foundLint = true
		}
	}
	if !foundArticle {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("no compiled article .md in %s; entries=%v\nstderr:\n%s", compiledDir, names, stderr.String())
	}
	if !foundLint {
		t.Errorf("lint-report.md missing from %s\nstderr:\n%s", compiledDir, stderr.String())
	}

	// Stderr must not carry the old regression signature.
	if strings.Contains(stderr.String(), "skills/compile/scripts/compile.sh: file does not exist") {
		t.Errorf("compile script resolution regressed; stderr: %s", stderr.String())
	}
}

func learningContent(slug, topic string) string {
	return fmt.Sprintf(`---
title: %s
tags: [learning]
---

# %s

Learning about %s.
`, slug, slug, topic)
}

func patternContent(slug string) string {
	return fmt.Sprintf(`---
title: %s
tags: [pattern]
---

# %s

Pattern description.
`, slug, slug)
}

func researchContent(topic string) string {
	return fmt.Sprintf(`---
title: research-%s
tags: [research]
---

# Research: %s

Research notes.
`, strings.ReplaceAll(topic, " ", "-"), topic)
}

// ensure io.Writer remains imported for related tests in this file
var _ io.Writer = (*bytes.Buffer)(nil)
