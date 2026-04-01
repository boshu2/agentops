package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	cliRPI "github.com/boshu2/agentops/cli/internal/rpi"
)

//go:embed assets/watch.html
var rpiWatchHTML []byte

var (
	rpiServePort        int
	rpiServeRunID       string
	rpiServeOpen        bool
	rpiServeNoOpen      bool
	rpiServeOrchestrate bool
	rpiServeRuntimeMode string
	rpiServeRuntimeCmd  string
)

// rpiRunIDPattern matches persisted run IDs: rpi-<8-12hex> or bare 12-hex.
// Bare 8-hex is excluded to avoid false positives with short git SHAs.
var rpiRunIDPattern = regexp.MustCompile(`^(rpi-[a-f0-9]{8,12}|[a-f0-9]{12})$`)

// classifyServeArg returns (goal, runID) from flags and positional args.
// Flag --run-id wins over the positional arg. A token matching rpiRunIDPattern
// is treated as a run ID to watch; anything else is a goal string.
func classifyServeArg(flagRunID string, args []string) (goal, runID string) {
	if tok := strings.TrimSpace(flagRunID); tok != "" {
		if rpiRunIDPattern.MatchString(tok) {
			return "", tok
		}
		return tok, ""
	}
	if len(args) > 0 {
		tok := strings.TrimSpace(args[0])
		if rpiRunIDPattern.MatchString(tok) {
			return "", tok
		}
		return tok, ""
	}
	return "", ""
}

func validateExplicitServeRunID(flagRunID string) (string, error) {
	tok := strings.TrimSpace(flagRunID)
	if tok == "" {
		return "", nil
	}
	if !rpiRunIDPattern.MatchString(tok) {
		return "", fmt.Errorf("invalid --run-id %q: expected rpi-<8-12 hex> or <12 hex>", tok)
	}
	return tok, nil
}

func init() {
	serveCmd := &cobra.Command{
		Use:   "serve [goal | run-id]",
		Short: "Orchestrate an RPI run or watch its live dashboard",
		Long: `Start a production RPI orchestration run or stream its live dashboard.

Orchestration mode (ao rpi serve "<goal>"):
  Runs the full 3-phase RPI lifecycle — discovery, implementation, validation —
  using the phased engine with fresh context per phase, budget enforcement,
  stall detection, and worktree isolation.

  ao rpi serve "add user authentication"   # run full RPI lifecycle
  ao rpi serve "fix the cache bug"         # any plain-text goal

Watch mode (ao rpi serve [run-id]):
  Stream C2 events for an already-active RPI run. Opens a real-time dashboard
  with phase status, telemetry, cost, and worker activity.

  ao rpi serve                      # auto-discover latest active run
  ao rpi serve rpi-a1b2c3d4         # watch a specific run by ID
  ao rpi serve --run-id 760fc86f0c0f
  ao rpi serve --port 8080          # use a custom port
  ao rpi serve --no-open            # start server without opening browser

The dashboard streams events via Server-Sent Events (SSE) and also polls
/runs and /state for discovery and reconciliation. It does not use WebSockets.`,
		RunE: runRPIServe,
	}
	serveCmd.Flags().IntVar(&rpiServePort, "port", 7799, "Port to listen on")
	serveCmd.Flags().StringVar(&rpiServeRunID, "run-id", "", "Run ID to watch explicitly (must match rpi-<8-12 hex> or <12 hex>)")
	serveCmd.Flags().BoolVar(&rpiServeOpen, "open", true, "Open browser automatically")
	serveCmd.Flags().BoolVar(&rpiServeNoOpen, "no-open", false, "Do not open browser automatically")
	serveCmd.Flags().BoolVar(&rpiServeOrchestrate, "orchestrate", false, "Treat first argument as a goal and run full RPI orchestration")
	serveCmd.Flags().StringVar(&rpiServeRuntimeMode, "runtime", "", "Phase runtime mode for orchestration: auto|direct|stream|tmux")
	serveCmd.Flags().StringVar(&rpiServeRuntimeCmd, "runtime-cmd", "", "Runtime command for orchestration phase prompts (Claude uses '-p'; Codex uses 'exec')")
	addRPISubcommand(serveCmd)
}

