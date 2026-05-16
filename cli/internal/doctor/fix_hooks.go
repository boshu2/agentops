package doctor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/boshu2/agentops/cli/embedded"
	"github.com/boshu2/agentops/cli/internal/bridge"
)

// The hooks subsystem detects and repairs AgentOps Claude Code hook coverage.
// Five failure modes are handled, in this resolution order:
//
//   - fm-hooks-contract-fallback   (P3, auto-fix) — materialize ~/.agentops/hooks.json
//   - fm-hooks-coverage-zero       (P1, auto-fix) — write hooks into settings.json
//   - fm-hooks-coverage-partial    (P2, auto-fix) — merge missing contract events
//   - fm-hooks-non-ao-shadow       (P2, gated)    — additive merge behind a confirm flag
//   - fm-hooks-settings-malformed  (P2, detect)   — unparseable settings, no safe merge
//
// All disk writes route through Mutate. Detectors are pure (read-only).

// hookConfirmForeignMerge, when set in a fixer environment, allows the gated
// fm-hooks-non-ao-shadow fixer to perform an additive merge. It mirrors the
// CLI's --confirm-foreign-merge flag.
const hookEnvConfirmForeignMerge = "AO_DOCTOR_CONFIRM_FOREIGN_MERGE"

// hookEnvAllowUnsafeRewrite, when set, allows the otherwise detect-only
// fm-hooks-settings-malformed fixer to quarantine the corrupt settings file.
// It mirrors the CLI's --allow-unsafe-rewrite flag.
const hookEnvAllowUnsafeRewrite = "AO_DOCTOR_ALLOW_UNSAFE_REWRITE"

// ----------------------------------------------------------------------------
// Shared pure helpers
// ----------------------------------------------------------------------------

// hookSettingsPath returns the absolute ~/.claude/settings.json path.
func hookSettingsPath(env *DetectEnv) string {
	return filepath.Join(env.HomeDir, ".claude", "settings.json")
}

// hookSettingsJSONPath returns the absolute ~/.claude/hooks.json path (the
// standalone-format fallback the live checkHookCoverage also probes).
func hookSettingsJSONPath(env *DetectEnv) string {
	return filepath.Join(env.HomeDir, ".claude", "hooks.json")
}

// hookHomeManifestPath returns the absolute ~/.agentops/hooks.json path.
func hookHomeManifestPath(env *DetectEnv) string {
	return filepath.Join(env.HomeDir, ".agentops", "hooks.json")
}

// hookInstallBase returns the ~/.agentops install base used for ${CLAUDE_PLUGIN_ROOT}.
func hookInstallBase(env *DetectEnv) string {
	return filepath.Join(env.HomeDir, ".agentops")
}

// hookExtractMap mirrors cmd/ao.extractHooksMap: it returns the hooks map for a
// settings.json ("hooks" object) or a standalone hooks.json (top-level events).
func hookExtractMap(data []byte) (map[string]any, bool) {
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, false
	}
	if hooksRaw, ok := parsed["hooks"]; ok {
		if hooksMap, ok := hooksRaw.(map[string]any); ok {
			return hooksMap, true
		}
	}
	for _, event := range bridge.AllEventNames() {
		if _, ok := parsed[event]; ok {
			return parsed, true
		}
	}
	return nil, false
}

// hookResolveContract mirrors cmd/ao.resolveHookCoverageContract: it resolves
// the active coverage contract from a hooks.json manifest, falling back to the
// all-12-event set when the manifest is missing, unparseable, or empty.
//
// Manifest search order (matches findHooksManifest):
//
//	<repoRoot>/hooks/hooks.json -> ~/.agentops/hooks.json -> embedded blob
//
// The bool reports whether a non-fallback contract was resolved.
func hookResolveContract(env *DetectEnv) bridge.HookCoverageContract {
	data, _, err := hookFindManifest(env)
	if err != nil {
		return bridge.FallbackHookCoverageContract("hooks.json not found: " + err.Error())
	}
	cfg, err := bridge.ReadHooksManifest(data)
	if err != nil {
		return bridge.FallbackHookCoverageContract("parse hooks manifest: " + err.Error())
	}
	active := bridge.ActiveEventNamesFromConfig(cfg)
	if len(active) == 0 {
		return bridge.FallbackHookCoverageContract("hooks manifest contains zero active events")
	}
	return bridge.HookCoverageContract{ActiveEvents: active}
}

// hookFindManifest resolves the hooks.json manifest bytes plus a source label
// ("repo", "home", or "embedded"). It mirrors findHooksManifest's disk search
// but anchors the repo probe at env.RepoRoot for deterministic, env-scoped
// behavior (detectors must be pure and not depend on process cwd).
func hookFindManifest(env *DetectEnv) ([]byte, string, error) {
	repoManifest := filepath.Join(env.RepoRoot, "hooks", "hooks.json")
	if data, err := os.ReadFile(repoManifest); err == nil {
		return data, "repo", nil
	}
	homeManifest := hookHomeManifestPath(env)
	if data, err := os.ReadFile(homeManifest); err == nil {
		return data, "home", nil
	}
	if len(embedded.HooksJSON) > 0 {
		return append([]byte(nil), embedded.HooksJSON...), "embedded", nil
	}
	return nil, "", fmt.Errorf("hooks.json not found in any search path or embedded data")
}

