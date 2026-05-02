package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/agentworker"
	daemonpkg "github.com/boshu2/agentops/cli/internal/daemon"
	"github.com/boshu2/agentops/cli/internal/gascity"
	"github.com/boshu2/agentops/cli/internal/llmwiki"
	ovn "github.com/boshu2/agentops/cli/internal/overnight"
	"github.com/boshu2/agentops/cli/internal/schedule"
	"github.com/boshu2/agentops/cli/internal/wikiworker"
	"github.com/spf13/cobra"
)

const daemonActivationRelPath = ".agents/daemon/activation.json"

var (
	daemonAddr              string
	daemonURL               string
	daemonToken             string
	daemonTokenFile         string
	daemonServiceExecutable string
	daemonWorkers           int
	daemonWorkerOnce        bool
	daemonExecutorPolicy    string
	daemonGasCityEndpoint   string
	daemonGasCityCity       string
	daemonGasCityToken      string
	daemonGasCityTokenFile  string
	daemonWorkerTimeout     time.Duration
	daemonWorkerMemoryMax   int64
	daemonWorkerCgroupRoot  string
	daemonScheduleFile      string
)

type agentopsDaemonActivation struct {
	URL       string `json:"url"`
	Address   string `json:"address"`
	PID       int    `json:"pid"`
	Ready     bool   `json:"ready"`
	StartedAt string `json:"started_at"`
	Token     string `json:"token,omitempty"`
	TokenFile string `json:"token_file,omitempty"`
}

type agentopsDaemonRunOptions struct {
	Addr              string
	Token             string
	TokenFile         string
	Workers           int
	WorkerOnce        bool
	ExecutorPolicy    string
	GasCityEndpoint   string
	GasCityCity       string
	GasCityToken      string
	GasCityTokenFile  string
	WorkerTimeout     time.Duration
	WorkerMemoryMax   int64
	WorkerCgroupRoot  string
	ScheduleFile      string
	PollInterval      time.Duration
	HeartbeatInterval time.Duration
	Now               func() time.Time
	// PlansBdSource (optional, atom-2 / soc-acwf) wires the plans.projection
	// executor against a real bd source. Nil disables the plans executor —
	// production callers pass NewDbBdSource(dsn); tests pass an in-memory fake.
	PlansBdSource daemonpkg.PlansBdSource
	// RecurrenceClock (optional) overrides the cron supervisor's clock. nil
	// → RealClock in production; tests inject a FakeClock to drive ticks
	// deterministically (soc-63n0).
	RecurrenceClock daemonpkg.Clock
	// RecurrencePollInterval (optional) overrides the cron supervisor's poll
	// cadence. nil/0 → 1 minute default. Tests use a small value to advance
	// schedule firing within a single test run.
	RecurrencePollInterval time.Duration
}

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Run and inspect the AgentOps daemon",
}

var daemonRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run agentopsd in the foreground",
	Args:  cobra.NoArgs,
	RunE:  runAgentOpsDaemonCommand,
}

var daemonReadyCmd = &cobra.Command{
	Use:   "ready",
	Short: "Check daemon readiness",
	Args:  cobra.NoArgs,
	RunE:  runAgentOpsDaemonReadyCommand,
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon status",
	Args:  cobra.NoArgs,
	RunE:  runAgentOpsDaemonStatusCommand,
}

var daemonServiceCmd = &cobra.Command{
	Use:   "service",
	Short: "Service lifecycle scaffolding for agentopsd",
}

var daemonServiceInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Print the service install plan",
	Args:  cobra.NoArgs,
	RunE:  runAgentOpsDaemonServiceInstallCommand,
}

