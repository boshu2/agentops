package quality

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// FileExists reports whether a path exists on disk.
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

const (
	CodexAgentOpsPluginName      = "agentops"
	CodexAgentOpsMarketplaceName = "agentops-marketplace"
)

// CodexInstallMeta describes the installed Codex plugin metadata.
type CodexInstallMeta struct {
	InstallMode  string `json:"install_mode"`
	PluginRoot   string `json:"plugin_root"`
	Version      string `json:"version"`
	ManifestHash string `json:"manifest_hash"`
	SkillCount   int    `json:"skill_count"`
}

func CodexNativePluginSkillsPath(home string) string {
	return filepath.Join(
		home,
		".codex",
		"plugins",
		"cache",
		CodexAgentOpsMarketplaceName,
		CodexAgentOpsPluginName,
		"local",
		"skills-codex",
	)
}

func CodexNativePluginHealPath(home string) string {
	return filepath.Join(CodexNativePluginSkillsPath(home), "heal-skill", "scripts", "heal.sh")
}

func CodexNativePluginManifestPath(home string) string {
	return filepath.Join(CodexNativePluginSkillsPath(home), ".agentops-manifest.json")
}

func CodexInstallMetaPath(home string) string {
	return filepath.Join(home, ".codex", ".agentops-codex-install.json")
}

