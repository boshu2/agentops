package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/embedded"
	"github.com/spf13/cobra"
)

var (
	hooksOutputFormat string
	hooksDryRun       bool
	hooksForce        bool
	hooksFull         bool
	hooksSourceDir    string
)

// HookEntry represents a single hook command (e.g., {"type": "command", "command": "..."}).
type HookEntry struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"`
}

// HookGroup represents a hook group with optional matcher and a hooks array.
// Claude Code format: {"matcher": "Write|Edit", "hooks": [{"type": "command", "command": "..."}]}
type HookGroup struct {
	Matcher string      `json:"matcher,omitempty"`
	Hooks   []HookEntry `json:"hooks"`
}

// AllEventNames returns all 12 Claude Code hook event names in canonical order.
func AllEventNames() []string {
	return []string{
		"SessionStart", "SessionEnd",
		"PreToolUse", "PostToolUse",
		"UserPromptSubmit", "TaskCompleted",
		"Stop", "PreCompact",
		"SubagentStop", "WorktreeCreate",
		"WorktreeRemove", "ConfigChange",
	}
}

// HooksConfig represents the hooks section of Claude settings.
// Supports all 12 Claude Code hook events.
type HooksConfig struct {
	SessionStart     []HookGroup `json:"SessionStart,omitempty"`
	SessionEnd       []HookGroup `json:"SessionEnd,omitempty"`
	PreToolUse       []HookGroup `json:"PreToolUse,omitempty"`
	PostToolUse      []HookGroup `json:"PostToolUse,omitempty"`
	UserPromptSubmit []HookGroup `json:"UserPromptSubmit,omitempty"`
	TaskCompleted    []HookGroup `json:"TaskCompleted,omitempty"`
	Stop             []HookGroup `json:"Stop,omitempty"`
	PreCompact       []HookGroup `json:"PreCompact,omitempty"`
	SubagentStop     []HookGroup `json:"SubagentStop,omitempty"`
	WorktreeCreate   []HookGroup `json:"WorktreeCreate,omitempty"`
	WorktreeRemove   []HookGroup `json:"WorktreeRemove,omitempty"`
	ConfigChange     []HookGroup `json:"ConfigChange,omitempty"`
}

// GetEventGroups returns the hook groups for a given event name.
func (c *HooksConfig) GetEventGroups(event string) []HookGroup {
	switch event {
	case "SessionStart":
		return c.SessionStart
	case "SessionEnd":
		return c.SessionEnd
	case "PreToolUse":
		return c.PreToolUse
	case "PostToolUse":
		return c.PostToolUse
	case "UserPromptSubmit":
		return c.UserPromptSubmit
	case "TaskCompleted":
		return c.TaskCompleted
	case "Stop":
		return c.Stop
	case "PreCompact":
		return c.PreCompact
	case "SubagentStop":
		return c.SubagentStop
	case "WorktreeCreate":
		return c.WorktreeCreate
	case "WorktreeRemove":
		return c.WorktreeRemove
	case "ConfigChange":
		return c.ConfigChange
	default:
		return nil
	}
}

// SetEventGroups sets the hook groups for a given event name.
func (c *HooksConfig) SetEventGroups(event string, groups []HookGroup) {
	switch event {
	case "SessionStart":
		c.SessionStart = groups
	case "SessionEnd":
		c.SessionEnd = groups
	case "PreToolUse":
		c.PreToolUse = groups
	case "PostToolUse":
		c.PostToolUse = groups
	case "UserPromptSubmit":
		c.UserPromptSubmit = groups
	case "TaskCompleted":
		c.TaskCompleted = groups
	case "Stop":
		c.Stop = groups
	case "PreCompact":
		c.PreCompact = groups
	case "SubagentStop":
		c.SubagentStop = groups
	case "WorktreeCreate":
		c.WorktreeCreate = groups
	case "WorktreeRemove":
		c.WorktreeRemove = groups
	case "ConfigChange":
		c.ConfigChange = groups
	}
}

// ClaudeSettings represents the Claude Code settings.json structure.
type ClaudeSettings struct {
	Hooks *HooksConfig           `json:"hooks,omitempty"`
	Other map[string]interface{} `json:"-"` // Preserve other settings
}

