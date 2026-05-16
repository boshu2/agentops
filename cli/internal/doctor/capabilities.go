package doctor

import (
	"fmt"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// Contract / schema version constants for the doctor surface.
const (
	// SchemaVersion is the JSON schema version emitted in every doctor JSON output.
	SchemaVersion = "1.0"
	// DoctorVersion is the semantic version of the doctor engine itself.
	DoctorVersion = "1.0.0"
	// DoctorContractVersion is the version of the capabilities contract.
	DoctorContractVersion = "1.0"
	// ToolName is the host CLI binary name.
	ToolName = "ao"

	runArtifactSchemaURL = "https://schemas.agentops.dev/doctor/run-artifact/1.0.json"
	reportSchemaURL      = "https://schemas.agentops.dev/doctor/report/1.0.json"
)

// canonicalWriteScopes is the strict set of repo-relative and home-relative
// path prefixes `ao doctor --fix` may write to, derived verbatim from
// analysis/safety_envelope.md. Any write outside this set refuses with exit 4.
var canonicalWriteScopes = []string{
	".doctor",
	".agents/daemon",
	".agents/handoffs/sha256",
	".agents/ao",
	".agents/learnings",
	"~/.claude/settings.json",
	"~/.claude/skills",
	"~/.claude/hooks.json",
	"~/.agentops/hooks.json",
	"~/.agentops/hooks",
	"~/.codex/plugins/cache/agentops-marketplace",
	"~/.codex/.agentops-codex-install.json",
	"skills-codex",
	"skills",
	"hooks",
	"docs",
	"scripts",
}

// canonicalSubsystems is the fixed list of doctor subsystems.
var canonicalSubsystems = []string{
	"daemon", "bridges", "hooks", "knowledge", "skills", "cli_config",
}

// canonicalExitCodes documents every exit code the doctor surface can return.
var canonicalExitCodes = map[string]string{
	"0":  "success_or_healthy",
	"1":  "findings_present_no_fix",
	"2":  "fix_partial",
	"3":  "fix_failed_rolled_back",
	"4":  "refused_unsafe",
	"5":  "concurrency_lost",
	"6":  "online_required",
	"64": "usage_error",
	"66": "no_input",
	"73": "cant_create",
	"74": "io_error",
}

// canonicalEnvVars documents the environment variables the doctor honors.
var canonicalEnvVars = map[string]string{
	"AO_DOCTOR_LOG_LEVEL":   "trace|debug|info|warn|error",
	"AO_DOCTOR_BACKUPS_DIR": "override the default .doctor/ location",
	"NO_COLOR":              "disable ANSI",
}

// platformInfo captures the OS/arch the doctor is running on.
type platformInfo struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
}

// detectorCapability is the capabilities-document view of a registered detector.
type detectorCapability struct {
	ID              string `json:"id"`
	Subsystem       string `json:"subsystem"`
	Severity        string `json:"severity"`
	Description     string `json:"description"`
	EstimatedCostMS int    `json:"estimated_cost_ms"`
	OnlineRequired  bool   `json:"online_required"`
}

// fixerCapability is the capabilities-document view of a registered fixer.
type fixerCapability struct {
	ID            string   `json:"id"`
	Preconditions []string `json:"preconditions"`
	WritesTo      []string `json:"writes_to"`
	Ops           []string `json:"ops"`
	Reversible    bool     `json:"reversible"`
	Idempotent    bool     `json:"idempotent"`
	AutoFixable   bool     `json:"auto_fixable"`
}

// manualRemediation documents a finding class the doctor cannot auto-fix.
type manualRemediation struct {
	ID          string `json:"id"`
	Instruction string `json:"instruction"`
	Reason      string `json:"reason"`
}

