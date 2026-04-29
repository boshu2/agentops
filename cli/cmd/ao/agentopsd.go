package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	daemonpkg "github.com/boshu2/agentops/cli/internal/daemon"
	ovn "github.com/boshu2/agentops/cli/internal/overnight"
	"github.com/spf13/cobra"
)

const daemonActivationRelPath = ".agents/daemon/activation.json"

var (
	daemonAddr              string
	daemonURL               string
	daemonToken             string
	daemonTokenFile         string
	daemonServiceExecutable string
)

type agentopsDaemonActivation struct {
	URL       string `json:"url"`
	Address   string `json:"address"`
	PID       int    `json:"pid"`
	Ready     bool   `json:"ready"`
	StartedAt string `json:"started_at"`
}

type agentopsDaemonRunOptions struct {
	Addr      string
	Token     string
	TokenFile string
	Now       func() time.Time
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
	return serveAgentOpsDaemon(cmd.Context(), cwd, agentopsDaemonRunOptions{
		Addr:      daemonAddr,
		Token:     daemonToken,
		TokenFile: daemonTokenFile,
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
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
		err := <-errCh
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case err := <-errCh:
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

type anyWriter interface {
	Write([]byte) (int, error)
}

func startAgentOpsDaemon(ctx context.Context, cwd string, opts agentopsDaemonRunOptions) (*http.Server, net.Listener, agentopsDaemonActivation, error) {
	addr := opts.Addr
	if addr == "" {
		addr = "127.0.0.1:8765"
	}
	if err := daemonpkg.ValidateLocalBindAddress(addr); err != nil {
		return nil, nil, agentopsDaemonActivation{}, err
	}
	token, err := resolveDaemonMutationToken(opts.Token, opts.TokenFile)
	if err != nil {
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
	router := daemonpkg.NewDaemonRouter(store, daemonpkg.ServerOptions{
		Now: now,
		MutationPolicy: daemonpkg.DefaultMutationPolicy(token, []string{
			"/jobs",
			"/v1/jobs",
			"/openclaw/v1/triggers/jobs",
		}),
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

func resolveDaemonMutationToken(token, tokenFile string) (string, error) {
	if tokenFile != "" {
		return daemonpkg.LoadMutationTokenFile(tokenFile)
	}
	return token, nil
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