var hooksCmd = &cobra.Command{
	Use:   "hooks",
	Short: "Manage Claude Code hooks for automatic knowledge flywheel",
	Long: `The hooks command manages Claude Code hooks that automate the CASS knowledge flywheel.

Subcommands:
  init      Generate hooks configuration
  install   Install hooks to ~/.claude/settings.json
  show      Display current hook configuration
  test      Verify hooks work correctly

The knowledge flywheel automates:
  1. SessionStart: Inject prior knowledge with confidence decay
  2. Stop: Extract learnings and update feedback loop

Example workflow:
  ao hooks init                    # Generate configuration
  ao hooks install                 # Install to Claude Code
  ao hooks test                    # Verify everything works`,
}

var hooksInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate hooks configuration",
	Long: `Generate Claude Code hooks configuration for the CASS knowledge flywheel.

The generated hooks will:
  SessionStart:
    - Apply confidence decay to stale learnings
    - Inject CASS-weighted knowledge (up to 1500 tokens)

  Stop:
    - Extract learnings from completed session
    - Sync task completion signals
    - Update feedback loop

Output formats:
  json     JSON for manual settings.json editing
  shell    Shell commands for verification`,
	RunE: runHooksInit,
}

var hooksInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install hooks to Claude Code settings",
	Long: `Install ao hooks to ~/.claude/settings.json.

This command:
  1. Reads existing settings.json (if any)
  2. Merges ao hooks with existing configuration
  3. Creates a backup of the original settings
  4. Writes the updated configuration

Default mode installs flywheel hooks only (SessionStart + Stop).

Use --full to install all 12 events with hook scripts copied to ~/.agentops/:
  SessionStart, SessionEnd, PreToolUse, PostToolUse,
  UserPromptSubmit, TaskCompleted, Stop, PreCompact,
  SubagentStop, WorktreeCreate, WorktreeRemove, ConfigChange

Use --source-dir with --full to specify the agentops repo checkout path.
Use --force to overwrite existing ao hooks.`,
	RunE: runHooksInstall,
}

var hooksShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Display current hook configuration",
	Long:  `Display the current Claude Code hooks configuration from ~/.claude/settings.json.`,
	RunE:  runHooksShow,
}

var hooksTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Test hooks configuration",
	Long: `Test that all hook dependencies are available and working.

This command:
  1. Verifies ao is in PATH
  2. Checks that required subcommands exist
  3. Dry-runs the SessionStart hook
  4. Reports any issues`,
	RunE: runHooksTest,
}

func init() {
	rootCmd.AddCommand(hooksCmd)
	hooksCmd.AddCommand(hooksInitCmd)
	hooksCmd.AddCommand(hooksInstallCmd)
	hooksCmd.AddCommand(hooksShowCmd)
	hooksCmd.AddCommand(hooksTestCmd)

	// Init flags
	hooksInitCmd.Flags().StringVar(&hooksOutputFormat, "format", "json", "Output format: json, shell")

	// Install flags
	hooksInstallCmd.Flags().BoolVar(&hooksDryRun, "dry-run", false, "Show what would be installed without making changes")
	hooksInstallCmd.Flags().BoolVar(&hooksForce, "force", false, "Overwrite existing ao hooks")
	hooksInstallCmd.Flags().BoolVar(&hooksFull, "full", false, "Install all 12 events with hook scripts copied to ~/.agentops/")
	hooksInstallCmd.Flags().StringVar(&hooksSourceDir, "source-dir", "", "Path to agentops repo checkout (for --full script installation)")

	// Test flags
	hooksTestCmd.Flags().BoolVar(&hooksDryRun, "dry-run", false, "Show test steps without running hooks")
}

// hooksManifest wraps the hooks.json file format which has a top-level "hooks" key.
type hooksManifest struct {
	Hooks *HooksConfig `json:"hooks"`
}

// ReadHooksManifest parses a hooks.json manifest from raw bytes.
// The manifest wraps events in a top-level "hooks" key and may contain a "$schema" key.
func ReadHooksManifest(data []byte) (*HooksConfig, error) {
	var manifest hooksManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse hooks manifest: %w", err)
	}
	if manifest.Hooks == nil {
		return nil, fmt.Errorf("hooks manifest missing 'hooks' key")
	}
	return manifest.Hooks, nil
}