// Capabilities is the machine-readable contract for the doctor surface,
// emitted by `ao doctor capabilities --json`.
type Capabilities struct {
	SchemaVersion         string               `json:"schema_version"`
	Tool                  string               `json:"tool"`
	ToolVersion           string               `json:"tool_version"`
	DoctorVersion         string               `json:"doctor_version"`
	DoctorContractVersion string               `json:"doctor_contract_version"`
	Platform              platformInfo         `json:"platform"`
	Subsystems            []string             `json:"subsystems"`
	Detectors             []detectorCapability `json:"detectors"`
	Fixers                []fixerCapability    `json:"fixers"`
	ManualRemediations    []manualRemediation  `json:"manual_remediations"`
	ExitCodes             map[string]string    `json:"exit_codes"`
	EnvVars               map[string]string    `json:"env_vars"`
	WriteScopes           []string             `json:"write_scopes"`
	RunArtifactSchema     string               `json:"run_artifact_schema"`
	ReportSchema          string               `json:"report_schema"`
}

// NewCapabilities builds the capabilities document from the live registry and
// the supplied host tool version.
func NewCapabilities(toolVersion string) *Capabilities {
	caps := &Capabilities{
		SchemaVersion:         SchemaVersion,
		Tool:                  ToolName,
		ToolVersion:           toolVersion,
		DoctorVersion:         DoctorVersion,
		DoctorContractVersion: DoctorContractVersion,
		Platform:              platformInfo{OS: runtime.GOOS, Arch: runtime.GOARCH},
		Subsystems:            append([]string(nil), canonicalSubsystems...),
		Detectors:             []detectorCapability{},
		Fixers:                []fixerCapability{},
		ManualRemediations:    []manualRemediation{},
		ExitCodes:             canonicalExitCodes,
		EnvVars:               canonicalEnvVars,
		WriteScopes:           append([]string(nil), canonicalWriteScopes...),
		RunArtifactSchema:     runArtifactSchemaURL,
		ReportSchema:          reportSchemaURL,
	}
	for _, d := range Detectors() {
		caps.Detectors = append(caps.Detectors, detectorCapability{
			ID:              d.ID(),
			Subsystem:       d.Subsystem(),
			Severity:        d.Severity(),
			Description:     d.Describe(),
			EstimatedCostMS: d.EstimatedCostMS(),
			OnlineRequired:  d.OnlineRequired(),
		})
	}
	for _, f := range Fixers() {
		caps.Fixers = append(caps.Fixers, fixerCapability{
			ID:            f.ID(),
			Preconditions: f.Preconditions(),
			WritesTo:      f.WritesTo(),
			Ops:           f.Ops(),
			Reversible:    f.Reversible(),
			Idempotent:    f.Idempotent(),
			AutoFixable:   f.AutoFixable(),
		})
	}
	sort.Slice(caps.Detectors, func(i, j int) bool { return caps.Detectors[i].ID < caps.Detectors[j].ID })
	sort.Slice(caps.Fixers, func(i, j int) bool { return caps.Fixers[i].ID < caps.Fixers[j].ID })
	return caps
}

// resolveScope turns a write-scope entry into an absolute path. Entries
// beginning with "~/" are anchored at homeDir; all others at repoRoot.
func resolveScope(scope, repoRoot, homeDir string) string {
	if strings.HasPrefix(scope, "~/") {
		return filepath.Clean(filepath.Join(homeDir, scope[2:]))
	}
	return filepath.Clean(filepath.Join(repoRoot, scope))
}

// EnsureInScope returns nil if path resolves inside one of the capabilities'
// write scopes, and a descriptive error (mapped to exit 4) otherwise.
func EnsureInScope(caps *Capabilities, repoRoot, homeDir, path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("doctor: resolve path %s: %w", path, err)
	}
	abs = filepath.Clean(abs)
	for _, scope := range caps.WriteScopes {
		base := resolveScope(scope, repoRoot, homeDir)
		if abs == base {
			return nil
		}
		rel, err := filepath.Rel(base, abs)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil
		}
	}
	return fmt.Errorf("doctor: path %s is outside write_scopes (refused_unsafe)", path)
}

// EnsureOpAllowed returns nil if op is an executable op, and an error otherwise.
// DbExec and DbMigrate are declared for contract completeness but unsupported.
func EnsureOpAllowed(_ *Capabilities, op Op) error {
	switch op.(type) {
	case WriteFile, AppendFile, Rename, Chmod, SymlinkAtomic:
		return nil
	case DbExec, DbMigrate:
		return ErrDBOpsUnused
	default:
		return fmt.Errorf("doctor: unknown op %T", op)
	}
}
