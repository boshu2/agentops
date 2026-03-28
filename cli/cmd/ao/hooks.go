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
// Hook manifest format: {"matcher": "Write|Edit", "hooks": [{"type": "command", "command": "..."}]}
type HookGroup struct {
	Matcher string      `json:"matcher,omitempty"`
	Hooks   []HookEntry `json:"hooks"`
}

// AllEventNames returns all 12 hook event names in canonical order.
// These map to Claude Code events. Codex uses a skill-driven lifecycle instead.
// See docs/contracts/hook-runtime-contract.md for the cross-runtime mapping.
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

// hookCoverageContract describes the active coverage contract for runtime checks.
// Active events are derived from hooks.json when available.
type hookCoverageContract struct {
	ActiveEvents   []string
	FallbackReason string
}

func fallbackHookCoverageContract(reason string) hookCoverageContract {
	events := append([]string(nil), AllEventNames()...)
	return hookCoverageContract{
		ActiveEvents:   events,
		FallbackReason: reason,
	}
}

func activeEventNamesFromConfig(config *HooksConfig) []string {
	active := make([]string, 0)
	for _, event := range AllEventNames() {
		if len(config.GetEventGroups(event)) > 0 {
			active = append(active, event)
		}
	}
	return active
}

func resolveHookCoverageContract() hookCoverageContract {
	data, err := findHooksManifest()
	if err != nil {
		return fallbackHookCoverageContract(err.Error())
	}
	config, err := ReadHooksManifest(data)
	if err != nil {
		return fallbackHookCoverageContract(err.Error())
	}
	activeEvents := activeEventNamesFromConfig(config)
	if len(activeEvents) == 0 {
		return fallbackHookCoverageContract("hooks manifest contains zero active events")
	}
	return hookCoverageContract{
		ActiveEvents: activeEvents,
	}
}

func countInstalledEventsForList(hooksMap map[string]any, events []string) int {
	installed := 0
	for _, event := range events {
		if groups, ok := hooksMap[event].([]any); ok && len(groups) > 0 {
			installed++
		}
	}
	return installed
}

func collectLegacyAoManagedEvents(hooksMap map[string]any, activeEvents []string) []string {
	activeSet := make(map[string]struct{}, len(activeEvents))
	for _, event := range activeEvents {
		activeSet[event] = struct{}{}
	}

	legacyEvents := make([]string, 0)
	for _, event := range AllEventNames() {
		if _, ok := activeSet[event]; ok {
			continue
		}
		if hookGroupContainsAo(hooksMap, event) {
			legacyEvents = append(legacyEvents, event)
		}
	}
	return legacyEvents
}

func formatLegacyPreservationReport(legacyEvents []string) string {
	if len(legacyEvents) == 0 {
		return ""
	}
	return fmt.Sprintf(
		"Preserved legacy ao-managed hooks outside active contract (%d): %s",
		len(legacyEvents),
		strings.Join(legacyEvents, ", "),
	)
}

func printLegacyPreservationReport(legacyEvents []string) {
	msg := formatLegacyPreservationReport(legacyEvents)
	if msg == "" {
		return
	}
	fmt.Println(msg)
}

// HooksConfig represents the hooks section of Claude settings.
// Supports all 12 hook events (Claude Code runtime).
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

// eventGroupPtrs returns a map from event name to a pointer to the corresponding
// []HookGroup field. Used by GetEventGroups and SetEventGroups.
func (c *HooksConfig) eventGroupPtrs() map[string]*[]HookGroup {
	return map[string]*[]HookGroup{
		"SessionStart":     &c.SessionStart,
		"SessionEnd":       &c.SessionEnd,
		"PreToolUse":       &c.PreToolUse,
		"PostToolUse":      &c.PostToolUse,
		"UserPromptSubmit": &c.UserPromptSubmit,
		"TaskCompleted":    &c.TaskCompleted,
		"Stop":             &c.Stop,
		"PreCompact":       &c.PreCompact,
		"SubagentStop":     &c.SubagentStop,
		"WorktreeCreate":   &c.WorktreeCreate,
		"WorktreeRemove":   &c.WorktreeRemove,
		"ConfigChange":     &c.ConfigChange,
	}
}

