// Package doctor contains concrete runtime adapters for doctor use cases.
package doctor

import (
	"context"
	"fmt"
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
	checks := []quality.Check{quality.CheckCLIDependencies(adapter.LookPath)}
	cwd, cwdErr := adapter.WorkingDir()
	if cwdErr != nil {
		checks = append(checks,
			quality.Check{Name: "Knowledge Base", Status: "fail", Detail: "cannot determine working directory", Required: true},
			quality.Check{Name: "Knowledge Freshness", Status: "warn", Detail: "cannot determine working directory"},
			quality.Check{Name: "Search Index", Status: "warn", Detail: "cannot determine working directory"},
			quality.Check{Name: "Flywheel Health", Status: "warn", Detail: "cannot determine working directory"},
		)
	} else {
		base := filepath.Join(cwd, storage.DefaultBaseDir)
		checks = append(checks,
			quality.CheckKnowledgeBase(base),
			quality.CheckKnowledgeFreshness(filepath.Join(base, "sessions")),
			quality.CheckSearchIndex(filepath.Join(cwd, adapter.IndexDir, adapter.IndexFile)),
			quality.CheckFlywheelHealth(base),
		)
	}
	checks = append(checks,
		quality.CheckSkills(), quality.CheckCodexSync(), quality.CheckSkillIntegrity(),
		quality.CheckStaleReferences([]string{
			"skills/*/SKILL.md", "skills/*/references/*.md", "skills-codex/*/SKILL.md",
			"skills-codex-overrides/*/SKILL.md", "docs/*.md", "scripts/*.sh",
			"docs/contracts/*.md", "docs/plans/*.md",
		}),
		quality.CheckOptionalCLI("codex", "needed for --mixed council"),
	)
	reviewerChecks, live := adapter.Reviewers.Check(ctx, reviewerTimeout)
	checks = append(checks, reviewerChecks...)
	checks = append(checks, CrossFamilyCheck(live))
	if cwdErr != nil {
		checks = append(checks, quality.Check{Name: "Binary Freshness", Status: "warn", Detail: "cannot determine working directory"})
	} else {
		checks = append(checks, BinaryFreshnessCheck(cwd, adapter.ToolVersion))
	}
	ledgerPath := ""
	if adapter.LedgerPath != nil {
		ledgerPath = adapter.LedgerPath()
	}
	checks = append(checks, CheckLedgerHealth(ledgerPath, adapter.Now), CheckLaw0Guard(adapter.Environment()))
	return checks
}

func SystemLegacyChecks(toolVersion, indexDir, indexFile string, ledgerPath func() string, reviewers reviewerhealth.Service) LegacyChecks {
	return LegacyChecks{
		ToolVersion: toolVersion, IndexDir: indexDir, IndexFile: indexFile, LedgerPath: ledgerPath, Reviewers: reviewers,
		WorkingDir: os.Getwd, Environment: os.Environ, LookPath: exec.LookPath, Now: time.Now,
	}
}

func CrossFamilyCheck(live []string) quality.Check {
	if len(live) > 0 {
		return quality.Check{Name: "Cross-Family Review", Status: "pass", Detail: fmt.Sprintf("cross-family capable: yes (live families: %s)", strings.Join(live, ", "))}
	}
	return quality.Check{Name: "Cross-Family Review", Status: "warn", Detail: "cross-family capable: no (no reviewer CLI reachable) — install one: npm install -g @openai/codex && codex login"}
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
		data, err := os.ReadFile(filepath.Join(dir, "cli", "go.mod"))
		if err == nil && len(data) <= 1<<16 && strings.Contains(string(data), agentopsModuleLine) {
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
	data, err := os.ReadFile(filepath.Join(root, "cli", "cmd", "ao", "main.go"))
	if err != nil || len(data) > 1<<16 {
		return "", false
	}
	match := repoVersionPattern.FindSubmatch(data)
	return stringMatch(match)
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
	return quality.Check{Name: "LAW-0 Guard", Status: "pass", Detail: "no reviewer path configured through claude print-mode (scanned PAWL_*/AGENTOPS_*/AO_*/*REVIEWER* env)", Required: true}
}

func law0RelevantEnv(name string) bool {
	return strings.HasPrefix(name, "PAWL_") || strings.HasPrefix(name, "AGENTOPS_") || strings.HasPrefix(name, "AO_") || strings.Contains(name, "REVIEWER")
}