func init() {
	daemonCmd.GroupID = "workflow"
	rootCmd.AddCommand(daemonCmd)
	daemonCmd.AddCommand(daemonRunCmd, daemonReadyCmd, daemonStatusCmd, daemonServiceCmd)
	daemonServiceCmd.AddCommand(daemonServiceInstallCmd)

	daemonRunCmd.Flags().StringVar(&daemonAddr, "addr", "127.0.0.1:8765", "Loopback address for foreground daemon")
	daemonRunCmd.Flags().StringVar(&daemonToken, "token", "", "Mutation token for daemon write routes")
	daemonRunCmd.Flags().StringVar(&daemonTokenFile, "token-file", "", "Path to mutation token file")
	daemonRunCmd.Flags().IntVar(&daemonWorkers, "workers", 0, "Number of daemon worker loops to run in the foreground")
	daemonRunCmd.Flags().BoolVar(&daemonWorkerOnce, "worker-once", false, "Exit after each worker makes one claim attempt")
	daemonRunCmd.Flags().StringVar(&daemonExecutorPolicy, "executor-policy", "fake", "Daemon executor policy for workers (fake, gascity, cli-fallback)")
	daemonRunCmd.Flags().DurationVar(&daemonWorkerTimeout, "worker-timeout", 0, "Per-job worker wall-clock cap (0 disables)")
	daemonRunCmd.Flags().Int64Var(&daemonWorkerMemoryMax, "worker-memory-max-bytes", 0, "Linux cgroup v2 memory.max cap for CLI fallback workers in bytes (0 disables)")
	daemonRunCmd.Flags().StringVar(&daemonWorkerCgroupRoot, "worker-cgroup-root", "", "Linux cgroup v2 root for worker caps (default /sys/fs/cgroup)")
	daemonRunCmd.Flags().StringVar(&daemonScheduleFile, "schedule-file", "", "Path to .agents/schedule.yaml. If empty, auto-detect at .agents/schedule.yaml relative to cwd. Skip schedule loading entirely if neither flag nor default file exists.")
	daemonRunCmd.Flags().StringVar(&daemonGasCityEndpoint, "gascity-endpoint", "", "GasCity API endpoint for gascity executor policy")
	daemonRunCmd.Flags().StringVar(&daemonGasCityCity, "gascity-city", "", "GasCity city name for gascity executor policy")
	daemonRunCmd.Flags().StringVar(&daemonGasCityToken, "gascity-token", "", "GasCity mutation token for gascity executor policy")
	daemonRunCmd.Flags().StringVar(&daemonGasCityTokenFile, "gascity-token-file", "", "Path to GasCity mutation token file")
	daemonReadyCmd.Flags().StringVar(&daemonURL, "url", "", "Daemon base URL (defaults to activation file)")
	daemonStatusCmd.Flags().StringVar(&daemonURL, "url", "", "Daemon base URL (defaults to activation file)")
	daemonServiceInstallCmd.Flags().StringVar(&daemonAddr, "addr", "127.0.0.1:8765", "Loopback address for service plan")
	daemonServiceInstallCmd.Flags().StringVar(&daemonServiceExecutable, "executable", "ao", "ao executable path for service plan")
}

func runAgentOpsDaemonCommand(cmd *cobra.Command, args []string) error {
	cwd, err := resolveProjectDir()
	if err != nil {
		return err
	}
	return serveAgentOpsDaemon(cobraContext(cmd), cwd, agentopsDaemonRunOptions{
		Addr:             daemonAddr,
		Token:            daemonToken,
		TokenFile:        daemonTokenFile,
		Workers:          daemonWorkers,
		WorkerOnce:       daemonWorkerOnce,
		ExecutorPolicy:   daemonExecutorPolicy,
		GasCityEndpoint:  daemonGasCityEndpoint,
		GasCityCity:      daemonGasCityCity,
		GasCityToken:     daemonGasCityToken,
		GasCityTokenFile: daemonGasCityTokenFile,
		WorkerTimeout:    daemonWorkerTimeout,
		WorkerMemoryMax:  daemonWorkerMemoryMax,
		WorkerCgroupRoot: daemonWorkerCgroupRoot,
		ScheduleFile:     daemonScheduleFile,
	}, cmd.OutOrStdout())
}

func runAgentOpsDaemonReadyCommand(cmd *cobra.Command, args []string) error {
	cwd, err := resolveProjectDir()
	if err != nil {
		return err
	}
	baseURL, err := resolveDaemonURL(cwd, daemonURL)
	if err != nil {
		return err
	}
	ready, err := fetchDaemonReady(cobraContext(cmd), baseURL)
	if err != nil {
		return err
	}
	if !ready.Ready {
		return fmt.Errorf("agentopsd not ready: replay=%s projection=%s degraded=%v", ready.LedgerReplayStatus, ready.ProjectionStatus, ready.DegradedReasons)
	}
	if GetOutput() == "json" {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(ready)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "agentopsd ready: %s\n", baseURL)
	return nil
}