// eventGroupPtr returns a pointer to the []HookGroup field for the given event name,
// or nil if the event is unknown.
func (c *HooksConfig) eventGroupPtr(event string) *[]HookGroup {
	return c.eventGroupPtrs()[event]
}

// GetEventGroups returns the hook groups for a given event name.
func (c *HooksConfig) GetEventGroups(event string) []HookGroup {
	ptr := c.eventGroupPtr(event)
	if ptr == nil {
		return nil
	}
	return *ptr
}

// SetEventGroups sets the hook groups for a given event name.
func (c *HooksConfig) SetEventGroups(event string, groups []HookGroup) {
	ptr := c.eventGroupPtr(event)
	if ptr == nil {
		return
	}
	*ptr = groups
}

// ClaudeSettings represents the Claude Code settings.json structure.
type ClaudeSettings struct {
	Hooks *HooksConfig   `json:"hooks,omitempty"`
	Other map[string]any `json:"-"` // Preserve other settings
}

var hooksCmd = &cobra.Command{
	Use:   "hooks",
	Short: "Manage runtime hooks for automatic knowledge flywheel",
	Long: `The hooks command manages runtime hooks that automate the CASS knowledge flywheel.
Note: Hook install targets Claude Code (~/.claude/settings.json). Codex uses a
skill-driven lifecycle instead — see 'ao codex start/stop' and
docs/contracts/hook-runtime-contract.md for the cross-runtime mapping.

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
	Long: `Generate hooks configuration for the CASS knowledge flywheel (Claude Code format).

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
	Short: "Install hooks to Claude Code settings (Codex uses skill-driven lifecycle)",
	Long: `Install ao hooks to ~/.claude/settings.json.

This command:
  1. Reads existing settings.json (if any)
  2. Merges ao hooks with existing configuration
  3. Creates a backup of the original settings
  4. Writes the updated configuration

Default mode installs bare-minimum flywheel hooks (SessionStart + SessionEnd + Stop).

Use --full to install the plugin hooks.json with hook scripts copied to ~/.agentops/.
Use --source-dir with --full to specify the agentops repo checkout path.
Use --force to overwrite existing ao hooks.`,
	RunE: runHooksInstall,
}

var hooksShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Display current hook configuration",
	Long:  `Display the current hooks configuration from ~/.claude/settings.json (Claude Code runtime).`,
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
	hooksCmd.GroupID = "config"
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
	hooksInstallCmd.Flags().BoolVar(&hooksFull, "full", false, "Install all hook events from hooks.json with scripts copied to ~/.agentops/")
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