// shouldOpenBrowser returns true unless the user passed --no-open or --open=false.
func shouldOpenBrowser() bool {
	if rpiServeNoOpen {
		return false
	}
	return rpiServeOpen
}

func runRPIServe(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	if _, err := validateExplicitServeRunID(rpiServeRunID); err != nil {
		return err
	}

	goal, watchRunID := classifyServeArg(rpiServeRunID, args)

	// --orchestrate flag forces the first arg to be interpreted as a goal.
	if rpiServeOrchestrate {
		if goal == "" && watchRunID != "" {
			goal, watchRunID = watchRunID, ""
		}
		if goal == "" {
			return fmt.Errorf("--orchestrate requires a goal string (e.g. ao rpi serve \"add auth\")")
		}
	}

	// Orchestration mode: a goal was provided — run the full RPI lifecycle.
	if goal != "" {
		toolchain, err := resolveRPIToolchain(
			cliRPI.Toolchain{
				RuntimeMode:    rpiServeRuntimeMode,
				RuntimeCommand: rpiServeRuntimeCmd,
			},
			rpiToolchainFlagSet{
				RuntimeMode:    cmd.Flags().Changed("runtime"),
				RuntimeCommand: cmd.Flags().Changed("runtime-cmd"),
			},
		)
		if err != nil {
			return err
		}
		return runServeOrchestrate(cwd, goal, toolchain)
	}

	// Watch mode: observe an existing or upcoming run.
	return runServeWatch(cwd, watchRunID)
}

// runServeOrchestrate starts the phased engine with a live dashboard.
func runServeOrchestrate(cwd, goal string, toolchain cliRPI.Toolchain) error {
	runID := generateRunID()
	opts := buildServeEngineOptions(cwd, runID, toolchain)

	addr := fmt.Sprintf("localhost:%d", rpiServePort)
	dashURL := fmt.Sprintf("http://%s?run=%s", addr, runID)

	muxRoot := &serveMuxRoot{path: cwd}
	mux := buildServeMux(muxRoot, runID)
	opts.OnSpawnCwdReady = func(spawnCwd string) {
		muxRoot.set(spawnCwd)
	}
	srv := newDashboardServer(addr, mux)

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("port %d unavailable: %w", rpiServePort, err)
	}

	fmt.Printf("RPI orchestration starting\n")
	fmt.Printf("Goal:            %s\n", goal)
	fmt.Printf("Run ID:          %s\n", runID)
	fmt.Printf("Mission control: %s\n", dashURL)
	fmt.Printf("Press Ctrl-C to stop.\n")

	if shouldOpenBrowser() {
		openBrowserURL(dashURL)
	}

	orchCtx, orchCancel := context.WithCancel(context.Background())

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(stop)
	go func() {
		<-stop
		orchCancel()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()

	orchErrCh := make(chan error, 1)
	go func() {
		orchErrCh <- runPhasedEngine(orchCtx, cwd, goal, opts)
	}()

	srvErr := srv.Serve(ln)
	orchCancel()

	if srvErr != nil && srvErr != http.ErrServerClosed {
		return fmt.Errorf("server: %w", srvErr)
	}
	fmt.Println("\nDashboard stopped.")

	if orchErr := <-orchErrCh; orchErr != nil && orchErr != context.Canceled {
		return fmt.Errorf("orchestration: %w", orchErr)
	}
	return nil
}

func buildServeEngineOptions(cwd, runID string, toolchain cliRPI.Toolchain) phasedEngineOptions {
	opts := defaultPhasedEngineOptions()
	opts.WorkingDir = cwd
	opts.RunID = runID
	opts.NoDashboard = true // serve manages its own dashboard
	if strings.TrimSpace(toolchain.RuntimeMode) != "" {
		opts.RuntimeMode = toolchain.RuntimeMode
	}
	if strings.TrimSpace(toolchain.RuntimeCommand) != "" {
		opts.RuntimeCommand = toolchain.RuntimeCommand
	}
	if strings.TrimSpace(toolchain.AOCommand) != "" {
		opts.AOCommand = toolchain.AOCommand
	}
	if strings.TrimSpace(toolchain.BDCommand) != "" {
		opts.BDCommand = toolchain.BDCommand
	}
	if strings.TrimSpace(toolchain.TmuxCommand) != "" {
		opts.TmuxCommand = toolchain.TmuxCommand
	}
	return opts
}

