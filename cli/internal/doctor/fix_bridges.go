package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/openclaw"
)

// The bridges subsystem detects failures in the GasCity (`gc`) bridge and the
// OpenClaw consumer surface. Per the Phase 2 analysis, 5 of the 6 failure
// modes are detect-only — the doctor must never install binaries, start or
// kill processes, or rewrite activation files. The single partial fixer is
// fm-bridges-openclaw-snapshot-stale, whose torn-`latest.json` sub-case is
// safely reconstructible from an intact versioned snapshot.

// init registers every bridges detector and fixer with the package registry.
func init() {
	RegisterDetector(gcBinaryMissingDetector{})
	RegisterDetector(gcVersionIncompatibleDetector{})
	RegisterDetector(gcControllerDownDetector{})
	RegisterDetector(gcStatusParseErrorDetector{})
	RegisterDetector(openclawHealthUnreachableDetector{})
	RegisterDetector(openclawSnapshotStaleDetector{})

	RegisterFixer(gcBinaryMissingFixer{})
	RegisterFixer(gcVersionIncompatibleFixer{})
	RegisterFixer(gcControllerDownFixer{})
	RegisterFixer(gcStatusParseErrorFixer{})
	RegisterFixer(openclawHealthUnreachableFixer{})
	RegisterFixer(openclawSnapshotStaleFixer{})
}

const (
	subsystemBridges = "bridges"

	fmGCBinaryMissing           = "fm-bridges-gc-binary-missing"
	fmGCVersionIncompatible     = "fm-bridges-gc-version-incompatible"
	fmGCControllerDown          = "fm-bridges-gc-controller-down"
	fmGCStatusParseError        = "fm-bridges-gc-status-parse-error"
	fmOpenClawHealthUnreachable = "fm-bridges-openclaw-health-unreachable"
	fmOpenClawSnapshotStale     = "fm-bridges-openclaw-snapshot-stale"

	// gcBridgeMinVersion is the minimum gc version the bridge supports. Kept
	// as a literal here so the doctor package does not depend on cmd/ao.
	gcBridgeMinVersion = "0.13.0"
)

// ---------------------------------------------------------------------------
// Shared helpers (all pure / read-only).
// ---------------------------------------------------------------------------

// lookGC resolves the `gc` binary on the current process PATH. It is pure:
// exec.LookPath performs no disk writes.
func lookGC() (string, bool) {
	p, err := exec.LookPath("gc")
	if err != nil {
		return "", false
	}
	return p, true
}

// bridgeCityPath walks up from cwd looking for city.toml. It returns the
// directory containing city.toml, or "" if none is found. stat-only, pure.
func bridgeCityPath(cwd string) string {
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "city.toml")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// gcStatusArgs returns the argument vector for `gc status --json`, scoped to a
// city path when one is known.
func gcStatusArgs(cityPath string) []string {
	if strings.TrimSpace(cityPath) == "" {
		return []string{"status", "--json"}
	}
	return []string{"--city", cityPath, "status", "--json"}
}

// gcProbeResult is the outcome of a bounded `gc` subprocess probe.
type gcProbeResult struct {
	timedOut bool
	exitErr  bool
	stdout   []byte
	stderr   string
}

// runGCBounded runs `gc <args...>` under a hard wall-clock deadline so a wedged
// controller cannot hang `ao doctor`. It is read-only: `gc status`/`gc version`
// only print to stdout. The deadline is enforced via context cancellation.
func runGCBounded(args []string, deadline time.Duration) gcProbeResult {
	ctx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()
	cmd := exec.CommandContext(ctx, "gc", args...)
	var errBuf strings.Builder
	cmd.Stderr = &errBuf
	out, err := cmd.Output()
	if ctx.Err() == context.DeadlineExceeded {
		return gcProbeResult{timedOut: true, stderr: errBuf.String()}
	}
	if err != nil {
		return gcProbeResult{exitErr: true, stdout: out, stderr: errBuf.String()}
	}
	return gcProbeResult{stdout: out, stderr: errBuf.String()}
}

// gcVersionToken extracts the first semver-looking token from gc version output.
// Returns "" if none is present. Pure string work.
func gcVersionToken(raw string) string {
	for _, field := range strings.Fields(raw) {
		t := strings.TrimPrefix(strings.TrimSpace(field), "v")
		if t == "" {
			continue
		}
		first := t[0]
		if first >= '0' && first <= '9' && strings.Count(t, ".") >= 1 {
			// Trim any trailing non-version punctuation.
			return strings.TrimRight(t, ",;")
		}
	}
	return ""
}

// semverParts parses up to three dotted integer components. Non-numeric
// components coerce to 0 (matching the bridge's lenient parser).
func semverParts(v string) [3]int {
	var parts [3]int
	core := v
	if i := strings.IndexByte(core, '-'); i >= 0 {
		core = core[:i]
	}
	for i, seg := range strings.SplitN(core, ".", 3) {
		if i > 2 {
			break
		}
		n := 0
		ok := false
		for _, r := range seg {
			if r < '0' || r > '9' {
				ok = false
				break
			}
			n = n*10 + int(r-'0')
			ok = true
		}
		if ok {
			parts[i] = n
		}
	}
	return parts
}

// compareSemverParts returns -1, 0, 1 comparing two parsed semver triples.
func compareSemverParts(a, b [3]int) int {
	for i := 0; i < 3; i++ {
		switch {
		case a[i] < b[i]:
			return -1
		case a[i] > b[i]:
			return 1
		}
	}
	return 0
}

// hasNonNumeric reports whether v's core (pre-prerelease) has any non-numeric,
// non-dot rune — a marker of version-string format drift.
func hasNonNumeric(v string) bool {
	core := v
	if i := strings.IndexByte(core, '-'); i >= 0 {
		core = core[:i]
	}
	for _, r := range core {
		if (r < '0' || r > '9') && r != '.' {
			return true
		}
	}
	return false
}