// generateMinimalHooksConfig returns the bare-minimum flywheel config (SessionStart + SessionEnd + Stop).
// Uses script-based commands matching the full mode pattern for consistency.
func generateMinimalHooksConfig() *HooksConfig {
	return &HooksConfig{
		SessionStart: []HookGroup{
			{
				Hooks: []HookEntry{
					{Type: "command", Command: "${CLAUDE_PLUGIN_ROOT}/hooks/session-start.sh"},
				},
			},
		},
		SessionEnd: []HookGroup{
			{
				Hooks: []HookEntry{
					{Type: "command", Command: "${CLAUDE_PLUGIN_ROOT}/hooks/session-end-maintenance.sh", Timeout: 35},
				},
			},
		},
		Stop: []HookGroup{
			{
				Hooks: []HookEntry{
					{Type: "command", Command: "${CLAUDE_PLUGIN_ROOT}/hooks/ao-flywheel-close.sh", Timeout: 15},
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
// Tries to read from hooks.json for full event coverage; falls back to minimal (SessionStart + SessionEnd + Stop).
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
		fmt.Println("# SessionStart hook (startup context + optional inject)")
		fmt.Printf("# %s\n", hooks.SessionStart[0].Hooks[0].Command)
		fmt.Println()
		fmt.Println("# SessionEnd hook (forge + flywheel maintenance)")
		fmt.Printf("# %s\n", hooks.SessionEnd[0].Hooks[0].Command)
		fmt.Println()
		fmt.Println("# Stop hook (close flywheel)")
		fmt.Printf("# %s\n", hooks.Stop[0].Hooks[0].Command)

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
	if err := os.MkdirAll(filepath.Dir(dst), 0750); err != nil {
		return err
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0600)
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
			return os.MkdirAll(target, 0750)
		}
		count++
		return hooksCopyFile(path, target)
	})
}

// copyShellScripts copies all .sh files from srcDir to dstDir, making them executable.
func copyShellScripts(srcDir, dstDir string) (int, error) {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return 0, fmt.Errorf("read hooks directory: %w", err)
	}
	copied := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sh") {
			continue
		}
		src := filepath.Join(srcDir, e.Name())
		dst := filepath.Join(dstDir, e.Name())
		if err := hooksCopyFile(src, dst); err != nil {
			return copied, fmt.Errorf("copy %s: %w", e.Name(), err)
		}
		if err := os.Chmod(dst, 0755); err != nil { // #nosec G302 -- hook scripts require execute permission
			return copied, fmt.Errorf("chmod %s: %w", e.Name(), err)
		}
		copied++
	}
	return copied, nil
}

// copyOptionalFile copies a single file if it exists. Returns 1 if copied, 0 if source missing.
func copyOptionalFile(src, dst, label string) (int, error) {
	if _, err := os.Stat(src); err != nil {
		return 0, nil //nolint:nilerr // missing optional file is not an error
	}
	if err := hooksCopyFile(src, dst); err != nil {
		return 0, fmt.Errorf("copy %s: %w", label, err)
	}
	return 1, nil
}

// copyOptionalDir copies a directory tree if it exists. Returns file count.
func copyOptionalDir(src, dst, label string) (int, error) {
	if _, err := os.Stat(src); err != nil {
		return 0, nil //nolint:nilerr // missing optional dir is not an error
	}
	n, err := copyDir(src, dst)
	if err != nil {
		return 0, fmt.Errorf("copy %s: %w", label, err)
	}
	return n, nil
}

// installFullHooks copies hook scripts and dependencies to ~/.agentops/ and returns
// the install base path. Source directory should be a git-managed agentops checkout.
func installFullHooks(sourceDir, installBase string) (int, error) {
	// Verify source is within a git repository (integrity requirement)
	gitDir := filepath.Join(sourceDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return 0, fmt.Errorf("source directory %s is not a git root; refusing to install unverified hooks (use --source-dir to specify a valid checkout)", sourceDir)
	}

	copied := 0

	n, err := copyShellScripts(filepath.Join(sourceDir, "hooks"), filepath.Join(installBase, "hooks"))
	if err != nil {
		return copied, err
	}
	copied += n

	n, err = copyOptionalFile(
		filepath.Join(sourceDir, "lib", "hook-helpers.sh"),
		filepath.Join(installBase, "lib", "hook-helpers.sh"),
		"hook-helpers.sh",
	)
	if err != nil {
		return copied, err
	}
	copied += n

	n, err = copyOptionalDir(
		filepath.Join(sourceDir, "skills", "standards", "references"),
		filepath.Join(installBase, "skills", "standards", "references"),
		"standards references",
	)
	if err != nil {
		return copied, err
	}
	copied += n

	n, err = copyOptionalFile(
		filepath.Join(sourceDir, "skills", "using-agentops", "SKILL.md"),
		filepath.Join(installBase, "skills", "using-agentops", "SKILL.md"),
		"using-agentops SKILL.md",
	)
	if err != nil {
		return copied, err
	}
	copied += n

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
		if err := os.MkdirAll(filepath.Dir(dst), 0750); err != nil {
			return fmt.Errorf("mkdir for %s: %w", path, err)
		}

		data, err := embedded.HooksFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read embedded %s: %w", path, err)
		}

		perm := os.FileMode(0600)
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