// hookLoadSettingsMap loads ~/.claude/settings.json then ~/.claude/hooks.json,
// returning the first parseable hooks map. It returns (nil, false) when neither
// yields a map. This mirrors checkHookCoverage's resolution order.
func hookLoadSettingsMap(env *DetectEnv) (map[string]any, bool) {
	if data, err := os.ReadFile(hookSettingsPath(env)); err == nil {
		if m, ok := hookExtractMap(data); ok {
			return m, true
		}
	}
	if data, err := os.ReadFile(hookSettingsJSONPath(env)); err == nil {
		if m, ok := hookExtractMap(data); ok {
			return m, true
		}
	}
	return nil, false
}

// hookIsParseableJSON reports whether data is parseable as a JSON value.
func hookIsParseableJSON(data []byte) bool {
	return json.Valid(data)
}

// hookCollectGroupCommands returns every command string under one event group.
func hookCollectGroupCommands(hooksMap map[string]any, event string) []string {
	var out []string
	groups, ok := hooksMap[event].([]any)
	if !ok {
		return out
	}
	for _, g := range groups {
		group, ok := g.(map[string]any)
		if !ok {
			continue
		}
		hooks, ok := group["hooks"].([]any)
		if !ok {
			continue
		}
		for _, h := range hooks {
			hook, ok := h.(map[string]any)
			if !ok {
				continue
			}
			if cmd, ok := hook["command"].(string); ok {
				out = append(out, cmd)
			}
		}
	}
	return out
}

// hookFullContractConfig returns the full HooksConfig from the resolved
// manifest with ${CLAUDE_PLUGIN_ROOT} replaced by installBase, plus the list
// of active events to install. It is the doctor's plan-only stand-in for
// generateHooksForInstall (which depends on the cobra hooksFull flag); the
// doctor always installs the full contract so a fix does not re-trigger
// fm-hooks-coverage-partial.
func hookFullContractConfig(env *DetectEnv) (*bridge.HooksConfig, []string, error) {
	data, _, err := hookFindManifest(env)
	if err != nil {
		return nil, nil, fmt.Errorf("find hooks manifest: %w", err)
	}
	cfg, err := bridge.ReadHooksManifest(data)
	if err != nil {
		return nil, nil, fmt.Errorf("parse hooks manifest: %w", err)
	}
	bridge.ReplacePluginRoot(cfg, hookInstallBase(env))
	active := bridge.ActiveEventNamesFromConfig(cfg)
	if len(active) == 0 {
		return nil, nil, fmt.Errorf("hooks manifest contains zero active events")
	}
	return cfg, active, nil
}

// hookCloneMap mirrors cmd/ao.cloneHooksMap: a shallow copy of an existing
// "hooks" object so the merge does not mutate the caller's map.
func hookCloneMap(rawSettings map[string]any) map[string]any {
	out := make(map[string]any)
	if existing, ok := rawSettings["hooks"].(map[string]any); ok {
		for k, v := range existing {
			out[k] = v
		}
	}
	return out
}

// hookFilterNonAoGroups mirrors cmd/ao.filterNonAoHookGroups: it returns the
// non-ao (foreign) hook groups for one event, so the merge preserves them.
func hookFilterNonAoGroups(hooksMap map[string]any, event string) []map[string]any {
	out := make([]map[string]any, 0)
	groups, ok := hooksMap[event].([]any)
	if !ok {
		return out
	}
	for _, g := range groups {
		group, ok := g.(map[string]any)
		if !ok {
			continue
		}
		if !bridge.RawGroupIsAoManaged(group) {
			out = append(out, group)
		}
	}
	return out
}

// hookMergeEvents mirrors cmd/ao.mergeHookEvents: for each event to install it
// keeps the foreign (non-ao) groups and appends the ao groups. Foreign hooks
// are never dropped; ao groups already present are replaced (not duplicated).
// It returns the number of events that received at least one ao group.
func hookMergeEvents(hooksMap map[string]any, newHooks *bridge.HooksConfig, eventsToInstall []string) int {
	installed := 0
	for _, event := range eventsToInstall {
		foreign := hookFilterNonAoGroups(hooksMap, event)
		newGroups := newHooks.GetEventGroups(event)
		merged := make([]any, 0, len(foreign)+len(newGroups))
		for _, fg := range foreign {
			merged = append(merged, fg)
		}
		for _, g := range newGroups {
			merged = append(merged, bridge.HookGroupToMap(g))
		}
		if len(newGroups) > 0 {
			hooksMap[event] = merged
			installed++
		}
	}
	return installed
}

// hookPlanMergedSettings builds the desired settings.json bytes by merging the
// full contract into the current settings map. It is pure (no disk writes).
// rawSettings is the parsed current settings object (or an empty object).
func hookPlanMergedSettings(env *DetectEnv, rawSettings map[string]any) ([]byte, error) {
	newHooks, events, err := hookFullContractConfig(env)
	if err != nil {
		return nil, err
	}
	hooksMap := hookCloneMap(rawSettings)
	hookMergeEvents(hooksMap, newHooks, events)
	desired := make(map[string]any, len(rawSettings)+1)
	for k, v := range rawSettings {
		desired[k] = v
	}
	desired["hooks"] = hooksMap
	out, err := json.MarshalIndent(desired, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal merged settings: %w", err)
	}
	return out, nil
}