// truncatePayload bounds an observed-payload string for evidence storage.
func truncatePayload(s string) string {
	const maxLen = 240
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// reportPath returns the run-dir-relative report file path for a finding id.
func reportPath(ctx *MutateContext, name string) string {
	return filepath.Join(ctx.RunDir, "reports", name)
}

// writeReport routes one advisory-report write through the Mutate chokepoint.
// It returns the number of actions taken (0 or 1).
func writeReport(ctx *MutateContext, name, content string) (int, error) {
	res, err := Mutate(ctx, reportPath(ctx, name), WriteFile{
		Content: []byte(content),
		Mode:    0o644,
	})
	if err != nil {
		return 0, err
	}
	if res.OK {
		return 1, nil
	}
	return 0, nil
}

// detectOnlyRemediation builds the standard detect-only remediation block.
func detectOnlyRemediation(id string) Remediation {
	return Remediation{
		Command:          "ao doctor --fix --only " + id,
		ExplainCommand:   "ao doctor explain " + id,
		AutoFixable:      false,
		EstimatedActions: 1,
	}
}

// ---------------------------------------------------------------------------
// FM 1: fm-bridges-gc-binary-missing — DETECT-ONLY.
// ---------------------------------------------------------------------------

// gcBinaryMissingDetector fires when the `gc` binary is not on PATH.
type gcBinaryMissingDetector struct{}

func (gcBinaryMissingDetector) ID() string        { return fmGCBinaryMissing }
func (gcBinaryMissingDetector) Subsystem() string { return subsystemBridges }
func (gcBinaryMissingDetector) Severity() string  { return "P2" }
func (gcBinaryMissingDetector) Describe() string {
	return "GasCity `gc` binary is not resolvable on the doctor's PATH"
}
func (gcBinaryMissingDetector) EstimatedCostMS() int { return 10 }
func (gcBinaryMissingDetector) OnlineRequired() bool { return false }
func (gcBinaryMissingDetector) QuickPath() bool      { return true }

// Detect resolves `gc` on PATH and, if missing, enriches the finding with the
// off-PATH install directories where a `gc` binary actually exists. PURE.
func (gcBinaryMissingDetector) Detect(env *DetectEnv) ([]Finding, error) {
	if _, ok := lookGC(); ok {
		return nil, nil
	}
	var offPath []string
	for _, rel := range []string{".local/bin/gc", "go/bin/gc", "bin/gc"} {
		cand := filepath.Join(env.HomeDir, rel)
		if isExecutableFile(cand) {
			offPath = append(offPath, cand)
		}
	}
	if isExecutableFile("/usr/local/bin/gc") {
		offPath = append(offPath, "/usr/local/bin/gc")
	}
	return []Finding{{
		ID:         fmGCBinaryMissing,
		Severity:   "P2",
		Subsystem:  subsystemBridges,
		Title:      "GasCity `gc` binary not found on PATH",
		Confidence: 1.0,
		Evidence: Evidence{
			Query: "command -v gc || echo MISSING",
			File:  strings.Join(offPath, ","),
		},
		Remediation: detectOnlyRemediation(fmGCBinaryMissing),
	}}, nil
}

// isExecutableFile reports whether path is a regular file with an executable
// bit set. stat-only, pure.
func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode().Perm()&0o111 != 0
}

// gcBinaryMissingFixer is detect-only: it never installs a binary or edits
// PATH. It refuses with a precise operator instruction routed through Mutate.
type gcBinaryMissingFixer struct{}

func (gcBinaryMissingFixer) ID() string              { return fmGCBinaryMissing }
func (gcBinaryMissingFixer) Preconditions() []string { return []string{"run_dir_writable"} }
func (gcBinaryMissingFixer) WritesTo() []string {
	return []string{".doctor/runs/<run-id>/reports"}
}
func (gcBinaryMissingFixer) Ops() []string     { return []string{"WriteFile"} }
func (gcBinaryMissingFixer) Reversible() bool  { return true }
func (gcBinaryMissingFixer) Idempotent() bool  { return true }
func (gcBinaryMissingFixer) AutoFixable() bool { return false }

// Fix re-runs the pure detector and writes a precise operator report. It
// installs nothing and edits no PATH. The finding legitimately persists.
func (gcBinaryMissingFixer) Fix(ctx *MutateContext, env *DetectEnv, _ []Finding) (FixResult, error) {
	fs, err := gcBinaryMissingDetector{}.Detect(env)
	if err != nil {
		return FixResult{FixerID: fmGCBinaryMissing, Err: err}, err
	}
	if len(fs) == 0 {
		return FixResult{FixerID: fmGCBinaryMissing, Fixed: true}, nil
	}
	offPath := fs[0].Evidence.File
	var report string
	if strings.TrimSpace(offPath) == "" {
		report = "GasCity `gc` binary not found. Install GasCity, then re-run " +
			"`ao doctor`. The doctor will not install third-party binaries."
	} else {
		first := strings.SplitN(offPath, ",", 2)[0]
		report = fmt.Sprintf("GasCity `gc` found at %s but it is not on the PATH "+
			"`ao` runs under. For interactive shells add that directory to PATH "+
			"in your shell rc. For systemd-user / agentopsd / hook contexts set "+
			"PATH in the unit's Environment= or the hook wrapper. The doctor will "+
			"not edit shell rc files.", first)
	}
	actions, err := writeReport(ctx, fmGCBinaryMissing+".txt", report)
	if err != nil {
		return FixResult{FixerID: fmGCBinaryMissing, ActionsTaken: actions, Err: err}, err
	}
	return FixResult{
		FixerID:      fmGCBinaryMissing,
		FindingIDs:   []string{fmGCBinaryMissing},
		ActionsTaken: actions,
		Fixed:        false,
	}, nil
}

// ---------------------------------------------------------------------------
// FM 2: fm-bridges-gc-version-incompatible — DETECT-ONLY.
// ---------------------------------------------------------------------------

// gcVersionObservation classifies how a `gc version` probe failed.
type gcVersionObservation struct {
	kind   string // "cmd_failed" | "unparseable" | "format_drift" | "genuinely_old"
	value  string
	rawOut string
}

// gcVersionIncompatibleDetector fires when an installed `gc` is below the
// bridge minimum, or its version output cannot be parsed.
type gcVersionIncompatibleDetector struct{}

func (gcVersionIncompatibleDetector) ID() string        { return fmGCVersionIncompatible }
func (gcVersionIncompatibleDetector) Subsystem() string { return subsystemBridges }
func (gcVersionIncompatibleDetector) Severity() string  { return "P2" }
func (gcVersionIncompatibleDetector) Describe() string {
	return "Installed `gc` is below the bridge minimum version or unparseable"
}
func (gcVersionIncompatibleDetector) EstimatedCostMS() int { return 80 }
func (gcVersionIncompatibleDetector) OnlineRequired() bool { return false }
func (gcVersionIncompatibleDetector) QuickPath() bool      { return false }