// findHooksManifest searches for hooks.json in known locations.
// Priority: ./hooks/hooks.json (repo checkout), ~/.agentops/hooks.json (installed),
// next to the ao binary.
func findHooksManifest() ([]byte, error) {
	paths := []string{
		"hooks/hooks.json", // repo checkout (cwd)
	}

	// Check relative to ao binary
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		// ao binary might be in cli/ or bin/, hooks.json in sibling hooks/
		paths = append(paths,
			filepath.Join(exeDir, "..", "hooks", "hooks.json"),
			filepath.Join(exeDir, "hooks", "hooks.json"),
		)
	}

	// Global install location
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".agentops", "hooks.json"))
	}

	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err == nil {
			return data, nil
		}
	}

	// Fallback: use hooks.json embedded in the binary
	if len(embedded.HooksJSON) > 0 {
		return embedded.HooksJSON, nil
	}

	return nil, fmt.Errorf("hooks.json not found in any search path or embedded data")
}

// replacePluginRoot replaces ${CLAUDE_PLUGIN_ROOT} in command strings with the given base path.
// If basePath is empty, the placeholder is removed (leaving commands that reference scripts broken
// until --full resolves them with absolute paths).
func replacePluginRoot(config *HooksConfig, basePath string) {
	for _, event := range AllEventNames() {
		groups := config.GetEventGroups(event)
		for i := range groups {
			for j := range groups[i].Hooks {
				groups[i].Hooks[j].Command = strings.ReplaceAll(
					groups[i].Hooks[j].Command,
					"${CLAUDE_PLUGIN_ROOT}",
					basePath,
				)
			}
		}
	}
}

// generateMinimalHooksConfig returns the backwards-compatible minimal config (SessionStart + Stop only).
func generateMinimalHooksConfig() *HooksConfig {
	return &HooksConfig{
		SessionStart: []HookGroup{
			{
				Hooks: []HookEntry{
					{Type: "command", Command: "ao inject --apply-decay --max-tokens 1500 2>/dev/null || true"},
				},
			},
		},
		Stop: []HookGroup{
			{
				Hooks: []HookEntry{
					{Type: "command", Command: "ao forge transcript --last-session --quiet --queue 2>/dev/null; ao task-sync --promote 2>/dev/null || true"},
				},
			},
		},
	}
}

// generateFullHooksConfig attempts to load the full hooks configuration from hooks.json
// (filesystem or embedded). Returns the config and any error encountered.
func generateFullHooksConfig() (*HooksConfig, error) {
	data, err := findHooksManifest()
	if err != nil {
		return nil, fmt.Errorf("find hooks manifest: %w", err)
	}
	config, err := ReadHooksManifest(data)
	if err != nil {
		return nil, fmt.Errorf("parse hooks manifest: %w", err)
	}
	return config, nil
}

// generateHooksConfig creates the ao hooks configuration.
// Tries to read from hooks.json for full 12-event coverage; falls back to minimal (SessionStart + Stop).
func generateHooksConfig() *HooksConfig {
	config, err := generateFullHooksConfig()
	if err != nil {
		return generateMinimalHooksConfig()
	}
	return config
}

func runHooksInit(cmd *cobra.Command, args []string) error {
	hooks := generateHooksConfig()

	switch hooksOutputFormat {
	case "json":
		wrapper := struct {
			Hooks *HooksConfig `json:"hooks"`
		}{Hooks: hooks}

		data, err := json.MarshalIndent(wrapper, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal hooks: %w", err)
		}
		fmt.Println(string(data))

	case "shell":
		fmt.Println("# SessionStart hook (knowledge injection)")
		fmt.Printf("# %s\n", hooks.SessionStart[0].Hooks[0].Command)
		fmt.Println("ao inject --apply-decay --max-tokens 1500")
		fmt.Println()
		fmt.Println("# Stop hook (learning extraction)")
		fmt.Printf("# %s\n", hooks.Stop[0].Hooks[0].Command)
		fmt.Println("ao forge transcript --last-session --quiet --queue")
		fmt.Println("ao task-sync --promote")

	default:
		return fmt.Errorf("unknown format: %s (use json or shell)", hooksOutputFormat)
	}

	return nil
}