// runServeWatch starts the dashboard in watch mode for an existing or upcoming run.
func runServeWatch(cwd, watchRunID string) error {
	runID, _, root, resolveErr := resolveNudgeRun(cwd, watchRunID)
	if resolveErr != nil {
		if watchRunID != "" {
			return fmt.Errorf("resolve run: %w", resolveErr)
		}
		root = cwd
		runID = ""
	}

	addr := fmt.Sprintf("localhost:%d", rpiServePort)
	dashURL := fmt.Sprintf("http://%s", addr)
	if runID != "" {
		dashURL = fmt.Sprintf("http://%s?run=%s", addr, runID)
	}

	srv := newDashboardServer(addr, buildServeMux(&serveMuxRoot{path: root}, runID))

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("port %d unavailable: %w", rpiServePort, err)
	}

	fmt.Printf("Mission control: %s\n", dashURL)
	if runID != "" {
		fmt.Printf("Run:             %s\n", runID)
	} else {
		fmt.Printf("Mode:            waiting for runs (start an RPI session to see events)\n")
	}
	fmt.Printf("Press Ctrl-C to stop.\n")

	if shouldOpenBrowser() {
		openBrowserURL(dashURL)
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(stop)
	go func() {
		<-stop
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server: %w", err)
	}
	fmt.Println("\nDashboard stopped.")
	return nil
}

// newDashboardServer creates an http.Server with standard timeouts.
func newDashboardServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    8192,
	}
}

// serveMuxRoot provides thread-safe mutable root path for the HTTP mux.
// In orchestration mode the root switches from the original repo to the
// worktree once the engine has created it; handlers read the current value
// on every request via get().
type serveMuxRoot struct {
	mu   sync.RWMutex
	path string
}

func (r *serveMuxRoot) get() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.path
}

func (r *serveMuxRoot) set(p string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.path = p
}

func buildServeMux(root *serveMuxRoot, runID string) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", serveRPIIndex)
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			setCORSHeaders(w, r)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		serveRPIEvents(w, r, root.get(), runID)
	})
	mux.HandleFunc("/runs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			setCORSHeaders(w, r)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		serveRPIRuns(w, r, root.get())
	})
	mux.HandleFunc("/state", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			setCORSHeaders(w, r)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		serveRPIState(w, r, root.get(), runID)
	})
	mux.HandleFunc("/artifacts", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			setCORSHeaders(w, r)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		serveRPIArtifacts(w, r, root.get(), runID)
	})
	mux.HandleFunc("/artifact", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			setCORSHeaders(w, r)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		serveRPIArtifact(w, r, root.get(), runID)
	})
	return mux
}

func discoverServeRuns(root string) []rpiRunInfo {
	active, historical := discoverRPIRunsRegistryFirst(root)
	runs := make([]rpiRunInfo, 0, len(active)+len(historical))
	runs = append(runs, active...)
	runs = append(runs, historical...)
	sortServeRuns(runs)
	return runs
}

func sortServeRuns(runs []rpiRunInfo) {
	sort.SliceStable(runs, func(i, j int) bool {
		left := runs[i]
		right := runs[j]
		if left.IsActive != right.IsActive {
			return left.IsActive
		}
		if left.LastHeartbeat != right.LastHeartbeat {
			if left.LastHeartbeat.IsZero() {
				return false
			}
			if right.LastHeartbeat.IsZero() {
				return true
			}
			return left.LastHeartbeat.After(right.LastHeartbeat)
		}
		leftStart := parseServeRunTime(left.StartedAt)
		rightStart := parseServeRunTime(right.StartedAt)
		if !leftStart.Equal(rightStart) {
			return leftStart.After(rightStart)
		}
		return left.RunID > right.RunID
	})
}

func parseServeRunTime(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	if ts, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return ts
	}
	if ts, err := time.Parse(time.RFC3339, raw); err == nil {
		return ts
	}
	return time.Time{}
}