// classifyGCVersion runs `gc version` and classifies the result. Pure.
func classifyGCVersion() (gcVersionObservation, bool) {
	probe := runGCBounded([]string{"version"}, 4*time.Second)
	if probe.timedOut || probe.exitErr {
		return gcVersionObservation{kind: "cmd_failed", rawOut: truncatePayload(string(probe.stdout) + probe.stderr)}, true
	}
	raw := strings.TrimSpace(string(probe.stdout))
	token := gcVersionToken(raw)
	if token == "" {
		return gcVersionObservation{kind: "unparseable", rawOut: truncatePayload(raw)}, true
	}
	parts := semverParts(token)
	if compareSemverParts(parts, semverParts(gcBridgeMinVersion)) >= 0 {
		return gcVersionObservation{}, false // compatible
	}
	if parts == [3]int{0, 0, 0} && token != "0.0.0" && hasNonNumeric(token) {
		return gcVersionObservation{kind: "format_drift", value: token, rawOut: truncatePayload(raw)}, true
	}
	return gcVersionObservation{kind: "genuinely_old", value: token, rawOut: truncatePayload(raw)}, true
}

// Detect resolves `gc`, runs `gc version`, and classifies any incompatibility.
// It early-returns nil when `gc` is missing (that is fm-bridges-gc-binary-missing).
// PURE.
func (gcVersionIncompatibleDetector) Detect(_ *DetectEnv) ([]Finding, error) {
	if _, ok := lookGC(); !ok {
		return nil, nil // upstream precedence: binary-missing handles this
	}
	obs, found := classifyGCVersion()
	if !found {
		return nil, nil
	}
	return []Finding{{
		ID:         fmGCVersionIncompatible,
		Severity:   "P2",
		Subsystem:  subsystemBridges,
		Title:      "GasCity `gc` version incompatible (" + obs.kind + ")",
		Confidence: 1.0,
		Evidence: Evidence{
			Query: "gc version  # compare against " + gcBridgeMinVersion + " floor",
			File:  obs.kind + ":" + obs.value,
		},
		Remediation: detectOnlyRemediation(fmGCVersionIncompatible),
	}}, nil
}

// gcVersionIncompatibleFixer is detect-only: it never upgrades `gc` or patches
// ao source. It writes a precise per-sub-case operator report.
type gcVersionIncompatibleFixer struct{}

func (gcVersionIncompatibleFixer) ID() string { return fmGCVersionIncompatible }
func (gcVersionIncompatibleFixer) Preconditions() []string {
	return []string{"gc_on_path", "run_dir_writable"}
}
func (gcVersionIncompatibleFixer) WritesTo() []string {
	return []string{".doctor/runs/<run-id>/reports"}
}
func (gcVersionIncompatibleFixer) Ops() []string     { return []string{"WriteFile"} }
func (gcVersionIncompatibleFixer) Reversible() bool  { return true }
func (gcVersionIncompatibleFixer) Idempotent() bool  { return true }
func (gcVersionIncompatibleFixer) AutoFixable() bool { return false }

// Fix re-runs the detector, disambiguates the sub-case, and reports. No binary
// upgrade, no ao source edit. The finding legitimately persists.
func (gcVersionIncompatibleFixer) Fix(ctx *MutateContext, _ *DetectEnv, _ []Finding) (FixResult, error) {
	if _, ok := lookGC(); !ok {
		return FixResult{FixerID: fmGCVersionIncompatible, Fixed: true}, nil
	}
	obs, found := classifyGCVersion()
	if !found {
		return FixResult{FixerID: fmGCVersionIncompatible, Fixed: true}, nil
	}
	report := gcVersionReport(obs)
	actions, err := writeReport(ctx, fmGCVersionIncompatible+".txt", report)
	if err != nil {
		return FixResult{FixerID: fmGCVersionIncompatible, ActionsTaken: actions, Err: err}, err
	}
	return FixResult{
		FixerID:      fmGCVersionIncompatible,
		FindingIDs:   []string{fmGCVersionIncompatible},
		ActionsTaken: actions,
		Fixed:        false,
	}, nil
}

// gcVersionReport builds the operator instruction for a version observation.
func gcVersionReport(obs gcVersionObservation) string {
	switch obs.kind {
	case "genuinely_old":
		return fmt.Sprintf("Installed `gc` is %s, below the bridge minimum %s. "+
			"Upgrade GasCity to >= %s, then re-run `ao doctor`. Verify with "+
			"`gc version`. The doctor will not upgrade third-party binaries.",
			obs.value, gcBridgeMinVersion, gcBridgeMinVersion)
	case "format_drift":
		return fmt.Sprintf("`gc version` printed %q; ao parsed it as %s and treated "+
			"it as 0.0.0 (parse coercion). `gc` may actually be new enough. Confirm "+
			"the real version with `gc version`. If it is >= %s this is an ao "+
			"version-parser drift — file a bd issue against the bridges subsystem; "+
			"the doctor will not patch ao source.", obs.rawOut, obs.value, gcBridgeMinVersion)
	case "unparseable":
		return fmt.Sprintf("`gc version` output %q contains no recognizable semver "+
			"token. Confirm `gc version` works in your shell. If gc prints a "+
			"banner/JSON now, this is an ao parser drift — file a bd issue; the "+
			"doctor will not patch ao.", obs.rawOut)
	default: // cmd_failed
		return fmt.Sprintf("`gc version` exited non-zero or hung: %q. The gc binary "+
			"is on PATH but broken. Reinstall/repair GasCity, then re-run "+
			"`ao doctor`.", obs.rawOut)
	}
}

// ---------------------------------------------------------------------------
// FM 3: fm-bridges-gc-controller-down — DETECT-ONLY (bounded probe).
// ---------------------------------------------------------------------------

// gcStatusProbe runs a bounded `gc status --json` and returns the raw result.
// It is the shared probe for the controller-down and status-parse-error
// detectors. The 4s deadline prevents a wedged controller from hanging
// `ao doctor`.
func gcStatusProbe(env *DetectEnv) gcProbeResult {
	city := bridgeCityPath(env.CWD)
	return runGCBounded(gcStatusArgs(city), 4*time.Second)
}

