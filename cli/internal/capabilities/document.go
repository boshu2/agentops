// Package capabilities owns the application policy for the machine-readable
// CLI contract. It is independent of Cobra and the host runtime.
package capabilities

import (
	"strings"

	"github.com/boshu2/agentops/cli/internal/clicontract"
)

const ContractVersion = "1.1"

type Document struct {
	SchemaVersion    string                       `json:"schema_version"`
	Tool             string                       `json:"tool"`
	ToolVersion      string                       `json:"tool_version"`
	ContractVersion  string                       `json:"contract_version"`
	Platform         Platform                     `json:"platform"`
	GlobalFlags      []Flag                       `json:"global_flags"`
	OutputFormats    []string                     `json:"output_formats"`
	ExitCodes        map[string]string            `json:"exit_codes"`
	CommandExitCodes map[string]map[string]string `json:"command_exit_codes"`
	EnvVars          map[string]string            `json:"env_vars"`
	RobotSurfaces    map[string]string            `json:"robot_surfaces"`
	CommandGroups    []CommandGroup               `json:"command_groups"`
	Commands         []clicontract.Command        `json:"commands"`
}

type Platform struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
}

type Flag struct {
	Name        string `json:"name"`
	Shorthand   string `json:"shorthand,omitempty"`
	Description string `json:"description"`
}

type Command struct {
	Name  string `json:"name"`
	Short string `json:"short"`
}

type CommandGroup struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	Commands []Command `json:"commands"`
}

type Snapshot struct {
	GlobalFlags   []Flag
	CommandGroups []CommandGroup
	Commands      []clicontract.Command
}

type SurfaceReader interface {
	Snapshot(commandExitCodes map[string]map[string]string) Snapshot
}

type PlatformReader interface {
	Platform() Platform
}

type Builder interface {
	Build() Document
}

type Service struct {
	toolVersion string
	surface     SurfaceReader
	platform    PlatformReader
}

func NewService(toolVersion string, surface SurfaceReader, platform PlatformReader) Service {
	return Service{toolVersion: toolVersion, surface: surface, platform: platform}
}

func (service Service) Build() Document {
	commandExits := commandExitCodes()
	snapshot := service.surface.Snapshot(commandExits)
	return Document{
		SchemaVersion:    ContractVersion,
		Tool:             "ao",
		ToolVersion:      service.toolVersion,
		ContractVersion:  ContractVersion,
		Platform:         service.platform.Platform(),
		GlobalFlags:      snapshot.GlobalFlags,
		OutputFormats:    []string{"table", "json", "yaml"},
		ExitCodes:        exitCodes(),
		CommandExitCodes: commandExits,
		EnvVars:          envVars(),
		RobotSurfaces:    robotSurfaces(),
		CommandGroups:    snapshot.CommandGroups,
		Commands:         snapshot.Commands,
	}
}

func exitCodes() map[string]string {
	return map[string]string{
		"0": "success",
		"1": "error: usage error, runtime failure, or — for diagnostic commands — findings present",
		"2": "diagnostic: partial result or bead claimed (command-specific; see that command's --help)",
	}
}

func commandExitCodes() map[string]map[string]string {
	return map[string]map[string]string{
		"pawl review": {
			"0": "CONFIRMED — cross-family verdict written",
			"1": "hard error",
			"2": "usage error",
			"3": "REFUTED — auto-redo",
			"4": "--converge advisory-only (no lineage written)",
			"5": "HOLD / UNAVAILABLE — strict quorum could not run at two-family strength",
		},
		"verify": {
			"0": "CONFIRMED — verdict written and bound to the commit",
			"1": "hard error",
			"2": "usage error",
			"3": "REFUTED — defects printed; fix and re-run",
			"4": "--converge advisory-only (no lineage written)",
			"5": "HOLD / UNAVAILABLE — committed policy is invalid or strict quorum cannot run at two-family strength",
		},
		"plan-pawl decide": {
			"0": "PASS — the door opens",
			"2": "usage error",
			"3": "REDO — auto-redo loop (no human)",
			"4": "BLOCKED — a circuit breaker tripped (andon)",
			"5": "DEGRADED — transient panel lane loss below quorum; retry the panel",
		},
		"governor budget": {
			"0": "ship — within error budget",
			"3": "HARDEN — burn-rate over tolerance; stop the line",
		},
	}
}