func resolveServeRun(root, runID string) (*phasedState, string) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil, root
	}
	state, resolvedRoot, err := locateRunMetadata(root, runID)
	if err != nil || state == nil {
		return nil, root
	}
	if strings.TrimSpace(resolvedRoot) == "" {
		return state, root
	}
	return state, resolvedRoot
}

func serveRPIIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(rpiWatchHTML) // nosemgrep: go.lang.security.audit.xss.no-direct-write-to-responsewriter.no-direct-write-to-responsewriter -- static embedded asset, no user input
}

// serveRPIEvents streams C2 events as Server-Sent Events (SSE).
// It sends all existing events immediately, then polls for new ones every 500ms.
func serveRPIEvents(w http.ResponseWriter, r *http.Request, root, defaultRunID string) {
	runID := strings.TrimSpace(r.URL.Query().Get("run-id"))
	if runID != "" && (strings.Contains(runID, "..") || strings.Contains(runID, "/") || strings.Contains(runID, "\\")) {
		http.Error(w, "invalid run-id", http.StatusBadRequest)
		return
	}
	if runID == "" {
		runID = defaultRunID
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported by this server", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	setCORSHeaders(w, r)

	// Immediately flush headers so clients know the SSE connection is established.
	if _, err := fmt.Fprintf(w, ": connected\n\n"); err != nil { // nosemgrep: go.lang.security.audit.xss.no-fprintf-to-responsewriter.no-fprintf-to-responsewriter -- SSE comment to text/event-stream, not HTML; localhost-only
		return
	}
	flusher.Flush()

	seen := 0
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			eventRoot := root
			if _, resolvedRoot := resolveServeRun(root, runID); strings.TrimSpace(resolvedRoot) != "" {
				eventRoot = resolvedRoot
			}

			events, err := loadRPIC2Events(eventRoot, runID)
			if err != nil {
				errEvent := RPIC2Event{Type: "error", Message: err.Error(), Timestamp: time.Now().UTC().Format(time.RFC3339)}
				if writeSSEEvent(w, errEvent) != nil {
					return
				}
				flusher.Flush()
				continue
			}
			for ; seen < len(events); seen++ {
				if writeSSEEvent(w, events[seen]) != nil {
					return
				}
				flusher.Flush()
			}
		}
	}
}

func writeSSEEvent(w http.ResponseWriter, ev RPIC2Event) error {
	data, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "data: %s\n\n", data) // nosemgrep: go.lang.security.audit.xss.no-fprintf-to-responsewriter.no-fprintf-to-responsewriter -- SSE stream writes JSON to text/event-stream, not HTML; localhost-only
	return err
}

func serveRPIRuns(w http.ResponseWriter, r *http.Request, root string) {
	setCORSHeaders(w, r)
	runs := discoverServeRuns(root)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(runs)
}

