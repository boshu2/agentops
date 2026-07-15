// Package doctor contains concrete runtime adapters for doctor use cases.
package doctor

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/provenancegraph"
	"github.com/boshu2/agentops/cli/internal/quality"
	"github.com/boshu2/agentops/cli/internal/reviewerhealth"
	"github.com/boshu2/agentops/cli/internal/storage"
)

const reviewerTimeout = 10 * time.Second

type LegacyChecks struct {
	ToolVersion string
	IndexDir    string
	IndexFile   string
	LedgerPath  func() string
	Reviewers   reviewerhealth.Service
	WorkingDir  func() (string, error)
	Environment func() []string
	LookPath    func(string) (string, error)
	Now         func() time.Time
}

func (adapter LegacyChecks) Checks(ctx context.Context) []quality.Check {
	cwd, cwdErr := adapter.WorkingDir()
	inRepo := false
	if cwdErr == nil {
		_, inRepo = FindAgentopsRepoRoot(cwd)
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

	// Installed-user: optional external tooling (gt/bd), reported as optional.
	add(quality.CheckCLIDependencies(adapter.LookPath), quality.AudienceInstalledUser)

	// Installed-user: local knowledge state.
	if cwdErr != nil {
		add(quality.Check{Name: "Knowledge Base", Status: "fail", Detail: "cannot determine working directory", Required: true}, quality.AudienceInstalledUser)
		add(quality.Check{Name: "Knowledge Freshness", Status: quality.StatusInfo, Detail: "cannot determine working directory"}, quality.AudienceInstalledUser)
		add(quality.Check{Name: "Search Index", Status: quality.StatusInfo, Detail: "cannot determine working directory"}, quality.AudienceInstalledUser)
		add(quality.Check{Name: "Flywheel Health", Status: quality.StatusInfo, Detail: "cannot determine working directory"}, quality.AudienceInstalledUser)
	} else {
		base := filepath.Join(cwd, storage.DefaultBaseDir)
		add(quality.CheckKnowledgeBase(base), quality.AudienceInstalledUser)
		add(quality.CheckKnowledgeFreshness(filepath.Join(base, "sessions")), quality.AudienceInstalledUser)
		add(quality.CheckSearchIndex(filepath.Join(cwd, adapter.IndexDir, adapter.IndexFile)), quality.AudienceInstalledUser)
		add(quality.CheckFlywheelHealth(base), quality.AudienceInstalledUser)
	}

	// Installed-user: optional cross-family review capability.
	add(quality.CheckOptionalCLI("codex", "enables --mixed council review", "npm install -g @openai/codex"), quality.AudienceInstalledUser)
	reviewerChecks, live := adapter.Reviewers.Check(ctx, reviewerTimeout)
	for _, reviewerCheck := range reviewerChecks {
		add(softenOptional(reviewerCheck), quality.AudienceInstalledUser)
	}
	add(CrossFamilyCheck(live), quality.AudienceInstalledUser)

	// Repo-dev: skill/codex/plugin/binary hygiene — meaningful only inside a
	// clone, where the remediations (heal.sh, refresh-codex-local.sh, rebuild)
	// are runnable. Outside a clone, collapse to a single info line so a
	// pristine install never sees repo-internal warnings.
	if inRepo {
		add(quality.CheckSkills(), quality.AudienceRepoDev)
		add(quality.CheckCodexSync(), quality.AudienceRepoDev)
		add(quality.CheckSkillIntegrity(), quality.AudienceRepoDev)
		add(quality.CheckStaleReferences([]string{
			"skills/*/SKILL.md", "skills/*/references/*.md", "skills-codex/*/SKILL.md",
			"skills-codex-overrides/*/SKILL.md", "docs/*.md", "scripts/*.sh",
			"docs/contracts/*.md", "docs/plans/*.md",
		}), quality.AudienceRepoDev)
		add(BinaryFreshnessCheck(cwd, adapter.ToolVersion), quality.AudienceRepoDev)
	} else {
		add(quality.Check{
			Name:   "Repo-dev checks",
			Status: quality.StatusInfo,
			Detail: "skipped — outside an agentops repo clone (skill hygiene, codex sync, stale refs, plugin manifest, binary freshness)",
		}, quality.AudienceRepoDev)
	}

	// Installed-user: required integrity.
	ledgerPath := ""
	if adapter.LedgerPath != nil {
		ledgerPath = adapter.LedgerPath()
	}
	add(CheckLedgerHealth(ledgerPath, adapter.Now), quality.AudienceInstalledUser)
	add(CheckLaw0Guard(adapter.Environment()), quality.AudienceInstalledUser)
	return checks
}

// softenOptional downgrades an optional-capability check from "warn" to "info":
// a missing optional reviewer CLI is expected on a fresh install and must not
// read as something wrong. A pass/fail is left untouched.
func softenOptional(check quality.Check) quality.Check {
	if check.Status == quality.StatusWarn {
		check.Status = quality.StatusInfo
	}
	return check
}

func SystemLegacyChecks(toolVersion, indexDir, indexFile string, ledgerPath func() string, reviewers reviewerhealth.Service) LegacyChecks {
	return LegacyChecks{
		ToolVersion: toolVersion, IndexDir: indexDir, IndexFile: indexFile, LedgerPath: ledgerPath, Reviewers: reviewers,
		WorkingDir: os.Getwd, Environment: os.Environ, LookPath: exec.LookPath, Now: time.Now,
	}
}

func CrossFamilyCheck(live []string) quality.Check {
	if len(live) > 0 {
		return quality.Check{Name: "Cross-Family Review", Status: "pass", Detail: fmt.Sprintf("cross-family capable: yes (live families: %s)", strings.Join(live, ", ")), Audience: quality.AudienceInstalledUser}
	}
	// Optional capability: no reviewer CLI is expected on a fresh install, so
	// this is informational, not a warning.
	return quality.Check{Name: "Cross-Family Review", Status: quality.StatusInfo, Detail: "cross-family capable: no (no reviewer CLI reachable) — install one to enable cross-family review", Audience: quality.AudienceInstalledUser, Fix: "npm install -g @openai/codex"}
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
		return quality.Check{Name: name, Status: "fail", Detail: fmt.Sprintf("cannot read ledger at %s (%v) — check the path and permissions: ls -l %s", path, err, path), Required: true}
	}
	if !result.Pass {
		return quality.Check{Name: name, Status: "fail", Detail: fmt.Sprintf("chain breaks at line %d: %s — inspect: ao provenance verify", result.FirstBrokenLine, result.Message), Required: true}
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