// gcStatusControllerRunning reports whether a parsed `gc status` JSON shows the
// controller as running. Pure JSON inspection.
func gcStatusControllerRunning(stdout []byte) (running, parsed bool) {
	var top map[string]json.RawMessage
	if json.Unmarshal(stdout, &top) != nil {
		return false, false
	}
	ctrlRaw, ok := top["controller"]
	if !ok {
		return false, false
	}
	var ctrl struct {
		Running bool `json:"running"`
	}
	if json.Unmarshal(ctrlRaw, &ctrl) != nil {
		return false, false
	}
	// A valid status payload also carries agents + summary.
	_, hasAgents := top["agents"]
	_, hasSummary := top["summary"]
	if !hasAgents || !hasSummary {
		return false, false
	}
	return ctrl.Running, true
}

// gcControllerDownDetector fires when the GasCity controller is not running,
// the gc API is unavailable, or the status probe wedged.
type gcControllerDownDetector struct{}

func (gcControllerDownDetector) ID() string        { return fmGCControllerDown }
func (gcControllerDownDetector) Subsystem() string { return subsystemBridges }
func (gcControllerDownDetector) Severity() string  { return "P1" }
func (gcControllerDownDetector) Describe() string {
	return "GasCity controller is not running, unreachable, or wedged"
}
func (gcControllerDownDetector) EstimatedCostMS() int { return 4000 }
func (gcControllerDownDetector) OnlineRequired() bool { return false }
func (gcControllerDownDetector) QuickPath() bool      { return false }

// Detect probes `gc status` under a bounded 4s deadline. It honors the GasCity
// precedence chain: it early-returns nil when `gc` is missing or below the
// minimum version, and nil when status parses (parse errors are a downstream
// FM). PURE.
func (gcControllerDownDetector) Detect(env *DetectEnv) ([]Finding, error) {
	if _, ok := lookGC(); !ok {
		return nil, nil // upstream: binary-missing
	}
	if _, found := classifyGCVersion(); found {
		return nil, nil // upstream: version-incompatible
	}
	probe := gcStatusProbe(env)
	var kind, detail string
	switch {
	case probe.timedOut:
		kind, detail = "controller_wedged", "gc status did not respond within 4s"
	case probe.exitErr:
		kind, detail = "api_unavailable", truncatePayload(probe.stderr)
	default:
		running, parsed := gcStatusControllerRunning(probe.stdout)
		if !parsed {
			return nil, nil // downstream: status-parse-error
		}
		if running {
			return nil, nil // healthy
		}
		kind, detail = "controller_not_running", "controller.running == false"
	}
	return []Finding{{
		ID:         fmGCControllerDown,
		Severity:   "P1",
		Subsystem:  subsystemBridges,
		Title:      "GasCity controller down (" + kind + ")",
		Confidence: 1.0,
		Evidence: Evidence{
			Query: "gc status --json | jq .controller.running   # expect true",
			File:  kind + ":" + detail,
		},
		Remediation: detectOnlyRemediation(fmGCControllerDown),
	}}, nil
}

// gcControllerDownFixer is detect-only: it never starts, stops, or signals the
// controller. It writes a precise per-sub-case operator report.
type gcControllerDownFixer struct{}

func (gcControllerDownFixer) ID() string { return fmGCControllerDown }
func (gcControllerDownFixer) Preconditions() []string {
	return []string{"gc_on_path", "run_dir_writable"}
}
func (gcControllerDownFixer) WritesTo() []string {
	return []string{".doctor/runs/<run-id>/reports"}
}
func (gcControllerDownFixer) Ops() []string     { return []string{"WriteFile"} }
func (gcControllerDownFixer) Reversible() bool  { return true }
func (gcControllerDownFixer) Idempotent() bool  { return true }
func (gcControllerDownFixer) AutoFixable() bool { return false }

// Fix re-runs the bounded detector and reports the exact `gc` command to bring
// the controller up. It starts no process. The finding legitimately persists.
func (gcControllerDownFixer) Fix(ctx *MutateContext, env *DetectEnv, _ []Finding) (FixResult, error) {
	fs, err := gcControllerDownDetector{}.Detect(env)
	if err != nil {
		return FixResult{FixerID: fmGCControllerDown, Err: err}, err
	}
	if len(fs) == 0 {
		return FixResult{FixerID: fmGCControllerDown, Fixed: true}, nil
	}
	kind := strings.SplitN(fs[0].Evidence.File, ":", 2)[0]
	cityFlag := ""
	if city := bridgeCityPath(env.CWD); city != "" {
		cityFlag = "--city " + city + " "
	}
	report := gcControllerReport(kind, cityFlag)
	actions, err := writeReport(ctx, fmGCControllerDown+".txt", report)
	if err != nil {
		return FixResult{FixerID: fmGCControllerDown, ActionsTaken: actions, Err: err}, err
	}
	return FixResult{
		FixerID:      fmGCControllerDown,
		FindingIDs:   []string{fmGCControllerDown},
		ActionsTaken: actions,
		Fixed:        false,
	}, nil
}

// gcControllerReport builds the operator instruction for a controller-down kind.
func gcControllerReport(kind, cityFlag string) string {
	switch kind {
	case "controller_not_running":
		return fmt.Sprintf("GasCity controller is registered but not running. "+
			"Start it with `gc %scontroller start` (or the city's supervisor: "+
			"`gc %sup`), then re-run `ao doctor`. The doctor will not start "+
			"runtime processes.", cityFlag, cityFlag)
	case "api_unavailable":
		return fmt.Sprintf("`gc %sstatus` failed: the controller process / socket "+
			"is gone. Bring the city daemon up with `gc %sup`, confirm with "+
			"`gc %sstatus --json`, then re-run `ao doctor`. The doctor will not "+
			"start runtime processes.", cityFlag, cityFlag, cityFlag)
	default: // controller_wedged
		return fmt.Sprintf("`gc %sstatus` did not respond within 4s — the "+
			"controller is wedged. Inspect with `gc %sstatus` directly; you may "+
			"need to stop and restart the city supervisor. Reconcile any lingering "+
			"`agentworker-gascity` processes. The doctor will not kill or restart "+
			"processes.", cityFlag, cityFlag)
	}
}

// ---------------------------------------------------------------------------
// FM 4: fm-bridges-gc-status-parse-error — DETECT-ONLY.
// ---------------------------------------------------------------------------

// gcStatusParseErrorDetector fires when `gc status --json` returns a payload
// the bridge parser cannot consume.
type gcStatusParseErrorDetector struct{}

