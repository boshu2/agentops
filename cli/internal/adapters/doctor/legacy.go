// Package doctor contains concrete runtime adapters for doctor use cases.
package doctor

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/provenancegraph"
	"github.com/boshu2/agentops/cli/internal/quality"
)

type LegacyChecks struct {
	ToolVersion string
	LedgerPath  func() string
	WorkingDir  func() (string, error)
	HomeDir     func() (string, error)
	Environment func() []string
	Now         func() time.Time
}

func (adapter LegacyChecks) Checks(_ context.Context) []quality.Check {
	cwd, cwdErr := adapter.WorkingDir()
	repoRoot := ""
	if cwdErr == nil {
		repoRoot, _ = FindAgentopsRepoRoot(cwd)
	}

	var checks []quality.Check
	// add appends a check, defaulting its audience when the producer did not set
	// one so every rendered check is guaranteed to declare an audience.
	add := func(c quality.Check, audience string) {
		if c.Audience == "" {
			c.Audience = audience
		}
		checks = append(checks, c)
	}

	// Doctor is installation health, not a second operating loop. It checks the
	// live source-link contract and local integrity facts; tracker, reviewer,
	// search, flywheel, plugin-cache, and repair policy belong to their own
	// explicit commands or skills.
	home := ""
	if adapter.HomeDir != nil {
		home, _ = adapter.HomeDir()
	}
	add(CheckSkillLinks(repoRoot, home), quality.AudienceInstalledUser)
	add(BinaryFreshnessCheck(cwd, adapter.ToolVersion), quality.AudienceRepoDev)

	// Installed-user: required integrity.
	ledgerPath := ""
	if adapter.LedgerPath != nil {
		ledgerPath = adapter.LedgerPath()
	}
	add(CheckLedgerHealth(ledgerPath, adapter.Now), quality.AudienceInstalledUser)
	add(CheckLaw0Guard(adapter.Environment()), quality.AudienceInstalledUser)
	return checks
}

func SystemLegacyChecks(toolVersion string, ledgerPath func() string) LegacyChecks {
	return LegacyChecks{
		ToolVersion: toolVersion, LedgerPath: ledgerPath,
		WorkingDir: os.Getwd, HomeDir: os.UserHomeDir, Environment: os.Environ, Now: time.Now,
	}
}

var runtimeSkillRoots = []string{".claude", ".codex", ".gemini", ".cursor", ".pi"}

// CheckSkillLinks verifies the source-linked distribution contract: canonical skills are
// consumed directly from one checkout through exact symlinks. Portable
// ~/.agents/skills is always checked; installed runtime roots are checked when
// their config directory exists. Plugin caches are intentionally irrelevant.
func CheckSkillLinks(repoRoot, home string) quality.Check {
	check := quality.Check{Name: "Skill Links", Required: false}
	if home == "" {
		check.Status = quality.StatusWarn
		check.Detail = "cannot determine home directory"
		return check
	}
	if repoRoot == "" {
		portable := filepath.Join(home, ".agents", "skills")
		live, broken := countLiveSkillLinks(portable)
		if broken > 0 {
			check.Status = quality.StatusWarn
			check.Detail = fmt.Sprintf("%d live portable skill link(s), %d broken; run from the AgentOps checkout and inspect `ao skills link --dry-run`", live, broken)
			return check
		}
		check.Status = quality.StatusInfo
		check.Detail = fmt.Sprintf("%d live portable skill link(s); exact source identity is checked from an AgentOps checkout", live)
		return check
	}

	source := filepath.Join(repoRoot, "skills")
	names, err := canonicalSkillNames(source)
	if err != nil {
		check.Status = quality.StatusWarn
		check.Detail = fmt.Sprintf("cannot read canonical skills: %v", err)
		return check
	}
	dests := []string{filepath.Join(home, ".agents", "skills")}
	for _, config := range runtimeSkillRoots {
		if info, err := os.Stat(filepath.Join(home, config)); err == nil && info.IsDir() {
			dests = append(dests, filepath.Join(home, config, "skills"))
		}
	}

	missing, conflicts, stale := 0, 0, 0
	for _, dest := range dests {
		for _, name := range names {
			path := filepath.Join(dest, name)
			info, err := os.Lstat(path)
			if os.IsNotExist(err) {
				missing++
				continue
			}
			if err != nil || info.Mode()&os.ModeSymlink == 0 || !linkTargets(path, filepath.Join(source, name)) {
				conflicts++
			}
		}
		stale += staleLinksToSource(dest, source, names)
	}
	if missing > 0 || conflicts > 0 || stale > 0 {
		check.Status = quality.StatusWarn
		check.Detail = fmt.Sprintf("%d expected link(s) across %d root(s): %d missing, %d conflicting, %d stale; inspect `ao skills link --dry-run`", len(names)*len(dests), len(dests), missing, conflicts, stale)
		check.Fix = "ao skills link --dry-run"
		return check
	}
	check.Status = quality.StatusPass
	check.Detail = fmt.Sprintf("%d canonical skills live-linked across %d root(s)", len(names), len(dests))
	return check
}