// hookReadSettingsObject reads ~/.claude/settings.json into a JSON object. A
// missing file yields an empty object. An unparseable file is an error (the
// caller refuses — that case belongs to fm-hooks-settings-malformed).
func hookReadSettingsObject(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, fmt.Errorf("read settings: %w", err)
	}
	if strings.TrimSpace(string(data)) == "" {
		return map[string]any{}, nil
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, fmt.Errorf("settings.json is not a JSON object: %w", err)
	}
	return obj, nil
}

// ----------------------------------------------------------------------------
// fm-hooks-coverage-zero — no AgentOps hooks installed (P1, auto-fix)
// ----------------------------------------------------------------------------

// hooksCoverageZeroDetector flags a workspace with no installed contract events.
type hooksCoverageZeroDetector struct{}

func (hooksCoverageZeroDetector) ID() string              { return "fm-hooks-coverage-zero" }
func (hooksCoverageZeroDetector) Subsystem() string       { return "hooks" }
func (hooksCoverageZeroDetector) Severity() string        { return "P1" }
func (hooksCoverageZeroDetector) EstimatedCostMS() int    { return 5 }
func (hooksCoverageZeroDetector) OnlineRequired() bool    { return false }
func (hooksCoverageZeroDetector) QuickPath() bool         { return true }
func (hooksCoverageZeroDetector) Describe() string {
	return "Detects a Claude install with zero AgentOps hook events wired into settings.json."
}

// Detect reports fm-hooks-coverage-zero when no hooks map yields any installed
// contract event. It cedes to fm-hooks-settings-malformed when settings.json
// exists but is unparseable JSON.
func (d hooksCoverageZeroDetector) Detect(env *DetectEnv) ([]Finding, error) {
	contract := hookResolveContract(env)
	hooksMap, haveMap := hookLoadSettingsMap(env)

	installed := 0
	if haveMap {
		installed = bridge.CountInstalledEventsForList(hooksMap, contract.ActiveEvents)
	}
	if installed != 0 {
		return nil, nil
	}

	settings := hookSettingsPath(env)
	if data, err := os.ReadFile(settings); err == nil {
		if strings.TrimSpace(string(data)) != "" && !hookIsParseableJSON(data) {
			// Unparseable settings — fm-hooks-settings-malformed owns this.
			return nil, nil
		}
	}

	return []Finding{{
		ID:         d.ID(),
		Severity:   d.Severity(),
		Subsystem:  d.Subsystem(),
		Title:      fmt.Sprintf("No AgentOps hooks installed (0/%d contract events)", len(contract.ActiveEvents)),
		Confidence: 1.0,
		Evidence:   Evidence{File: settings, Query: "hooks key MISSING or empty"},
		Remediation: Remediation{
			Command:          "ao doctor --fix --only fm-hooks-coverage-zero",
			ExplainCommand:   "ao doctor explain fm-hooks-coverage-zero",
			AutoFixable:      true,
			EstimatedActions: 1,
		},
	}}, nil
}

// hooksCoverageZeroFixer materializes a full-contract hooks block in settings.json.
type hooksCoverageZeroFixer struct{}

func (hooksCoverageZeroFixer) ID() string              { return "fm-hooks-coverage-zero" }
func (hooksCoverageZeroFixer) Preconditions() []string {
	return []string{
		"~/.claude/settings.json is absent or parseable JSON",
		"a hooks coverage contract resolves (repo, home, or embedded manifest)",
	}
}
func (hooksCoverageZeroFixer) WritesTo() []string  { return []string{"~/.claude/settings.json"} }
func (hooksCoverageZeroFixer) Ops() []string       { return []string{"WriteFile"} }
func (hooksCoverageZeroFixer) Reversible() bool    { return true }
func (hooksCoverageZeroFixer) Idempotent() bool    { return true }
func (hooksCoverageZeroFixer) AutoFixable() bool   { return true }

// Fix writes the merged full-contract settings.json through Mutate. It refuses
// (exit 4) if settings.json exists but is unparseable JSON.
func (f hooksCoverageZeroFixer) Fix(ctx *MutateContext, env *DetectEnv, findings []Finding) (FixResult, error) {
	return hookWriteMergedSettings(ctx, env, f.ID())
}

// ----------------------------------------------------------------------------
// fm-hooks-coverage-partial — only a subset of contract events wired (P2, auto-fix)
// ----------------------------------------------------------------------------

// hooksCoveragePartialDetector flags a partial hook install.
type hooksCoveragePartialDetector struct{}