func runAgentOpsDaemonStatusCommand(cmd *cobra.Command, args []string) error {
	cwd, err := resolveProjectDir()
	if err != nil {
		return err
	}
	baseURL, err := resolveDaemonURL(cwd, daemonURL)
	if err != nil {
		return err
	}
	status, err := fetchDaemonStatus(cobraContext(cmd), baseURL)
	if err != nil {
		return err
	}
	if GetOutput() == "json" {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(status)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "agentopsd status\n")
	fmt.Fprintf(cmd.OutOrStdout(), "ready: %v\n", status.Ready)
	fmt.Fprintf(cmd.OutOrStdout(), "events: %d\n", status.ProjectionLag.EventCount)
	fmt.Fprintf(cmd.OutOrStdout(), "jobs: %d\n", len(status.Queue.Jobs))
	if status.ProjectionLag.Degraded {
		fmt.Fprintf(cmd.OutOrStdout(), "degraded: %d corrupt record(s)\n", status.ProjectionLag.CorruptRecordCount)
	}
	return nil
}

func runAgentOpsDaemonServiceInstallCommand(cmd *cobra.Command, args []string) error {
	cwd, err := resolveProjectDir()
	if err != nil {
		return err
	}
	plan := daemonpkg.BuildServiceInstallPlan(cwd, daemonServiceExecutable, daemonAddr, GetDryRun())
	if !GetDryRun() {
		return errors.New("daemon service install is dry-run only in this slice")
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(plan)
}

func serveAgentOpsDaemon(ctx context.Context, cwd string, opts agentopsDaemonRunOptions, out anyWriter) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if opts.Workers < 0 {
		return errors.New("daemon workers must be >= 0")
	}
	server, listener, activation, err := startAgentOpsDaemon(ctx, cwd, opts)
	if err != nil {
		return err
	}
	defer listener.Close()
	fmt.Fprintf(out, "agentopsd ready: %s\n", activation.URL)
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Serve(listener)
	}()
	// Cron supervisor: runs regardless of Workers setting because schedule
	// firing is independent of job execution — it appends schedule.fired
	// events and submits jobs into the queue, which workers (if present)
	// consume. Without this goroutine the daemon emits schedule.created on
	// load and then never fires anything (soc-63n0 — RecurrenceSupervisor
	// was implemented but unwired in the daemon-run path).
	startAgentOpsDaemonRecurrence(ctx, cwd, opts)
	if opts.Workers > 0 {
		return serveAgentOpsDaemonWithWorkers(ctx, cwd, opts, server, errCh)
	}
	return waitForAgentOpsDaemonServer(ctx, server, errCh)
}

func serveAgentOpsDaemonWithWorkers(ctx context.Context, cwd string, opts agentopsDaemonRunOptions, server *http.Server, errCh <-chan error) error {
	supervisor, err := buildAgentOpsDaemonSupervisor(cwd, opts)
	if err != nil {
		serverErr := shutdownAgentOpsDaemon(ctx, server)
		serveErr := normalizeAgentOpsDaemonServeError(<-errCh)
		return firstAgentOpsDaemonError(err, serverErr, serveErr)
	}
	if opts.WorkerOnce {
		return serveAgentOpsDaemonWorkersOnce(ctx, supervisor, opts.Workers, server, errCh)
	}
	return serveAgentOpsDaemonWorkerLoops(ctx, supervisor, opts.Workers, server, errCh)
}

// startAgentOpsDaemonRecurrence builds a RecurrenceSupervisor over the
// daemon's store + queue and runs Start(ctx) in a goroutine. The supervisor
// shares persistence with the worker pool via O_APPEND atomicity (each
// NewQueue/NewStore is cheap and lightweight; the daemon already follows
// this pattern in NewSupervisor and the HTTP handlers). Returns the
// supervisor for tests that want to assert wiring; production callers
// ignore the return.
func startAgentOpsDaemonRecurrence(ctx context.Context, cwd string, opts agentopsDaemonRunOptions) *daemonpkg.RecurrenceSupervisor {
	store := daemonpkg.NewStore(cwd)
	queueOpts := daemonpkg.QueueOptions{}
	if opts.Now != nil {
		queueOpts.Now = opts.Now
	}
	queue := daemonpkg.NewQueue(store, queueOpts)
	clock := opts.RecurrenceClock
	if clock == nil {
		clock = daemonpkg.RealClock{}
	}
	sup := daemonpkg.NewRecurrenceSupervisor(store, queue, clock)
	if opts.RecurrencePollInterval > 0 {
		sup.WithPollInterval(opts.RecurrencePollInterval)
	}
	go func() {
		for ctx.Err() == nil {
			recovered := false
			func() {
				defer func() {
					if r := recover(); r != nil {
						recovered = true
						log.Printf("[recurrence] supervisor panic recovered: %v", r)
					}
				}()
				if err := sup.Start(ctx); err != nil {
					log.Printf("[recurrence] supervisor exited: %v", err)
				}
			}()
			if !recovered || ctx.Err() != nil {
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-clock.After(sup.PollInterval()):
			}
		}
	}()
	return sup
}