func (gcStatusParseErrorDetector) ID() string        { return fmGCStatusParseError }
func (gcStatusParseErrorDetector) Subsystem() string { return subsystemBridges }
func (gcStatusParseErrorDetector) Severity() string  { return "P2" }
func (gcStatusParseErrorDetector) Describe() string {
	return "`gc status --json` output drifted from the bridge schema"
}
func (gcStatusParseErrorDetector) EstimatedCostMS() int { return 4000 }
func (gcStatusParseErrorDetector) OnlineRequired() bool { return false }
func (gcStatusParseErrorDetector) QuickPath() bool      { return false }

// classifyGCStatusDrift inspects an unparseable `gc status` payload and names
// the drift class. Pure JSON inspection.
func classifyGCStatusDrift(stdout []byte) (kind string, missing []string) {
	var top map[string]json.RawMessage
	if json.Unmarshal(stdout, &top) != nil {
		return "not_json", nil
	}
	for _, key := range []string{"controller", "agents", "summary"} {
		raw, ok := top[key]
		if !ok || string(raw) == "null" {
			missing = append(missing, key)
		}
	}
	sort.Strings(missing)
	if len(missing) == 0 {
		return "nested_shape_drift", nil
	}
	if _, ok := top["data"]; ok {
		return "wrapped_envelope", missing
	}
	if _, ok := top["status"]; ok {
		return "wrapped_envelope", missing
	}
	if _, ok := top["result"]; ok {
		return "wrapped_envelope", missing
	}
	return "missing_top_level_fields", missing
}

// Detect probes `gc status` and classifies a parse failure. It honors the
// precedence chain: nil when `gc` is missing, version-incompatible, the probe
// failed/wedged (controller-down), or the payload parses. PURE.
func (gcStatusParseErrorDetector) Detect(env *DetectEnv) ([]Finding, error) {
	if _, ok := lookGC(); !ok {
		return nil, nil // upstream: binary-missing
	}
	if _, found := classifyGCVersion(); found {
		return nil, nil // upstream: version-incompatible
	}
	probe := gcStatusProbe(env)
	if probe.timedOut || probe.exitErr {
		return nil, nil // upstream: controller-down
	}
	if _, parsed := gcStatusControllerRunning(probe.stdout); parsed {
		return nil, nil // parses fine — not this FM
	}
	kind, missing := classifyGCStatusDrift(probe.stdout)
	return []Finding{{
		ID:         fmGCStatusParseError,
		Severity:   "P2",
		Subsystem:  subsystemBridges,
		Title:      "`gc status --json` schema drift (" + kind + ")",
		Confidence: 1.0,
		Evidence: Evidence{
			Query: "gc status --json | jq 'has(\"controller\") and has(\"agents\") and has(\"summary\")'",
			File:  kind + ":" + strings.Join(missing, ","),
		},
		Remediation: detectOnlyRemediation(fmGCStatusParseError),
	}}, nil
}

// gcStatusParseErrorFixer is detect-only: it cannot repair GasCity's transient
// stdout. It captures the offending payload and writes an operator report —
// two advisory writes, both through Mutate.
type gcStatusParseErrorFixer struct{}

func (gcStatusParseErrorFixer) ID() string { return fmGCStatusParseError }
func (gcStatusParseErrorFixer) Preconditions() []string {
	return []string{"gc_on_path", "run_dir_writable"}
}
func (gcStatusParseErrorFixer) WritesTo() []string {
	return []string{".doctor/runs/<run-id>/reports"}
}
func (gcStatusParseErrorFixer) Ops() []string     { return []string{"WriteFile"} }
func (gcStatusParseErrorFixer) Reversible() bool  { return true }
func (gcStatusParseErrorFixer) Idempotent() bool  { return true }
func (gcStatusParseErrorFixer) AutoFixable() bool { return false }

// Fix re-probes `gc status`, captures the offending payload verbatim, and
// writes an operator report. No ao source edit. The finding legitimately
// persists.
func (gcStatusParseErrorFixer) Fix(ctx *MutateContext, env *DetectEnv, _ []Finding) (FixResult, error) {
	fs, err := gcStatusParseErrorDetector{}.Detect(env)
	if err != nil {
		return FixResult{FixerID: fmGCStatusParseError, Err: err}, err
	}
	if len(fs) == 0 {
		return FixResult{FixerID: fmGCStatusParseError, Fixed: true}, nil
	}
	kind := strings.SplitN(fs[0].Evidence.File, ":", 2)[0]
	missing := ""
	if parts := strings.SplitN(fs[0].Evidence.File, ":", 2); len(parts) == 2 {
		missing = parts[1]
	}
	// Re-probe once for a fresh verbatim payload capture.
	payload := gcStatusProbe(env).stdout
	actions := 0
	n, err := writeReport(ctx, fmGCStatusParseError+".payload.json", string(payload))
	actions += n
	if err != nil {
		return FixResult{FixerID: fmGCStatusParseError, ActionsTaken: actions, Err: err}, err
	}
	n, err = writeReport(ctx, fmGCStatusParseError+".txt", gcStatusParseReport(kind, missing))
	actions += n
	if err != nil {
		return FixResult{FixerID: fmGCStatusParseError, ActionsTaken: actions, Err: err}, err
	}
	return FixResult{
		FixerID:      fmGCStatusParseError,
		FindingIDs:   []string{fmGCStatusParseError},
		ActionsTaken: actions,
		Fixed:        false,
	}, nil
}

// gcStatusParseReport builds the operator instruction for a status-drift class.
func gcStatusParseReport(kind, missing string) string {
	switch kind {
	case "missing_top_level_fields":
		return fmt.Sprintf("`gc status --json` is missing required top-level "+
			"field(s): [%s]. GasCity's status schema drifted. Confirm with "+
			"`gc status --json | jq 'keys'`. This is an upstream contract drift "+
			"between GasCity and the ao bridge — file a bd issue against the "+
			"bridges subsystem and pin/downgrade `gc` to a known-compatible "+
			"version. The doctor will not patch ao source.", missing)
	case "wrapped_envelope":
		return fmt.Sprintf("`gc status --json` now wraps the status in an "+
			"envelope; top-level field(s) [%s] are not where the bridge expects "+
			"them. The bridge parser needs an unwrap shim. File a bd issue against "+
			"bridges; pin `gc` to a compatible version meanwhile.", missing)
	case "nested_shape_drift":
		return "`gc status --json` top-level keys are present but a nested shape " +
			"changed — a JSON shape the bridge's UnmarshalJSON shims do not cover. " +
			"File a bd issue against bridges; pin `gc` to a compatible version " +
			"meanwhile."
	default: // not_json
		return "`gc status --json` did not return valid JSON. The gc binary may " +
			"be printing a banner/error on stdout. Confirm `gc status --json` " +
			"output directly; reinstall/repair gc if it is malfunctioning."
	}
}