// resolveSourceDir finds the agentops repo root for --full script installation.
func resolveSourceDir() (string, error) {
	if hooksSourceDir != "" {
		// Verify it has hooks/
		if _, err := os.Stat(filepath.Join(hooksSourceDir, "hooks", "hooks.json")); err != nil {
			return "", fmt.Errorf("--source-dir %s does not contain hooks/hooks.json", hooksSourceDir)
		}
		return hooksSourceDir, nil
	}

	// Try cwd
	if _, err := os.Stat("hooks/hooks.json"); err == nil {
		abs, _ := filepath.Abs(".")
		return abs, nil
	}

	// Try relative to ao binary
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidate := filepath.Join(exeDir, "..")
		if _, err := os.Stat(filepath.Join(candidate, "hooks", "hooks.json")); err == nil {
			abs, _ := filepath.Abs(candidate)
			return abs, nil
		}
	}

	return "", fmt.Errorf("cannot find agentops repo. Use --source-dir to specify the path, or run from the repo checkout")
}

// copyFile copies a single file, creating parent directories as needed.
func hooksCopyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

// copyDir copies all files from src to dst recursively.
func copyDir(src, dst string) (int, error) {
	count := 0
	return count, filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		count++
		return hooksCopyFile(path, target)
	})
}

// installFullHooks copies hook scripts and dependencies to ~/.agentops/ and returns
// the install base path. Source directory should be a git-managed agentops checkout.
func installFullHooks(sourceDir, installBase string) (int, error) {
	copied := 0

	// Verify source is within a git repository (integrity requirement)
	gitDir := filepath.Join(sourceDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return 0, fmt.Errorf("source directory %s is not a git root; refusing to install unverified hooks (use --source-dir to specify a valid checkout)", sourceDir)
	}

	// Copy hook scripts
	hooksDir := filepath.Join(sourceDir, "hooks")
	entries, err := os.ReadDir(hooksDir)
	if err != nil {
		return 0, fmt.Errorf("read hooks directory: %w", err)
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sh") {
			src := filepath.Join(hooksDir, e.Name())
			dst := filepath.Join(installBase, "hooks", e.Name())
			if err := hooksCopyFile(src, dst); err != nil {
				return copied, fmt.Errorf("copy %s: %w", e.Name(), err)
			}
			// Make executable
			if err := os.Chmod(dst, 0755); err != nil {
				return copied, fmt.Errorf("chmod %s: %w", e.Name(), err)
			}
			copied++
		}
	}

	// Copy lib/hook-helpers.sh
	libSrc := filepath.Join(sourceDir, "lib", "hook-helpers.sh")
	if _, err := os.Stat(libSrc); err == nil {
		if err := hooksCopyFile(libSrc, filepath.Join(installBase, "lib", "hook-helpers.sh")); err != nil {
			return copied, fmt.Errorf("copy hook-helpers.sh: %w", err)
		}
		copied++
	}

	// Copy skills/standards/references/
	stdRefSrc := filepath.Join(sourceDir, "skills", "standards", "references")
	if _, err := os.Stat(stdRefSrc); err == nil {
		n, err := copyDir(stdRefSrc, filepath.Join(installBase, "skills", "standards", "references"))
		if err != nil {
			return copied, fmt.Errorf("copy standards references: %w", err)
		}
		copied += n
	}

	// Copy skills/using-agentops/SKILL.md
	uaSrc := filepath.Join(sourceDir, "skills", "using-agentops", "SKILL.md")
	if _, err := os.Stat(uaSrc); err == nil {
		if err := hooksCopyFile(uaSrc, filepath.Join(installBase, "skills", "using-agentops", "SKILL.md")); err != nil {
			return copied, fmt.Errorf("copy using-agentops SKILL.md: %w", err)
		}
		copied++
	}

	return copied, nil
}

// installFullHooksFromEmbed extracts hook scripts and dependencies from the embedded filesystem
// to the install base directory (typically ~/.agentops/). Used when no repo checkout is available.
func installFullHooksFromEmbed(installBase string) (int, error) {
	copied := 0

	err := fs.WalkDir(embedded.HooksFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		dst := filepath.Join(installBase, path)
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return fmt.Errorf("mkdir for %s: %w", path, err)
		}

		data, err := embedded.HooksFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read embedded %s: %w", path, err)
		}

		perm := os.FileMode(0644)
		if strings.HasSuffix(path, ".sh") {
			perm = 0755
		}

		if err := os.WriteFile(dst, data, perm); err != nil {
			return fmt.Errorf("write %s: %w", dst, err)
		}
		copied++
		return nil
	})

	return copied, err
}

