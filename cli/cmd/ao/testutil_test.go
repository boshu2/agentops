// practices: [tdd, pragmatic-programmer]
package main

// testutil_test.go consolidates test helper functions that are shared across
// multiple test files in package main. Each helper was originally defined in a
// single file but called from others; moving them here avoids duplication and
// makes cross-file dependencies explicit.

import (
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/spf13/pflag"
)

// packageDir holds the absolute path to the test package directory, captured
// at init time before any test can call os.Chdir. Use this as the base for
// resolving testdata/ paths so that concurrent os.Chdir calls in other tests
// cannot break fixture loading.
var packageDir string

func init() {
	var err error
	packageDir, err = os.Getwd()
	if err != nil {
		panic("testutil_test: cannot get package directory: " + err.Error())
	}
}

// ---------------------------------------------------------------------------
// Stdout capture helpers
//
// WARNING: These helpers redirect the global os.Stdout, so concurrent capture
// sessions serialize on a package-level mutex. If you need truly parallel
// stdout capture, refactor the code under test to accept an io.Writer instead.
// ---------------------------------------------------------------------------

// stdoutCaptureMu guards the process-global os.Stdout redirect for the full
// capture session.
var stdoutCaptureMu sync.Mutex

// stdoutCaptureSession holds the state for one capture: the saved os.Stdout
// and the pipe endpoints used to intercept writes.
type stdoutCaptureSession struct {
	oldStdout *os.File
	reader    *os.File
	writer    *os.File
	restore   sync.Once
}

// stdoutCaptureResult pairs the captured output with any read error.
type stdoutCaptureResult struct {
	output string
	err    error
}

// beginStdoutCaptureSession opens a pipe, redirects os.Stdout to the write
// end, and returns a session that must be closed via closeAndRestore.
func beginStdoutCaptureSession() (*stdoutCaptureSession, error) {
	stdoutCaptureMu.Lock()

	reader, writer, err := os.Pipe()
	if err != nil {
		stdoutCaptureMu.Unlock()
		return nil, err
	}

	session := &stdoutCaptureSession{
		oldStdout: os.Stdout,
		reader:    reader,
		writer:    writer,
	}
	os.Stdout = writer
	return session, nil
}

// closeAndRestore closes the write end of the pipe and restores the original
// os.Stdout. Safe to call multiple times; subsequent calls are no-ops.
func (session *stdoutCaptureSession) closeAndRestore() {
	if session == nil {
		return
	}
	session.restore.Do(func() {
		if session.writer != nil {
			_ = session.writer.Close()
			session.writer = nil
		}
		if session.reader != nil {
			_ = session.reader.Close()
			session.reader = nil
		}
		if session.oldStdout != nil {
			os.Stdout = session.oldStdout
		}
		stdoutCaptureMu.Unlock()
	})
}

// startReader spawns a goroutine that drains the read end of the pipe and
// sends the result on the returned channel. Must be called before
// closeAndRestore so the write end is still open when the reader starts.
func (session *stdoutCaptureSession) startReader() <-chan stdoutCaptureResult {
	results := make(chan stdoutCaptureResult, 1)
	if session == nil || session.reader == nil {
		results <- stdoutCaptureResult{}
		return results
	}

	reader := session.reader
	session.reader = nil
	go func() {
		data, err := io.ReadAll(reader)
		_ = reader.Close()
		results <- stdoutCaptureResult{
			output: string(data),
			err:    err,
		}
	}()
	return results
}

// captureStdout redirects os.Stdout to a pipe, calls fn, and returns everything
// written to stdout along with fn's error.
// Origin: rpi_verify_test.go
func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	session, err := beginStdoutCaptureSession()
	if err != nil {
		t.Fatalf("capture stdout: %v", err)
	}
	results := session.startReader()
	restored := false
	restore := func() {
		if restored {
			return
		}
		restored = true
		session.closeAndRestore()
	}
	t.Cleanup(restore)

	var runErr error
	var panicValue any
	func() {
		defer func() {
			panicValue = recover()
			restore()
		}()
		runErr = fn()
	}()

	result := <-results
	if result.err != nil {
		t.Fatalf("read captured stdout: %v", result.err)
	}
	if panicValue != nil {
		panic(panicValue)
	}
	return result.output, runErr
}

