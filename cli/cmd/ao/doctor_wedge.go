// practices: [sre, resilience-patterns]
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/provenancegraph"
)

// wedgeReviewerTimeout bounds each live reviewer probe so a hanging reviewer
// binary can never wedge `ao doctor` itself: on expiry the reviewer is
// reported unreachable and doctor moves on.
const wedgeReviewerTimeout = 10 * time.Second

// reviewerProbe describes one cross-family reviewer CLI probed by the wedge
// preflight, with the exact install command a failing check must teach.
type reviewerProbe struct {
	name       string
	installCmd string
}

// wedgeReviewers is the closed set of reviewer families the verification
// wedge can route reviews through. Claude is the producer family, so
// cross-family capability requires at least one of these to answer live.
// LAW 0: claude print-mode is never a reviewer path and never appears here.
var wedgeReviewers = []reviewerProbe{
	{name: "codex", installCmd: "npm install -g @openai/codex && codex login"},
	{name: "agy", installCmd: "install the AGY CLI, then verify with 'agy models'"},
}

// wedgeDoctorChecks runs the wedge/verification preflight: reviewer
// reachability, cross-family capability, binary freshness, ledger health,
// and the LAW-0 guard. Purely additive to the legacy doctor check table and
// serialized with the same quality.Check schema.
func wedgeDoctorChecks() []doctorCheck {
	checks, live := reviewerReachabilityChecks(wedgeReviewerTimeout)
	checks = append(checks,
		crossFamilyCheck(live),
		checkBinaryFreshness(),
		checkLedgerHealth(resolveLedgerPath()),
		checkLaw0Guard(os.Environ()),
	)
	return checks
}

// reviewerReachabilityChecks probes every reviewer family and returns the
// per-family checks plus the names of the families that answered live. An
// absent binary is a degraded warn (CI-safe: no invocation, no hang); a
// present-but-unresponsive binary fails after the hard timeout.
func reviewerReachabilityChecks(timeout time.Duration) ([]doctorCheck, []string) {
	checks := make([]doctorCheck, 0, len(wedgeReviewers))
	var live []string
	for _, r := range wedgeReviewers {
		c, ok := checkReviewerReachable(r, timeout)
		if ok {
			live = append(live, r.name)
		}
		checks = append(checks, c)
	}
	return checks, live
}

// checkReviewerReachable reports whether one reviewer CLI is on PATH and
// answers a trivial `--version` invocation within timeout. The invocation
// runs under exec.CommandContext so a hanging binary is killed at the
// deadline and reported unreachable instead of wedging doctor.
func checkReviewerReachable(r reviewerProbe, timeout time.Duration) (doctorCheck, bool) {
	name := "Reviewer: " + r.name
	if _, err := exec.LookPath(r.name); err != nil {
		return doctorCheck{
			Name:     name,
			Status:   "warn",
			Detail:   fmt.Sprintf("not found on PATH — install: %s", r.installCmd),
			Required: false,
		}, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	start := time.Now()
	err := exec.CommandContext(ctx, r.name, "--version").Run()
	elapsed := time.Since(start).Round(time.Millisecond)
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return doctorCheck{
			Name:     name,
			Status:   "fail",
			Detail:   fmt.Sprintf("unreachable: '%s --version' timed out after %s — check for a hung or unauthenticated binary: run '%s --version' manually", r.name, timeout, r.name),
			Required: false,
		}, false
	case err != nil:
		return doctorCheck{
			Name:     name,
			Status:   "fail",
			Detail:   fmt.Sprintf("'%s --version' failed (%v) — reinstall: %s", r.name, err, r.installCmd),
			Required: false,
		}, false
	}
	return doctorCheck{
		Name:     name,
		Status:   "pass",
		Detail:   fmt.Sprintf("reachable ('--version' answered in %s)", elapsed),
		Required: false,
	}, true
}

// crossFamilyCheck summarizes reviewer reachability into the one line agents
// and operators need: can a cross-family review be routed right now?
func crossFamilyCheck(live []string) doctorCheck {
	name := "Cross-Family Review"
	if len(live) > 0 {
		return doctorCheck{
			Name:     name,
			Status:   "pass",
			Detail:   fmt.Sprintf("cross-family capable: yes (live families: %s)", strings.Join(live, ", ")),
			Required: false,
		}
	}
	return doctorCheck{
		Name:     name,
		Status:   "warn",
		Detail:   "cross-family capable: no (no reviewer CLI reachable) — install one: npm install -g @openai/codex && codex login",
		Required: false,
	}
}

// agentopsModuleLine identifies THE agentops checkout, the same way session
// bootstrap does (sessionBootstrapIsAgentopsRepo).
const agentopsModuleLine = "module github.com/boshu2/agentops/cli"

// repoVersionRe extracts the fallback version literal from cli/cmd/ao/main.go
// (the var release builds override via -X main.version ldflags).
var repoVersionRe = regexp.MustCompile(`var version = "([^"]+)"`)

// checkBinaryFreshness compares the running binary's embedded version against
// the agentops repo's declared version when doctor runs inside the repo. This
// catches the stale-installed-binary trap (operator running a build hundreds
// of commits behind the checkout). Outside the repo it reports version only.
func checkBinaryFreshness() doctorCheck {
	cwd, err := os.Getwd()
	if err != nil {
		return doctorCheck{Name: "Binary Freshness", Status: "warn", Detail: "cannot determine working directory", Required: false}
	}
	return binaryFreshnessCheck(cwd, version)
}