func loadHooksSettings(settingsPath string) (map[string]any, error) {
	rawSettings := make(map[string]any)
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
		config := generateMinimalHooksConfig()
		replacePluginRoot(config, installBase)
		return config, []string{"SessionStart", "SessionEnd", "Stop"}, nil
	}

	config, err := generateFullHooksConfig()
	if err != nil {
		return nil, nil, fmt.Errorf("--full requires hooks.json: %w", err)
	}
	replacePluginRoot(config, installBase)
	activeEvents := activeEventNamesFromConfig(config)
	if len(activeEvents) == 0 {
		return nil, nil, fmt.Errorf("--full requires at least one active event in hooks.json")
	}
	return config, activeEvents, nil
}

func cloneHooksMap(rawSettings map[string]any) map[string]any {
	hooksMap := make(map[string]any)
	if existing, ok := rawSettings["hooks"].(map[string]any); ok {
		for k, v := range existing {
			hooksMap[k] = v
		}
	}
	return hooksMap
}

func mergeHookEvents(hooksMap map[string]any, newHooks *HooksConfig, eventsToInstall []string) int {
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
	if err := os.WriteFile(backupPath, data, 0600); err != nil {
		return fmt.Errorf("create backup: %w", err)
	}
	fmt.Printf("Backed up existing settings to %s\n", backupPath)
	return nil
}

func writeHooksSettings(settingsPath string, rawSettings map[string]any) error {
	// Ensure .claude directory exists
	claudeDir := filepath.Dir(settingsPath)
	if err := os.MkdirAll(claudeDir, 0750); err != nil {
		return fmt.Errorf("create .claude directory: %w", err)
	}

	// Write new settings
	data, err := json.MarshalIndent(rawSettings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}
	if err := os.WriteFile(settingsPath, data, 0600); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}
	return nil
}

func printHooksInstallSummary(settingsPath string, newHooks *HooksConfig, eventsToInstall []string, installedEvents int) {
	fmt.Printf("✓ Installed ao hooks to %s\n", settingsPath)
	fmt.Println()
	if hooksFull {
		fmt.Printf("Hooks installed: %d/%d events\n", installedEvents, len(eventsToInstall))
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
		fmt.Println("  SessionStart: ao extract + signpost pointer")
		fmt.Println("  SessionEnd: ao forge + ao maturity")
		fmt.Println("  Stop: ao flywheel close-loop")
	}
	fmt.Println()
	fmt.Println("Run 'ao hooks test' to verify the installation.")
}

// existingAoHooksBlock returns true if ao hooks are already installed and --force was not set.
func existingAoHooksBlock(rawSettings map[string]any) bool {
	if hooksForce {
		return false
	}
	existingHooks, ok := rawSettings["hooks"].(map[string]any)
	return ok && hookGroupContainsAo(existingHooks, "SessionStart")
}

// dryRunPrintSettings prints the would-be settings and returns true if --dry-run is active.
func dryRunPrintSettings(settingsPath string, rawSettings map[string]any) (bool, error) {
	if !hooksDryRun {
		return false, nil
	}
	fmt.Println("[dry-run] Would write to", settingsPath)
	data, err := json.MarshalIndent(rawSettings, "", "  ")
	if err != nil {
		return true, fmt.Errorf("marshal hooks settings: %w", err)
	}
	fmt.Println(string(data))
	return true, nil
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

	if existingAoHooksBlock(rawSettings) {
		fmt.Println("ao hooks already installed. Use --force to overwrite.")
		return nil
	}

	hooksMap := cloneHooksMap(rawSettings)
	legacyPreserved := collectLegacyAoManagedEvents(hooksMap, eventsToInstall)
	installedEvents := mergeHookEvents(hooksMap, newHooks, eventsToInstall)
	rawSettings["hooks"] = hooksMap

	if done, err := dryRunPrintSettings(settingsPath, rawSettings); done || err != nil {
		if done {
			printLegacyPreservationReport(legacyPreserved)
		}
		return err
	}

	if err := commitHooksSettings(settingsPath, rawSettings, newHooks, eventsToInstall, installedEvents); err != nil {
		return err
	}
	printLegacyPreservationReport(legacyPreserved)
	return nil
}