// serveRPIState returns the current phased-state.json and run registry info
// so the frontend can read ground truth instead of inferring from events.
func serveRPIState(w http.ResponseWriter, r *http.Request, root, defaultRunID string) {
	setCORSHeaders(w, r)

	runID := strings.TrimSpace(r.URL.Query().Get("run-id"))
	if runID != "" && (strings.Contains(runID, "..") || strings.Contains(runID, "/") || strings.Contains(runID, "\\")) {
		http.Error(w, "invalid run-id", http.StatusBadRequest)
		return
	}
	if runID == "" {
		runID = defaultRunID
	}

	resp := map[string]any{
		"root":   root,
		"run_id": runID,
	}

	resolvedRoot := root
	if runID != "" {
		if state, stateRoot := resolveServeRun(root, runID); state != nil {
			resolvedRoot = stateRoot
			respState, err := json.Marshal(state)
			if err == nil {
				var mapped map[string]any
				if json.Unmarshal(respState, &mapped) == nil {
					resp["phased_state"] = mapped
				}
			}
		}
	}
	resp["root"] = resolvedRoot

	// Read phased-state.json if it exists and a run-specific lookup did not already populate it.
	if _, ok := resp["phased_state"]; !ok {
		statePath := filepath.Join(resolvedRoot, ".agents", "rpi", "phased-state.json")
		if data, err := os.ReadFile(statePath); err == nil {
			var state map[string]any
			if json.Unmarshal(data, &state) == nil {
				resp["phased_state"] = state
			}
		}
	}

	// Read per-phase results
	phaseResults := make(map[string]any)
	for i := 1; i <= 3; i++ {
		resultPath := filepath.Join(resolvedRoot, ".agents", "rpi", fmt.Sprintf("phase-%d-result.json", i))
		if data, err := os.ReadFile(resultPath); err == nil {
			var result map[string]any
			if json.Unmarshal(data, &result) == nil {
				phaseResults[fmt.Sprintf("phase_%d", i)] = result
			}
		}
	}
	if len(phaseResults) > 0 {
		resp["phase_results"] = phaseResults
	}
	if artifacts := collectRunArtifacts(resolvedRoot, runID); len(artifacts) > 0 {
		resp["artifacts"] = artifacts
	}

	// Scan for active runs if no specific run requested
	if runID == "" {
		runs := discoverServeRuns(root)
		resp["runs"] = runs
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func serveRPIArtifacts(w http.ResponseWriter, r *http.Request, root, defaultRunID string) {
	setCORSHeaders(w, r)

	runID := strings.TrimSpace(r.URL.Query().Get("run-id"))
	if runID != "" && (strings.Contains(runID, "..") || strings.Contains(runID, "/") || strings.Contains(runID, "\\")) {
		http.Error(w, "invalid run-id", http.StatusBadRequest)
		return
	}
	if runID == "" {
		runID = defaultRunID
	}

	resolvedRoot := root
	if runID != "" {
		if _, stateRoot := resolveServeRun(root, runID); strings.TrimSpace(stateRoot) != "" {
			resolvedRoot = stateRoot
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(collectRunArtifacts(resolvedRoot, runID))
}

func serveRPIArtifact(w http.ResponseWriter, r *http.Request, root, defaultRunID string) {
	setCORSHeaders(w, r)

	runID := strings.TrimSpace(r.URL.Query().Get("run-id"))
	if runID != "" && (strings.Contains(runID, "..") || strings.Contains(runID, "/") || strings.Contains(runID, "\\")) {
		http.Error(w, "invalid run-id", http.StatusBadRequest)
		return
	}
	if runID == "" {
		runID = defaultRunID
	}

	relPath := pathClean(r.URL.Query().Get("path"))
	if !isSafeArtifactRelPath(relPath) {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	resolvedRoot := root
	if runID != "" {
		if _, stateRoot := resolveServeRun(root, runID); strings.TrimSpace(stateRoot) != "" {
			resolvedRoot = stateRoot
		}
	}

	var matched *rpiArtifactRef
	for _, ref := range collectRunArtifacts(resolvedRoot, runID) {
		if ref.Path == relPath {
			refCopy := ref
			matched = &refCopy
			break
		}
	}
	if matched == nil {
		http.Error(w, "artifact not found", http.StatusNotFound)
		return
	}

	content, err := readRunArtifactContent(resolvedRoot, *matched, artifactPreviewByteLimit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(content)
}

func setCORSHeaders(w http.ResponseWriter, r ...*http.Request) {
	origin := ""
	if len(r) > 0 && r[0] != nil {
		origin = r[0].Header.Get("Origin")
	}
	// Only allow localhost origins to prevent cross-site data exfiltration.
	if origin == "" || isLocalhostOrigin(origin) {
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		} else {
			w.Header().Set("Access-Control-Allow-Origin", "http://localhost")
		}
	}
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Max-Age", "86400")
}

// isLocalhostOrigin returns true if the origin is a localhost URL.
func isLocalhostOrigin(origin string) bool {
	for _, prefix := range []string{
		"http://localhost", "https://localhost",
		"http://127.0.0.1", "https://127.0.0.1",
		"http://[::1]", "https://[::1]",
	} {
		if strings.HasPrefix(origin, prefix) {
			return true
		}
	}
	return false
}

// openBrowserURL opens url in the default system browser.
func openBrowserURL(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
