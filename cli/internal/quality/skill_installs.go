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
	"strconv"
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

	// codexPluginSkillsDir is the skill tree the AgentOps Codex plugin ships,
	// as declared by `.codex-plugin/plugin.json` ("skills": "./skills"). The
	// plugin loads the same skills/ tree every other runtime does.
	codexPluginSkillsDir = "skills"
)

// CodexInstallMeta describes the Codex plugin install metadata a legacy
// installer may have left behind. Only the recorded plugin root is consulted,
// to locate the live plugin cache.
type CodexInstallMeta struct {
	InstallMode string `json:"install_mode"`
	PluginRoot  string `json:"plugin_root"`
	Version     string `json:"version"`
}

func codexNativePluginCacheBase(home string) string {
	return filepath.Join(
		home,
		".codex",
		"plugins",
		"cache",
		CodexAgentOpsMarketplaceName,
		CodexAgentOpsPluginName,
	)
}

func hasCodexSkills(root string) bool {
	info, err := os.Stat(filepath.Join(root, codexPluginSkillsDir))
	return err == nil && info.IsDir()
}

// pluginCacheNameLess orders dotted numeric cache versions naturally and
// falls back to lexical ordering for non-version labels.
func pluginCacheNameLess(a, b string) bool {
	parse := func(s string) ([]int, bool) {
		parts := strings.Split(s, ".")
		out := make([]int, len(parts))
		for i, part := range parts {
			n, err := strconv.Atoi(part)
			if err != nil {
				return nil, false
			}
			out[i] = n
		}
		return out, true
	}
	av, aok := parse(a)
	bv, bok := parse(b)
	if aok && bok {
		for i := 0; i < len(av) || i < len(bv); i++ {
			var ai, bi int
			if i < len(av) {
				ai = av[i]
			}
			if i < len(bv) {
				bi = bv[i]
			}
			if ai != bi {
				return ai < bi
			}
		}
	}
	return a < b
}

// CodexNativePluginRootPath resolves the live AgentOps plugin cache. Codex
// installs released plugins under a version directory; older local installs
// used "local". Prefer a live metadata target, then a live local tree, then the
// highest live version. Return the historical local target only for a fresh
// install where no cache exists yet.
func CodexNativePluginRootPath(home string) string {
	base := codexNativePluginCacheBase(home)
	local := filepath.Join(base, "local")
	if meta, err := ReadCodexInstallMeta(home); err == nil && meta.PluginRoot != "" && hasCodexSkills(meta.PluginRoot) {
		return filepath.Clean(meta.PluginRoot)
	}
	if hasCodexSkills(local) {
		return local
	}
	entries, err := os.ReadDir(base)
	if err != nil {
		return local
	}
	var candidates []string
	for _, entry := range entries {
		if entry.IsDir() && entry.Name() != "local" && hasCodexSkills(filepath.Join(base, entry.Name())) {
			candidates = append(candidates, entry.Name())
		}
	}
	if len(candidates) == 0 {
		return local
	}
	sort.Slice(candidates, func(i, j int) bool { return pluginCacheNameLess(candidates[i], candidates[j]) })
	return filepath.Join(base, candidates[len(candidates)-1])
}

// CodexNativePluginSkillsPath returns the skills tree inside the live AgentOps
// Codex plugin cache.
func CodexNativePluginSkillsPath(home string) string {
	return filepath.Join(CodexNativePluginRootPath(home), codexPluginSkillsDir)
}

func CodexNativePluginHealPath(home string) string {
	return filepath.Join(CodexNativePluginSkillsPath(home), "skill-builder", "scripts", "heal.sh")
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

// SkillInstall describes a candidate skill installation directory.
type SkillInstall struct {
	Path        string
	Label       string
	DisplayPath string
	Legacy      bool
}

// SkillInstallDirs returns the ordered list of candidate skill install locations.
func SkillInstallDirs(home string) []SkillInstall {
	nativeSkills := CodexNativePluginSkillsPath(home)
	nativeDisplay := strings.Replace(filepath.ToSlash(nativeSkills), filepath.ToSlash(home), "~", 1)
	return []SkillInstall{
		{
			Path:        nativeSkills,
			Label:       "Codex Native Plugin",
			DisplayPath: nativeDisplay,
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
	nativeDisplay := installs[0].DisplayPath
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

	nativeNames := installedNames[nativeDisplay]
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

	return Check{
		Name:     "Plugin",
		Status:   "pass",
		Detail:   fmt.Sprintf("%d skills found in %s", primaryCount, primary),
		Required: false,
	}
}

func pluginInstallHint() string {
	if runtime.GOOS == "windows" {
		return "clone AgentOps and run 'ao skills link' (CLI: irm https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-ao.ps1 | iex)"
	}
	return "clone AgentOps and run 'ao skills link' (CLI: brew install agentops)"
}

// FindHealScript searches for heal.sh in known locations and returns the path if found.
func FindHealScript() string {
	if p := "skills/skill-builder/scripts/heal.sh"; FileExists(p) {
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
	if p := filepath.Join(home, ".codex", "skills", "skill-builder", "scripts", "heal.sh"); FileExists(p) {
		return p
	}
	if p := filepath.Join(home, ".claude", "skills", "skill-builder", "scripts", "heal.sh"); FileExists(p) {
		return p
	}
	if p := filepath.Join(home, ".agents", "skills", "skill-builder", "scripts", "heal.sh"); FileExists(p) {
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
			Detail:   "heal.sh not installed, skipping integrity check",
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
// fix is an optional remediation runnable from the reader's own context (e.g.
// "npm install -g @openai/codex"); it must never be a repo-relative script.
func CheckOptionalCLI(name, reason, fix string) Check {
	_, err := exec.LookPath(name)
	if err != nil {
		return Check{
			Name:     strings.Title(name) + " CLI", //nolint:staticcheck
			Status:   "info",
			Detail:   fmt.Sprintf("not found (optional — %s)", reason),
			Required: false,
			Audience: AudienceInstalledUser,
			Fix:      fix,
		}
	}

	return Check{
		Name:     strings.Title(name) + " CLI", //nolint:staticcheck
		Status:   "pass",
		Detail:   "available",
		Required: false,
		Audience: AudienceInstalledUser,
	}
}
