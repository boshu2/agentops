package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// gcMock provides injectable exec/lookPath functions for deterministic testing.
// It records all command invocations and returns preconfigured outputs.
type gcMock struct {
	mu       sync.Mutex
	calls    []gcMockCall
	handlers map[string]gcMockHandler
	// If true, lookPath returns success (binary "found")
	binaryAvailable bool
}

// gcMockCall records a single invocation of execCommand.
type gcMockCall struct {
	Args []string
}

// gcMockHandler defines what a mocked command should return.
type gcMockHandler struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// newGCMock creates a mock with gc available by default.
func newGCMock() *gcMock {
	return &gcMock{
		handlers:        make(map[string]gcMockHandler),
		binaryAvailable: true,
	}
}

// install is a no-op kept for backward compat; tests should use m.execCommand and m.lookPathFn.
func (m *gcMock) install(t *testing.T) {
	t.Helper()
	// No globals to swap — tests pass m.execCommand / m.lookPathFn directly.
}

// execCommand returns an exec.Command-compatible function that records calls and returns mock output.
func (m *gcMock) execCommand(name string, args ...string) *exec.Cmd {
	m.mu.Lock()
	m.calls = append(m.calls, gcMockCall{Args: append([]string{name}, args...)})
	m.mu.Unlock()

	key := m.commandKey(args)
	handler, ok := m.handlers[key]
	if !ok {
		handler = gcMockHandler{ExitCode: 0}
	}

	if runtime.GOOS != "windows" {
		cmd := exec.Command("/bin/sh", "-c", `printf "%s" "$GC_MOCK_STDOUT"; printf "%s" "$GC_MOCK_STDERR" >&2; exit "$GC_MOCK_EXIT"`)
		cmd.Env = append(os.Environ(),
			fmt.Sprintf("GC_MOCK_EXIT=%d", handler.ExitCode),
			"GC_MOCK_STDOUT="+handler.Stdout,
			"GC_MOCK_STDERR="+handler.Stderr,
		)
		return cmd
	}

	cs := []string{"-test.run=TestGCHelperProcess", "--", fmt.Sprintf("exit=%d", handler.ExitCode), fmt.Sprintf("stdout=%s", handler.Stdout), fmt.Sprintf("stderr=%s", handler.Stderr)}
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = append(os.Environ(), "GO_TEST_HELPER_PROCESS=1")
	return cmd
}

// lookPathFn returns a LookPath-compatible function based on binaryAvailable.
func (m *gcMock) lookPathFn(file string) (string, error) {
	if m.binaryAvailable && file == "gc" {
		return "/usr/local/bin/gc", nil
	}
	return "", &exec.Error{Name: file, Err: exec.ErrNotFound}
}

// commandKey produces a lookup key from command args (skipping "gc" binary name).
// It also tries prefix matching: "event emit ao:phase" matches args like
// "event emit ao:phase --data {...json...}".
func (m *gcMock) commandKey(args []string) string {
	// Strip --city <path> prefix if present
	cleaned := args
	if len(cleaned) >= 2 && cleaned[0] == "--city" {
		cleaned = cleaned[2:]
	}
	full := strings.Join(cleaned, " ")
	// Try exact match first
	if _, ok := m.handlers[full]; ok {
		return full
	}
	// Try prefix match (longest first)
	bestKey := ""
	for key := range m.handlers {
		if strings.HasPrefix(full, key) && len(key) > len(bestKey) {
			bestKey = key
		}
	}
	if bestKey != "" {
		return bestKey
	}
	return full
}

// on registers a handler for a specific command pattern.
func (m *gcMock) on(argsPattern string, h gcMockHandler) {
	m.handlers[argsPattern] = h
}

// callCount returns the number of times any gc command was invoked.
func (m *gcMock) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

// callsMatching returns calls whose args contain the given substring.
func (m *gcMock) callsMatching(substr string) []gcMockCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	var matched []gcMockCall
	for _, c := range m.calls {
		full := strings.Join(c.Args, " ")
		if strings.Contains(full, substr) {
			matched = append(matched, c)
		}
	}
	return matched
}

// TestGCHelperProcess is the subprocess helper for mocked exec.Command.
// It's invoked by the mocked gcExecCommand, not directly by tests.
func TestGCHelperProcess(t *testing.T) {
	if os.Getenv("GO_TEST_HELPER_PROCESS") != "1" {
		return
	}
	args := os.Args
	// Find the "--" separator
	idx := -1
	for i, a := range args {
		if a == "--" {
			idx = i
			break
		}
	}
	if idx < 0 || idx+1 >= len(args) {
		os.Exit(0)
	}

	exitCode := 0
	stdout := ""
	stderr := ""
	for _, arg := range args[idx+1:] {
		if strings.HasPrefix(arg, "exit=") {
			fmt.Sscanf(arg, "exit=%d", &exitCode)
		} else if strings.HasPrefix(arg, "stdout=") {
			stdout = strings.TrimPrefix(arg, "stdout=")
		} else if strings.HasPrefix(arg, "stderr=") {
			stderr = strings.TrimPrefix(arg, "stderr=")
		}
	}
	if stdout != "" {
		fmt.Fprint(os.Stdout, stdout)
	}
	if stderr != "" {
		fmt.Fprint(os.Stderr, stderr)
	}
	os.Exit(exitCode)
}

// setupCityDir creates a temp directory with a city.toml file.
func setupCityDir(t *testing.T, cityName string) string {
	t.Helper()
	dir := t.TempDir()
	content := fmt.Sprintf("[city]\nname = %q\n", cityName)
	if err := os.WriteFile(filepath.Join(dir, "city.toml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}