// ---------------------------------------------------------------------------
// FM 5: fm-bridges-openclaw-health-unreachable — DETECT-ONLY (online probe).
// ---------------------------------------------------------------------------

// daemonActivation is the subset of .agents/daemon/activation.json the bridge
// health probe needs.
type daemonActivation struct {
	BaseURL string `json:"base_url"`
}

// readDaemonActivation reads the per-project daemon activation file. Pure.
func readDaemonActivation(repoRoot string) (daemonActivation, bool) {
	path := filepath.Join(repoRoot, ".agents", "daemon", "activation.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return daemonActivation{}, false
	}
	var act daemonActivation
	if json.Unmarshal(data, &act) != nil {
		return daemonActivation{}, false
	}
	return act, true
}

// httpGetBounded performs a read-only HTTP GET under a hard wall-clock
// deadline. It returns the response body, HTTP status, and any transport
// error. A wedged endpoint cannot hang `ao doctor` past the deadline.
func httpGetBounded(url string, deadline time.Duration) ([]byte, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	client := &http.Client{Timeout: deadline}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

// openclawHealthUnreachableDetector fires when the OpenClaw consumer health
// endpoint cannot be confirmed healthy.
type openclawHealthUnreachableDetector struct{}

func (openclawHealthUnreachableDetector) ID() string        { return fmOpenClawHealthUnreachable }
func (openclawHealthUnreachableDetector) Subsystem() string { return subsystemBridges }
func (openclawHealthUnreachableDetector) Severity() string  { return "P2" }
func (openclawHealthUnreachableDetector) Describe() string {
	return "OpenClaw consumer health endpoint is unreachable or not ok"
}
func (openclawHealthUnreachableDetector) EstimatedCostMS() int { return 3000 }
func (openclawHealthUnreachableDetector) OnlineRequired() bool { return true }
func (openclawHealthUnreachableDetector) QuickPath() bool      { return false }

// Detect reads the activation file and probes <base_url>/openclaw/v1/health
// under a bounded 3s deadline. The probe is read-only (HTTP GET). PURE.
func (openclawHealthUnreachableDetector) Detect(env *DetectEnv) ([]Finding, error) {
	act, ok := readDaemonActivation(env.RepoRoot)
	if !ok {
		return []Finding{healthFinding("activation_missing", "")}, nil
	}
	kind, detail := probeOpenClawHealth(act.BaseURL)
	if kind == "" {
		return nil, nil // healthy
	}
	return []Finding{healthFinding(kind, detail)}, nil
}

// healthFinding builds the openclaw-health finding for a given sub-case.
func healthFinding(kind, detail string) Finding {
	return Finding{
		ID:         fmOpenClawHealthUnreachable,
		Severity:   "P2",
		Subsystem:  subsystemBridges,
		Title:      "OpenClaw consumer health unreachable (" + kind + ")",
		Confidence: 1.0,
		Evidence: Evidence{
			Query: "curl -fsS --max-time 2 \"$DAEMON_URL/openclaw/v1/health\"",
			File:  kind + ":" + detail,
		},
		Remediation: detectOnlyRemediation(fmOpenClawHealthUnreachable),
	}
}

// probeOpenClawHealth performs a bounded read-only GET against the health
// endpoint and classifies the result. Returns ("", "") when healthy.
func probeOpenClawHealth(baseURL string) (kind, detail string) {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		return "unreachable", "empty base_url in activation.json"
	}
	url := base + "/openclaw/v1/health"
	body, status, err := httpGetBounded(url, 3*time.Second)
	switch {
	case err != nil:
		if strings.Contains(strings.ToLower(err.Error()), "timeout") ||
			strings.Contains(strings.ToLower(err.Error()), "deadline") {
			return "slow_or_hung", base
		}
		return "unreachable", base + " (" + truncatePayload(err.Error()) + ")"
	case status == 404:
		return "route_missing", base
	case status/100 != 2:
		return "http_error", fmt.Sprintf("%s HTTP %d", base, status)
	default:
		var h struct {
			Status string `json:"status"`
		}
		if json.Unmarshal(body, &h) == nil && h.Status == "ok" {
			return "", ""
		}
		return "not_ok", base + " " + truncatePayload(string(body))
	}
}

// openclawHealthUnreachableFixer is detect-only: it never starts/restarts
// agentopsd and never rewrites the activation file.
type openclawHealthUnreachableFixer struct{}

func (openclawHealthUnreachableFixer) ID() string { return fmOpenClawHealthUnreachable }
func (openclawHealthUnreachableFixer) Preconditions() []string {
	return []string{"online_required", "run_dir_writable"}
}
func (openclawHealthUnreachableFixer) WritesTo() []string {
	return []string{".doctor/runs/<run-id>/reports"}
}
func (openclawHealthUnreachableFixer) Ops() []string     { return []string{"WriteFile"} }
func (openclawHealthUnreachableFixer) Reversible() bool  { return true }
func (openclawHealthUnreachableFixer) Idempotent() bool  { return true }
func (openclawHealthUnreachableFixer) AutoFixable() bool { return false }

// Fix re-runs the detector and reports the exact `ao agentopsd` action. It
// starts nothing and rewrites no activation file. The finding persists.
func (openclawHealthUnreachableFixer) Fix(ctx *MutateContext, env *DetectEnv, _ []Finding) (FixResult, error) {
	fs, err := openclawHealthUnreachableDetector{}.Detect(env)
	if err != nil {
		return FixResult{FixerID: fmOpenClawHealthUnreachable, Err: err}, err
	}
	if len(fs) == 0 {
		return FixResult{FixerID: fmOpenClawHealthUnreachable, Fixed: true}, nil
	}
	parts := strings.SplitN(fs[0].Evidence.File, ":", 2)
	kind := parts[0]
	detail := ""
	if len(parts) == 2 {
		detail = parts[1]
	}
	report := openclawHealthReport(kind, detail)
	actions, err := writeReport(ctx, fmOpenClawHealthUnreachable+".txt", report)
	if err != nil {
		return FixResult{FixerID: fmOpenClawHealthUnreachable, ActionsTaken: actions, Err: err}, err
	}
	return FixResult{
		FixerID:      fmOpenClawHealthUnreachable,
		FindingIDs:   []string{fmOpenClawHealthUnreachable},
		ActionsTaken: actions,
		Fixed:        false,
	}, nil
}