func loadHooksSettings(settingsPath string) (map[string]interface{}, error) {
	rawSettings := make(map[string]interface{})
	data, err := os.ReadFile(settingsPath)
	if err == nil {
		if err := json.Unmarshal(data, &rawSettings); err != nil {
			return nil, fmt.Errorf("parse existing settings: %w", err)
		}
		return rawSettings, nil
	}
	if os.IsNotExist(err) {
		return rawSettings, nil
	}
	return nil, fmt.Errorf("read settings: %w", err)
}

func installFullHookScripts(installBase string) error {
	sourceDir, err := resolveSourceDir()
	if err != nil {
		// No repo checkout available — fall back to embedded files
		if hooksDryRun {
			fmt.Printf("[dry-run] Would extract embedded files to %s\n", installBase)
			return nil
		}
		copied, embedErr := installFullHooksFromEmbed(installBase)
		if embedErr != nil {
			return fmt.Errorf("install from embedded: %w (repo resolve failed: %v)", embedErr, err)
		}
		fmt.Printf("Extracted %d embedded files to %s\n", copied, installBase)
		return nil
	}

	// Repo checkout available — copy from filesystem (dev override)
	if hooksDryRun {
		fmt.Printf("[dry-run] Would copy scripts to %s\n", installBase)
		return nil
	}
	copied, err := installFullHooks(sourceDir, installBase)
	if err != nil {
		return fmt.Errorf("install scripts: %w", err)
	}
	fmt.Printf("Copied %d files to %s\n", copied, installBase)
	return nil
}

func generateHooksForInstall(installBase string) (*HooksConfig, []string, error) {
	if !hooksFull {
		return generateMinimalHooksConfig(), []string{"SessionStart", "Stop"}, nil
	}

	config, err := generateFullHooksConfig()
	if err != nil {
		return nil, nil, fmt.Errorf("--full requires hooks.json: %w", err)
	}
	replacePluginRoot(config, installBase)
	return config, AllEventNames(), nil
}

func cloneHooksMap(rawSettings map[string]interface{}) map[string]interface{} {
	hooksMap := make(map[string]interface{})
	if existing, ok := rawSettings["hooks"].(map[string]interface{}); ok {
		for k, v := range existing {
			hooksMap[k] = v
		}
	}
	return hooksMap
}

func mergeHookEvents(hooksMap map[string]interface{}, newHooks *HooksConfig, eventsToInstall []string) int {
	installedEvents := 0
	for _, event := range eventsToInstall {
		groups := filterNonAoHookGroups(hooksMap, event)
		newGroups := newHooks.GetEventGroups(event)
		for _, g := range newGroups {
			groups = append(groups, hookGroupToMap(g))
		}
		if len(newGroups) > 0 {
			hooksMap[event] = groups
			installedEvents++
		}
	}
	return installedEvents
}

func backupHooksSettings(settingsPath string) error {
	if _, err := os.Stat(settingsPath); err != nil {
		return nil
	}
	backupPath := fmt.Sprintf("%s.backup.%s", settingsPath, time.Now().Format("20060102-150405"))
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return nil
	}
	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return fmt.Errorf("create backup: %w", err)
	}
	fmt.Printf("Backed up existing settings to %s\n", backupPath)
	return nil
}

func writeHooksSettings(settingsPath string, rawSettings map[string]interface{}) error {
	// Ensure .claude directory exists
	claudeDir := filepath.Dir(settingsPath)
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		return fmt.Errorf("create .claude directory: %w", err)
	}

	// Write new settings
	data, err := json.MarshalIndent(rawSettings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}
	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}
	return nil
}

