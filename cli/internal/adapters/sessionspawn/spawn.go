// practices: [event-sourcing-cqrs, distributed-tracing]
package sessionspawn

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/boshu2/agentops/cli/internal/shellutil"
)

// Template is the session-spawn TOML contract.
type Template struct {
	SchemaVersion int               `toml:"schema_version" json:"schema_version"`
	Role          string            `toml:"role" json:"role"`
	Description   string            `toml:"description" json:"description"`
	Identity      Identity          `toml:"identity" json:"identity"`
	Workspace     Workspace         `toml:"workspace" json:"workspace"`
	Init          Init              `toml:"init" json:"init"`
	Tmux          Tmux              `toml:"tmux" json:"tmux"`
	Heartbeat     Heartbeat         `toml:"heartbeat" json:"heartbeat"`
	OnExit        OnExit            `toml:"on_exit" json:"on_exit"`
	Invariants    map[string]string `toml:"invariants" json:"invariants,omitempty"`
	References    map[string]string `toml:"references" json:"references,omitempty"`
}

// Identity configures the spawned session identity fields.
type Identity struct {
	BeadsActorTemplate  string `toml:"beads_actor_template" json:"beads_actor_template"`
	SessionNameTemplate string `toml:"session_name_template" json:"session_name_template"`
	Agent               string `toml:"agent" json:"agent"`
}

// Workspace configures the spawned session workspace.
type Workspace struct {
	Cwd                       string `toml:"cwd" json:"cwd"`
	ValidatorIdentityFile     string `toml:"validator_identity_file" json:"validator_identity_file,omitempty"`
	ValidatorIdentityTemplate string `toml:"validator_identity_template" json:"validator_identity_template,omitempty"`
}

// Init configures session initialization steps.
type Init struct {
	Steps []InitStep `toml:"steps" json:"steps"`
}

// InitStep is one shell command to run before tmux pane creation.
type InitStep struct {
	Name string `toml:"name" json:"name"`
	Cmd  string `toml:"cmd" json:"cmd"`
	Note string `toml:"note" json:"note,omitempty"`
}

// Tmux configures the tmux session and panes.
type Tmux struct {
	SessionName string `toml:"session_name" json:"session_name"`
	Panes       []Pane `toml:"panes" json:"panes"`
}

// Pane is one tmux pane command.
type Pane struct {
	Position string `toml:"position" json:"position"`
	Cmd      string `toml:"cmd" json:"cmd"`
}

// Heartbeat configures optional session heartbeat behavior.
type Heartbeat struct {
	CadenceMinutes int    `toml:"cadence_minutes" json:"cadence_minutes,omitempty"`
	TargetIssue    string `toml:"target_issue" json:"target_issue,omitempty"`
	Template       string `toml:"template" json:"template,omitempty"`
}

// OnExit configures optional session closeout behavior.
type OnExit struct {
	AutoHandoff         bool   `toml:"auto_handoff" json:"auto_handoff,omitempty"`
	HandoffPathTemplate string `toml:"handoff_path_template" json:"handoff_path_template,omitempty"`
	HandoffCommand      string `toml:"handoff_command" json:"handoff_command,omitempty"`
}

// Options configures session spawning.
type Options struct {
	TemplatePath string
	DryRun       bool
	NoTmux       bool
	Date         string
	Out          io.Writer
}

// LoadTemplate reads and validates a session template.
func LoadTemplate(path string) (*Template, error) {
	var tmpl Template
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

// SanitizeHostname strips any character outside the RFC-1123 hostname charset
// so a hostile or unusual hostname cannot inject shell metacharacters when it
// is expanded into init-step commands or tmux session names.
func SanitizeHostname(h string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		case r == '.', r == '-', r == '_':
			return r
		default:
			return -1
		}
	}, h)
}