func (hooksCoveragePartialDetector) ID() string           { return "fm-hooks-coverage-partial" }
func (hooksCoveragePartialDetector) Subsystem() string    { return "hooks" }
func (hooksCoveragePartialDetector) Severity() string     { return "P2" }
func (hooksCoveragePartialDetector) EstimatedCostMS() int { return 5 }
func (hooksCoveragePartialDetector) OnlineRequired() bool { return false }
func (hooksCoveragePartialDetector) QuickPath() bool      { return true }
func (hooksCoveragePartialDetector) Describe() string {
	return "Detects an AgentOps hook install covering only a subset of the active contract events."
}

// Detect reports fm-hooks-coverage-partial when at least one event is installed
// and SessionStart is ao-managed but fewer than the full contract is wired. It
// cedes to coverage-zero (no install) and non-ao-shadow (foreign SessionStart).
func (d hooksCoveragePartialDetector) Detect(env *DetectEnv) ([]Finding, error) {
	contract := hookResolveContract(env)
	hooksMap, haveMap := hookLoadSettingsMap(env)
	if !haveMap {
		return nil, nil
	}
	total := len(contract.ActiveEvents)
	installed := bridge.CountInstalledEventsForList(hooksMap, contract.ActiveEvents)
	if installed == 0 {
		return nil, nil // coverage-zero owns it
	}
	if !bridge.HookGroupContainsAo(hooksMap, "SessionStart") {
		return nil, nil // non-ao-shadow owns it
	}
	if installed >= total {
		return nil, nil // full coverage
	}

	missing := hookMissingEvents(hooksMap, contract.ActiveEvents)
	return []Finding{{
		ID:         d.ID(),
		Severity:   d.Severity(),
		Subsystem:  d.Subsystem(),
		Title:      fmt.Sprintf("Partial hook coverage: %d/%d events (missing: %s)", installed, total, strings.Join(missing, ", ")),
		Confidence: 1.0,
		Evidence:   Evidence{File: hookSettingsPath(env), Query: strings.Join(missing, ",")},
		Remediation: Remediation{
			Command:          "ao doctor --fix --only fm-hooks-coverage-partial",
			ExplainCommand:   "ao doctor explain fm-hooks-coverage-partial",
			AutoFixable:      true,
			EstimatedActions: 1,
		},
	}}, nil
}

// hookMissingEvents returns the contract events with no installed hook group.
func hookMissingEvents(hooksMap map[string]any, active []string) []string {
	var missing []string
	for _, event := range active {
		if groups, ok := hooksMap[event].([]any); !ok || len(groups) == 0 {
			missing = append(missing, event)
		}
	}
	return missing
}

// hooksCoveragePartialFixer fills in the missing contract events.
type hooksCoveragePartialFixer struct{}

func (hooksCoveragePartialFixer) ID() string { return "fm-hooks-coverage-partial" }
func (hooksCoveragePartialFixer) Preconditions() []string {
	return []string{
		"~/.claude/settings.json is parseable JSON with an object-typed hooks key",
		"fm-hooks-contract-fallback resolved first (a fallback contract can inflate the gap)",
	}
}
func (hooksCoveragePartialFixer) WritesTo() []string { return []string{"~/.claude/settings.json"} }
func (hooksCoveragePartialFixer) Ops() []string      { return []string{"WriteFile"} }
func (hooksCoveragePartialFixer) Reversible() bool   { return true }
func (hooksCoveragePartialFixer) Idempotent() bool   { return true }
func (hooksCoveragePartialFixer) AutoFixable() bool  { return true }

// Fix merges the missing contract events into settings.json through Mutate.
func (f hooksCoveragePartialFixer) Fix(ctx *MutateContext, env *DetectEnv, findings []Finding) (FixResult, error) {
	return hookWriteMergedSettings(ctx, env, f.ID())
}

// hookWriteMergedSettings is the shared coverage-zero / coverage-partial fixer
// body. It re-reads settings.json (refusing on unparseable JSON), plans the
// merged full-contract bytes in memory, and issues a single WriteFile Mutate.
// A second run with identical desired bytes issues zero Mutate calls.
func hookWriteMergedSettings(ctx *MutateContext, env *DetectEnv, fixerID string) (FixResult, error) {
	settings := hookSettingsPath(env)
	raw, err := hookReadSettingsObject(settings)
	if err != nil {
		// Unparseable JSON — refuse; fm-hooks-settings-malformed owns this.
		return FixResult{FixerID: fixerID, Err: err},
			fmt.Errorf("doctor: refused_unsafe: %w", err)
	}
	desired, err := hookPlanMergedSettings(env, raw)
	if err != nil {
		return FixResult{FixerID: fixerID, Err: err},
			fmt.Errorf("doctor: %s: plan merge: %w", fixerID, err)
	}
	current, err := readOrEmpty(settings)
	if err != nil {
		return FixResult{FixerID: fixerID, Err: err}, err
	}
	if string(desired) == string(current) {
		return FixResult{FixerID: fixerID, FindingIDs: []string{fixerID}, Fixed: true}, nil
	}
	res, err := Mutate(ctx, settings, WriteFile{Content: desired, Mode: 0o600})
	if err != nil {
		return FixResult{FixerID: fixerID, Err: err}, err
	}
	actions := 0
	if res.OK {
		actions = 1
	}
	return FixResult{
		FixerID:      fixerID,
		FindingIDs:   []string{fixerID},
		ActionsTaken: actions,
		Fixed:        res.OK,
	}, nil
}