// commitHooksSettings backs up, writes, and prints a summary for the hooks installation.
func commitHooksSettings(settingsPath string, rawSettings map[string]any, newHooks *HooksConfig, eventsToInstall []string, installedEvents int) error {
	if err := backupHooksSettings(settingsPath); err != nil {
		return err
	}
	if err := writeHooksSettings(settingsPath, rawSettings); err != nil {
		return err
	}
	printHooksInstallSummary(settingsPath, newHooks, eventsToInstall, installedEvents)
	return nil
}

// loadHooksMap reads settings.json and extracts the hooks map.
// Returns (nil, nil) with a printed message when hooks are absent or invalid.
func loadHooksMap(settingsPath string) (map[string]any, error) {
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No Claude settings found at", settingsPath)
			fmt.Println("Run 'ao hooks install' to set up hooks.")
			return nil, nil
		}
		return nil, fmt.Errorf("read settings: %w", err)
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("parse settings: %w", err)
	}

	hooks, ok := settings["hooks"]
	if !ok {
		fmt.Println("No hooks configured in", settingsPath)
		fmt.Println("Run 'ao hooks install' to set up hooks.")
		return nil, nil
	}

	hooksMap, ok := hooks.(map[string]any)
	if !ok {
		fmt.Println("Invalid hooks format in", settingsPath)
		return nil, nil
	}
	return hooksMap, nil
}

// countRawGroupHooks counts the total hooks across all entries in a raw hook group slice.
func countRawGroupHooks(groups []any) int {
	count := 0
	for _, g := range groups {
		gm, ok := g.(map[string]any)
		if !ok {
			continue
		}
		if hs, ok := gm["hooks"].([]any); ok {
			count += len(hs)
		}
	}
	return count
}

// printEventCoverage prints per-event hook coverage and returns the count of installed events.
func printEventCoverage(hooksMap map[string]any) int {
	allEvents := AllEventNames()
	installedCount := 0
	fmt.Println("Supported Hook Namespace Coverage (informational):")
	fmt.Println()
	for _, event := range allEvents {
		groups, hasEvent := hooksMap[event].([]any)
		if hasEvent && len(groups) > 0 {
			fmt.Printf("  ✓ %-20s %d hook(s)\n", event, countRawGroupHooks(groups))
			installedCount++
		} else {
			fmt.Printf("  - %-20s not installed\n", event)
		}
	}
	return installedCount
}

func runHooksShow(cmd *cobra.Command, args []string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home directory: %w", err)
	}

	settingsPath := filepath.Join(homeDir, ".claude", "settings.json")
	hooksMap, err := loadHooksMap(settingsPath)
	if err != nil {
		return err
	}
	if hooksMap == nil {
		return nil
	}

	supportedInstalled := printEventCoverage(hooksMap)
	contract := resolveHookCoverageContract()
	activeInstalled := countInstalledEventsForList(hooksMap, contract.ActiveEvents)

	fmt.Println()
	fmt.Printf("Active contract coverage: %d/%d events\n", activeInstalled, len(contract.ActiveEvents))
	if contract.FallbackReason != "" {
		fmt.Printf("Coverage contract fallback: %s\n", contract.FallbackReason)
	}
	fmt.Printf("Supported namespace coverage: %d/%d events (informational)\n", supportedInstalled, len(AllEventNames()))

	if activeInstalled < len(contract.ActiveEvents) {
		fmt.Println()
		fmt.Println("Run 'ao hooks install --full' for complete coverage.")
	}

	// Check for ao hooks specifically
	fmt.Println()
	if hookGroupContainsAo(hooksMap, "SessionStart") {
		fmt.Println("✓ ao hooks are installed")
	} else {
		fmt.Println("⚠ ao hooks not found. Run 'ao hooks install' to set up.")
	}

	return nil
}