// BuildTemplateVars resolves standard variables for a session template.
func BuildTemplateVars(tmpl *Template, dateOverride string) map[string]string {
	dateVal := time.Now().UTC().Format("2006-01-02")
	if dateOverride != "" {
		dateVal = dateOverride
	}
	hostname, _ := os.Hostname()
	hostname = SanitizeHostname(hostname)
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

// ExpandVars applies template variables to a string.
func ExpandVars(s string, vars map[string]string) string {
	result := s
	for k, v := range vars {
		result = strings.ReplaceAll(result, k, v)
	}
	return result
}

// RunInitSteps executes or prints the template init steps.
func RunInitSteps(steps []InitStep, vars map[string]string, cwd, beadsActor string, dryRun bool, out io.Writer) error {
	out = writerOrStdout(out)
	for i, step := range steps {
		expanded := ExpandVars(step.Cmd, vars)
		if dryRun {
			fmt.Fprintf(out, "  [%d] %s\n", i+1, step.Name)
			if step.Note != "" {
				fmt.Fprintf(out, "      # %s\n", step.Note)
			}
			fmt.Fprintf(out, "      $ %s\n", strings.Split(expanded, "\n")[0])
			if strings.Contains(expanded, "\n") {
				fmt.Fprintf(out, "      ... (%d lines)\n", strings.Count(expanded, "\n")+1)
			}
			continue
		}
		fmt.Fprintf(out, "  [%d/%d] %s ... ", i+1, len(steps), step.Name)
		cmd := shellutil.SanitizedBashCommand(context.Background(), expanded)
		cmd.Dir = cwd
		if beadsActor != "" {
			// SanitizedBashCommand already populates cmd.Env with the sanitized
			// parent environment; append rather than overwrite it.
			cmd.Env = append(cmd.Env, "BEADS_ACTOR="+beadsActor)
		}
		cmdOut, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Fprintln(out, "FAILED")
			if len(cmdOut) > 0 {
				fmt.Fprintf(out, "      %s\n", strings.TrimSpace(string(cmdOut)))
			}
			return fmt.Errorf("init step %q failed: %w", step.Name, err)
		}
		fmt.Fprintln(out, "ok")
	}
	return nil
}

// CreateTmuxSession creates or prints the configured tmux session.
func CreateTmuxSession(tmuxCfg Tmux, vars map[string]string, cwd string, dryRun bool, out io.Writer) error {
	out = writerOrStdout(out)
	sessionName := ExpandVars(tmuxCfg.SessionName, vars)

	if !dryRun {
		check := exec.Command("tmux", "has-session", "-t", sessionName)
		if check.Run() == nil {
			return fmt.Errorf("tmux session %q already exists", sessionName)
		}
	}

	if dryRun {
		fmt.Fprintf(out, "\nTmux session: %s\n", sessionName)
		for _, pane := range tmuxCfg.Panes {
			fmt.Fprintf(out, "  [%s] %s\n", pane.Position, ExpandVars(pane.Cmd, vars))
		}
		return nil
	}

	newCmd := exec.Command("tmux", "new-session", "-d", "-s", sessionName, "-c", cwd)
	if cmdOut, err := newCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tmux new-session: %s: %w", strings.TrimSpace(string(cmdOut)), err)
	}

	for i, pane := range tmuxCfg.Panes {
		expandedCmd := ExpandVars(pane.Cmd, vars)
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
		if cmdOut, err := split.CombinedOutput(); err != nil {
			return fmt.Errorf("tmux split-window %s: %s: %w", pane.Position, strings.TrimSpace(string(cmdOut)), err)
		}

		paneTarget := fmt.Sprintf("%s.%d", sessionName, i)
		send := exec.Command("tmux", "send-keys", "-t", paneTarget, expandedCmd, "Enter")
		if err := send.Run(); err != nil {
			return fmt.Errorf("tmux send-keys pane %d: %w", i, err)
		}
	}

	fmt.Fprintf(out, "Tmux session created: %s (%d panes)\n", sessionName, len(tmuxCfg.Panes))
	fmt.Fprintf(out, "Attach with: tmux attach-session -t %s\n", sessionName)
	return nil
}

// Run loads a template and spawns the requested session.
func Run(opts Options) error {
	out := writerOrStdout(opts.Out)
	templatePath := expandHome(opts.TemplatePath)

	tmpl, err := LoadTemplate(templatePath)
	if err != nil {
		return err
	}

	vars := BuildTemplateVars(tmpl, opts.Date)
	sessionName := vars["{{session_name}}"]
	cwd := ExpandVars(tmpl.Workspace.Cwd, vars)

	fmt.Fprintf(out, "Session: %s (role: %s, agent: %s)\n", sessionName, tmpl.Role, tmpl.Identity.Agent)
	fmt.Fprintf(out, "Working directory: %s\n", cwd)

	if opts.DryRun {
		fmt.Fprintf(out, "\nInit steps:\n")
	} else {
		fmt.Fprintf(out, "\nRunning %d init steps:\n", len(tmpl.Init.Steps))
	}

	beadsActor := ExpandVars(tmpl.Identity.BeadsActorTemplate, vars)
	if err := RunInitSteps(tmpl.Init.Steps, vars, cwd, beadsActor, opts.DryRun, out); err != nil {
		return err
	}

	if opts.NoTmux {
		fmt.Fprintf(out, "\n--no-tmux: skipping tmux session creation\n")
		return nil
	}

	if err := CreateTmuxSession(tmpl.Tmux, vars, cwd, opts.DryRun, out); err != nil {
		return err
	}

	return nil
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

func writerOrStdout(out io.Writer) io.Writer {
	if out == nil {
		return os.Stdout
	}
	return out
}
