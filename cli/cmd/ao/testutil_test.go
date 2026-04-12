package main

// testutil_test.go consolidates test helper functions that are shared across
// multiple test files in package main. Each helper was originally defined in a
// single file but called from others; moving them here avoids duplication and
// makes cross-file dependencies explicit.

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

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

	// Save originals.
	origDryRun := dryRun
	origVerbose := verbose
	origOutput := output
	origJSON := jsonFlag
	origCfg := cfgFile
	origDemoConcepts := demoConcepts
	origDemoQuick := demoQuick
	origConfigShow := configShow
	origSeedForce := seedForce
	origGoalsJSON := goalsJSON
	origMemorySyncQuiet := memorySyncQuiet
	origMemorySyncMaxEntries := memorySyncMaxEntries
	origMemorySyncOutput := memorySyncOutput
	origHooksFull := hooksFull
	origHooksDryRun := hooksDryRun
	origHooksForce := hooksForce
	origSearchLimit := searchLimit
	origSearchType := searchType
	origSearchCiteType := searchCiteType
	origSearchSession := searchSession
	origSearchUseSC := searchUseSC
	origSearchUseCASS := searchUseCASS
	origSearchUseLocal := searchUseLocal
	origCodexStartLimit := codexStartLimit
	origCodexStartQuery := codexStartQuery
	origCodexStartNoMaintenance := codexStartNoMaintenance
	origFactoryStartGoal := factoryStartGoal
	origFactoryStartLimit := factoryStartLimit
	origFactoryStartNoMaintenance := factoryStartNoMaintenance
	origCodexStopSessionID := codexStopSessionID
	origCodexStopTranscriptPath := codexStopTranscriptPath
	origCodexStopAutoExtract := codexStopAutoExtract
	origCodexStopNoHistoryFallback := codexStopNoHistoryFallback
	origCodexStopNoCloseLoop := codexStopNoCloseLoop
	origCodexStatusDays := codexStatusDays
	origAutodevFile := autodevFile
	origAutodevForce := autodevForce
	origFindingsListLimit := findingsListLimit
	origFindingsListAll := findingsListAll
	origFindingsExportTo := findingsExportTo
	origFindingsExportAll := findingsExportAll
	origFindingsExportForce := findingsExportForce
	origFindingsPullFrom := findingsPullFrom
	origFindingsPullAll := findingsPullAll
	origFindingsPullForce := findingsPullForce
	origFindingsRetireBy := findingsRetireBy
	origScenarioListStatus := scenarioListStatus
	origContextPacketFlags := contextPacketFlags
	origContextExplainFlags := contextExplainFlags
	origContextPacketStatusFlags := contextPacketStatusFlags
	origDoctorJSON := doctorJSON
	origMaturityApply := maturityApply
	origMaturityScan := maturityScan
	origMaturityCurate := maturityCurate
	origMaturityExpire := maturityExpire
	origMaturityArchive := maturityArchive
	origMaturityEvict := maturityEvict
	origMaturityGlobal := maturityGlobal
	origMaturityMigrateMd := maturityMigrateMd
	origMaturityRecalibrate := maturityRecalibrate
	origMaturityUncitedDays := maturityUncitedDays
	origCompileSourcesDir := compileSourcesDir
	origCompileOutputDir := compileOutputDir
	origCompileSince := compileSince
	origCompileRuntime := compileRuntime
	origCompileIncremental := compileIncremental
	origCompileForce := compileForce
	origCompileOnly := compileOnly
	origCompileLintOnly := compileLintOnly
	origCompileDefragOnly := compileDefragOnly
	origCompileMineOnly := compileMineOnly
	origCompileFull := compileFull
	origCompileQuiet := compileQuiet

	t.Cleanup(func() {
		dryRun = origDryRun
		verbose = origVerbose
		output = origOutput
		jsonFlag = origJSON
		cfgFile = origCfg
		demoConcepts = origDemoConcepts
		demoQuick = origDemoQuick
		configShow = origConfigShow
		seedForce = origSeedForce
		goalsJSON = origGoalsJSON
		memorySyncQuiet = origMemorySyncQuiet
		memorySyncMaxEntries = origMemorySyncMaxEntries
		memorySyncOutput = origMemorySyncOutput
		hooksFull = origHooksFull
		hooksDryRun = origHooksDryRun
		hooksForce = origHooksForce
		searchLimit = origSearchLimit
		searchType = origSearchType
		searchCiteType = origSearchCiteType
		searchSession = origSearchSession
		searchUseSC = origSearchUseSC
		searchUseCASS = origSearchUseCASS
		searchUseLocal = origSearchUseLocal
		codexStartLimit = origCodexStartLimit
		codexStartQuery = origCodexStartQuery
		codexStartNoMaintenance = origCodexStartNoMaintenance
		factoryStartGoal = origFactoryStartGoal
		factoryStartLimit = origFactoryStartLimit
		factoryStartNoMaintenance = origFactoryStartNoMaintenance
		codexStopSessionID = origCodexStopSessionID
		codexStopTranscriptPath = origCodexStopTranscriptPath
		codexStopAutoExtract = origCodexStopAutoExtract
		codexStopNoHistoryFallback = origCodexStopNoHistoryFallback
		codexStopNoCloseLoop = origCodexStopNoCloseLoop
		codexStatusDays = origCodexStatusDays
		autodevFile = origAutodevFile
		autodevForce = origAutodevForce
		findingsListLimit = origFindingsListLimit
		findingsListAll = origFindingsListAll
		findingsExportTo = origFindingsExportTo
		findingsExportAll = origFindingsExportAll
		findingsExportForce = origFindingsExportForce
		findingsPullFrom = origFindingsPullFrom
		findingsPullAll = origFindingsPullAll
		findingsPullForce = origFindingsPullForce
		findingsRetireBy = origFindingsRetireBy
		scenarioListStatus = origScenarioListStatus
		contextPacketFlags = origContextPacketFlags
		contextExplainFlags = origContextExplainFlags
		contextPacketStatusFlags = origContextPacketStatusFlags
		doctorJSON = origDoctorJSON
		maturityApply = origMaturityApply
		maturityScan = origMaturityScan
		maturityCurate = origMaturityCurate
		maturityExpire = origMaturityExpire
		maturityArchive = origMaturityArchive
		maturityEvict = origMaturityEvict
		maturityGlobal = origMaturityGlobal
		maturityMigrateMd = origMaturityMigrateMd
		maturityRecalibrate = origMaturityRecalibrate
		maturityUncitedDays = origMaturityUncitedDays
		compileSourcesDir = origCompileSourcesDir
		compileOutputDir = origCompileOutputDir
		compileSince = origCompileSince
		compileRuntime = origCompileRuntime
		compileIncremental = origCompileIncremental
		compileForce = origCompileForce
		compileOnly = origCompileOnly
		compileLintOnly = origCompileLintOnly
		compileDefragOnly = origCompileDefragOnly
		compileMineOnly = origCompileMineOnly
		compileFull = origCompileFull
		compileQuiet = origCompileQuiet
	})

	// Reset to defaults.
	dryRun = false
	verbose = false
	output = ""
	jsonFlag = false
	cfgFile = ""
	demoConcepts = false
	demoQuick = false
	configShow = false
	seedForce = false
	goalsJSON = false
	memorySyncQuiet = false
	memorySyncMaxEntries = 10
	memorySyncOutput = ""
	hooksFull = false
	hooksDryRun = false
	hooksForce = false
	searchLimit = 10
	searchType = ""
	searchCiteType = ""
	searchSession = ""
	searchUseSC = false
	searchUseCASS = false
	searchUseLocal = false
	codexStartLimit = 3
	codexStartQuery = ""
	codexStartNoMaintenance = false
	codexStopSessionID = ""
	codexStopTranscriptPath = ""
	codexStopAutoExtract = true
	codexStopNoHistoryFallback = false
	codexStopNoCloseLoop = false
	codexStatusDays = 7
	autodevFile = ""
	autodevForce = false
	findingsListLimit = 20
	findingsListAll = false
	findingsExportTo = ""
	findingsExportAll = false
	findingsExportForce = false
	findingsPullFrom = ""
	findingsPullAll = false
	findingsPullForce = false
	findingsRetireBy = ""
	scenarioListStatus = ""
	doctorJSON = false
	maturityApply = false
	maturityScan = false
	maturityCurate = false
	maturityExpire = false
	maturityArchive = false
	maturityEvict = false
	maturityGlobal = false
	maturityMigrateMd = false
	maturityRecalibrate = false
	maturityUncitedDays = 0
	compileSourcesDir = ".agents"
	compileOutputDir = ".agents/compiled"
	compileSince = "26h"
	compileRuntime = ""
	compileIncremental = true
	compileForce = false
	compileOnly = false
	compileLintOnly = false
	compileDefragOnly = false
	compileMineOnly = false
	compileFull = false
	compileQuiet = false
	contextPacketFlags = struct {
		goal  string
		epic  string
		repo  string
		limit int
		json  bool
	}{limit: defaultStigmergicPacketLimit}
	contextExplainFlags = struct {
		task  string
		phase string
		limit int
	}{}
	contextPacketStatusFlags = struct {
		task  string
		phase string
		limit int
	}{}

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
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
	return tmp
}