func ReadCodexInstallMeta(home string) (*CodexInstallMeta, error) {
	data, err := os.ReadFile(CodexInstallMetaPath(home))
	if err != nil {
		return nil, err
	}
	var meta CodexInstallMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

func ReadCodexManifestSkillCount(path string) (int, error) {
	var manifest struct {
		Skills []struct {
			Name string `json:"name"`
		} `json:"skills"`
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return 0, err
	}
	return len(manifest.Skills), nil
}

// CheckCodexNativePluginManifest validates the active native plugin manifest.
func CheckCodexNativePluginManifest(home, primary string, primaryCount int) *Check {
	manifestPath := CodexNativePluginManifestPath(home)
	manifestHash, err := SHA256File(manifestPath)
	if err != nil {
		return &Check{
			Name:   "Plugin",
			Status: "warn",
			Detail: fmt.Sprintf("%d skills found in %s; native plugin is missing .agentops-manifest.json — run 'bash scripts/refresh-codex-local.sh' from the repo checkout.",
				primaryCount, primary),
		}
	}

	manifestSkillCount, err := ReadCodexManifestSkillCount(manifestPath)
	if err != nil {
		return &Check{
			Name:   "Plugin",
			Status: "warn",
			Detail: fmt.Sprintf("%d skills found in %s; native plugin manifest is unreadable — run 'bash scripts/refresh-codex-local.sh'.",
				primaryCount, primary),
		}
	}

	meta, err := ReadCodexInstallMeta(home)
	if err != nil {
		return &Check{
			Name:   "Plugin",
			Status: "warn",
			Detail: fmt.Sprintf("%d skills found in %s; native plugin install metadata is missing — run 'bash scripts/refresh-codex-local.sh' from the repo checkout.",
				primaryCount, primary),
		}
	}

	expectedRoot := filepath.Join(home, ".codex", "plugins", "cache", CodexAgentOpsMarketplaceName, CodexAgentOpsPluginName, "local")
	if meta.InstallMode != "native-plugin" {
		return &Check{
			Name:   "Plugin",
			Status: "warn",
			Detail: fmt.Sprintf("%d skills found in %s; install metadata says install_mode=%q instead of native-plugin — run 'bash scripts/refresh-codex-local.sh'.",
				primaryCount, primary, meta.InstallMode),
		}
	}
	if meta.PluginRoot != "" && filepath.Clean(meta.PluginRoot) != expectedRoot {
		return &Check{
			Name:   "Plugin",
			Status: "warn",
			Detail: fmt.Sprintf("%d skills found in %s; install metadata points at %s instead of %s — run 'bash scripts/refresh-codex-local.sh'.",
				primaryCount, primary, meta.PluginRoot, expectedRoot),
		}
	}
	if meta.ManifestHash != "" && meta.ManifestHash != manifestHash {
		return &Check{
			Name:   "Plugin",
			Status: "warn",
			Detail: fmt.Sprintf("%d skills found in %s; install metadata manifest hash does not match the active native plugin manifest — run 'bash scripts/refresh-codex-local.sh'.",
				primaryCount, primary),
		}
	}
	if meta.SkillCount > 0 && manifestSkillCount > 0 && meta.SkillCount != manifestSkillCount {
		return &Check{
			Name:   "Plugin",
			Status: "warn",
			Detail: fmt.Sprintf("%d skills found in %s; install metadata says %d skills but manifest says %d — run 'bash scripts/refresh-codex-local.sh'.",
				primaryCount, primary, meta.SkillCount, manifestSkillCount),
		}
	}
	if manifestSkillCount > 0 && manifestSkillCount != primaryCount {
		return &Check{
			Name:   "Plugin",
			Status: "warn",
			Detail: fmt.Sprintf("%d skills found in %s; active native plugin manifest lists %d skills — run 'bash scripts/refresh-codex-local.sh'.",
				primaryCount, primary, manifestSkillCount),
		}
	}

	return &Check{
		Name:     "Plugin",
		Status:   "pass",
		Detail:   fmt.Sprintf("%d skills found in %s (native manifest OK)", primaryCount, primary),
		Required: false,
	}
}

// SkillInstall describes a candidate skill installation directory.
type SkillInstall struct {
	Path        string
	Label       string
	DisplayPath string
	Legacy      bool
}

// SkillInstallDirs returns the ordered list of candidate skill install locations.
func SkillInstallDirs(home string) []SkillInstall {
	return []SkillInstall{
		{
			Path:        CodexNativePluginSkillsPath(home),
			Label:       "Codex Native Plugin",
			DisplayPath: "~/.codex/plugins/cache/agentops-marketplace/agentops/local/skills-codex",
		},
		{
			Path:        filepath.Join(home, ".codex", "skills"),
			Label:       "Codex",
			DisplayPath: "~/.codex/skills",
		},
		{
			Path:        filepath.Join(home, ".claude", "skills"),
			Label:       "Claude",
			DisplayPath: "~/.claude/skills",
		},
		{
			Path:        filepath.Join(home, ".agents", "skills"),
			Label:       "User Skills",
			DisplayPath: "~/.agents/skills",
			Legacy:      true,
		},
	}
}

// ScanSkillDir returns the set of skill names found in a directory, or nil if none.
func ScanSkillDir(dir string) map[string]struct{} {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	names := make(map[string]struct{})
	for _, e := range entries {
		info, err := os.Stat(filepath.Join(dir, e.Name()))
		if err != nil || !info.IsDir() {
			continue
		}
		skillFile := filepath.Join(dir, e.Name(), "SKILL.md")
		if _, err := os.Stat(skillFile); err == nil {
			names[e.Name()] = struct{}{}
		}
	}
	if len(names) == 0 {
		return nil
	}
	return names
}

// SkillOverlapWarning returns a Check warning if base overlaps with any of others, or nil.
func SkillOverlapWarning(base map[string]struct{}, primaryCount int, primary, msgFmt string, others ...map[string]struct{}) *Check {
	overlaps := OverlappingSkillNames(base, others...)
	if len(overlaps) == 0 {
		return nil
	}
	sample := overlaps
	if len(sample) > 3 {
		sample = sample[:3]
	}
	return &Check{
		Name:   "Plugin",
		Status: "warn",
		Detail: fmt.Sprintf(msgFmt, primaryCount, primary, len(overlaps), strings.Join(sample, ", ")),
	}
}

func OverlappingSkillNames(base map[string]struct{}, others ...map[string]struct{}) []string {
	if len(base) == 0 {
		return nil
	}
	overlaps := make(map[string]struct{})
	for name := range base {
		for _, other := range others {
			if len(other) == 0 {
				continue
			}
			if _, ok := other[name]; ok {
				overlaps[name] = struct{}{}
				break
			}
		}
	}
	if len(overlaps) == 0 {
		return nil
	}
	names := make([]string, 0, len(overlaps))
	for name := range overlaps {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// CheckSkills validates the installed skill set across known install locations.
func CheckSkills() Check {
	home, err := os.UserHomeDir()
	if err != nil {
		return Check{Name: "Plugin", Status: "warn", Detail: "cannot determine home directory", Required: false}
	}

	installs := SkillInstallDirs(home)
	installedNames := make(map[string]map[string]struct{}, len(installs))
	primary := ""
	primaryCount := 0
	legacyNames := map[string]struct{}{}

	for _, install := range installs {
		names := ScanSkillDir(install.Path)
		if names == nil {
			continue
		}
		installedNames[install.DisplayPath] = names
		if primary == "" {
			primary = install.DisplayPath
			primaryCount = len(names)
		}
		if install.Legacy {
			legacyNames = names
		}
	}

	if primaryCount == 0 {
		return Check{Name: "Plugin", Status: "warn", Detail: "no skills found — " + pluginInstallHint(), Required: false}
	}

	nativeNames := installedNames["~/.codex/plugins/cache/agentops-marketplace/agentops/local/skills-codex"]
	rawCodexNames := installedNames["~/.codex/skills"]

	if len(nativeNames) > 0 && len(rawCodexNames) > 0 {
		if w := SkillOverlapWarning(rawCodexNames, primaryCount, primary,
			"%d skills found in %s; duplicate raw Codex install also present in ~/.codex/skills (%d overlapping skill names, e.g. %s). Remove or archive the AgentOps skill folders in ~/.codex/skills.",
			nativeNames); w != nil {
			return *w
		}
	}

	if len(legacyNames) > 0 && len(nativeNames) > 0 {
		if w := SkillOverlapWarning(legacyNames, primaryCount, primary,
			"%d skills found in %s; duplicate raw skill install also present in ~/.agents/skills (%d overlapping skill names, e.g. %s). Remove or archive the AgentOps-managed folders in ~/.agents/skills.",
			nativeNames); w != nil {
			return *w
		}
	}

	if len(legacyNames) > 0 {
		if w := SkillOverlapWarning(legacyNames, primaryCount, primary,
			"%d skills found in %s; duplicate raw skill install also present in ~/.agents/skills (%d overlapping skill names, e.g. %s). Remove or archive the AgentOps-managed folders in ~/.agents/skills.",
			rawCodexNames, installedNames["~/.claude/skills"]); w != nil {
			return *w
		}
	}

	if primary == "~/.codex/plugins/cache/agentops-marketplace/agentops/local/skills-codex" {
		return *CheckCodexNativePluginManifest(home, primary, primaryCount)
	}

	return Check{
		Name:     "Plugin",
		Status:   "pass",
		Detail:   fmt.Sprintf("%d skills found in %s", primaryCount, primary),
		Required: false,
	}
}

func pluginInstallHint() string {
	if runtime.GOOS == "windows" {
		return "for Codex run 'irm https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-codex.ps1 | iex'; for Claude Code use 'claude plugin install agentops@agentops-marketplace'"
	}
	return "run 'bash <(curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install.sh)'"
}

func FindAgentOpsRepoRoot(start string) string {
	dir := start
	for {
		if FileExists(filepath.Join(dir, ".git")) && FileExists(filepath.Join(dir, "skills-codex", ".agentops-manifest.json")) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func CurrentRepoVersion(repoRoot string) string {
	out, err := exec.Command("git", "-C", repoRoot, "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func ModeOrDefault(mode string) string {
	if mode == "" {
		return "install"
	}
	return mode
}

func ValueOrUnknown(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}

// CheckCodexSync verifies the installed Codex plugin matches the local repo.
func CheckCodexSync() Check {
	home, err := os.UserHomeDir()
	if err != nil {
		return Check{Name: "Codex Sync", Status: "warn", Detail: "cannot determine home directory", Required: false}
	}

	meta, err := ReadCodexInstallMeta(home)
	if err != nil {
		return Check{Name: "Codex Sync", Status: "pass", Detail: "no Codex install metadata found", Required: false}
	}

	cwd, err := os.Getwd()
	if err != nil {
		return Check{Name: "Codex Sync", Status: "warn", Detail: "cannot determine current directory", Required: false}
	}

	repoRoot := FindAgentOpsRepoRoot(cwd)
	if repoRoot == "" {
		return Check{Name: "Codex Sync", Status: "pass", Detail: "no local AgentOps repo context detected", Required: false}
	}

	repoManifest := filepath.Join(repoRoot, "skills-codex", ".agentops-manifest.json")
	repoManifestHash, err := SHA256File(repoManifest)
	if err != nil {
		return Check{Name: "Codex Sync", Status: "warn", Detail: "cannot read local skills-codex manifest", Required: false}
	}

	repoVersion := CurrentRepoVersion(repoRoot)
	if meta.ManifestHash == "" {
		return Check{
			Name:   "Codex Sync",
			Status: "warn",
			Detail: fmt.Sprintf("Codex install metadata is missing manifest hash — run 'cd %s && bash scripts/refresh-codex-local.sh'", repoRoot),
		}
	}

	if meta.ManifestHash != repoManifestHash {
		if repoVersion != "" && meta.Version != "" && meta.Version == repoVersion {
			return Check{
				Name:   "Codex Sync",
				Status: "warn",
				Detail: fmt.Sprintf("installed Codex %s manifest differs from repo %s — run 'cd %s && bash scripts/refresh-codex-local.sh'",
					ModeOrDefault(meta.InstallMode), ValueOrUnknown(repoVersion), repoRoot),
			}
		}
		return Check{
			Name:   "Codex Sync",
			Status: "warn",
			Detail: fmt.Sprintf("installed Codex %s is stale relative to repo (%s -> %s) — run 'cd %s && bash scripts/refresh-codex-local.sh'",
				ModeOrDefault(meta.InstallMode), ValueOrUnknown(meta.Version), ValueOrUnknown(repoVersion), repoRoot),
		}
	}

	if repoVersion != "" && meta.Version != "" && meta.Version != repoVersion {
		return Check{
			Name:   "Codex Sync",
			Status: "warn",
			Detail: fmt.Sprintf("installed Codex %s is stale relative to repo (%s -> %s) — run 'cd %s && bash scripts/refresh-codex-local.sh'",
				ModeOrDefault(meta.InstallMode), ValueOrUnknown(meta.Version), ValueOrUnknown(repoVersion), repoRoot),
		}
	}

	return Check{
		Name:     "Codex Sync",
		Status:   "pass",
		Detail:   fmt.Sprintf("installed Codex %s matches repo %s", ModeOrDefault(meta.InstallMode), ValueOrUnknown(repoVersion)),
		Required: false,
	}
}

// FindHealScript searches for heal.sh in known locations and returns the path if found.
func FindHealScript() string {
	if p := "skills/heal-skill/scripts/heal.sh"; FileExists(p) {
		abs, err := filepath.Abs(p)
		if err == nil {
			return abs
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	if p := CodexNativePluginHealPath(home); FileExists(p) {
		return p
	}
	if p := filepath.Join(home, ".codex", "skills", "heal-skill", "scripts", "heal.sh"); FileExists(p) {
		return p
	}
	if p := filepath.Join(home, ".claude", "skills", "heal-skill", "scripts", "heal.sh"); FileExists(p) {
		return p
	}
	if p := filepath.Join(home, ".agents", "skills", "heal-skill", "scripts", "heal.sh"); FileExists(p) {
		return p
	}

	return ""
}

// CheckSkillIntegrity runs heal.sh --strict to validate skill hygiene.
// healStrictDefaultTimeout is the wall-clock budget for `heal.sh --strict` run
// as part of `ao doctor health`. On a fresh checkout (first run, cold caches)
// a strict sweep through the full skills tree typically takes 45-75s, so the
// budget needs to exceed that with margin. Operators with slower disks or
// busier machines can override via `AO_DOCTOR_HEAL_TIMEOUT` (Go duration
// string, e.g. "3m").
const healStrictDefaultTimeout = 120 * time.Second

func healStrictTimeout() time.Duration {
	if v := strings.TrimSpace(os.Getenv("AO_DOCTOR_HEAL_TIMEOUT")); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return healStrictDefaultTimeout
}

func CheckSkillIntegrity() Check {
	healPath := FindHealScript()
	if healPath == "" {
		return Check{
			Name:     "Skill Integrity",
			Status:   "warn",
			Detail:   "heal-skill not installed, skipping integrity check",
			Required: false,
		}
	}

	timeout := healStrictTimeout()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", healPath, "--strict")
	output, err := cmd.CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		return Check{
			Name:     "Skill Integrity",
			Status:   "warn",
			Detail:   fmt.Sprintf("heal.sh timed out after %s", timeout),
			Required: false,
		}
	}

	if err == nil {
		return Check{
			Name:     "Skill Integrity",
			Status:   "pass",
			Detail:   "All skill integrity checks passed",
			Required: false,
		}
	}

	findings := CountHealFindings(string(output))
	return Check{
		Name:     "Skill Integrity",
		Status:   "warn",
		Detail:   fmt.Sprintf("%d skill hygiene finding(s) — run 'heal.sh --check' for details", findings),
		Required: false,
	}
}

// CheckOptionalCLI reports whether an optional CLI dependency is installed.
func CheckOptionalCLI(name string, reason string) Check {
	_, err := exec.LookPath(name)
	if err != nil {
		return Check{
			Name:     strings.Title(name) + " CLI", //nolint:staticcheck
			Status:   "warn",
			Detail:   fmt.Sprintf("not found (optional — %s)", reason),
			Required: false,
		}
	}

	return Check{
		Name:     strings.Title(name) + " CLI", //nolint:staticcheck
		Status:   "pass",
		Detail:   "available",
		Required: false,
	}
}