// ----------------------------------------------------------------------------
// fm-hooks-contract-fallback — manifest unresolvable, coverage falls back (P3, auto-fix)
// ----------------------------------------------------------------------------

// hooksContractFallbackDetector flags a coverage contract that fell back to the
// all-12-event set.
type hooksContractFallbackDetector struct{}

func (hooksContractFallbackDetector) ID() string           { return "fm-hooks-contract-fallback" }
func (hooksContractFallbackDetector) Subsystem() string    { return "hooks" }
func (hooksContractFallbackDetector) Severity() string     { return "P3" }
func (hooksContractFallbackDetector) EstimatedCostMS() int { return 5 }
func (hooksContractFallbackDetector) OnlineRequired() bool { return false }
func (hooksContractFallbackDetector) QuickPath() bool      { return true }
func (hooksContractFallbackDetector) Describe() string {
	return "Detects a hook coverage contract that fell back to the all-12-event set because no manifest resolved."
}

// Detect reports fm-hooks-contract-fallback when the manifest cannot be found,
// is unparseable, or declares zero active events.
func (d hooksContractFallbackDetector) Detect(env *DetectEnv) ([]Finding, error) {
	contract := hookResolveContract(env)
	if contract.FallbackReason == "" {
		return nil, nil // contract resolved cleanly
	}
	_, source, _ := hookFindManifest(env)
	usedEmbedded := source == "embedded"

	return []Finding{{
		ID:         d.ID(),
		Severity:   d.Severity(),
		Subsystem:  d.Subsystem(),
		Title:      "Hook coverage contract fell back to all-12 events: " + contract.FallbackReason,
		Confidence: 1.0,
		Evidence: Evidence{
			File:  hookHomeManifestPath(env),
			Query: fmt.Sprintf("fallback_reason=%q used_embedded=%t", contract.FallbackReason, usedEmbedded),
		},
		Remediation: Remediation{
			Command:          "ao doctor --fix --only fm-hooks-contract-fallback",
			ExplainCommand:   "ao doctor explain fm-hooks-contract-fallback",
			AutoFixable:      true,
			EstimatedActions: 1,
		},
	}}, nil
}

// hooksContractFallbackFixer materializes ~/.agentops/hooks.json from the
// canonical repo manifest, or the embedded blob, so the contract resolves.
type hooksContractFallbackFixer struct{}

func (hooksContractFallbackFixer) ID() string { return "fm-hooks-contract-fallback" }
func (hooksContractFallbackFixer) Preconditions() []string {
	return []string{
		"a valid manifest source exists (repo hooks/hooks.json or embedded blob with >=1 active event)",
		"~/.agentops/ is writable",
	}
}
func (hooksContractFallbackFixer) WritesTo() []string { return []string{"~/.agentops/hooks.json"} }
func (hooksContractFallbackFixer) Ops() []string      { return []string{"WriteFile"} }
func (hooksContractFallbackFixer) Reversible() bool   { return true }
func (hooksContractFallbackFixer) Idempotent() bool   { return true }
func (hooksContractFallbackFixer) AutoFixable() bool  { return true }

// Fix materializes ~/.agentops/hooks.json through Mutate. It prefers the repo's
// canonical hooks/hooks.json, falling back to the embedded blob. It refuses
// (exit 4) if no source parses or the source has zero active events.
func (f hooksContractFallbackFixer) Fix(ctx *MutateContext, env *DetectEnv, findings []Finding) (FixResult, error) {
	desired, err := hookManifestSourceBytes(env)
	if err != nil {
		return FixResult{FixerID: f.ID(), Err: err},
			fmt.Errorf("doctor: refused_unsafe: %s: %w", f.ID(), err)
	}
	homeManifest := hookHomeManifestPath(env)
	current, err := readOrEmpty(homeManifest)
	if err != nil {
		return FixResult{FixerID: f.ID(), Err: err}, err
	}
	if string(desired) == string(current) {
		return FixResult{FixerID: f.ID(), FindingIDs: []string{f.ID()}, Fixed: true}, nil
	}
	res, err := Mutate(ctx, homeManifest, WriteFile{Content: desired, Mode: 0o644})
	if err != nil {
		return FixResult{FixerID: f.ID(), Err: err}, err
	}
	actions := 0
	if res.OK {
		actions = 1
	}
	return FixResult{
		FixerID:      f.ID(),
		FindingIDs:   []string{f.ID()},
		ActionsTaken: actions,
		Fixed:        res.OK,
	}, nil
}

// hookManifestSourceBytes returns validated manifest bytes for materialization:
// the repo's hooks/hooks.json if present, else the embedded blob. The chosen
// bytes are parsed and required to declare at least one active event.
func hookManifestSourceBytes(env *DetectEnv) ([]byte, error) {
	var chosen []byte
	repoManifest := filepath.Join(env.RepoRoot, "hooks", "hooks.json")
	if data, err := os.ReadFile(repoManifest); err == nil {
		chosen = data
	} else if len(embedded.HooksJSON) > 0 {
		chosen = append([]byte(nil), embedded.HooksJSON...)
	} else {
		return nil, fmt.Errorf("no manifest source: repo hooks/hooks.json absent and embedded blob empty")
	}
	cfg, err := bridge.ReadHooksManifest(chosen)
	if err != nil {
		return nil, fmt.Errorf("manifest source invalid: %w", err)
	}
	if len(bridge.ActiveEventNamesFromConfig(cfg)) == 0 {
		return nil, fmt.Errorf("manifest source has zero active events; cannot self-heal")
	}
	return chosen, nil
}