func envVars() map[string]string {
	names := []string{
		"AGENTOPS_ACTOR", "AGENTOPS_ACTOR_EMAIL", "AGENTOPS_AUTO_PRUNE", "AGENTOPS_BASE_DIR",
		"AGENTOPS_CANON_JUDGE", "AGENTOPS_CANON_VERIFIER_CMD", "AGENTOPS_COMPILE_RUNTIME",
		"AGENTOPS_CONFIG", "AGENTOPS_COUNCIL_MODEL_TIER", "AGENTOPS_DEDUP_CONFIRM_THRESHOLD",
		"AGENTOPS_DREAM_CURATOR_ENABLED", "AGENTOPS_DREAM_CURATOR_ENGINE", "AGENTOPS_DREAM_CURATOR_MODEL",
		"AGENTOPS_DREAM_CURATOR_OLLAMA_URL", "AGENTOPS_DREAM_CURATOR_VAULT_DIR",
		"AGENTOPS_DREAM_CURATOR_WORKER_DIR", "AGENTOPS_DREAM_KEEP_AWAKE", "AGENTOPS_DREAM_REPORT_DIR",
		"AGENTOPS_DREAM_RUN_TIMEOUT", "AGENTOPS_EVALS_ROOT", "AGENTOPS_EVALS_VENV",
		"AGENTOPS_EVICTION_DISABLED", "AGENTOPS_FLYWHEEL_AUTO_PROMOTE_THRESHOLD",
		"AGENTOPS_FORGE_LEGACY_LOCAL_LLM", "AGENTOPS_FORGE_TIER1_DISABLE", "AGENTOPS_FORGE_TYPED",
		"AGENTOPS_GATE_RANGE", "AGENTOPS_HOLDOUT_EVALUATOR", "AGENTOPS_HOOKS_DISABLED",
		"AGENTOPS_LEGACY", "AGENTOPS_LLM_ENDPOINT", "AGENTOPS_MODEL_TIER", "AGENTOPS_NO_SC",
		"AGENTOPS_ORCHESTRATION", "AGENTOPS_OUTPUT", "AGENTOPS_REDACTION_DENYLIST",
		"AGENTOPS_REPO_ROOT", "AGENTOPS_RESERVATION_ID", "AGENTOPS_RETRIEVAL_RERANK_ENDPOINT",
		"AGENTOPS_RPI_AO_COMMAND", "AGENTOPS_RPI_BD_COMMAND", "AGENTOPS_RPI_MERGE_RETRY_DELAY",
		"AGENTOPS_RPI_RUNTIME", "AGENTOPS_RPI_RUNTIME_COMMAND", "AGENTOPS_RPI_RUNTIME_MODE",
		"AGENTOPS_RPI_TMUX_COMMAND", "AGENTOPS_RPI_WORKTREE_MODE", "AGENTOPS_TRACKER",
		"AGENTOPS_TRUST_REPO", "AGENTOPS_VERBOSE", "AGENTOPS_VERIFY_PREPUSH_SKIP",
		"AO_AGENTS_DIR", "AO_BIN", "AO_CITATION_NAMESPACE", "AO_CONTEXT_VARIANT", "AO_COUNCIL_DIR",
		"AO_DECISIONS_DIR", "AO_DEPOSIT_GATE", "AO_DOCTOR_BACKUPS_DIR", "AO_DOCTOR_HEAL_TIMEOUT",
		"AO_DOCTOR_LOG_LEVEL", "AO_FINDINGS_DIR", "AO_HOME", "AO_HOOKS_DIR", "AO_KNOWLEDGE_ROOT",
		"AO_LAND_REEXECED", "AO_LAND_REVIEW_BASE", "AO_LEARNINGS_DIR", "AO_MAX_PROMOTIONS",
		"AO_PATTERNS_DIR", "AO_PLANS_DIR", "AO_RESERVATION_ID", "AO_RETRIEVAL_EPSILON",
		"AO_RPI_DIR", "AO_SCOPE_LOCK", "AO_SESSION_ID", "NO_COLOR", "PAWL_AUTHOR_FAMILY",
		"PAWL_AUTOBIND", "PAWL_BUNDLE_STATUS", "PAWL_DRY_RUN", "PAWL_IDLE_TTL", "PAWL_LABEL",
		"PAWL_NO_SERVICE", "PAWL_PROJECT", "PAWL_REVIEWER_CHAIN", "PAWL_REVIEW_TIMEOUT",
		"PAWL_SESSION", "PAWL_SMOKE_CMD", "PAWL_STRICT", "PAWL_UNTRUSTED_REPO",
	}
	docs := make(map[string]string, len(names))
	for _, name := range names {
		prefix, class := "", "ao runtime input"
		switch {
		case strings.HasPrefix(name, "AGENTOPS_"):
			prefix, class = "AGENTOPS_", "AgentOps runtime input"
		case strings.HasPrefix(name, "AO_"):
			prefix, class = "AO_", "ao command/runtime input"
		case strings.HasPrefix(name, "PAWL_"):
			prefix, class = "PAWL_", "pawl review/runtime input"
		}
		summary := strings.ToLower(strings.ReplaceAll(strings.TrimPrefix(name, prefix), "_", " "))
		docs[name] = class + ": " + summary
	}
	docs["AGENTOPS_CONFIG"] = "path to the config file (overridden by --config)"
	docs["NO_COLOR"] = "disable ANSI styling on all output"
	docs["AO_DOCTOR_LOG_LEVEL"] = "trace|debug|info|warn|error — verbosity of the doctor surface"
	docs["AGENTOPS_ORCHESTRATION"] = "set to off to disable spawning and use the single-agent beads floor"
	docs["AGENTOPS_TRACKER"] = "force the selected tracker backend when supported"
	docs["PAWL_STRICT"] = "require the strict pawl quorum; unavailable quorum HOLDs instead of degrading"
	docs["PAWL_AUTOBIND"] = "enable or disable automatic provenance binding after an authorizing verdict"
	return docs
}

func robotSurfaces() map[string]string {
	return map[string]string{
		"capabilities":        "ao capabilities — this contract",
		"robot_docs":          "ao robot-docs — paste-ready agent handbook (Markdown)",
		"doctor_triage":       "ao doctor --robot-triage — mega-command health triage JSON",
		"doctor_capabilities": "ao doctor capabilities — extended doctor contract + exit codes",
		"json_everywhere":     "append --json (or -o json) to any read-side command for structured output",
	}
}