func serveAgentOpsDaemonWorkersOnce(ctx context.Context, supervisor *daemonpkg.Supervisor, workers int, server *http.Server, errCh <-chan error) error {
	workerErr := runAgentOpsDaemonWorkersOnce(ctx, supervisor, workers)
	serverErr := shutdownAgentOpsDaemon(ctx, server)
	serveErr := normalizeAgentOpsDaemonServeError(<-errCh)
	return firstAgentOpsDaemonError(workerErr, serverErr, serveErr)
}

func serveAgentOpsDaemonWorkerLoops(ctx context.Context, supervisor *daemonpkg.Supervisor, workers int, server *http.Server, errCh <-chan error) error {
	workerErrCh := startAgentOpsDaemonWorkerLoops(ctx, supervisor, workers)
	select {
	case <-ctx.Done():
		return firstAgentOpsDaemonError(shutdownAgentOpsDaemon(ctx, server), normalizeAgentOpsDaemonServeError(<-errCh))
	case err := <-workerErrCh:
		serverErr := shutdownAgentOpsDaemon(ctx, server)
		if err != nil && !errors.Is(err, context.Canceled) {
			return firstAgentOpsDaemonError(err, serverErr)
		}
		return serverErr
	case err := <-errCh:
		return normalizeAgentOpsDaemonServeError(err)
	}
}

func waitForAgentOpsDaemonServer(ctx context.Context, server *http.Server, errCh <-chan error) error {
	select {
	case <-ctx.Done():
		return firstAgentOpsDaemonError(shutdownAgentOpsDaemon(ctx, server), normalizeAgentOpsDaemonServeError(<-errCh))
	case err := <-errCh:
		return normalizeAgentOpsDaemonServeError(err)
	}
}