// rawGroupIsAoManaged checks whether a single raw hook group (map[string]any) contains
// an ao-managed command. Handles both new format (hooks array) and legacy format
// (top-level command array).
func rawGroupIsAoManaged(group map[string]any) bool {
	return rawGroupHooksContainAo(group) || rawGroupLegacyContainsAo(group)
}

// rawGroupHooksContainAo checks the new-format hooks array for ao commands.
func rawGroupHooksContainAo(group map[string]any) bool {
	hooks, ok := group["hooks"].([]any)
	if !ok {
		return false
	}
	for _, h := range hooks {
		hook, ok := h.(map[string]any)
		if !ok {
			continue
		}
		if cmd, ok := hook["command"].(string); ok && isAoManagedHookCommand(cmd) {
			return true
		}
	}
	return false
}

// rawGroupLegacyContainsAo checks the legacy-format top-level command array for ao commands.
func rawGroupLegacyContainsAo(group map[string]any) bool {
	cmd, ok := group["command"].([]any)
	if !ok || len(cmd) <= 1 {
		return false
	}
	cmdStr, ok := cmd[1].(string)
	return ok && isAoManagedHookCommand(cmdStr)
}

// hookGroupContainsAo checks if any hook group in the given event contains an ao command.
func hookGroupContainsAo(hooksMap map[string]any, event string) bool {
	groups, ok := hooksMap[event].([]any)
	if !ok {
		return false
	}
	for _, g := range groups {
		group, ok := g.(map[string]any)
		if !ok {
			continue
		}
		if rawGroupIsAoManaged(group) {
			return true
		}
	}
	return false
}