func printHooksInstallSummary(settingsPath string, newHooks *HooksConfig, eventsToInstall []string, installedEvents int) {
	fmt.Printf("✓ Installed ao hooks to %s\n", settingsPath)
	fmt.Println()
	if hooksFull {
		fmt.Printf("Hooks installed: %d/%d events\n", installedEvents, len(AllEventNames()))
		for _, event := range eventsToInstall {
			groups := newHooks.GetEventGroups(event)
			if len(groups) > 0 {
				hookCount := 0
				for _, g := range groups {
					hookCount += len(g.Hooks)
				}
				fmt.Printf("  %s: %d hook(s)\n", event, hookCount)
			}
		}
	} else {
		fmt.Println("Hooks installed:")
		fmt.Println("  SessionStart: ao inject --apply-decay")
		fmt.Println("  Stop: ao forge + ao task-sync")
		fmt.Println()
		fmt.Println("Run 'ao hooks install --full' for complete hook coverage (all 12 events).")
	}
	fmt.Println()
	fmt.Println("Run 'ao hooks test' to verify the installation.")
}

func runHooksInstall(cmd *cobra.Command, args []string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home directory: %w", err)
	}

	settingsPath := filepath.Join(homeDir, ".claude", "settings.json")
	rawSettings, err := loadHooksSettings(settingsPath)
	if err != nil {
		return err
	}

	installBase := filepath.Join(homeDir, ".agentops")
	if hooksFull {
		if err := installFullHookScripts(installBase); err != nil {
			return err
		}
	}

	newHooks, eventsToInstall, err := generateHooksForInstall(installBase)
	if err != nil {
		return err
	}

	// Check for existing ao hooks
	if existingHooks, ok := rawSettings["hooks"].(map[string]interface{}); ok && !hooksForce {
		if hookGroupContainsAo(existingHooks, "SessionStart") {
			fmt.Println("ao hooks already installed. Use --force to overwrite.")
			return nil
		}
	}

	hooksMap := cloneHooksMap(rawSettings)
	installedEvents := mergeHookEvents(hooksMap, newHooks, eventsToInstall)
	rawSettings["hooks"] = hooksMap

	if hooksDryRun {
		fmt.Println("[dry-run] Would write to", settingsPath)
		data, _ := json.MarshalIndent(rawSettings, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	if err := backupHooksSettings(settingsPath); err != nil {
		return err
	}
	if err := writeHooksSettings(settingsPath, rawSettings); err != nil {
		return err
	}
	printHooksInstallSummary(settingsPath, newHooks, eventsToInstall, installedEvents)

	return nil
}

func runHooksShow(cmd *cobra.Command, args []string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home directory: %w", err)
	}

	settingsPath := filepath.Join(homeDir, ".claude", "settings.json")

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No Claude settings found at", settingsPath)
			fmt.Println("Run 'ao hooks install' to set up hooks.")
			return nil
		}
		return fmt.Errorf("read settings: %w", err)
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return fmt.Errorf("parse settings: %w", err)
	}

	hooks, ok := settings["hooks"]
	if !ok {
		fmt.Println("No hooks configured in", settingsPath)
		fmt.Println("Run 'ao hooks install' to set up hooks.")
		return nil
	}

	hooksMap, ok := hooks.(map[string]interface{})
	if !ok {
		fmt.Println("Invalid hooks format in", settingsPath)
		return nil
	}

	// Show coverage summary
	allEvents := AllEventNames()
	installedCount := 0
	fmt.Println("Hook Event Coverage:")
	fmt.Println()
	for _, event := range allEvents {
		groups, hasEvent := hooksMap[event].([]interface{})
		if hasEvent && len(groups) > 0 {
			hookCount := 0
			for _, g := range groups {
				if gm, ok := g.(map[string]interface{}); ok {
					if hs, ok := gm["hooks"].([]interface{}); ok {
						hookCount += len(hs)
					}
				}
			}
			fmt.Printf("  ✓ %-20s %d hook(s)\n", event, hookCount)
			installedCount++
		} else {
			fmt.Printf("  - %-20s not installed\n", event)
		}
	}

	fmt.Println()
	fmt.Printf("%d/%d events installed\n", installedCount, len(allEvents))

	if installedCount < len(allEvents) {
		fmt.Println()
		fmt.Println("Run 'ao hooks install --full' for complete coverage.")
	}

	// Check for ao hooks specifically
	if hookGroupContainsAo(hooksMap, "SessionStart") {
		fmt.Println()
		fmt.Println("✓ ao hooks are installed")
	} else {
		fmt.Println()
		fmt.Println("⚠ ao hooks not found. Run 'ao hooks install' to set up.")
	}

	return nil
}