func normalizeAgentOpsDaemonServeError(err error) error {
	if err == nil || errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func firstAgentOpsDaemonError(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func buildAgentOpsDaemonSupervisor(cwd string, opts agentopsDaemonRunOptions) (*daemonpkg.Supervisor, error) {
	policy := opts.ExecutorPolicy
	if policy == "" {
		policy = "fake"
	}
	llmwikiExecutor := buildAgentOpsDaemonLLMWikiExecutor()
	var executors []daemonpkg.JobExecutor
	switch policy {
	case "fake":
		wikiExecutor, err := buildAgentOpsDaemonFakeWikiExecutor(cwd)
		if err != nil {
			return nil, err
		}
		dreamExecutor, err := buildAgentOpsDaemonDreamExecutor(cwd)
		if err != nil {
			return nil, err
		}
		rpiExecutor, err := buildAgentOpsDaemonFakeRPIExecutor(cwd)
		if err != nil {
			return nil, err
		}
		executors = []daemonpkg.JobExecutor{daemonFakeOpenClawSnapshotExecutor{}, wikiExecutor, dreamExecutor, rpiExecutor, llmwikiExecutor}
	case "gascity":
		wikiExecutor, err := buildAgentOpsDaemonGasCityWikiExecutor(cwd, opts)
		if err != nil {
			return nil, err
		}
		dreamExecutor, err := buildAgentOpsDaemonDreamExecutor(cwd)
		if err != nil {
			return nil, err
		}
		rpiExecutor, err := buildAgentOpsDaemonGasCityRPIExecutor(cwd, opts)
		if err != nil {
			return nil, err
		}
		executors = []daemonpkg.JobExecutor{wikiExecutor, dreamExecutor, rpiExecutor, llmwikiExecutor}
	case "cli-fallback":
		wikiExecutor, err := buildAgentOpsDaemonCLIFallbackWikiExecutor(cwd, opts)
		if err != nil {
			return nil, err
		}
		executors = []daemonpkg.JobExecutor{wikiExecutor, llmwikiExecutor}
	default:
		return nil, fmt.Errorf("unsupported daemon executor policy %q", policy)
	}
	// Plans projection executor (atom-2 / soc-acwf). Registered for the
	// fake + gascity policies (read-side absorption #6 / plans.projection).
	// Cli-fallback policy intentionally omitted per pilot spec §c (G2): that
	// policy strips down to the wiki executor and does not run plans jobs.
	// dbBdSource lazy-init: nil here means tests don't open a Dolt connection;
	// production callers can call NewDbBdSource and re-register. atom-2 ships
	// with executors that need bd connectivity gated by opts.PlansBdSource.
	if policy == "fake" || policy == "gascity" {
		if opts.PlansBdSource != nil {
			plansExec, err := daemonpkg.NewPlansProjectionExecutor(daemonpkg.PlansProjectionExecutorOptions{
				Store:    daemonpkg.NewStore(cwd),
				BdSource: opts.PlansBdSource,
				Now:      opts.Now,
			})
			if err != nil {
				return nil, err
			}
			executors = append(executors, plansExec)
		}
	}
	queueOpts := daemonpkg.QueueOptions{}
	if opts.Now != nil {
		queueOpts.Now = opts.Now
	}
	return daemonpkg.NewSupervisor(daemonpkg.SupervisorOptions{
		Queue:             daemonpkg.NewQueue(daemonpkg.NewStore(cwd), queueOpts),
		Executors:         executors,
		Actor:             "agentopsd-worker",
		PollInterval:      opts.PollInterval,
		HeartbeatInterval: opts.HeartbeatInterval,
		ExecutionTimeout:  opts.WorkerTimeout,
	})
}

// buildAgentOpsDaemonLLMWikiExecutor wires the Karpathy llmwiki loop executor
// with its four stage handlers. The PROMOTE stage's HarvestPromoter is the
// adapter in llmwiki_adapter.go that bridges to the harvest package's
// catalog-based API. Registered under all three policies (fake/gascity/cli-fallback)
// because the stages are deterministic and policy-agnostic — actual LLM
// extraction is stubbed inside the stage handlers and lands in a follow-up.
func buildAgentOpsDaemonLLMWikiExecutor() daemonpkg.JobExecutor {
	return &llmwiki.LLMWikiLoopExecutor{
		Ingest:  &llmwiki.IngestStage{},
		Query:   &llmwiki.QueryStage{},
		Lint:    &llmwiki.LintStage{},
		Promote: &llmwiki.PromoteStage{Harvest: llmwikiHarvestAdapter{}},
	}
}

func buildAgentOpsDaemonFakeWikiExecutor(cwd string) (daemonpkg.JobExecutor, error) {
	worker, err := wikiworker.NewWorker(newDaemonFakeWikiAgentWorker())
	if err != nil {
		return nil, err
	}
	return daemonpkg.NewWikiForgeExecutor(daemonpkg.WikiForgeExecutorOptions{
		Store:  daemonpkg.NewStore(cwd),
		Worker: worker,
	})
}

func buildAgentOpsDaemonDreamExecutor(cwd string) (daemonpkg.JobExecutor, error) {
	return daemonpkg.NewDreamExecutor(daemonpkg.DreamExecutorOptions{
		Cwd: cwd,
		RunLoop: func(ctx context.Context, opts daemonpkg.DreamRunLoopOptions) (daemonpkg.DreamRunLoopResult, error) {
			result, err := ovn.RunLoop(ctx, ovn.RunLoopOptions{
				Cwd:           opts.Cwd,
				OutputDir:     opts.OutputDir,
				RunID:         opts.RunID,
				MaxIterations: opts.MaxIterations,
				WarnOnly:      opts.WarnOnly,
				LogWriter:     opts.LogWriter,
			})
			mapped := daemonpkg.DreamRunLoopResult{Raw: result}
			if result != nil {
				mapped.IterationCount = len(result.Iterations)
				mapped.BudgetExhausted = result.BudgetExhausted
			}
			return mapped, err
		},
	})
}

func buildAgentOpsDaemonFakeRPIExecutor(cwd string) (daemonpkg.JobExecutor, error) {
	return daemonpkg.NewRPIJobExecutor(daemonpkg.RPIJobExecutorOptions{
		Store:    daemonpkg.NewStore(cwd),
		Executor: fakeRPIPhaseExecutor{},
	})
}

func buildAgentOpsDaemonGasCityRPIExecutor(cwd string, opts agentopsDaemonRunOptions) (daemonpkg.JobExecutor, error) {
	if opts.GasCityEndpoint == "" || opts.GasCityCity == "" {
		return nil, errors.New("gascity executor policy requires --gascity-endpoint and --gascity-city for rpi jobs")
	}
	token, err := resolveDaemonMutationToken(opts.GasCityToken, opts.GasCityTokenFile)
	if err != nil {
		return nil, err
	}
	client, err := gascity.NewClient(gascity.Config{Endpoint: opts.GasCityEndpoint, MutationToken: token})
	if err != nil {
		return nil, err
	}
	rpiPhaseExecutor := daemonpkg.GasCityRPIPhaseExecutor{
		Client:   daemonpkg.GasCityClientAdapter{Client: client},
		CityName: opts.GasCityCity,
	}
	return daemonpkg.NewRPIJobExecutor(daemonpkg.RPIJobExecutorOptions{
		Store:    daemonpkg.NewStore(cwd),
		Executor: rpiPhaseExecutor,
	})
}

// fakeRPIPhaseExecutor is a deterministic, CI-safe phase executor that returns
// pre-baked artifacts. Used by the "fake" daemon executor policy so end-to-end
// daemon-submit → supervisor → terminal-event tests can exercise the rpi.run
// / rpi.phase path without needing a real GasCity instance.
type fakeRPIPhaseExecutor struct{}

func (fakeRPIPhaseExecutor) ExecuteRPIPhase(_ context.Context, req daemonpkg.RPIPhaseExecutionRequest) (daemonpkg.RPIPhaseExecutionResult, error) {
	return daemonpkg.RPIPhaseExecutionResult{
		Status: "completed",
		Artifacts: map[string]string{
			"executor_policy": "fake",
			"phase":           fmt.Sprintf("%d", req.Phase),
			"goal":            req.Goal,
		},
	}, nil
}

func buildAgentOpsDaemonGasCityWikiExecutor(cwd string, opts agentopsDaemonRunOptions) (daemonpkg.JobExecutor, error) {
	if opts.GasCityEndpoint == "" || opts.GasCityCity == "" {
		return nil, errors.New("gascity executor policy requires --gascity-endpoint and --gascity-city")
	}
	token, err := resolveDaemonMutationToken(opts.GasCityToken, opts.GasCityTokenFile)
	if err != nil {
		return nil, err
	}
	client, err := gascity.NewClient(gascity.Config{Endpoint: opts.GasCityEndpoint, MutationToken: token})
	if err != nil {
		return nil, err
	}
	agent, err := agentworker.NewGasCityWorker(agentworker.GasCityWorkerOptions{
		Client:       agentworker.GasCityClientAdapter{Client: client},
		CityName:     opts.GasCityCity,
		TemplateName: os.Getenv("AGENTOPS_GASCITY_WORKER_TEMPLATE"),
	})
	if err != nil {
		return nil, err
	}
	worker, err := wikiworker.NewWorker(agent)
	if err != nil {
		return nil, err
	}
	return daemonpkg.NewWikiForgeExecutor(daemonpkg.WikiForgeExecutorOptions{
		Store:  daemonpkg.NewStore(cwd),
		Worker: worker,
	})
}

func buildAgentOpsDaemonCLIFallbackWikiExecutor(cwd string, opts agentopsDaemonRunOptions) (daemonpkg.JobExecutor, error) {
	agent, err := agentworker.NewCLIFallbackWorker(agentworker.CLIFallbackWorkerOptions{
		WallClockTimeout: opts.WorkerTimeout,
		MemoryMaxBytes:   opts.WorkerMemoryMax,
		CgroupRoot:       opts.WorkerCgroupRoot,
	})
	if err != nil {
		return nil, err
	}
	worker, err := wikiworker.NewWorker(agent)
	if err != nil {
		return nil, err
	}
	return daemonpkg.NewWikiForgeExecutor(daemonpkg.WikiForgeExecutorOptions{
		Store: daemonpkg.NewStore(cwd),
		Worker: providerOverrideWikiForgeWorker{
			inner:    worker,
			provider: agentworker.ProviderCLIFallback,
		},
	})
}

type providerOverrideWikiForgeWorker struct {
	inner    daemonpkg.WikiForgeWorker
	provider agentworker.Provider
}

func (w providerOverrideWikiForgeWorker) RunExtractionWithRetry(ctx context.Context, req wikiworker.ExtractionRequest, opts wikiworker.RetryOptions) (wikiworker.ExtractionResult, error) {
	if w.provider != "" {
		req.Provider = w.provider
	}
	return w.inner.RunExtractionWithRetry(ctx, req, opts)
}

func runAgentOpsDaemonWorkersOnce(ctx context.Context, supervisor *daemonpkg.Supervisor, workers int) error {
	for range workers {
		if _, err := supervisor.RunOnce(ctx); err != nil {
			return err
		}
	}
	return nil
}

func startAgentOpsDaemonWorkerLoops(ctx context.Context, supervisor *daemonpkg.Supervisor, workers int) <-chan error {
	errCh := make(chan error, workers)
	for range workers {
		go func() {
			errCh <- supervisor.RunLoop(ctx)
		}()
	}
	return errCh
}

func shutdownAgentOpsDaemon(ctx context.Context, server *http.Server) error {
	if ctx == nil {
		ctx = context.Background()
	}
	timeout := 2 * time.Second
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining > 0 && remaining < timeout {
			timeout = remaining
		}
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

type anyWriter interface {
	Write([]byte) (int, error)
}

func startAgentOpsDaemon(ctx context.Context, cwd string, opts agentopsDaemonRunOptions) (*http.Server, net.Listener, agentopsDaemonActivation, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	addr := opts.Addr
	if addr == "" {
		addr = "127.0.0.1:8765"
	}
	if err := daemonpkg.ValidateLocalBindAddress(addr); err != nil {
		return nil, nil, agentopsDaemonActivation{}, err
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, nil, agentopsDaemonActivation{}, err
	}
	actualAddr := listener.Addr().String()
	now := time.Now
	if opts.Now != nil {
		now = opts.Now
	}
	if _, err := ovn.RecoverFromCrash(cwd); err != nil {
		_ = listener.Close()
		return nil, nil, agentopsDaemonActivation{}, fmt.Errorf("daemon startup recovery: %w", err)
	}
	store := daemonpkg.NewStore(cwd)
	if err := loadAgentOpsDaemonScheduleFile(cwd, opts.ScheduleFile, store); err != nil {
		_ = listener.Close()
		return nil, nil, agentopsDaemonActivation{}, err
	}
	mutationPolicy, err := resolveAgentOpsDaemonMutationPolicy(opts.Token, opts.TokenFile)
	if err != nil {
		_ = listener.Close()
		return nil, nil, agentopsDaemonActivation{}, err
	}
	router := daemonpkg.NewDaemonRouter(store, daemonpkg.ServerOptions{
		Now:            now,
		MutationPolicy: mutationPolicy,
	})
	server := &http.Server{
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}
	activation := agentopsDaemonActivation{
		URL:       "http://" + actualAddr,
		Address:   actualAddr,
		PID:       os.Getpid(),
		Ready:     true,
		StartedAt: now().UTC().Format(time.RFC3339Nano),
		Token:     opts.Token,
		TokenFile: opts.TokenFile,
	}
	if err := writeDaemonActivation(cwd, activation); err != nil {
		_ = listener.Close()
		return nil, nil, agentopsDaemonActivation{}, err
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	return server, listener, activation, nil
}

func agentOpsDaemonMutationPaths() []string {
	return []string{
		"/jobs",
		"/v1/jobs",
		"/jobs/cancel",
		"/v1/jobs/cancel",
		"/v1/jobs/*/cancel",
		"/openclaw/v1/triggers/jobs",
		"/v1/schedules",
		"/v1/schedules/*",
	}
}

func resolveAgentOpsDaemonMutationPolicy(token, tokenFile string) (daemonpkg.MutationPolicy, error) {
	allowedPaths := agentOpsDaemonMutationPaths()
	if tokenFile != "" {
		tokens, err := daemonpkg.LoadMutationTokensFile(tokenFile)
		if err != nil {
			return daemonpkg.MutationPolicy{}, err
		}
		policy := daemonpkg.DefaultMutationPolicy("", allowedPaths)
		policy.Tokens = tokens
		return policy, nil
	}
	if token == "" {
		return daemonpkg.DefaultMutationPolicy("", allowedPaths), nil
	}
	return daemonpkg.DefaultMutationPolicy(token, allowedPaths), nil
}

func resolveDaemonMutationToken(token, tokenFile string) (string, error) {
	if tokenFile != "" {
		return daemonpkg.LoadMutationTokenFile(tokenFile)
	}
	return token, nil
}

func resolveAgentOpsDaemonClientMutationToken(cwd, token, tokenFile string) (string, error) {
	if strings.TrimSpace(token) != "" {
		return token, nil
	}
	if strings.TrimSpace(tokenFile) != "" {
		return daemonpkg.LoadMutationTokenFile(tokenFile)
	}
	if envToken := strings.TrimSpace(os.Getenv("AGENTOPSD_TOKEN")); envToken != "" {
		return envToken, nil
	}
	if envToken := strings.TrimSpace(os.Getenv("AGENTOPS_DAEMON_TOKEN")); envToken != "" {
		return envToken, nil
	}
	if strings.TrimSpace(cwd) == "" {
		return "", nil
	}
	if _, err := os.Stat(daemonActivationPath(cwd)); err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	activation, err := readDaemonActivation(cwd)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(activation.Token) != "" {
		return activation.Token, nil
	}
	if strings.TrimSpace(activation.TokenFile) != "" {
		return daemonpkg.LoadMutationTokenFile(activation.TokenFile)
	}
	return "", nil
}

// loadAgentOpsDaemonScheduleFile resolves the schedule file path (flag wins over
// auto-detect at .agents/schedule.yaml), parses it, and saves each schedule
// into the Store. Per amendment C9, malformed YAML is fail-closed: the daemon
// refuses to start so operators see the error rather than silently running
// with zero schedules. If neither the flag is set nor the default file exists,
// schedule loading is skipped entirely (bit-identical pre-upgrade behavior).
func loadAgentOpsDaemonScheduleFile(cwd, flagPath string, store *daemonpkg.Store) error {
	schedulePath := flagPath
	if schedulePath == "" {
		autoPath := filepath.Join(cwd, ".agents", "schedule.yaml")
		if _, err := os.Stat(autoPath); err == nil {
			schedulePath = autoPath
		}
	}
	if schedulePath == "" {
		return nil
	}
	templates, err := schedule.Load(schedulePath)
	if err != nil {
		return fmt.Errorf("schedule load failed at %s: %w (daemon refuses to start with malformed schedule.yaml; fix the file or remove --schedule-file)", schedulePath, err)
	}
	for _, t := range templates {
		if saveErr := store.SaveSchedule(t); saveErr != nil {
			// Idempotent re-save: an existing schedule with the same name
			// returns an "already exists" error from the store. Treat that
			// as warn-and-continue rather than crash so daemon restarts that
			// replay an existing ledger remain functional.
			log.Printf("schedule %q: SaveSchedule warning: %v", t.Name, saveErr)
		}
	}
	log.Printf("schedule-file: loaded %d schedule(s) from %s", len(templates), schedulePath)
	return nil
}

func writeDaemonActivation(cwd string, activation agentopsDaemonActivation) error {
	path := daemonActivationPath(cwd)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(activation, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0600)
}

func readDaemonActivation(cwd string) (agentopsDaemonActivation, error) {
	var activation agentopsDaemonActivation
	data, err := os.ReadFile(daemonActivationPath(cwd))
	if err != nil {
		return activation, err
	}
	if err := json.Unmarshal(data, &activation); err != nil {
		return activation, err
	}
	if activation.URL == "" {
		return activation, errors.New("daemon activation file missing url")
	}
	return activation, nil
}

func daemonActivationPath(cwd string) string {
	return filepath.Join(cwd, daemonActivationRelPath)
}

func resolveDaemonURL(cwd, explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	activation, err := readDaemonActivation(cwd)
	if err != nil {
		return "", fmt.Errorf("read daemon activation: %w", err)
	}
	return activation.URL, nil
}

func fetchDaemonReady(ctx context.Context, baseURL string) (daemonpkg.ReadOnlyReadyResponse, error) {
	var ready daemonpkg.ReadOnlyReadyResponse
	if err := fetchDaemonJSON(ctx, baseURL+"/ready", &ready); err != nil {
		return ready, err
	}
	return ready, nil
}

func fetchDaemonStatus(ctx context.Context, baseURL string) (daemonpkg.ReadOnlyStatusResponse, error) {
	var status daemonpkg.ReadOnlyStatusResponse
	if err := fetchDaemonJSON(ctx, baseURL+"/status", &status); err != nil {
		return status, err
	}
	return status, nil
}

func fetchDaemonJSON(ctx context.Context, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("daemon returned HTTP %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func cobraContext(cmd *cobra.Command) context.Context {
	if cmd == nil || cmd.Context() == nil {
		return context.Background()
	}
	return cmd.Context()
}