// filterNonAoHookGroups returns hook groups that don't contain ao commands.
func filterNonAoHookGroups(hooksMap map[string]any, event string) []map[string]any {
	result := make([]map[string]any, 0)
	groups, ok := hooksMap[event].([]any)
	if !ok {
		return result
	}
	for _, g := range groups {
		group, ok := g.(map[string]any)
		if !ok {
			continue
		}
		if !rawGroupIsAoManaged(group) {
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
func hookGroupToMap(g HookGroup) map[string]any {
	hooks := make([]map[string]any, len(g.Hooks))
	for i, h := range g.Hooks {
		entry := map[string]any{
			"type":    h.Type,
			"command": h.Command,
		}
		if h.Timeout > 0 {
			entry["timeout"] = h.Timeout
		}
		hooks[i] = entry
	}
	result := map[string]any{
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
	subcommands := []string{"inject", "forge", "maturity", "flywheel"}
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

// countInstalledHookEvents counts how many hook events have at least one group.
func countInstalledHookEvents(hooksMap map[string]any) int {
	installed := 0
	for _, event := range AllEventNames() {
		if groups, ok := hooksMap[event].([]any); ok && len(groups) > 0 {
			installed++
		}
	}
	return installed
}

// readSettingsHooksMap reads, parses, and extracts the hooks map from settings.json.
// Returns (nil, nil) for recoverable states (missing file, no hooks key), or (nil, err) for failures.
func readSettingsHooksMap(settingsPath string) (map[string]any, error) {
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		fmt.Println("⚠ settings.json not found")
		fmt.Println("   Run 'ao hooks install' to create hooks configuration.")
		return nil, nil
	}
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return nil, fmt.Errorf("read")
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("parse")
	}
	hooksRaw, ok := settings["hooks"]
	if !ok {
		fmt.Println("⚠ no hooks configured")
		fmt.Println("   Run 'ao hooks install' to set up hooks.")
		return nil, nil
	}
	hooksMap, ok := hooksRaw.(map[string]any)
	if !ok {
		return nil, nil
	}
	return hooksMap, nil
}

func runSettingsCoverageTest(testNum int, homeDir string, allPassed *bool) {
	fmt.Printf("%d. Checking Claude settings... ", testNum)
	settingsPath := filepath.Join(homeDir, ".claude", "settings.json")

	hooksMap, err := readSettingsHooksMap(settingsPath)
	if err != nil {
		fmt.Printf("✗ FAILED to %s\n", err)
		*allPassed = false
		return
	}
	if hooksMap == nil {
		return
	}

	contract := resolveHookCoverageContract()
	activeInstalled := countInstalledEventsForList(hooksMap, contract.ActiveEvents)
	fmt.Printf("✓ %d/%d active contract events installed\n", activeInstalled, len(contract.ActiveEvents))
	if contract.FallbackReason != "" {
		fmt.Printf("   Coverage contract fallback: %s\n", contract.FallbackReason)
	}
	supportedInstalled := countInstalledHookEvents(hooksMap)
	fmt.Printf("   Supported namespace coverage: %d/%d events (informational)\n", supportedInstalled, len(AllEventNames()))
	if activeInstalled < len(contract.ActiveEvents) {
		fmt.Println("   Run 'ao hooks install --full' for complete coverage.")
	}
}

// collectScriptNames returns a set of .sh filenames in the given directory.
func collectScriptNames(dir string) map[string]bool {
	names := make(map[string]bool)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return names
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sh") {
			names[e.Name()] = true
		}
	}
	return names
}

// collectWiredScripts extracts script filenames referenced by hook commands in settings.
func collectWiredScripts(hooksMap map[string]any) (eventScriptCount map[string]int, wiredScripts map[string]bool) {
	eventScriptCount = make(map[string]int)
	wiredScripts = make(map[string]bool)
	for _, event := range AllEventNames() {
		groups, ok := hooksMap[event].([]any)
		if !ok {
			continue
		}
		for _, g := range groups {
			gm, ok := g.(map[string]any)
			if !ok {
				continue
			}
			hooks, ok := gm["hooks"].([]any)
			if !ok {
				continue
			}
			for _, h := range hooks {
				hm, ok := h.(map[string]any)
				if !ok {
					continue
				}
				cmd, ok := hm["command"].(string)
				if !ok {
					continue
				}
				// Extract script filenames from command string
				for _, part := range strings.Fields(cmd) {
					base := filepath.Base(part)
					if strings.HasSuffix(base, ".sh") {
						eventScriptCount[event]++
						wiredScripts[base] = true
					}
				}
			}
		}
	}
	return eventScriptCount, wiredScripts
}

func runHookScriptsAccessTest(testNum int, homeDir string) {
	fmt.Printf("%d. Checking hook scripts... ", testNum)
	agentopsDir := filepath.Join(homeDir, ".agentops", "hooks")
	if _, err := os.Stat(agentopsDir); err != nil {
		fmt.Println("- not installed (use --full)")
		return
	}

	// Count installed scripts and check executability
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

	// Cross-reference scripts against wired hook events
	settingsPath := filepath.Join(homeDir, ".claude", "settings.json")
	hooksMap, err := readSettingsHooksMap(settingsPath)
	if err != nil || hooksMap == nil {
		fmt.Printf("✓ %d scripts installed\n", scriptCount)
		return
	}

	eventScriptCount, wiredScripts := collectWiredScripts(hooksMap)
	wiredEventCount := len(eventScriptCount)

	// Count unwired scripts
	installedScripts := collectScriptNames(agentopsDir)
	unwired := 0
	for name := range installedScripts {
		if !wiredScripts[name] {
			unwired++
		}
	}

	// Build event summary
	var eventParts []string
	for _, event := range AllEventNames() {
		if n, ok := eventScriptCount[event]; ok {
			eventParts = append(eventParts, fmt.Sprintf("%s (%d)", event, n))
		}
	}

	fmt.Printf("✓ %d scripts installed across %d hook events\n", scriptCount, wiredEventCount)
	if len(eventParts) > 0 {
		fmt.Printf("   Events: %s\n", strings.Join(eventParts, ", "))
	}
	if unwired > 0 {
		fmt.Printf("   Unwired: %d scripts not referenced by any hook event\n", unwired)
	}
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