// binaryFreshnessCheck is the testable core of checkBinaryFreshness: dir is
// where doctor runs, running is the embedded version of the live binary.
func binaryFreshnessCheck(dir, running string) doctorCheck {
	name := "Binary Freshness"
	root, ok := findAgentopsRepoRoot(dir)
	if !ok {
		return doctorCheck{
			Name:     name,
			Status:   "pass",
			Detail:   fmt.Sprintf("ao %s (outside the agentops repo — freshness not applicable)", running),
			Required: false,
		}
	}
	repoVer, ok := repoDeclaredVersion(root)
	if !ok {
		return doctorCheck{
			Name:     name,
			Status:   "warn",
			Detail:   "cannot read the repo's declared version (cli/cmd/ao/main.go) — rebuild to be safe: scripts/preflight-uat-binary.sh",
			Required: false,
		}
	}
	if repoVer == running {
		return doctorCheck{
			Name:     name,
			Status:   "pass",
			Detail:   fmt.Sprintf("ao %s matches the repo's declared version", running),
			Required: false,
		}
	}
	return doctorCheck{
		Name:     name,
		Status:   "warn",
		Detail:   fmt.Sprintf("running ao %s but the repo declares %s (stale installed binary) — rebuild+install: scripts/preflight-uat-binary.sh (or: brew upgrade agentops)", running, repoVer),
		Required: false,
	}
}

// findAgentopsRepoRoot walks up from dir looking for THE agentops checkout:
// a directory whose cli/go.mod declares the agentops module path.
func findAgentopsRepoRoot(dir string) (string, bool) {
	for i := 0; i < 12; i++ {
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

// repoDeclaredVersion parses the version literal out of the repo's
// cli/cmd/ao/main.go.
func repoDeclaredVersion(root string) (string, bool) {
	data, ok := readRegularFileCapped(filepath.Join(root, "cli", "cmd", "ao", "main.go"), 1<<16)
	if !ok {
		return "", false
	}
	m := repoVersionRe.FindSubmatch(data)
	if m == nil {
		return "", false
	}
	return string(m[1]), true
}

// checkLedgerHealth verifies the provenance ledger in place through the SAME
// code path as `ao provenance verify` (provenancegraph VerifyFile — never a
// reimplementation) and reports feed recency informationally.
func checkLedgerHealth(path string) doctorCheck {
	name := "Ledger Health"
	store := provenancegraph.NewStore(path)
	res, err := store.VerifyFile()
	if err != nil {
		return doctorCheck{
			Name:     name,
			Status:   "fail",
			Detail:   fmt.Sprintf("cannot read ledger at %s (%v) — check the path and permissions: ls -l %s", path, err, path),
			Required: true,
		}
	}
	if !res.Pass {
		return doctorCheck{
			Name:     name,
			Status:   "fail",
			Detail:   fmt.Sprintf("chain breaks at line %d: %s — inspect: ao provenance verify", res.FirstBrokenLine, res.Message),
			Required: true,
		}
	}
	if res.RecordCount == 0 {
		return doctorCheck{
			Name:     name,
			Status:   "pass",
			Detail:   "no ledger records yet (an empty or absent ledger is an intact chain)",
			Required: false,
		}
	}
	return doctorCheck{
		Name:     name,
		Status:   "pass",
		Detail:   fmt.Sprintf("chain intact (%d records%s)", res.RecordCount, ledgerRecencySuffix(store)),
		Required: false,
	}
}

// ledgerRecencySuffix returns ", newest record <age> old" for the final
// ledger record, or "" when the timestamp cannot be read — recency is
// informational only and never fails the check.
func ledgerRecencySuffix(store *provenancegraph.Store) string {
	edges, err := store.Read()
	if err != nil || len(edges) == 0 {
		return ""
	}
	ts, perr := time.Parse(time.RFC3339, edges[len(edges)-1].TS)
	if perr != nil {
		return ""
	}
	return fmt.Sprintf(", newest record %s old", formatDuration(time.Since(ts)))
}

// law0Needles are the forbidden reviewer invocations (LAW 0): no reviewer
// path may ever shell out to claude print-mode (the forbidden -p/--print invocations).
// Needles are assembled so this source never contains the contiguous forbidden
// pattern the door9 gate greps for.
var law0Needles = []string{"claude" + " -p", "claude" + " --print"}

// checkLaw0Guard scans reviewer-scoped environment variables (PAWL_*,
// AGENTOPS_*, AO_*, and any *REVIEWER* name) for a claude-p reviewer path.
// There is no reviewer config-file surface today — env vars (PAWL_NO_SERVICE
// and friends) are the only runtime reviewer knobs, so a clean env scan is a
// clean report.
func checkLaw0Guard(environ []string) doctorCheck {
	name := "LAW-0 Guard"
	for _, kv := range environ {
		k, v, ok := strings.Cut(kv, "=")
		if !ok || !law0RelevantEnv(k) {
			continue
		}
		for _, needle := range law0Needles {
			if strings.Contains(v, needle) {
				return doctorCheck{
					Name:     name,
					Status:   "fail",
					Detail:   fmt.Sprintf("LAW-0 violation: $%s routes a reviewer through %q — remove it: unset %s", k, needle, k),
					Required: true,
				}
			}
		}
	}
	return doctorCheck{
		Name:     name,
		Status:   "pass",
		Detail:   "no reviewer path configured through claude print-mode (scanned PAWL_*/AGENTOPS_*/AO_*/*REVIEWER* env)",
		Required: true,
	}
}

// law0RelevantEnv reports whether an environment variable name is a reviewer
// configuration surface the LAW-0 guard must scan.
func law0RelevantEnv(name string) bool {
	return strings.HasPrefix(name, "PAWL_") ||
		strings.HasPrefix(name, "AGENTOPS_") ||
		strings.HasPrefix(name, "AO_") ||
		strings.Contains(name, "REVIEWER")
}