func canonicalSkillNames(source string) ([]string, error) {
	entries, err := os.ReadDir(source)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if info, err := os.Stat(filepath.Join(source, entry.Name(), "SKILL.md")); err == nil && info.Mode().IsRegular() {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

func linkTargets(path, want string) bool {
	target, err := os.Readlink(path)
	if err != nil {
		return false
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(path), target)
	}
	return filepath.Clean(target) == filepath.Clean(want)
}

func staleLinksToSource(dest, source string, canonical []string) int {
	wanted := make(map[string]bool, len(canonical))
	for _, name := range canonical {
		wanted[name] = true
	}
	entries, err := os.ReadDir(dest)
	if err != nil {
		return 0
	}
	stale := 0
	for _, entry := range entries {
		if wanted[entry.Name()] {
			continue
		}
		path := filepath.Join(dest, entry.Name())
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			continue
		}
		target, err := os.Readlink(path)
		if err != nil {
			continue
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(dest, target)
		}
		if filepath.Clean(filepath.Dir(target)) == filepath.Clean(source) {
			stale++
		}
	}
	return stale
}

func countLiveSkillLinks(dir string) (live, broken int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, 0
	}
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			continue
		}
		if _, err := os.Stat(filepath.Join(path, "SKILL.md")); err != nil {
			broken++
		} else {
			live++
		}
	}
	return live, broken
}

const agentopsModuleLine = "module github.com/boshu2/agentops/cli"

var repoVersionPattern = regexp.MustCompile(`var version = "([^"]+)"`)

func BinaryFreshnessCheck(dir, running string) quality.Check {
	name := "Binary Freshness"
	root, ok := FindAgentopsRepoRoot(dir)
	if !ok {
		return quality.Check{Name: name, Status: "pass", Detail: fmt.Sprintf("ao %s (outside the agentops repo — freshness not applicable)", running)}
	}
	repoVersion, ok := RepoDeclaredVersion(root)
	if !ok {
		return quality.Check{Name: name, Status: "warn", Detail: "cannot read the repo's declared version (cli/cmd/ao/main.go) — rebuild to be safe: scripts/preflight-uat-binary.sh"}
	}
	if repoVersion == running {
		return quality.Check{Name: name, Status: "pass", Detail: fmt.Sprintf("ao %s matches the repo's declared version", running)}
	}
	if strings.TrimSuffix(repoVersion, "-rc") == running {
		return quality.Check{Name: name, Status: "pass", Detail: fmt.Sprintf("ao %s is the release build for source series %s", running, repoVersion)}
	}
	return quality.Check{Name: name, Status: "warn", Detail: fmt.Sprintf("running ao %s but the repo declares %s (stale installed binary) — rebuild+install: scripts/preflight-uat-binary.sh (or: brew upgrade agentops)", running, repoVersion)}
}

func FindAgentopsRepoRoot(dir string) (string, bool) {
	for range 12 {
		data, ok := readRegularFileCapped(filepath.Join(dir, "cli", "go.mod"), 1<<16)
		if ok && strings.Contains(string(data), agentopsModuleLine) {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", false
}

func RepoDeclaredVersion(root string) (string, bool) {
	data, ok := readRegularFileCapped(filepath.Join(root, "cli", "cmd", "ao", "main.go"), 1<<16)
	if !ok {
		return "", false
	}
	match := repoVersionPattern.FindSubmatch(data)
	return stringMatch(match)
}

func readRegularFileCapped(path string, maxBytes int64) ([]byte, bool) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil, false
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, maxBytes))
	return data, err == nil
}

func stringMatch(match [][]byte) (string, bool) {
	if len(match) != 2 {
		return "", false
	}
	return string(match[1]), true
}

func CheckLedgerHealth(path string, now func() time.Time) quality.Check {
	name := "Ledger Health"
	store := provenancegraph.NewStore(path)
	result, err := store.VerifyFile()
	if err != nil {
		return quality.Check{Name: name, Status: quality.StatusWarn, Detail: fmt.Sprintf("optional provenance ledger at %s is unreadable (%v)", path, err), Required: false}
	}
	if !result.Pass {
		return quality.Check{Name: name, Status: quality.StatusWarn, Detail: fmt.Sprintf("optional provenance chain breaks at line %d: %s — inspect: ao provenance verify", result.FirstBrokenLine, result.Message), Required: false}
	}
	if result.RecordCount == 0 {
		return quality.Check{Name: name, Status: "pass", Detail: "no ledger records yet (an empty or absent ledger is an intact chain)"}
	}
	detail := fmt.Sprintf("chain intact (%d records", result.RecordCount)
	edges, readErr := store.Read()
	if readErr == nil && len(edges) > 0 && now != nil {
		if timestamp, parseErr := time.Parse(time.RFC3339, edges[len(edges)-1].TS); parseErr == nil {
			detail += fmt.Sprintf(", newest record %s old", quality.FormatDuration(now().Sub(timestamp)))
		}
	}
	return quality.Check{Name: name, Status: "pass", Detail: detail + ")"}
}

var law0Needles = []string{"claude" + " -p", "claude" + " --print"}

func CheckLaw0Guard(environment []string) quality.Check {
	for _, item := range environment {
		key, value, ok := strings.Cut(item, "=")
		if !ok || !law0RelevantEnv(key) {
			continue
		}
		for _, needle := range law0Needles {
			if strings.Contains(value, needle) {
				return quality.Check{Name: "LAW-0 Guard", Status: "fail", Detail: fmt.Sprintf("LAW-0 violation: $%s routes a reviewer through %q — remove it: unset %s", key, needle, key), Required: true}
			}
		}
	}
	return quality.Check{Name: "LAW-0 Guard", Status: "pass", Detail: "no reviewer path configured through claude print-mode (scanned AGENTOPS_*/AO_*/*REVIEWER* env)", Required: true}
}

func law0RelevantEnv(name string) bool {
	return strings.HasPrefix(name, "AGENTOPS_") || strings.HasPrefix(name, "AO_") || strings.Contains(name, "REVIEWER")
}
