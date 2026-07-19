package doctor

// fix_cliconfig.go implements the cli-config subsystem of `ao doctor`.
//
// All six cli-config failure modes are DETECT-ONLY. None has a safe on-disk
// auto-fix: the doctor must never install a third-party CLI, never silently
// rewrite a user's YAML, never move a user file, and never swap an executable.
// Each FM therefore pairs a pure Detector with a detect-only refuser Fixer
// whose AutoFixable() returns false and whose Fix refuses with exit-4
// (refused_unsafe) semantics while naming the exact operator command to run.
//
// Two FMs (config-flag-not-threaded, dev-version-build-integrity) describe
// defects in `ao`'s own source rather than user state; the detector still
// observes the condition and the remediation points at the Phase-8 code fix.
//
// Symlinked-root audit (age-knowledge-symlink-root-inbpg): the repo-relative
// symlink class (a symlinked .agents routing doctor MUTATIONS to an external
// target past the lexical scope check) does NOT apply to this subsystem —
// every fixer is a detect-only cliConfigRefuser that issues zero Mutate calls,
// so there is no write path to guard. Detectors read the project
// .agents/ao/config.yaml purely for evidence; reads produce findings, not
// mutations, so no guard is added.

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

// ---------------------------------------------------------------------------
// Shared helpers (pure, read-only)
// ---------------------------------------------------------------------------

// homeConfigPathFor returns the home config path for the given home directory.
func homeConfigPathFor(homeDir string) string {
	if homeDir == "" {
		homeDir, _ = os.UserHomeDir()
	}
	return filepath.Join(homeDir, ".agents", "ao", "config.yaml")
}

// projectConfigPathFor returns the project config path for the given cwd.
func projectConfigPathFor(cwd string) string {
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	return filepath.Join(cwd, ".agents", "ao", "config.yaml")
}

// lookPathAll resolves every occurrence of name on PATH, preserving order.
// It is read-only: it stats candidate files but writes nothing.
func lookPathAll(name string) []string {
	var out []string
	seen := make(map[string]bool)
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			continue
		}
		cand := filepath.Join(dir, name)
		if runtime.GOOS == "windows" {
			cand += ".exe"
		}
		info, err := os.Stat(cand)
		if err != nil || info.IsDir() {
			continue
		}
		if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
			continue
		}
		key := cand
		if resolved, err := filepath.EvalSymlinks(cand); err == nil {
			key = resolved
		}
		if !seen[key] {
			seen[key] = true
			out = append(out, cand)
		}
	}
	return out
}

// agentopsModuleLine identifies the agentops CLI module in cli/go.mod.
const agentopsModuleLine = "module github.com/boshu2/agentops/cli"