// openclawHealthReport builds the operator instruction for a health sub-case.
func openclawHealthReport(kind, detail string) string {
	switch kind {
	case "activation_missing":
		return "agentopsd was never started for this project: " +
			"`.agents/daemon/activation.json` is absent. Start the daemon with " +
			"`ao agentopsd start`, then re-run `ao doctor`. The doctor will not " +
			"start the daemon."
	case "unreachable":
		return fmt.Sprintf("OpenClaw health endpoint %s is unreachable. The daemon "+
			"is down or the activation file points at a dead port. Run "+
			"`ao agentopsd status`; if dead, `ao agentopsd restart`; if the "+
			"activation file is stale, `ao agentopsd start` rewrites it. The "+
			"doctor will not restart the daemon or rewrite the activation file.",
			detail)
	case "slow_or_hung":
		return fmt.Sprintf("OpenClaw health endpoint %s did not respond within 3s. "+
			"The daemon is slow or wedged. Inspect with `ao agentopsd status` and "+
			"the daemon log; restart with `ao agentopsd restart` if wedged.", detail)
	case "route_missing":
		return fmt.Sprintf("The daemon at %s answers but `/openclaw/v1/health` "+
			"returns 404 — this agentopsd build predates the OpenClaw consumer "+
			"routes. Upgrade `ao`/agentopsd and restart the daemon "+
			"(`ao agentopsd restart`).", detail)
	case "http_error":
		return fmt.Sprintf("OpenClaw health endpoint %s returned an HTTP error. "+
			"Inspect the daemon log; restart with `ao agentopsd restart` if it "+
			"crashed.", detail)
	default: // not_ok
		return fmt.Sprintf("OpenClaw health endpoint %s responded but status != ok. "+
			"Inspect the daemon log and restart with `ao agentopsd restart` if "+
			"needed.", detail)
	}
}

// ---------------------------------------------------------------------------
// FM 6: fm-bridges-openclaw-snapshot-stale — PARTIAL (torn-latest auto-fix).
// ---------------------------------------------------------------------------

// snapshotObservation classifies the OpenClaw consumer snapshot state.
type snapshotObservation struct {
	kind            string // "file_ok" | "latest_missing" | "schema_mismatch" | "torn_latest" | "torn_no_recovery"
	schemaVersion   int
	goodVersionFile string // basename of the recovery snap_*.json (torn_latest only)
}

// openclawSnapshotStaleDetector fires when the OpenClaw consumer snapshot is
// torn, schema-mismatched, or absent.
type openclawSnapshotStaleDetector struct{}

func (openclawSnapshotStaleDetector) ID() string        { return fmOpenClawSnapshotStale }
func (openclawSnapshotStaleDetector) Subsystem() string { return subsystemBridges }
func (openclawSnapshotStaleDetector) Severity() string  { return "P2" }
func (openclawSnapshotStaleDetector) Describe() string {
	return "OpenClaw consumer snapshot is stale, torn, or schema-mismatched"
}
func (openclawSnapshotStaleDetector) EstimatedCostMS() int { return 30 }
func (openclawSnapshotStaleDetector) OnlineRequired() bool { return false }
func (openclawSnapshotStaleDetector) QuickPath() bool      { return false }

// observeSnapshot inspects the on-disk OpenClaw snapshot directory. Pure: it
// only reads files and parses JSON.
func observeSnapshot(repoRoot string) snapshotObservation {
	snapDir := filepath.Join(repoRoot, openclaw.SnapshotDirRel)
	latestPath := filepath.Join(snapDir, "latest.json")
	raw, err := os.ReadFile(latestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return snapshotObservation{kind: "latest_missing"}
		}
		return snapshotObservation{kind: "latest_missing"}
	}
	snap, parseErr := openclaw.ParseConsumerSnapshot(raw)
	if parseErr == nil {
		return snapshotObservation{kind: "file_ok", schemaVersion: snap.SchemaVersion}
	}
	// A parseable-but-wrong-version file: classify as schema mismatch.
	if v, ok := schemaVersionOf(raw); ok && v != openclaw.ConsumerSnapshotSchemaVersion {
		return snapshotObservation{kind: "schema_mismatch", schemaVersion: v}
	}
	// Torn/truncated/empty: look for a recoverable versioned sibling.
	good := newestValidVersionSnapshot(snapDir)
	if good == "" {
		return snapshotObservation{kind: "torn_no_recovery"}
	}
	return snapshotObservation{kind: "torn_latest", goodVersionFile: good}
}

// schemaVersionOf extracts the schema_version field from raw JSON without full
// validation. Pure.
func schemaVersionOf(raw []byte) (int, bool) {
	var probe struct {
		SchemaVersion *int `json:"schema_version"`
	}
	if json.Unmarshal(raw, &probe) != nil || probe.SchemaVersion == nil {
		return 0, false
	}
	return *probe.SchemaVersion, true
}

// newestValidVersionSnapshot scans snapDir for snap_*.json files that parse
// cleanly as schema-v1 snapshots and returns the basename of the newest by
// generated_at. Returns "" when none is recoverable. Pure.
func newestValidVersionSnapshot(snapDir string) string {
	entries, err := os.ReadDir(snapDir)
	if err != nil {
		return ""
	}
	var bestName string
	var bestTime time.Time
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasPrefix(name, "snap_") || !strings.HasSuffix(name, ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(snapDir, name))
		if err != nil {
			continue
		}
		snap, err := openclaw.ParseConsumerSnapshot(raw)
		if err != nil || snap.SchemaVersion != openclaw.ConsumerSnapshotSchemaVersion {
			continue
		}
		gen, err := time.Parse(time.RFC3339Nano, snap.GeneratedAt)
		if err != nil {
			continue
		}
		if bestName == "" || gen.After(bestTime) {
			bestName, bestTime = name, gen
		}
	}
	return bestName
}