// hookGroupContainsAo checks if any hook group in the given event contains an ao command.
func hookGroupContainsAo(hooksMap map[string]interface{}, event string) bool {
	groups, ok := hooksMap[event].([]interface{})
	if !ok {
		return false
	}
	for _, g := range groups {
		group, ok := g.(map[string]interface{})
		if !ok {
			continue
		}
		// New format: check hooks array
		if hooks, ok := group["hooks"].([]interface{}); ok {
			for _, h := range hooks {
				if hook, ok := h.(map[string]interface{}); ok {
					if cmd, ok := hook["command"].(string); ok && isAoManagedHookCommand(cmd) {
						return true
					}
				}
			}
		}
		// Legacy format: check top-level command array
		if cmd, ok := group["command"].([]interface{}); ok && len(cmd) > 1 {
			if cmdStr, ok := cmd[1].(string); ok && isAoManagedHookCommand(cmdStr) {
				return true
			}
		}
	}
	return false
}

// filterNonAoHookGroups returns hook groups that don't contain ao commands.
func filterNonAoHookGroups(hooksMap map[string]interface{}, event string) []map[string]interface{} {
	result := make([]map[string]interface{}, 0)
	groups, ok := hooksMap[event].([]interface{})
	if !ok {
		return result
	}
	for _, g := range groups {
		group, ok := g.(map[string]interface{})
		if !ok {
			continue
		}
		isAo := false
		// Check new format
		if hooks, ok := group["hooks"].([]interface{}); ok {
			for _, h := range hooks {
				if hook, ok := h.(map[string]interface{}); ok {
					if cmd, ok := hook["command"].(string); ok && isAoManagedHookCommand(cmd) {
						isAo = true
						break
					}
				}
			}
		}
		// Check legacy format
		if cmd, ok := group["command"].([]interface{}); ok && len(cmd) > 1 {
			if cmdStr, ok := cmd[1].(string); ok && isAoManagedHookCommand(cmdStr) {
				isAo = true
			}
		}
		if !isAo {
			result = append(result, group)
		}
	}
	return result
}

func isAoManagedHookCommand(cmd string) bool {
	if strings.Contains(cmd, "ao ") {
		return true
	}

	// Installed scripts live under ~/.agentops/hooks/*.sh and should be treated as ao-managed.
	normalized := filepath.ToSlash(cmd)
	return strings.Contains(normalized, "/.agentops/hooks/")
}

// hookGroupToMap converts a HookGroup to a map for JSON serialization.
func hookGroupToMap(g HookGroup) map[string]interface{} {
	hooks := make([]map[string]interface{}, len(g.Hooks))
	for i, h := range g.Hooks {
		entry := map[string]interface{}{
			"type":    h.Type,
			"command": h.Command,
		}
		if h.Timeout > 0 {
			entry["timeout"] = h.Timeout
		}
		hooks[i] = entry
	}
	result := map[string]interface{}{
		"hooks": hooks,
	}
	if g.Matcher != "" {
		result["matcher"] = g.Matcher
	}
	return result
}

func runAoPathTest(testNum int, allPassed *bool) {
	fmt.Printf("%d. Checking ao is in PATH... ", testNum)
	aoPath, err := exec.LookPath("ao")
	if err != nil {
		fmt.Println("✗ FAILED")
		fmt.Printf("   ao not found in PATH. Ensure ao is installed and in your PATH.\n")
		*allPassed = false
		return
	}
	fmt.Printf("✓ found at %s\n", aoPath)
}

func runRequiredSubcommandsTest(testNum int, allPassed *bool) {
	subcommands := []string{"inject", "forge", "task-sync", "feedback-loop"}
	fmt.Printf("%d. Checking required subcommands... ", testNum)
	missingCmds := []string{}
	for _, subcmd := range subcommands {
		testCmd := exec.Command("ao", subcmd, "--help")
		if err := testCmd.Run(); err != nil {
			missingCmds = append(missingCmds, subcmd)
		}
	}
	if len(missingCmds) > 0 {
		fmt.Println("✗ FAILED")
		fmt.Printf("   Missing subcommands: %s\n", strings.Join(missingCmds, ", "))
		*allPassed = false
		return
	}
	fmt.Println("✓ all present")
}