// insideAgentopsRepo reports whether dir is within an agentops repo clone,
// walking up a bounded number of parents looking for cli/go.mod declaring the
// agentops module. It is read-only. An empty dir is treated as "not in repo".
func insideAgentopsRepo(dir string) bool {
	if dir == "" {
		return false
	}
	for range 12 {
		data, err := os.ReadFile(filepath.Join(dir, "cli", "go.mod"))
		if err == nil && strings.Contains(string(data), agentopsModuleLine) {
			return true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return false
}

// installHintFor returns a platform-specific install command for a CLI.
func installHintFor(name string) string {
	switch name {
	case "br":
		if runtime.GOOS == "windows" {
			return "br: install beads_rust from its Windows release or use WSL/Homebrew — https://github.com/Dicklesworthstone/beads_rust"
		}
		return "br: install beads_rust — https://github.com/Dicklesworthstone/beads_rust ('ao beads dir' prints the resolved ledger)"
	case "git":
		if runtime.GOOS == "windows" {
			return "git: choco install git  |  https://git-scm.com/download/win"
		}
		return "git: brew install git  (macOS)  |  apt-get install -y git  (Linux)"
	case "codex":
		if runtime.GOOS == "windows" {
			return "codex: install codex for Windows or use WSL — https://github.com/openai/codex"
		}
		return "codex: npm i -g @openai/codex  OR  see https://github.com/openai/codex"
	default:
		return "install " + name
	}
}

// refusedUnsafeResult builds the FixResult returned by every cli-config
// detect-only fixer. The error carries the precise operator command so callers
// (and `ao doctor explain`) can surface error-names-the-fix messaging. Exit-4
// (ExitRefusedUnsafe) is the contractual exit code for these refusals.
func refusedUnsafeResult(fixerID string, findings []Finding, reason, operatorCmd string) (FixResult, error) {
	ids := make([]string, 0, len(findings))
	for _, f := range findings {
		ids = append(ids, f.ID)
	}
	err := fmt.Errorf(
		"doctor: refused_unsafe (exit %d): %s — run: %s",
		ExitRefusedUnsafe, reason, operatorCmd,
	)
	return FixResult{
		FixerID:      fixerID,
		FindingIDs:   ids,
		ActionsTaken: 0,
		Fixed:        false,
		Err:          err,
	}, err
}

// cliConfigRefuser is the shared detect-only Fixer for every cli-config FM.
// AutoFixable() is false, so the engine's applyFixers never invokes it during
// `--fix`; it only runs when an operator explicitly scopes a fix to its ID,
// and then it refuses with exit-4 semantics. It performs no mutation.
type cliConfigRefuser struct {
	id          string
	reason      string
	operatorCmd string
}

func (r cliConfigRefuser) ID() string              { return r.id }
func (r cliConfigRefuser) Preconditions() []string { return nil }
func (r cliConfigRefuser) WritesTo() []string      { return nil }
func (r cliConfigRefuser) Ops() []string           { return nil }
func (r cliConfigRefuser) Reversible() bool        { return true }
func (r cliConfigRefuser) Idempotent() bool        { return true }
func (r cliConfigRefuser) AutoFixable() bool       { return false }

// Fix refuses with exit-4 (refused_unsafe) and names the operator command.
// It issues zero Mutate calls.
func (r cliConfigRefuser) Fix(_ *MutateContext, _ *DetectEnv, findings []Finding) (FixResult, error) {
	return refusedUnsafeResult(r.id, findings, r.reason, r.operatorCmd)
}

// ---------------------------------------------------------------------------
// FM 1: fm-cli-config-invalid-config-yaml-swallowed (P1)
// ---------------------------------------------------------------------------

const fmInvalidConfigYAML = "fm-cli-config-invalid-config-yaml-swallowed"

// invalidConfigYAMLDetector flags a config.yaml that fails YAML parse — `ao`
// silently falls back to defaults instead of surfacing the error.
type invalidConfigYAMLDetector struct{}

func (invalidConfigYAMLDetector) ID() string           { return fmInvalidConfigYAML }
func (invalidConfigYAMLDetector) Subsystem() string    { return "cli-config" }
func (invalidConfigYAMLDetector) Severity() string     { return "P1" }
func (invalidConfigYAMLDetector) EstimatedCostMS() int { return 5 }
func (invalidConfigYAMLDetector) OnlineRequired() bool { return false }
func (invalidConfigYAMLDetector) QuickPath() bool      { return true }
func (invalidConfigYAMLDetector) Describe() string {
	return "Detects a config.yaml (home or project) that fails YAML parse and is silently discarded."
}

// Detect re-does what the loader does (parse the YAML) but only inspects the
// error. It is pure: it reads config files and writes nothing.
func (invalidConfigYAMLDetector) Detect(env *DetectEnv) ([]Finding, error) {
	candidates := []struct{ path, layer string }{
		{homeConfigPathFor(env.HomeDir), "home"},
		{projectConfigPathFor(env.CWD), "project"},
	}
	var brokenFiles, parseErrors, layers []string
	for _, c := range candidates {
		raw, err := os.ReadFile(c.path)
		if err != nil {
			continue // absence is not this FM
		}
		var probe map[string]interface{}
		if perr := yaml.Unmarshal(raw, &probe); perr != nil {
			brokenFiles = append(brokenFiles, c.path)
			parseErrors = append(parseErrors, perr.Error())
			layers = append(layers, c.layer)
		}
	}
	if len(brokenFiles) == 0 {
		return nil, nil
	}
	return []Finding{{
		ID:         fmInvalidConfigYAML,
		Severity:   "P1",
		Subsystem:  "cli-config",
		Title:      "config.yaml fails YAML parse — ao silently fell back to defaults",
		Confidence: 1.0,
		Evidence: Evidence{
			File:  brokenFiles[0],
			Query: "broken_files=" + strings.Join(brokenFiles, ",") + " layers=" + strings.Join(layers, ",") + " parse_errors=" + strings.Join(parseErrors, " | "),
		},
		Remediation: Remediation{
			Command: "Open " + brokenFiles[0] + ", fix the YAML at the line in the parse error, then verify: " +
				"python3 -c \"import yaml; yaml.safe_load(open('" + brokenFiles[0] + "'))\"",
			ExplainCommand:   "ao doctor explain " + fmInvalidConfigYAML,
			AutoFixable:      false,
			EstimatedActions: 0,
		},
	}}, nil
}

// ---------------------------------------------------------------------------
// FM 2: fm-cli-config-config-flag-not-threaded (P2) — ao-code defect
// ---------------------------------------------------------------------------

const fmConfigFlagNotThreaded = "fm-cli-config-config-flag-not-threaded"

// configFlagNotThreadedDetector flags that `--config` is wired only into the
// project layer; the home config still merges underneath. This is a defect in
// `ao`'s own source, surfaced by a behavioral probe and a source-shape probe.
type configFlagNotThreadedDetector struct{}

func (configFlagNotThreadedDetector) ID() string           { return fmConfigFlagNotThreaded }
func (configFlagNotThreadedDetector) Subsystem() string    { return "cli-config" }
func (configFlagNotThreadedDetector) Severity() string     { return "P2" }
func (configFlagNotThreadedDetector) EstimatedCostMS() int { return 60 }
func (configFlagNotThreadedDetector) OnlineRequired() bool { return false }
func (configFlagNotThreadedDetector) QuickPath() bool      { return false }
func (configFlagNotThreadedDetector) Describe() string {
	return "Detects whether the ao binary threads --config into the home/full config load (an ao-code defect)."
}

// Detect runs a behavioral probe (an `ao` with a nonexistent --config path)
// and a source-shape probe (when inside the ao repo). Both are read-only: the
// probe subprocess performs no repo write and the source files are only read.
func (configFlagNotThreadedDetector) Detect(env *DetectEnv) ([]Finding, error) {
	silentlyAccepted, probeExit := probeConfigFlag()
	sourceBuggy := probeConfigSourceShape(env.RepoRoot)
	if !silentlyAccepted && !sourceBuggy {
		return nil, nil
	}
	return []Finding{{
		ID:         fmConfigFlagNotThreaded,
		Severity:   "P2",
		Subsystem:  "cli-config",
		Title:      "--config only overrides the project config; home config still merges underneath",
		Confidence: 0.9,
		Evidence: Evidence{
			Query: fmt.Sprintf("probe_exit_code=%d silently_accepted_bad_path=%t source_buggy=%t — "+
				"ao --config /nonexistent/path.yaml config --show --json exits 0 with no warning",
				probeExit, silentlyAccepted, sourceBuggy),
			File: filepath.Join("cli", "cmd", "ao", "root.go"),
		},
		Remediation: Remediation{
			Command: "This is an ao-code defect, not your config. Thread App.CfgFile through " +
				"config.Load/config.Resolve so --config replaces BOTH layers, and validate the " +
				"path exists. See: ao doctor explain " + fmConfigFlagNotThreaded,
			ExplainCommand:   "ao doctor explain " + fmConfigFlagNotThreaded,
			AutoFixable:      false,
			EstimatedActions: 0,
		},
	}}, nil
}

// probeConfigFlag runs `ao --config <nonexistent> config --show --json` and
// reports whether the bad path was silently accepted (exit 0, no warning).
func probeConfigFlag() (silentlyAccepted bool, exitCode int) {
	aoPath, err := exec.LookPath("ao")
	if err != nil {
		return false, -1 // ao not on PATH — cannot probe; defer to source-shape probe
	}
	nonexistent := filepath.Join(os.TempDir(), "doctor-config-probe-DOES-NOT-EXIST.yaml")
	cmd := exec.Command(aoPath, "--config", nonexistent, "config", "--show", "--json")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	exitCode = 0
	if runErr != nil {
		var ee *exec.ExitError
		if errors.As(runErr, &ee) {
			exitCode = ee.ExitCode()
		} else {
			return false, -1
		}
	}
	errText := stderr.String()
	silentlyAccepted = exitCode == 0 &&
		!strings.Contains(errText, "config") &&
		!strings.Contains(errText, nonexistent)
	return silentlyAccepted, exitCode
}

// probeConfigSourceShape reads root.go and config.go (read-only) to detect the
// buggy plumbing shape: syncConfigFlagToEnv sets AGENTOPS_CONFIG only and
// homeConfigPath has no override.
func probeConfigSourceShape(repoRoot string) bool {
	if repoRoot == "" {
		return false
	}
	rootPath := filepath.Join(repoRoot, "cli", "cmd", "ao", "root.go")
	cfgPath := filepath.Join(repoRoot, "cli", "internal", "config", "config.go")
	root, err := os.ReadFile(rootPath)
	if err != nil {
		return false
	}
	cfg, err := os.ReadFile(cfgPath)
	if err != nil {
		return false
	}
	rootStr := string(root)
	cfgStr := string(cfg)
	usesSync := strings.Contains(rootStr, "syncConfigFlagToEnv")
	setsOnlyAgentopsConfig := strings.Contains(rootStr, "AGENTOPS_CONFIG")
	homeHonorsOverride := strings.Contains(cfgStr, "AGENTOPS_CONFIG") &&
		strings.Contains(cfgStr, "homeConfigPath")
	return usesSync && setsOnlyAgentopsConfig && !homeHonorsOverride
}

// ---------------------------------------------------------------------------
// FM 3: fm-cli-config-missing-required-cli (P1)
// ---------------------------------------------------------------------------

const fmMissingRequiredCLI = "fm-cli-config-missing-required-cli"

// missingRequiredCLIDetector flags a required external CLI absent from PATH, or
// shadowed by a duplicate earlier on PATH. Only `git` is universally required;
// `br` (this repo's own tracker) is required only when running inside an
// agentops repo clone — an installed user who tracks work with `bd` (or nothing)
// must never see `br` reported as a missing "required" CLI.
type missingRequiredCLIDetector struct{}

func (missingRequiredCLIDetector) ID() string           { return fmMissingRequiredCLI }
func (missingRequiredCLIDetector) Subsystem() string    { return "cli-config" }
func (missingRequiredCLIDetector) Severity() string     { return "P1" }
func (missingRequiredCLIDetector) EstimatedCostMS() int { return 5 }
func (missingRequiredCLIDetector) OnlineRequired() bool { return false }
func (missingRequiredCLIDetector) QuickPath() bool      { return true }
func (missingRequiredCLIDetector) Describe() string {
	return "Detects required external CLIs (git always; br inside an agentops clone) missing from PATH or shadowed by a duplicate."
}

// Detect resolves every match for the required CLIs on PATH. It is pure:
// lookPathAll only reads $PATH and stats candidate files, and insideAgentopsRepo
// only stats a marker file.
func (missingRequiredCLIDetector) Detect(env *DetectEnv) ([]Finding, error) {
	required := []string{"git"}
	if env != nil && insideAgentopsRepo(env.RepoRoot) {
		required = append(required, "br")
	}
	var missing, shadowed, hints []string
	for _, name := range required {
		resolved := lookPathAll(name)
		switch {
		case len(resolved) == 0:
			missing = append(missing, name)
			hints = append(hints, installHintFor(name))
		case len(resolved) > 1:
			shadowed = append(shadowed, name+" -> "+strings.Join(resolved, ", "))
		}
	}
	if len(missing) == 0 && len(shadowed) == 0 {
		return nil, nil
	}
	title := "required external CLI missing from PATH"
	if len(missing) == 0 {
		title = "required external CLI shadowed by a duplicate on PATH"
	}
	return []Finding{{
		ID:         fmMissingRequiredCLI,
		Severity:   "P1",
		Subsystem:  "cli-config",
		Title:      title,
		Confidence: 1.0,
		Evidence: Evidence{
			Query: "missing_clis=" + strings.Join(missing, ",") +
				" shadowed_clis=" + strings.Join(shadowed, "; "),
		},
		Remediation: Remediation{
			Command: "The doctor does not install software. Install the missing CLI yourself — " +
				strings.Join(hints, "  ||  ") + " — then re-run: ao doctor",
			ExplainCommand:   "ao doctor explain " + fmMissingRequiredCLI,
			AutoFixable:      false,
			EstimatedActions: 0,
		},
	}}, nil
}

// NOTE: the optional `codex` CLI absence is intentionally NOT a failure-mode
// detector. An optional dependency's absence must never flip `ao doctor`'s exit
// code on a pristine install. The legacy check table already reports codex as an
// informational "optional — enables --mixed council review" line with a runnable
// install hint (see internal/adapters/doctor/legacy.go), so a duplicate P3
// finding here would only fail every codex-less install for no actionable
// benefit. (Retired under FU2 / age-6g2du — doctor severity calibration.)

// ---------------------------------------------------------------------------
// FM 5: fm-cli-config-dev-version-build-integrity (P2) — build-time concern
// ---------------------------------------------------------------------------

const fmDevVersionBuildIntegrity = "fm-cli-config-dev-version-build-integrity"

// aoReleaseVersion matches published versions ("3.1.0", "v3.1.0") and the
// documented source fallback used before the v3.1.0 tag is cut ("3.1.0-rc").
var aoReleaseVersion = regexp.MustCompile(`^v?\d+\.\d+\.\d+(-rc(\.\d+)?)?$`)

// devVersionBuildIntegrityDetector flags an `ao` binary that reports a
// non-release version (dev/empty/non-semver) or is shadowed by another `ao`
// on PATH. This is a build-time / install-integrity concern.
type devVersionBuildIntegrityDetector struct{}

func (devVersionBuildIntegrityDetector) ID() string           { return fmDevVersionBuildIntegrity }
func (devVersionBuildIntegrityDetector) Subsystem() string    { return "cli-config" }
func (devVersionBuildIntegrityDetector) Severity() string     { return "P2" }
func (devVersionBuildIntegrityDetector) EstimatedCostMS() int { return 40 }
func (devVersionBuildIntegrityDetector) OnlineRequired() bool { return false }
func (devVersionBuildIntegrityDetector) QuickPath() bool      { return false }
func (devVersionBuildIntegrityDetector) Describe() string {
	return "Detects an ao binary reporting a non-release version, or multiple ao binaries shadowing each other."
}

// Detect inspects the running binary's reported version (via env.ToolVersion)
// and resolves every `ao` on PATH. All read-only.
func (devVersionBuildIntegrityDetector) Detect(_ *DetectEnv) ([]Finding, error) {
	reported := aoReportedVersion()
	suspect := reported == "" || reported == "dev" || reported == "vdev" ||
		!aoReleaseVersion.MatchString(reported)

	aoPaths := lookPathAll("ao")
	shadowed := len(aoPaths) > 1

	if !suspect && !shadowed {
		return nil, nil
	}
	pathDesc := strings.Join(aoPaths, ", ")
	if pathDesc == "" {
		pathDesc = "(ao not resolvable on PATH)"
	} else {
		pathDesc = describeAOPaths(aoPaths)
	}
	return []Finding{{
		ID:         fmDevVersionBuildIntegrity,
		Severity:   "P2",
		Subsystem:  "cli-config",
		Title:      "`ao` reports a non-release version, or multiple `ao` binaries shadow each other",
		Confidence: 0.9,
		Evidence: Evidence{
			Query: fmt.Sprintf("reported_version=%q suspect_version=%t shadowed=%t ao_paths=[%s]",
				reported, suspect, shadowed, pathDesc),
		},
		Remediation: Remediation{
			Command: "Reinstall a release build: " +
				"brew upgrade agentops " +
				"— or, if developing, rebuild with ldflags: cd cli && make build. " +
				"If `which -a ao` shows duplicates, remove the stale one from PATH.",
			ExplainCommand:   "ao doctor explain " + fmDevVersionBuildIntegrity,
			AutoFixable:      false,
			EstimatedActions: 0,
		},
	}}, nil
}

// aoReportedVersion returns the `ao` binary's reported version string by
// running `ao version` (read-only). If `ao` is unavailable it returns "" —
// itself a suspect value the detector flags.
func aoReportedVersion() string {
	aoPath, err := exec.LookPath("ao")
	if err != nil {
		return ""
	}
	return aoVersionAt(aoPath)
}

func aoVersionAt(aoPath string) string {
	out, err := exec.Command(aoPath, "version").Output()
	if err != nil {
		return ""
	}
	return parseAOVersionOutput(out)
}

func parseAOVersionOutput(out []byte) string {
	// `ao version` prints a first line like "ao version 3.1.0-rc", followed
	// by detail lines. Only the first version line carries the ao version.
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[0] == "ao" && fields[1] == "version" {
			return fields[2]
		}
	}
	return ""
}

func describeAOPaths(paths []string) string {
	described := make([]string, 0, len(paths))
	for _, path := range paths {
		if version := aoVersionAt(path); version != "" {
			described = append(described, fmt.Sprintf("%s (%s)", path, version))
			continue
		}
		described = append(described, path+" (version unknown)")
	}
	return strings.Join(described, ", ")
}

// ---------------------------------------------------------------------------
// FM 6: fm-cli-config-stale-project-config-shadows-home (P3)
// ---------------------------------------------------------------------------

const fmStaleProjectConfig = "fm-cli-config-stale-project-config-shadows-home"

// staleProjectConfigDetector flags a project .agents/ao/config.yaml in cwd that
// silently overrides the home config.
type staleProjectConfigDetector struct{}

func (staleProjectConfigDetector) ID() string           { return fmStaleProjectConfig }
func (staleProjectConfigDetector) Subsystem() string    { return "cli-config" }
func (staleProjectConfigDetector) Severity() string     { return "P3" }
func (staleProjectConfigDetector) EstimatedCostMS() int { return 5 }
func (staleProjectConfigDetector) OnlineRequired() bool { return false }
func (staleProjectConfigDetector) QuickPath() bool      { return true }
func (staleProjectConfigDetector) Describe() string {
	return "Detects a project .agents/ao/config.yaml in cwd that silently shadows the home config."
}

// flattenYAML recursively flattens a parsed YAML map into dotted keys.
func flattenYAML(prefix string, node map[string]interface{}, out map[string]interface{}) {
	for k, v := range node {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		if child, ok := v.(map[string]interface{}); ok {
			flattenYAML(key, child, out)
			continue
		}
		out[key] = v
	}
}

// Detect re-derives the layered config the same way Load does (read home, read
// project) but only to COMPARE them. It is pure: it reads both files and
// writes nothing.
func (staleProjectConfigDetector) Detect(env *DetectEnv) ([]Finding, error) {
	projectPath := projectConfigPathFor(env.CWD)
	projectRaw, err := os.ReadFile(projectPath)
	if err != nil {
		return nil, nil // no project layer → nothing shadows
	}
	var projectNode map[string]interface{}
	if perr := yaml.Unmarshal(projectRaw, &projectNode); perr != nil {
		return nil, nil // unparseable project file is a DIFFERENT FM — defer to it
	}
	projectFlat := make(map[string]interface{})
	flattenYAML("", projectNode, projectFlat)

	homePath := homeConfigPathFor(env.HomeDir)
	homeFlat := make(map[string]interface{})
	if homeRaw, herr := os.ReadFile(homePath); herr == nil {
		var homeNode map[string]interface{}
		if yaml.Unmarshal(homeRaw, &homeNode) == nil {
			flattenYAML("", homeNode, homeFlat)
		}
	}

	var shadowedKeys []string
	for key, pval := range projectFlat {
		hval, present := homeFlat[key]
		if !present || fmt.Sprint(pval) != fmt.Sprint(hval) {
			shadowedKeys = append(shadowedKeys, fmt.Sprintf("%s (project=%v home=%v)", key, pval, hval))
		}
	}
	if len(shadowedKeys) == 0 {
		return nil, nil // project file present but inert
	}
	return []Finding{{
		ID:         fmStaleProjectConfig,
		Severity:   "P3",
		Subsystem:  "cli-config",
		Title:      "a project .agents/ao/config.yaml is overriding home config in this directory",
		Confidence: 0.95,
		Evidence: Evidence{
			File:  projectPath,
			Query: "project_config_path=" + projectPath + " home_config_path=" + homePath + " shadowed_keys=" + strings.Join(shadowedKeys, "; "),
		},
		Remediation: Remediation{
			Command: "Review " + projectPath + ". If it is intentional repo config, keep it. " +
				"If it is a stale leftover, move it aside yourself (e.g. mv " + projectPath + " " +
				projectPath + ".bak), then re-run: ao doctor",
			ExplainCommand:   "ao doctor explain " + fmStaleProjectConfig,
			AutoFixable:      false,
			EstimatedActions: 0,
		},
	}}, nil
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------

func init() {
	RegisterDetector(invalidConfigYAMLDetector{})
	RegisterDetector(configFlagNotThreadedDetector{})
	RegisterDetector(missingRequiredCLIDetector{})
	RegisterDetector(devVersionBuildIntegrityDetector{})
	RegisterDetector(staleProjectConfigDetector{})

	RegisterFixer(cliConfigRefuser{
		id:          fmInvalidConfigYAML,
		reason:      "config.yaml is unparseable; the doctor will not rewrite a user-authored config",
		operatorCmd: "ao doctor explain " + fmInvalidConfigYAML,
	})
	RegisterFixer(cliConfigRefuser{
		id:          fmConfigFlagNotThreaded,
		reason:      "--config not threaded into the home/full config load — this is an ao-code defect, not user state",
		operatorCmd: "ao doctor explain " + fmConfigFlagNotThreaded,
	})
	RegisterFixer(cliConfigRefuser{
		id:          fmMissingRequiredCLI,
		reason:      "required CLI missing from PATH; the doctor does not install software",
		operatorCmd: "ao doctor explain " + fmMissingRequiredCLI,
	})
	RegisterFixer(cliConfigRefuser{
		id:          fmDevVersionBuildIntegrity,
		reason:      "ao reports a non-release version; the doctor does not recompile or replace binaries",
		operatorCmd: "ao doctor explain " + fmDevVersionBuildIntegrity,
	})
	RegisterFixer(cliConfigRefuser{
		id:          fmStaleProjectConfig,
		reason:      "a project .agents/ao/config.yaml shadows home config; the doctor will not move a possibly-intentional user file",
		operatorCmd: "ao doctor explain " + fmStaleProjectConfig,
	})
}