// Detect inspects the on-disk OpenClaw snapshot and emits a finding when it is
// not healthy. auto_fixable is true ONLY for the torn-latest sub-case. PURE.
func (openclawSnapshotStaleDetector) Detect(env *DetectEnv) ([]Finding, error) {
	obs := observeSnapshot(env.RepoRoot)
	if obs.kind == "file_ok" {
		return nil, nil
	}
	autoFixable := obs.kind == "torn_latest"
	rem := Remediation{
		Command:          "ao doctor --fix --only " + fmOpenClawSnapshotStale,
		ExplainCommand:   "ao doctor explain " + fmOpenClawSnapshotStale,
		AutoFixable:      autoFixable,
		EstimatedActions: 1,
	}
	return []Finding{{
		ID:         fmOpenClawSnapshotStale,
		Severity:   "P2",
		Subsystem:  subsystemBridges,
		Title:      "OpenClaw consumer snapshot not current (" + obs.kind + ")",
		Confidence: 1.0,
		Evidence: Evidence{
			Query: "jq -e '.schema_version==1' .agents/daemon/projections/openclaw/latest.json",
			File:  obs.kind + ":" + obs.goodVersionFile,
		},
		Remediation: rem,
	}}, nil
}

// openclawSnapshotStaleFixer is PARTIAL: only the torn-latest sub-case is
// auto-fixed (by reconstructing latest.json verbatim from the intact versioned
// snapshot). All other sub-cases are detect-only.
type openclawSnapshotStaleFixer struct{}

func (openclawSnapshotStaleFixer) ID() string { return fmOpenClawSnapshotStale }
func (openclawSnapshotStaleFixer) Preconditions() []string {
	return []string{"recoverable_versioned_snapshot", "latest_json_unlocked"}
}
func (openclawSnapshotStaleFixer) WritesTo() []string {
	return []string{openclaw.SnapshotDirRel, ".doctor/runs/<run-id>/reports"}
}
func (openclawSnapshotStaleFixer) Ops() []string     { return []string{"WriteFile"} }
func (openclawSnapshotStaleFixer) Reversible() bool  { return true }
func (openclawSnapshotStaleFixer) Idempotent() bool  { return true }
func (openclawSnapshotStaleFixer) AutoFixable() bool { return true }

// Fix reconstructs a torn latest.json from the newest intact versioned
// snapshot via a single Mutate WriteFile. For every other sub-case it refuses
// and writes a detect-only advisory report through Mutate.
func (openclawSnapshotStaleFixer) Fix(ctx *MutateContext, env *DetectEnv, _ []Finding) (FixResult, error) {
	obs := observeSnapshot(env.RepoRoot)
	if obs.kind == "file_ok" {
		return FixResult{FixerID: fmOpenClawSnapshotStale, Fixed: true}, nil
	}
	if obs.kind == "torn_latest" {
		return fixTornLatest(ctx, env, obs)
	}
	// Detect-only sub-cases: write an advisory report, do not bail.
	report := snapshotStaleReport(obs)
	actions, err := writeReport(ctx, fmOpenClawSnapshotStale+".txt", report)
	if err != nil {
		return FixResult{FixerID: fmOpenClawSnapshotStale, ActionsTaken: actions, Err: err}, err
	}
	return FixResult{
		FixerID:      fmOpenClawSnapshotStale,
		FindingIDs:   []string{fmOpenClawSnapshotStale},
		ActionsTaken: actions,
		Fixed:        false,
	}, nil
}

// fixTornLatest reconstructs latest.json from the intact versioned snapshot.
// It re-validates the recovery source NOW and refuses if it no longer parses.
func fixTornLatest(ctx *MutateContext, env *DetectEnv, obs snapshotObservation) (FixResult, error) {
	snapDir := filepath.Join(env.RepoRoot, openclaw.SnapshotDirRel)
	goodPath := filepath.Join(snapDir, obs.goodVersionFile)
	goodBytes, err := os.ReadFile(goodPath)
	if err != nil {
		return FixResult{FixerID: fmOpenClawSnapshotStale, Err: err}, fmt.Errorf("doctor: read recovery snapshot: %w", err)
	}
	snap, err := openclaw.ParseConsumerSnapshot(goodBytes)
	if err != nil || snap.SchemaVersion != openclaw.ConsumerSnapshotSchemaVersion {
		return FixResult{FixerID: fmOpenClawSnapshotStale, Err: err},
			fmt.Errorf("doctor: recovery source %s no longer valid (refused_unsafe)", obs.goodVersionFile)
	}
	latestPath := filepath.Join(snapDir, "latest.json")
	res, err := Mutate(ctx, latestPath, WriteFile{Content: goodBytes, Mode: 0o600})
	if err != nil {
		return FixResult{FixerID: fmOpenClawSnapshotStale, Err: err}, err
	}
	actions := 0
	if res.OK {
		actions = 1
	}
	// Verify the torn-latest finding was eliminated.
	post := observeSnapshot(env.RepoRoot)
	if !ctx.DryRun && post.kind != "file_ok" {
		return FixResult{FixerID: fmOpenClawSnapshotStale, ActionsTaken: actions, Err: fmt.Errorf("doctor: fix did not eliminate torn-latest finding")},
			fmt.Errorf("doctor: fix did not eliminate torn-latest finding")
	}
	return FixResult{
		FixerID:      fmOpenClawSnapshotStale,
		FindingIDs:   []string{fmOpenClawSnapshotStale},
		ActionsTaken: actions,
		Fixed:        true,
	}, nil
}

// snapshotStaleReport builds the operator instruction for a detect-only
// snapshot sub-case.
func snapshotStaleReport(obs snapshotObservation) string {
	switch obs.kind {
	case "schema_mismatch":
		return fmt.Sprintf("`latest.json` schema_version=%d, but the bridge "+
			"requires version %d. A daemon upgrade bumped the snapshot schema. "+
			"Rebuild the projection from the current daemon: `ao agentopsd "+
			"projection rebuild --consumer openclaw` (or restart agentopsd so it "+
			"re-emits a v%d snapshot). The doctor will not fabricate a schema "+
			"downgrade.", obs.schemaVersion, openclaw.ConsumerSnapshotSchemaVersion,
			openclaw.ConsumerSnapshotSchemaVersion)
	case "latest_missing":
		return "`latest.json` is absent and no versioned snapshot exists to " +
			"recover from. Rebuild via `ao agentopsd projection rebuild " +
			"--consumer openclaw`."
	case "torn_no_recovery":
		return "`latest.json` is torn/truncated and no byte-valid versioned " +
			"`snap_*.json` exists to recover from. Rebuild via `ao agentopsd " +
			"projection rebuild --consumer openclaw`."
	default:
		return "OpenClaw snapshot is not current. Rebuild via `ao agentopsd " +
			"projection rebuild --consumer openclaw`."
	}
}
