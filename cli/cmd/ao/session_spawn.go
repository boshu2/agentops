// practices: [event-sourcing-cqrs, distributed-tracing]
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"
)

type SessionTemplate struct {
	SchemaVersion int              `toml:"schema_version" json:"schema_version"`
	Role          string           `toml:"role" json:"role"`
	Description   string           `toml:"description" json:"description"`
	Identity      SessionIdentity  `toml:"identity" json:"identity"`
	Workspace     SessionWorkspace `toml:"workspace" json:"workspace"`
	Init          SessionInit      `toml:"init" json:"init"`
	Tmux          SessionTmux      `toml:"tmux" json:"tmux"`
	Heartbeat     SessionHeartbeat `toml:"heartbeat" json:"heartbeat"`
	OnExit        SessionOnExit    `toml:"on_exit" json:"on_exit"`
	Invariants    map[string]string `toml:"invariants" json:"invariants,omitempty"`
	References    map[string]string `toml:"references" json:"references,omitempty"`
}

type SessionIdentity struct {
	BeadsActorTemplate  string `toml:"beads_actor_template" json:"beads_actor_template"`
	SessionNameTemplate string `toml:"session_name_template" json:"session_name_template"`
	Agent               string `toml:"agent" json:"agent"`
}

type SessionWorkspace struct {
	Cwd                       string `toml:"cwd" json:"cwd"`
	ValidatorIdentityFile     string `toml:"validator_identity_file" json:"validator_identity_file,omitempty"`
	ValidatorIdentityTemplate string `toml:"validator_identity_template" json:"validator_identity_template,omitempty"`
}

type SessionInit struct {
	Steps []SessionInitStep `toml:"steps" json:"steps"`
}

type SessionInitStep struct {
	Name string `toml:"name" json:"name"`
	Cmd  string `toml:"cmd" json:"cmd"`
	Note string `toml:"note" json:"note,omitempty"`
}

type SessionTmux struct {
	SessionName string        `toml:"session_name" json:"session_name"`
	Panes       []SessionPane `toml:"panes" json:"panes"`
}

type SessionPane struct {
	Position string `toml:"position" json:"position"`
	Cmd      string `toml:"cmd" json:"cmd"`
}

type SessionHeartbeat struct {
	CadenceMinutes int    `toml:"cadence_minutes" json:"cadence_minutes,omitempty"`
	TargetIssue    string `toml:"target_issue" json:"target_issue,omitempty"`
	Template       string `toml:"template" json:"template,omitempty"`
}

type SessionOnExit struct {
	AutoHandoff         bool   `toml:"auto_handoff" json:"auto_handoff,omitempty"`
	HandoffPathTemplate string `toml:"handoff_path_template" json:"handoff_path_template,omitempty"`
	HandoffCommand      string `toml:"handoff_command" json:"handoff_command,omitempty"`
}

var (
	spawnDryRun bool
	spawnNoTmux bool
	spawnDate   string
)

var sessionSpawnCmd = &cobra.Command{
	Use:   "spawn <template-path>",
	Short: "Cold-start a session from a TOML template",
	Long: `Read a session template, expand variables, run init steps, and create
a tmux session with the configured panes.

The template defines the session role, init steps (context loading, handoff
replay, bead ownership scan), tmux layout, heartbeat cadence, and exit hooks.

Examples:
  ao session spawn ~/.agentops/sessions/claude-validator.toml
  ao session spawn ~/.agentops/sessions/claude-validator.toml --dry-run
  ao session spawn ~/.agentops/sessions/claude-validator.toml --no-tmux`,
	Args: cobra.ExactArgs(1),
	RunE: runSessionSpawn,
}

func init() {
	sessionsCmd.AddCommand(sessionSpawnCmd)
	sessionSpawnCmd.Flags().BoolVar(&spawnDryRun, "dry-run", false, "Print expanded template and init steps without executing")
	sessionSpawnCmd.Flags().BoolVar(&spawnNoTmux, "no-tmux", false, "Run init steps but skip tmux session creation")
	sessionSpawnCmd.Flags().StringVar(&spawnDate, "date", "", "Override date for template expansion (default: today, YYYY-MM-DD)")
}

func loadSessionTemplate(path string) (*SessionTemplate, error) {
	var tmpl SessionTemplate
	if _, err := toml.DecodeFile(path, &tmpl); err != nil {
		return nil, fmt.Errorf("parse template %s: %w", path, err)
	}
	if tmpl.SchemaVersion != 1 {
		return nil, fmt.Errorf("unsupported schema_version %d (want 1)", tmpl.SchemaVersion)
	}
	if tmpl.Role == "" {
		return nil, fmt.Errorf("template missing required field: role")
	}
	if tmpl.Identity.SessionNameTemplate == "" {
		return nil, fmt.Errorf("template missing required field: identity.session_name_template")
	}
	return &tmpl, nil
}

func buildTemplateVars(tmpl *SessionTemplate, dateOverride string) map[string]string {
	dateVal := time.Now().UTC().Format("2006-01-02")
	if dateOverride != "" {
		dateVal = dateOverride
	}
	hostname, _ := os.Hostname()
	home, _ := os.UserHomeDir()

	sessionName := tmpl.Identity.SessionNameTemplate
	sessionName = strings.ReplaceAll(sessionName, "{{date}}", dateVal)
	sessionName = strings.ReplaceAll(sessionName, "{{hostname}}", hostname)

	return map[string]string{
		"{{date}}":         dateVal,
		"{{hostname}}":     hostname,
		"{{session_name}}": sessionName,
		"{{timestamp}}":    time.Now().UTC().Format(time.RFC3339),
		"$HOME":            home,
	}
}