// ----------------------------------------------------------------------------
// fm-hooks-non-ao-shadow — SessionStart occupied by foreign hooks (P2, gated)
// ----------------------------------------------------------------------------

// hooksNonAoShadowDetector flags a SessionStart slot held by foreign hooks.
type hooksNonAoShadowDetector struct{}

func (hooksNonAoShadowDetector) ID() string           { return "fm-hooks-non-ao-shadow" }
func (hooksNonAoShadowDetector) Subsystem() string    { return "hooks" }
func (hooksNonAoShadowDetector) Severity() string     { return "P2" }
func (hooksNonAoShadowDetector) EstimatedCostMS() int { return 5 }
func (hooksNonAoShadowDetector) OnlineRequired() bool { return false }
func (hooksNonAoShadowDetector) QuickPath() bool      { return true }
func (hooksNonAoShadowDetector) Describe() string {
	return "Detects a SessionStart slot occupied only by non-ao hooks, suppressing the AgentOps lifecycle."
}

// Detect reports fm-hooks-non-ao-shadow when events are installed but no
// SessionStart group is ao-managed. It cedes to coverage-zero (no install).
func (d hooksNonAoShadowDetector) Detect(env *DetectEnv) ([]Finding, error) {
	contract := hookResolveContract(env)
	hooksMap, haveMap := hookLoadSettingsMap(env)
	if !haveMap {
		return nil, nil
	}
	installed := bridge.CountInstalledEventsForList(hooksMap, contract.ActiveEvents)
	if installed == 0 {
		return nil, nil // coverage-zero owns it
	}
	if bridge.HookGroupContainsAo(hooksMap, "SessionStart") {
		return nil, nil // SessionStart is ao-managed — not shadowed
	}

	foreign := hookCollectGroupCommands(hooksMap, "SessionStart")
	return []Finding{{
		ID:         d.ID(),
		Severity:   d.Severity(),
		Subsystem:  d.Subsystem(),
		Title:      "Non-ao hooks detected: SessionStart slot occupied by foreign hooks",
		Confidence: 1.0,
		Evidence: Evidence{
			File:  hookSettingsPath(env),
			Query: "SessionStart commands: " + strings.Join(foreign, " | "),
		},
		Remediation: Remediation{
			Command:          "ao doctor --fix --only fm-hooks-non-ao-shadow --confirm-foreign-merge",
			ExplainCommand:   "ao doctor explain fm-hooks-non-ao-shadow",
			AutoFixable:      false, // gated behind --confirm-foreign-merge
			EstimatedActions: 1,
		},
	}}, nil
}

// hooksNonAoShadowFixer additively merges ao hooks alongside the foreign group.
// It is gated: AutoFixable() is false, so the engine never runs it as part of a
// blanket --fix. It runs only when invoked directly with the confirm flag set.
type hooksNonAoShadowFixer struct{}

func (hooksNonAoShadowFixer) ID() string { return "fm-hooks-non-ao-shadow" }
func (hooksNonAoShadowFixer) Preconditions() []string {
	return []string{
		"--confirm-foreign-merge supplied (env AO_DOCTOR_CONFIRM_FOREIGN_MERGE)",
		"~/.claude/settings.json is parseable JSON with an object-typed hooks key",
		"the merge is provably additive (foreign SessionStart commands preserved)",
	}
}
func (hooksNonAoShadowFixer) WritesTo() []string { return []string{"~/.claude/settings.json"} }
func (hooksNonAoShadowFixer) Ops() []string      { return []string{"WriteFile"} }
func (hooksNonAoShadowFixer) Reversible() bool   { return true }
func (hooksNonAoShadowFixer) Idempotent() bool   { return true }

// AutoFixable reports false: this fixer is gated. Editing a third-party hook
// config is unsafe to do as part of a blanket --fix, so the engine advertises
// it as detect-only and skips it in applyFixers. It runs only when invoked
// directly with AO_DOCTOR_CONFIRM_FOREIGN_MERGE set.
func (hooksNonAoShadowFixer) AutoFixable() bool { return false }