// captureJSONStdout redirects os.Stdout, calls fn (no return value), and
// returns everything written. Useful for commands that print JSON.
// Origin: json_validity_test.go
func captureJSONStdout(t *testing.T, fn func()) string {
	t.Helper()
	session, err := beginStdoutCaptureSession()
	if err != nil {
		t.Fatalf("capture stdout: %v", err)
	}
	results := session.startReader()
	restored := false
	restore := func() {
		if restored {
			return
		}
		restored = true
		session.closeAndRestore()
	}
	t.Cleanup(restore)

	var panicValue any
	func() {
		defer func() {
			panicValue = recover()
			restore()
		}()
		fn()
	}()

	result := <-results
	if result.err != nil {
		t.Fatalf("read captured stdout: %v", result.err)
	}
	if panicValue != nil {
		panic(panicValue)
	}
	return result.output
}

// ---------------------------------------------------------------------------
// Global state reset helper
// ---------------------------------------------------------------------------

// resetCommandState saves all cobra-bound package-level globals, resets them
// to defaults, clears cobra flag Changed state, and registers t.Cleanup to
// restore originals. Call this at the start of any test that uses rootCmd
// directly instead of executeCommand.
func resetCommandState(t *testing.T) {
	t.Helper()

	// Reset any leaked cobra out/err writers to the default (nil → os.Stdout/Stderr).
	// An earlier test's unrestored SetOut leaves a command's writer pointing at a
	// dead buffer; cmd.OutOrStdout() parent-walks to it and silently swallows output
	// captured via os.Stdout — the TestGoals_Integration_MeasureDirectivesJSON
	// shuffle-order flake (age-2vzb). Writers have no meaningful saved state: nil is
	// always the correct baseline, so clean (don't save/restore) them here.
	rootCmd.SetOut(nil)
	rootCmd.SetErr(nil)

	// Save originals.
	origDryRun := dryRun
	origVerbose := verbose
	origOutput := output
	origJSON := jsonFlag
	origCfg := cfgFile
	origConfigShow := configShow
	// No alternate lifecycle build exists; the helper remains a no-op so shared
	// test setup has one stable call site.
	resetArchivedCommandGlobals(t)

	t.Cleanup(func() {
		dryRun = origDryRun
		verbose = origVerbose
		output = origOutput
		jsonFlag = origJSON
		cfgFile = origCfg
		configShow = origConfigShow
	})

	// Reset to defaults.
	dryRun = false
	verbose = false
	output = ""
	jsonFlag = false
	cfgFile = ""
	configShow = false
	output = "table"
	// Reset Cobra flag Changed state and values to defaults.
	resetFlagChangesRecursive(rootCmd)
	rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		f.Changed = false
		_ = f.Value.Set(f.DefValue)
	})
}

// ---------------------------------------------------------------------------
// Working-directory helpers
// ---------------------------------------------------------------------------

// chdirTemp creates a temp directory, chdir's into it, and registers cleanup
// to restore the original working directory.
// Origin: doctor_test.go
func chdirTemp(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	// ag-k38x: t.Chdir auto-restores on cleanup and fails fast if the test (or a
	// parent) is parallel — the isolation-safe replacement for os.Chdir + t.Cleanup.
	t.Chdir(tmp)
	// Match os.Getwd() canonicalization (macOS /var → /private/var symlinks).
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd after chdir: %v", err)
	}
	return cwd
}

// ---------------------------------------------------------------------------
// File / directory setup helpers
// ---------------------------------------------------------------------------

// setupAgentsDir creates the .agents/ao directory structure in the given dir.
// Origin: cobra_commands_test.go
func setupAgentsDir(t *testing.T, dir string) {
	t.Helper()
	dirs := []string{
		".agents/ao/sessions",
		".agents/ao/index",
		".agents/ao/provenance",
		".agents/ao/metrics",
		".agents/learnings",
		".agents/research",
		".agents/patterns",
		".agents/retro",
		".agents/plans",
		".agents/council",
		".agents/knowledge/pending",
		".agents/rpi",
		".agents/constraints",
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(dir, d), 0755); err != nil {
			t.Fatal(err)
		}
	}
}

// writeFile creates parent directories if needed and writes content to path.
// Origin: helpers2_test.go
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