func expandVars(s string, vars map[string]string) string {
	result := s
	for k, v := range vars {
		result = strings.ReplaceAll(result, k, v)
	}
	return result
}

func runInitSteps(steps []SessionInitStep, vars map[string]string, cwd string, dryRun bool) error {
	for i, step := range steps {
		expanded := expandVars(step.Cmd, vars)
		if dryRun {
			fmt.Printf("  [%d] %s\n", i+1, step.Name)
			if step.Note != "" {
				fmt.Printf("      # %s\n", step.Note)
			}
			fmt.Printf("      $ %s\n", strings.Split(expanded, "\n")[0])
			if strings.Contains(expanded, "\n") {
				fmt.Printf("      ... (%d lines)\n", strings.Count(expanded, "\n")+1)
			}
			continue
		}
		fmt.Printf("  [%d/%d] %s ... ", i+1, len(steps), step.Name)
		cmd := exec.Command("bash", "-c", expanded)
		cmd.Dir = cwd
		cmd.Env = append(os.Environ(), "BEADS_ACTOR="+expandVars(steps[0].Cmd, vars))
		out, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("FAILED\n")
			if len(out) > 0 {
				fmt.Printf("      %s\n", strings.TrimSpace(string(out)))
			}
			return fmt.Errorf("init step %q failed: %w", step.Name, err)
		}
		fmt.Printf("ok\n")
	}
	return nil
}

func createTmuxSession(tmuxCfg SessionTmux, vars map[string]string, cwd string, dryRun bool) error {
	sessionName := expandVars(tmuxCfg.SessionName, vars)

	if !dryRun {
		check := exec.Command("tmux", "has-session", "-t", sessionName)
		if check.Run() == nil {
			return fmt.Errorf("tmux session %q already exists", sessionName)
		}
	}

	if dryRun {
		fmt.Printf("\nTmux session: %s\n", sessionName)
		for _, pane := range tmuxCfg.Panes {
			fmt.Printf("  [%s] %s\n", pane.Position, expandVars(pane.Cmd, vars))
		}
		return nil
	}

	newCmd := exec.Command("tmux", "new-session", "-d", "-s", sessionName, "-c", cwd)
	if out, err := newCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tmux new-session: %s: %w", strings.TrimSpace(string(out)), err)
	}

	for i, pane := range tmuxCfg.Panes {
		expandedCmd := expandVars(pane.Cmd, vars)
		if i == 0 {
			send := exec.Command("tmux", "send-keys", "-t", sessionName, expandedCmd, "Enter")
			if err := send.Run(); err != nil {
				return fmt.Errorf("tmux send-keys main pane: %w", err)
			}
			continue
		}

		var splitArgs []string
		switch {
		case strings.HasPrefix(pane.Position, "right-"):
			pct := strings.TrimPrefix(pane.Position, "right-")
			pct = strings.TrimSuffix(pct, "%")
			splitArgs = []string{"split-window", "-t", sessionName, "-h", "-p", pct}
		case strings.HasPrefix(pane.Position, "bottom-"):
			pct := strings.TrimPrefix(pane.Position, "bottom-")
			pct = strings.TrimSuffix(pct, "%")
			splitArgs = []string{"split-window", "-t", sessionName, "-v", "-p", pct}
		default:
			splitArgs = []string{"split-window", "-t", sessionName}
		}

		split := exec.Command("tmux", splitArgs...)
		if out, err := split.CombinedOutput(); err != nil {
			return fmt.Errorf("tmux split-window %s: %s: %w", pane.Position, strings.TrimSpace(string(out)), err)
		}

		paneTarget := fmt.Sprintf("%s.%d", sessionName, i)
		send := exec.Command("tmux", "send-keys", "-t", paneTarget, expandedCmd, "Enter")
		if err := send.Run(); err != nil {
			return fmt.Errorf("tmux send-keys pane %d: %w", i, err)
		}
	}

	fmt.Printf("Tmux session created: %s (%d panes)\n", sessionName, len(tmuxCfg.Panes))
	fmt.Printf("Attach with: tmux attach-session -t %s\n", sessionName)
	return nil
}

func runSessionSpawn(cmd *cobra.Command, args []string) error {
	templatePath := args[0]
	if strings.HasPrefix(templatePath, "~/") {
		home, _ := os.UserHomeDir()
		templatePath = filepath.Join(home, templatePath[2:])
	}

	tmpl, err := loadSessionTemplate(templatePath)
	if err != nil {
		return err
	}

	vars := buildTemplateVars(tmpl, spawnDate)
	sessionName := vars["{{session_name}}"]
	cwd := expandVars(tmpl.Workspace.Cwd, vars)

	fmt.Printf("Session: %s (role: %s, agent: %s)\n", sessionName, tmpl.Role, tmpl.Identity.Agent)
	fmt.Printf("Working directory: %s\n", cwd)

	if spawnDryRun {
		fmt.Printf("\nInit steps:\n")
	} else {
		fmt.Printf("\nRunning %d init steps:\n", len(tmpl.Init.Steps))
	}

	if err := runInitSteps(tmpl.Init.Steps, vars, cwd, spawnDryRun); err != nil {
		return err
	}

	if spawnNoTmux {
		fmt.Printf("\n--no-tmux: skipping tmux session creation\n")
		return nil
	}

	if err := createTmuxSession(tmpl.Tmux, vars, cwd, spawnDryRun); err != nil {
		return err
	}

	return nil
}