// Fix additively merges ao hooks into settings.json, preserving the foreign
// SessionStart group. It refuses (exit 4) unless AO_DOCTOR_CONFIRM_FOREIGN_MERGE
// is set, and refuses if the merge would drop a foreign command.
func (f hooksNonAoShadowFixer) Fix(ctx *MutateContext, env *DetectEnv, findings []Finding) (FixResult, error) {
	if os.Getenv(hookEnvConfirmForeignMerge) == "" {
		return FixResult{FixerID: f.ID(), Fixed: false},
			fmt.Errorf("doctor: refused_unsafe: SessionStart contains non-ao hooks; " +
				"pass --confirm-foreign-merge to additively merge ao hooks alongside the foreign group")
	}
	settings := hookSettingsPath(env)
	raw, err := hookReadSettingsObject(settings)
	if err != nil {
		return FixResult{FixerID: f.ID(), Err: err},
			fmt.Errorf("doctor: refused_unsafe: %w", err)
	}

	// Capture the foreign SessionStart commands so we can prove additivity.
	beforeSession := hookCollectGroupCommands(raw, "SessionStart")

	desired, err := hookPlanMergedSettings(env, raw)
	if err != nil {
		return FixResult{FixerID: f.ID(), Err: err},
			fmt.Errorf("doctor: %s: plan merge: %w", f.ID(), err)
	}

	// Assert the merge is additive: every foreign command must survive.
	var mergedObj map[string]any
	if err := json.Unmarshal(desired, &mergedObj); err != nil {
		return FixResult{FixerID: f.ID(), Err: err}, err
	}
	mergedHooks, _ := mergedObj["hooks"].(map[string]any)
	afterSession := hookCollectGroupCommands(mergedHooks, "SessionStart")
	if !hookCommandsSubset(beforeSession, afterSession) {
		return FixResult{FixerID: f.ID(), Fixed: false},
			fmt.Errorf("doctor: refused_unsafe: merge would drop a foreign SessionStart command")
	}

	current, err := readOrEmpty(settings)
	if err != nil {
		return FixResult{FixerID: f.ID(), Err: err}, err
	}
	if string(desired) == string(current) {
		return FixResult{FixerID: f.ID(), FindingIDs: []string{f.ID()}, Fixed: true}, nil
	}
	res, err := Mutate(ctx, settings, WriteFile{Content: desired, Mode: 0o600})
	if err != nil {
		return FixResult{FixerID: f.ID(), Err: err}, err
	}
	actions := 0
	if res.OK {
		actions = 1
	}
	return FixResult{
		FixerID:      f.ID(),
		FindingIDs:   []string{f.ID()},
		ActionsTaken: actions,
		Fixed:        res.OK,
	}, nil
}

// hookCommandsSubset reports whether every command in want appears in have.
func hookCommandsSubset(want, have []string) bool {
	set := make(map[string]struct{}, len(have))
	for _, c := range have {
		set[c] = struct{}{}
	}
	for _, c := range want {
		if _, ok := set[c]; !ok {
			return false
		}
	}
	return true
}

// ----------------------------------------------------------------------------
// fm-hooks-settings-malformed — unparseable settings.json (P2, detect-only)
// ----------------------------------------------------------------------------

// hooksSettingsMalformedDetector flags an unparseable ~/.claude/settings.json.
type hooksSettingsMalformedDetector struct{}

func (hooksSettingsMalformedDetector) ID() string           { return "fm-hooks-settings-malformed" }
func (hooksSettingsMalformedDetector) Subsystem() string    { return "hooks" }
func (hooksSettingsMalformedDetector) Severity() string     { return "P2" }
func (hooksSettingsMalformedDetector) EstimatedCostMS() int { return 5 }
func (hooksSettingsMalformedDetector) OnlineRequired() bool { return false }
func (hooksSettingsMalformedDetector) QuickPath() bool      { return true }
func (hooksSettingsMalformedDetector) Describe() string {
	return "Detects an unparseable ~/.claude/settings.json, which makes Claude silently skip all hooks."
}

// Detect reports fm-hooks-settings-malformed when settings.json exists, is
// non-empty, and is either unparseable JSON or parses but has a wrong-typed
// "hooks" key. It classifies the corruption precisely. It is detect-only.
func (d hooksSettingsMalformedDetector) Detect(env *DetectEnv) ([]Finding, error) {
	settings := hookSettingsPath(env)
	data, err := os.ReadFile(settings)
	if err != nil {
		return nil, nil // absent / unreadable — coverage-zero or out of scope
	}
	if strings.TrimSpace(string(data)) == "" {
		return nil, nil // empty — coverage-zero owns it
	}

	reason, ok := hookClassifyMalformed(data)
	if !ok {
		return nil, nil // valid JSON, hooks ok/absent — not this FM
	}
	kind := hookCorruptionKind(data, reason)

	return []Finding{{
		ID:         d.ID(),
		Severity:   d.Severity(),
		Subsystem:  d.Subsystem(),
		Title:      "Malformed ~/.claude/settings.json (" + kind + "): " + reason,
		Confidence: 1.0,
		Evidence: Evidence{
			File:  settings,
			Hash:  sha256Hex(data),
			Query: "corruption=" + kind,
		},
		Remediation: Remediation{
			Command:          "(manual) edit ~/.claude/settings.json to valid JSON, then: ao hooks install --force",
			ExplainCommand:   "ao doctor explain fm-hooks-settings-malformed",
			AutoFixable:      false, // detect-only — no safe machine merge
			EstimatedActions: 0,
		},
	}}, nil
}