// chdirTo changes to the specified directory, registers a cleanup to restore
// the previous working directory, and returns the previous working directory
// for backward compatibility.
// Origin: constraint_cmd_test.go
func chdirTo(t *testing.T, wd string) string {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(wd); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
	return prev
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

// ---------------------------------------------------------------------------
// Git helpers
// ---------------------------------------------------------------------------

// gitCommitFixture describes a single commit to create in a test git repo.
// Path and Content default to auto-generated values when left empty.
type gitCommitFixture struct {
	Path      string
	Content   string
	Message   string
	Timestamp time.Time
}

// initGitHistoryFixtureRepo creates a temp directory with a git repo and
// replays the given commits in order, preserving author/committer timestamps.
func initGitHistoryFixtureRepo(t *testing.T, commits []gitCommitFixture) string {
	t.Helper()
	dir := t.TempDir()
	initHistoryFixtureGitRepo(t, dir)
	for i, commit := range commits {
		path := commit.Path
		if path == "" {
			path = fmt.Sprintf("fixture-%d.txt", i)
		}
		content := commit.Content
		if content == "" {
			content = fmt.Sprintf("%s at %s\n", commit.Message, commit.Timestamp.Format(time.RFC3339))
		}
		writeFile(t, filepath.Join(dir, path), content)
		runFixtureGit(
			t,
			dir,
			nil,
			"add",
			path,
		)
		runFixtureGit(
			t,
			dir,
			[]string{
				"GIT_AUTHOR_DATE=" + commit.Timestamp.Format(time.RFC3339),
				"GIT_COMMITTER_DATE=" + commit.Timestamp.Format(time.RFC3339),
			},
			"commit",
			"-m",
			commit.Message,
		)
	}
	return dir
}

// initTestRepo creates a temp directory with a git repo containing one commit.
// Origin: rpi_phased_worktree_test.go
func initTestRepo(t *testing.T) string {
	t.Helper()
	return initGitHistoryFixtureRepo(t, []gitCommitFixture{{
		Path:      "README.md",
		Content:   "# Test\n",
		Message:   "Initial commit",
		Timestamp: time.Now().Add(-1 * time.Hour).UTC(),
	}})
}

// initMinimalGitRepo creates a git repo with one empty commit. Use this when
// you need a valid git repo but don't care about file history.
// Origin: uat_smoke_test.go (was initGitRepo)
func initMinimalGitRepo(t *testing.T, dir string) {
	t.Helper()
	initHistoryFixtureGitRepo(t, dir)
	runFixtureGit(t, dir, nil, "commit", "--allow-empty", "-m", "init")
}

// initHistoryFixtureGitRepo runs git init and configures a test user in dir.
func initHistoryFixtureGitRepo(t *testing.T, dir string) {
	t.Helper()
	runFixtureGit(t, dir, nil, "init")
	runFixtureGit(t, dir, nil, "config", "user.email", "test@test.com")
	runFixtureGit(t, dir, nil, "config", "user.name", "Test")
	runFixtureGit(t, dir, nil, "config", "commit.gpgsign", "false")
}

// runFixtureGit executes a git command in dir with optional extra environment
// variables. Fatals the test on any non-zero exit.
func runFixtureGit(t *testing.T, dir string, extraEnv []string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
	return string(out)
}