func runSettingsCoverageTest(testNum int, homeDir string, allPassed *bool) {
	fmt.Printf("%d. Checking Claude settings... ", testNum)
	settingsPath := filepath.Join(homeDir, ".claude", "settings.json")
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		fmt.Println("⚠ settings.json not found")
		fmt.Println("   Run 'ao hooks install' to create hooks configuration.")
		return
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		fmt.Println("✗ FAILED to read")
		*allPassed = false
		return
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		fmt.Println("✗ FAILED to parse")
		*allPassed = false
		return
	}

	hooksRaw, ok := settings["hooks"]
	if !ok {
		fmt.Println("⚠ no hooks configured")
		fmt.Println("   Run 'ao hooks install' to set up hooks.")
		return
	}

	hooksMap, ok := hooksRaw.(map[string]interface{})
	if !ok {
		return
	}

	installed := 0
	for _, event := range AllEventNames() {
		if groups, ok := hooksMap[event].([]interface{}); ok && len(groups) > 0 {
			installed++
		}
	}
	fmt.Printf("✓ %d/%d events installed\n", installed, len(AllEventNames()))
	if installed < len(AllEventNames()) {
		fmt.Println("   Run 'ao hooks install --full' for complete coverage.")
	}
}

func runHookScriptsAccessTest(testNum int, homeDir string) {
	fmt.Printf("%d. Checking hook scripts... ", testNum)
	agentopsDir := filepath.Join(homeDir, ".agentops", "hooks")
	if _, err := os.Stat(agentopsDir); err != nil {
		fmt.Println("- not installed (use --full)")
		return
	}

	// Scripts were installed via --full
	entries, _ := os.ReadDir(agentopsDir)
	scriptCount := 0
	missingExec := []string{}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sh") {
			scriptCount++
			info, err := e.Info()
			if err == nil && info.Mode()&0111 == 0 {
				missingExec = append(missingExec, e.Name())
			}
		}
	}
	if len(missingExec) > 0 {
		fmt.Printf("⚠ %d scripts, %d not executable\n", scriptCount, len(missingExec))
		return
	}
	fmt.Printf("✓ %d scripts installed\n", scriptCount)
}

func runInjectCommandTest(testNum int, allPassed *bool) {
	fmt.Printf("%d. Testing inject command... ", testNum)
	if hooksDryRun {
		fmt.Println("⏭ skipped (--dry-run)")
		return
	}

	testCmd := exec.Command("ao", "inject", "--max-tokens", "100", "--no-cite")
	output, err := testCmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(output), "No prior knowledge") || len(output) > 0 {
			fmt.Println("✓ working")
			return
		}
		fmt.Println("✗ FAILED")
		fmt.Printf("   Error: %v\n", err)
		*allPassed = false
		return
	}
	fmt.Println("✓ working")
}

func runForgeTranscriptAccessTest(testNum int, homeDir string) {
	fmt.Printf("%d. Testing forge transcript access... ", testNum)
	if hooksDryRun {
		fmt.Println("⏭ skipped (--dry-run)")
		return
	}

	projectsDir := filepath.Join(homeDir, ".claude", "projects")
	if _, err := os.Stat(projectsDir); os.IsNotExist(err) {
		fmt.Println("⚠ no Claude projects found")
		fmt.Println("   This is OK for first-time setup.")
		return
	}
	fmt.Println("✓ projects directory found")
}

func runHooksTest(cmd *cobra.Command, args []string) error {
	fmt.Println("Testing ao hooks configuration...")
	fmt.Println()

	allPassed := true
	testNum := 0

	testNum++
	runAoPathTest(testNum, &allPassed)

	testNum++
	runRequiredSubcommandsTest(testNum, &allPassed)

	homeDir, _ := os.UserHomeDir()

	testNum++
	runSettingsCoverageTest(testNum, homeDir, &allPassed)

	testNum++
	runHookScriptsAccessTest(testNum, homeDir)

	testNum++
	runInjectCommandTest(testNum, &allPassed)

	testNum++
	runForgeTranscriptAccessTest(testNum, homeDir)

	fmt.Println()
	if allPassed {
		fmt.Println("✓ All tests passed! Hooks are ready to use.")
	} else {
		fmt.Println("⚠ Some tests failed. Please fix the issues above.")
	}

	return nil
}