// hookClassifyMalformed returns a parse-error reason and true when data is
// unparseable JSON, or parses but has a non-object "hooks" key. It returns
// ("", false) when the file is valid JSON with an ok/absent hooks key.
func hookClassifyMalformed(data []byte) (string, bool) {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return "invalid JSON: " + err.Error(), true
	}
	obj, ok := value.(map[string]any)
	if !ok {
		// A top-level non-object (array, string, number) is itself malformed
		// for a settings file.
		return fmt.Sprintf("top-level JSON is %s — expected object", hookJSONTypeName(value)), true
	}
	hooks, present := obj["hooks"]
	if !present {
		return "", false // valid JSON, no hooks key — coverage-zero territory
	}
	if _, isObj := hooks.(map[string]any); !isObj {
		return fmt.Sprintf("\"hooks\" is %s — expected object", hookJSONTypeName(hooks)), true
	}
	return "", false
}

// hookJSONTypeName returns a human-readable name for a decoded JSON value.
func hookJSONTypeName(v any) string {
	switch v.(type) {
	case map[string]any:
		return "object"
	case []any:
		return "array"
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "boolean"
	case nil:
		return "null"
	default:
		return "unknown"
	}
}

// hookCorruptionKind classifies the corruption for a precise report.
func hookCorruptionKind(data []byte, reason string) string {
	switch {
	case hookContainsAny(data, "<<<<<<<", "=======", ">>>>>>>"):
		return "git-merge-conflict-markers"
	case hookHasBOM(data):
		return "utf8-bom-prefix"
	case strings.HasPrefix(reason, "\"hooks\""):
		return "wrong-hooks-type"
	case strings.HasPrefix(reason, "top-level JSON is"):
		return "wrong-toplevel-type"
	case hookLooksTruncated(data):
		return "truncated-write"
	default:
		return "syntax-error"
	}
}

// hookContainsAny reports whether data contains any of the given substrings.
func hookContainsAny(data []byte, subs ...string) bool {
	s := string(data)
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// hookHasBOM reports whether data begins with a UTF-8 byte-order mark.
func hookHasBOM(data []byte) bool {
	return len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF
}

// hookLooksTruncated reports whether data appears to be a truncated JSON write
// (unbalanced braces/brackets, where the file does not end on a closing token).
func hookLooksTruncated(data []byte) bool {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return false
	}
	last := trimmed[len(trimmed)-1]
	if last == '}' || last == ']' {
		return false
	}
	depth := 0
	inString := false
	escaped := false
	for _, r := range trimmed {
		switch {
		case escaped:
			escaped = false
		case r == '\\' && inString:
			escaped = true
		case r == '"':
			inString = !inString
		case inString:
			// ignore structural chars inside strings
		case r == '{' || r == '[':
			depth++
		case r == '}' || r == ']':
			depth--
		}
	}
	return depth > 0 || inString
}

// hooksSettingsMalformedFixer is detect-only: it always refuses. Unparseable
// JSON has no machine-recoverable intent, so a fix would risk silently
// discarding the user's model / permissions / env settings.
type hooksSettingsMalformedFixer struct{}

func (hooksSettingsMalformedFixer) ID() string { return "fm-hooks-settings-malformed" }
func (hooksSettingsMalformedFixer) Preconditions() []string {
	return []string{"settings.json must be repaired by hand; the doctor will not guess a merge"}
}
func (hooksSettingsMalformedFixer) WritesTo() []string { return nil }
func (hooksSettingsMalformedFixer) Ops() []string      { return nil }
func (hooksSettingsMalformedFixer) Reversible() bool   { return true }
func (hooksSettingsMalformedFixer) Idempotent() bool   { return true }

// AutoFixable reports false: this FM is detect-only. There is no safe machine
// merge into invalid JSON.
func (hooksSettingsMalformedFixer) AutoFixable() bool { return false }

// Fix always refuses with exit 4. Even behind AO_DOCTOR_ALLOW_UNSAFE_REWRITE the
// doctor does not reconstruct the JSON; the operator must repair the file by
// hand (the recommended path) or quarantine it manually. Refusing here keeps
// the fixer faithful to the spec's "no safe automatic merge exists" mandate.
func (f hooksSettingsMalformedFixer) Fix(ctx *MutateContext, env *DetectEnv, findings []Finding) (FixResult, error) {
	return FixResult{FixerID: f.ID(), Fixed: false},
		fmt.Errorf("doctor: refused_unsafe: settings.json is unparseable JSON; " +
			"doctor will not guess a merge. Repair the file by hand, then run: ao hooks install --force")
}

// ----------------------------------------------------------------------------
// Registration
// ----------------------------------------------------------------------------

func init() {
	RegisterDetector(hooksCoverageZeroDetector{})
	RegisterDetector(hooksCoveragePartialDetector{})
	RegisterDetector(hooksContractFallbackDetector{})
	RegisterDetector(hooksNonAoShadowDetector{})
	RegisterDetector(hooksSettingsMalformedDetector{})

	RegisterFixer(hooksCoverageZeroFixer{})
	RegisterFixer(hooksCoveragePartialFixer{})
	RegisterFixer(hooksContractFallbackFixer{})
	RegisterFixer(hooksNonAoShadowFixer{})
	RegisterFixer(hooksSettingsMalformedFixer{})
}
