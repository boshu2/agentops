package gcmaintainer

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/BurntSushi/toml"
	"github.com/boshu2/agentops/cli/internal/storage"
)

// Codex gates project-local configuration behind two independent, persisted
// trust decisions, both stored in `$CODEX_HOME/config.toml`:
//
//  1. Workspace trust — `[projects."<dir>"] trust_level = "trusted"`. Any other
//     level (including an explicit "untrusted") makes Codex silently report the
//     directory as having no hooks at all, with no warning and no error.
//  2. Hook trust — `[hooks.state."<hooks.json>:<event>:<matcher>:<hook>"]
//     trusted_hash = "sha256:..."`, one entry per individual hook.
//
// A Gas City agent session home carries a `.codex/hooks.json`, so a first Codex
// session in a fresh session home can hit the interactive hooks-trust dialog
// ("Press t to trust all") and wait indefinitely: dispatches queue as pending
// with no active worker, and the stall is invisible except through pane capture.
//
// Presence of a table is NOT trust. A `trust_level` that is not "trusted", a
// missing or empty `trusted_hash`, or a hook Codex reports as "modified" all
// leave the wedge in place, so both prepare and check compare actual values.
//
// The `trusted_hash` value is derived from hook content by Codex itself and is
// deliberately not reimplemented here — guessing the digest input is how this
// silently rots on the next Codex release. prepare reads the exact identity
// back from Codex's own `hooks/list` app-server method. check never runs a
// subprocess: it derives the expected hook keys from each `hooks.json` and
// verifies the stored values, so it stays a pure read of local state.
//
// Empirically (codex-cli 0.145.0) the hash is semantic: the same logical hooks
// re-serialized with different key order or indentation produce the identical
// hash. Only the key carries the path.
const (
	// codexTrustScanDepth bounds the search for Gas City session directories
	// below the city's `.gc/agents` and `.gc/worktrees` roots.
	codexTrustScanDepth = 4
	// codexHookListTimeout bounds one `codex app-server` hooks/list exchange.
	codexHookListTimeout = 90 * time.Second
	// codexTrustedLevel is the only workspace trust level that loads a
	// directory's project-local Codex configuration.
	codexTrustedLevel = "trusted"
	// codexWedgeHint explains, in one line, why a missing pre-seed matters.
	codexWedgeHint = "the first codex session there blocks on the interactive trust dialog and the agent pane wedges with no visible error"
)

// codexHook is one hook identity as Codex itself reports it.
type codexHook struct {
	Key         string `json:"key"`
	CurrentHash string `json:"currentHash"`
	TrustStatus string `json:"trustStatus"`
}

// hookTrust is one recorded `[hooks.state."..."]` decision.
type hookTrust struct {
	TrustedHash string `toml:"trusted_hash"`
	Enabled     *bool  `toml:"enabled"`
}

// projectTrust is one recorded `[projects."..."]` decision.
type projectTrust struct {
	TrustLevel string `toml:"trust_level"`
}

// trustStore is the decoded Codex trust state. It is read with a real TOML
// parser so every valid spelling of a table header (trailing comments,
// single-quoted keys, literal strings) is understood; writes stay append-only
// and happen only where a real parse proved the entry absent.
type trustStore struct {
	Projects map[string]projectTrust `toml:"projects"`
	Hooks    struct {
		State map[string]hookTrust `toml:"state"`
	} `toml:"hooks"`
}

// workspaceTrusted reports whether dir is actually trusted, not merely listed.
func (s *trustStore) workspaceTrusted(dir string) bool {
	return s.Projects[dir].TrustLevel == codexTrustedLevel
}

// workspaceLevel returns the recorded level for dir, or "" when absent.
func (s *trustStore) workspaceLevel(dir string) string {
	return s.Projects[dir].TrustLevel
}

// hookTrustSatisfied is THE completeness rule. prepare seeds until it holds
// for every hook and check requires it for every derived key, so whatever one
// accepts as done the other accepts too - including a hook Codex reports as
// `managed`, which prepare records so check can round-trip it without asking
// Codex.
//
// A hook recorded with `enabled = false` is not satisfied: it is a disabled
// hook, not a trusted working one, and treating it as done would hide it.
func (s *trustStore) hookTrustSatisfied(key string) bool {
	return s.hookTrustDefect(key) == ""
}

// hookTrustDefect names why key is not satisfied, or "" when it is. Both
// commands report using this text so they never disagree about what is wrong.
func (s *trustStore) hookTrustDefect(key string) string {
	entry, ok := s.Hooks.State[key]
	switch {
	case !ok:
		return "has no recorded trust entry"
	case strings.TrimSpace(entry.TrustedHash) == "":
		return "has a trust entry with no usable trusted_hash"
	case entry.Enabled != nil && !*entry.Enabled:
		return "is recorded as enabled = false, so it is not a trusted working hook"
	default:
		return ""
	}
}

// hookRecorded reports whether key has any recorded decision at all.
func (s *trustStore) hookRecorded(key string) bool {
	_, ok := s.Hooks.State[key]
	return ok
}

// readCodexTrustStore decodes the trust store with a real TOML parser. A
// missing store is an empty one; an unparsable store is an error, never an
// empty result that would read as "nothing is trusted yet".
func readCodexTrustStore(path string) (*trustStore, error) {
	store := &trustStore{Projects: map[string]projectTrust{}}
	store.Hooks.State = map[string]hookTrust{}
	data, err := os.ReadFile(path) // #nosec G304 -- operator-owned codex trust store resolved from CODEX_HOME.
	if err != nil {
		if os.IsNotExist(err) {
			return store, nil
		}
		return nil, fmt.Errorf("read codex trust store %s: %w", path, err)
	}
	if err := toml.Unmarshal(data, store); err != nil {
		return nil, fmt.Errorf("parse codex trust store %s: %w", path, err)
	}
	if store.Projects == nil {
		store.Projects = map[string]projectTrust{}
	}
	if store.Hooks.State == nil {
		store.Hooks.State = map[string]hookTrust{}
	}
	return store, nil
}

// resolveCodexHome returns the Codex home directory that holds config.toml.
// It is not required to exist yet; prepare creates it.
func resolveCodexHome() (string, error) {
	if explicit := os.Getenv("CODEX_HOME"); explicit != "" {
		return filepath.Abs(explicit)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("HOME is required to locate the codex trust store: %w", err)
	}
	return filepath.Join(home, ".codex"), nil
}

// resolveCodexBin resolves the Codex CLI prepare uses to read hook identities.
// An explicit path must be executable; auto-detection degrades to "".
func resolveCodexBin(explicit string) (string, error) {
	if explicit != "" {
		if !isExecutableFile(explicit) {
			return "", fmt.Errorf("codex binary is not executable: %s", explicit)
		}
		return canonical(explicit)
	}
	bin, err := exec.LookPath("codex")
	if err != nil || !isExecutableFile(bin) {
		return "", nil
	}
	resolved, err := canonical(bin)
	if err != nil {
		return "", nil
	}
	return resolved, nil
}

// codexTrustTargets lists the directories a Gas City provider session runs in:
// the city and rig roots, every agent session home, and every rig worktree
// root. Discovery is by filesystem marker, so it covers whatever the city has
// materialized; homes created later are covered by the next prepare.
func (o *ops) codexTrustTargets() []string {
	seen := map[string]bool{}
	var targets []string
	add := func(dir string) {
		resolved, err := canonical(dir)
		if err != nil || seen[resolved] {
			return
		}
		seen[resolved] = true
		targets = append(targets, resolved)
	}
	add(o.city)
	add(o.rig)

	var discovered []string
	for _, root := range []string{
		filepath.Join(o.city, ".gc", "agents"),
		filepath.Join(o.city, ".gc", "worktrees"),
	} {
		discovered = append(discovered, sessionDirsUnder(root)...)
	}
	sort.Strings(discovered)
	for _, dir := range discovered {
		add(dir)
	}
	return targets
}

// sessionDirsUnder finds directories below root that look like a workspace a
// Codex session is launched in. A match is not descended into, which keeps the
// walk off the inside of rig worktrees.
func sessionDirsUnder(root string) []string {
	if !isDir(root) {
		return nil
	}
	var found []string
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry == nil {
			return nil //nolint:nilerr // an unreadable subtree is skipped, not fatal
		}
		if !entry.IsDir() || path == root {
			return nil
		}
		if strings.HasPrefix(entry.Name(), ".") {
			return fs.SkipDir
		}
		if isSessionDir(path) {
			found = append(found, path)
			return fs.SkipDir
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return fs.SkipDir
		}
		if len(strings.Split(rel, string(filepath.Separator))) >= codexTrustScanDepth {
			return fs.SkipDir
		}
		return nil
	})
	return found
}

// isSessionDir reports whether path carries a marker that makes it a directory
// a Codex session is launched in.
func isSessionDir(path string) bool {
	for _, marker := range []string{".codex", ".git"} {
		if _, err := os.Lstat(filepath.Join(path, marker)); err == nil {
			return true
		}
	}
	return false
}

// hookFile returns the project-local hooks file for a session directory.
func hookFile(dir string) string {
	return filepath.Join(dir, ".codex", "hooks.json")
}

// seedCodexTrust pre-seeds workspace trust and hook trust for every Gas City
// session directory. It appends only entries the store is missing, so an
// operator decision is never rewritten; an entry that exists but does not
// actually confer trust is a hard error naming that entry, because silently
// skipping it would report success while leaving the wedge in place.
func (o *ops) seedCodexTrust() error {
	targets := o.codexTrustTargets()
	store, err := readCodexTrustStore(o.codexConfig)
	if err != nil {
		return err
	}

	// Workspace trust first: Codex reports no hooks at all for a directory
	// whose trust level is anything other than "trusted".
	var block strings.Builder
	addedProjects := 0
	for _, dir := range targets {
		if store.workspaceTrusted(dir) {
			continue
		}
		if level := store.workspaceLevel(dir); level != "" {
			return fmt.Errorf("codex workspace %s is recorded as trust_level %q, not %q; "+
				"refusing to overwrite that decision - fix the [projects] entry in %s (%s)",
				dir, level, codexTrustedLevel, o.codexConfig, codexWedgeHint)
		}
		addedProjects++
		fmt.Fprintf(&block, "\n[projects.%s]\ntrust_level = \"trusted\"\n", tomlQuote(dir))
	}
	if block.Len() > 0 {
		if err := appendCodexConfig(o.codexConfig, block.String()); err != nil {
			return err
		}
	}

	entries, err := o.listCodexHooks(targets)
	if err != nil {
		return err
	}
	var hookBlock strings.Builder
	addedHooks := 0
	for _, entry := range entries {
		if entry.TrustStatus == "modified" {
			return fmt.Errorf("codex hook %s changed since it was trusted; "+
				"refusing to re-trust changed hook content automatically - review it and re-trust in codex (%s)",
				entry.Key, codexWedgeHint)
		}
		if store.hookTrustSatisfied(entry.Key) {
			if entry.TrustStatus == "untrusted" {
				return fmt.Errorf("codex hook %s has a recorded trust entry that codex still reports as %q; "+
					"refusing to overwrite that decision - fix the [hooks.state] entry in %s (%s)",
					entry.Key, entry.TrustStatus, o.codexConfig, codexWedgeHint)
			}
			continue
		}
		if store.hookRecorded(entry.Key) {
			return fmt.Errorf("codex hook %s %s; "+
				"refusing to overwrite that decision - fix the [hooks.state] entry in %s (%s)",
				entry.Key, store.hookTrustDefect(entry.Key), o.codexConfig, codexWedgeHint)
		}
		// A `managed` hook is recorded too, so `check` - which never asks
		// Codex - can confirm it by the same rule prepare just applied.
		addedHooks++
		store.Hooks.State[entry.Key] = hookTrust{TrustedHash: entry.CurrentHash}
		fmt.Fprintf(&hookBlock, "\n[hooks.state.%s]\ntrusted_hash = %s\n", tomlQuote(entry.Key), tomlQuote(entry.CurrentHash))
	}
	if hookBlock.Len() > 0 {
		if err := appendCodexConfig(o.codexConfig, hookBlock.String()); err != nil {
			return err
		}
	}

	fmt.Fprintf(o.stdout, "codex trust pre-seeded: %d workspace(s) and %d hook(s) added across %d session director(ies)\n",
		addedProjects, addedHooks, len(targets))
	o.reportUnmaterializedHomes()
	return nil
}

// checkCodexTrust verifies, from local state only, that every Gas City session
// directory is pre-seeded. It runs no subprocess and writes nothing, so it is
// a pure read.
//
// NAMED LIMITATION: because it never asks Codex, it cannot judge hash
// freshness. A recorded `trusted_hash` that no longer matches the hook's
// current content reads as satisfied here and still raises the trust dialog in
// a real session. Only `ao gc prepare`, which talks to Codex, detects that
// (Codex reports the hook as `modified`, and prepare refuses).
func (o *ops) checkCodexTrust() error {
	targets := o.codexTrustTargets()
	store, err := readCodexTrustStore(o.codexConfig)
	if err != nil {
		return err
	}
	for _, dir := range targets {
		if store.workspaceTrusted(dir) {
			continue
		}
		if level := store.workspaceLevel(dir); level != "" {
			return fmt.Errorf("codex workspace %s is recorded as trust_level %q, not %q; run `ao gc prepare` (%s)",
				dir, level, codexTrustedLevel, codexWedgeHint)
		}
		return fmt.Errorf("codex workspace trust is not pre-seeded for %s; run `ao gc prepare` (%s)", dir, codexWedgeHint)
	}
	for _, dir := range targets {
		keys, err := derivedHookKeys(dir)
		if err != nil {
			return err
		}
		for _, key := range keys {
			if defect := store.hookTrustDefect(key); defect != "" {
				return fmt.Errorf("codex hook %s %s; run `ao gc prepare` (%s)", key, defect, codexWedgeHint)
			}
		}
	}
	return nil
}

// derivedHookKeys reconstructs the hook-state keys Codex uses for a session
// directory's project-local hooks, so check can verify trust without asking
// Codex. Events are visited in sorted order so a failure names the same hook
// on every run.
func derivedHookKeys(dir string) ([]string, error) {
	path := hookFile(dir)
	data, err := os.ReadFile(path) // #nosec G304 -- project-local hooks file inside a discovered Gas City session directory.
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var document struct {
		Hooks map[string][]struct {
			Hooks []json.RawMessage `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	events := make([]string, 0, len(document.Hooks))
	for event := range document.Hooks {
		events = append(events, event)
	}
	sort.Strings(events)
	var keys []string
	for _, event := range events {
		for groupIndex, group := range document.Hooks[event] {
			for hookIndex := range group.Hooks {
				keys = append(keys, fmt.Sprintf("%s:%s:%d:%d", path, snakeCase(event), groupIndex, hookIndex))
			}
		}
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("%s contains no hook identities; refusing to accept an empty hook set", path)
	}
	return keys, nil
}

// snakeCase converts a Codex event name ("UserPromptSubmit") to the form used
// in a hook-state key ("user_prompt_submit").
func snakeCase(name string) string {
	var out strings.Builder
	for index, r := range name {
		if unicode.IsUpper(r) {
			if index > 0 {
				out.WriteByte('_')
			}
			out.WriteRune(unicode.ToLower(r))
			continue
		}
		out.WriteRune(r)
	}
	return out.String()
}

// listCodexHooks asks the Codex CLI for the hook identities visible from the
// target directories, then drops any identity that is not rooted under one of
// those directories - a `hooks/list` result also carries user-level and plugin
// hooks, and trust must never be granted beyond the discovered session roots.
func (o *ops) listCodexHooks(targets []string) ([]codexHook, error) {
	if len(targets) == 0 {
		return nil, nil
	}
	if o.codexBin == "" {
		fmt.Fprintf(o.stderr, "warning: no codex binary on PATH; codex hook trust was not resolved (workspace trust still applies)\n")
		return nil, nil
	}
	entries, err := codexHooksList(o.codexBin, o.codexHome, targets)
	if err != nil {
		return nil, fmt.Errorf("cannot read codex hook trust state: %w", err)
	}

	owned := map[string]bool{}
	for _, dir := range targets {
		owned[hookFile(dir)] = true
	}
	var kept []codexHook
	covered := map[string]bool{}
	for _, entry := range entries {
		source, ok := hookKeySourcePath(entry.Key)
		if !ok || !owned[source] {
			fmt.Fprintf(o.stderr, "note: ignoring codex hook outside the Gas City session directories: %s\n", entry.Key)
			continue
		}
		kept = append(kept, entry)
		covered[source] = true
	}

	// Fail closed: a session directory that has a hooks file must produce at
	// least one hook identity. Zero means the response shape changed or the
	// directory is not actually loading its project config - either way,
	// continuing on an empty list would report success while the wedge remains.
	for _, dir := range targets {
		source := hookFile(dir)
		if isRegularFile(source) && !covered[source] {
			return nil, fmt.Errorf("codex reported no hooks for %s even though %s exists; "+
				"refusing to continue on an empty hook list (%s)", dir, source, codexWedgeHint)
		}
	}
	return kept, nil
}

// hookKeySourcePath extracts the hooks file path from a hook-state key of the
// form `<path>:<event>:<matcher index>:<hook index>`.
func hookKeySourcePath(key string) (string, bool) {
	for i := 0; i < 3; i++ {
		cut := strings.LastIndex(key, ":")
		if cut < 0 {
			return "", false
		}
		key = key[:cut]
	}
	if key == "" {
		return "", false
	}
	return key, true
}

// codexHooksList drives one `codex app-server` stdio JSON-RPC exchange and
// returns every hook Codex reports for the given working directories.
func codexHooksList(bin, codexHome string, dirs []string) ([]codexHook, error) {
	ctx, cancel := context.WithTimeout(context.Background(), codexHookListTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, "app-server")
	cmd.Dir = dirs[0]
	cmd.Env = append(os.Environ(), "CODEX_HOME="+codexHome)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("open codex stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("open codex stdout: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start codex app-server: %w", err)
	}
	defer func() {
		_ = stdin.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	}()

	reader := bufio.NewReader(stdout)
	send := func(message any) error {
		return json.NewEncoder(stdin).Encode(message)
	}

	if err := send(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "initialize",
		"params": map[string]any{
			"clientInfo":   map[string]any{"name": "agentops", "version": "1"},
			"capabilities": map[string]any{"experimentalApi": true},
		},
	}); err != nil {
		return nil, fmt.Errorf("send codex initialize: %w", err)
	}
	if _, err := readRPCResult(reader, "1"); err != nil {
		return nil, decorateCodexError(err, &stderr)
	}
	if err := send(map[string]any{"jsonrpc": "2.0", "method": "initialized"}); err != nil {
		return nil, fmt.Errorf("send codex initialized: %w", err)
	}
	if err := send(map[string]any{
		"jsonrpc": "2.0", "id": 2, "method": "hooks/list",
		"params": map[string]any{"cwds": dirs},
	}); err != nil {
		return nil, fmt.Errorf("send codex hooks/list: %w", err)
	}
	raw, err := readRPCResult(reader, "2")
	if err != nil {
		return nil, decorateCodexError(err, &stderr)
	}
	return decodeHooksList(raw)
}

// decodeHooksList validates the hooks/list result shape and rejects anything it
// does not recognize, so a protocol change surfaces as a failure instead of an
// empty, silently successful trust pass.
func decodeHooksList(raw json.RawMessage) ([]codexHook, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("codex hooks/list returned no result payload")
	}
	var payload struct {
		Data *[]struct {
			Cwd   *string      `json:"cwd"`
			Hooks *[]codexHook `json:"hooks"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("codex hooks/list result has an unrecognized shape: %w", err)
	}
	if payload.Data == nil {
		return nil, fmt.Errorf("codex hooks/list result is missing its data array")
	}
	var hooks []codexHook
	for index, entry := range *payload.Data {
		if entry.Cwd == nil || entry.Hooks == nil {
			return nil, fmt.Errorf("codex hooks/list entry %d is missing cwd or hooks", index)
		}
		for _, hook := range *entry.Hooks {
			if strings.TrimSpace(hook.Key) == "" || strings.TrimSpace(hook.CurrentHash) == "" {
				return nil, fmt.Errorf("codex hooks/list reported a hook with no key or hash under %s", *entry.Cwd)
			}
			switch hook.TrustStatus {
			case "trusted", "untrusted", "modified", "managed":
			default:
				return nil, fmt.Errorf("codex hooks/list reported unknown trustStatus %q for %s", hook.TrustStatus, hook.Key)
			}
			hooks = append(hooks, hook)
		}
	}
	return hooks, nil
}

// readRPCResult consumes stdio JSON-RPC lines until the response carrying id
// arrives, ignoring the server notifications interleaved with it.
func readRPCResult(reader *bufio.Reader, id string) (json.RawMessage, error) {
	for {
		line, err := reader.ReadString('\n')
		if len(strings.TrimSpace(line)) > 0 {
			var message struct {
				ID     json.RawMessage `json:"id"`
				Result json.RawMessage `json:"result"`
				Error  *struct {
					Message string `json:"message"`
				} `json:"error"`
			}
			if json.Unmarshal([]byte(line), &message) == nil && string(message.ID) == id {
				if message.Error != nil {
					return nil, fmt.Errorf("codex returned an error: %s", message.Error.Message)
				}
				return message.Result, nil
			}
		}
		if err != nil {
			return nil, fmt.Errorf("codex app-server ended before answering request %s", id)
		}
	}
}

func decorateCodexError(err error, stderr *bytes.Buffer) error {
	detail := strings.TrimSpace(stderr.String())
	if detail == "" {
		return err
	}
	return fmt.Errorf("%w: %s", err, lastLine(detail))
}

func lastLine(text string) string {
	lines := strings.Split(text, "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

// reportUnmaterializedHomes names the configured agents that have no session
// home yet. Those homes do not exist, so their hooks cannot be trusted in
// advance and prepare must be re-run once they appear.
//
// Identities are compared, not counts: a stale home left behind by a removed
// agent would make the counts agree while a real agent is still missing.
type configuredAgent struct {
	name          string
	qualifiedName string
}

func (a configuredAgent) materialized(homes map[string]bool) bool {
	return (a.name != "" && homes[a.name]) ||
		(a.qualifiedName != "" && homes[a.qualifiedName])
}

func (a configuredAgent) displayName() string {
	if a.qualifiedName != "" {
		return a.qualifiedName
	}
	return a.name
}

func (o *ops) reportUnmaterializedHomes() {
	configured, err := o.configuredAgents()
	if err != nil || len(configured) == 0 {
		return
	}
	root := filepath.Join(o.city, ".gc", "agents")
	materialized := map[string]bool{}
	for _, home := range sessionDirsUnder(root) {
		if rel, relErr := filepath.Rel(root, home); relErr == nil {
			materialized[filepath.ToSlash(rel)] = true
			materialized[filepath.Base(rel)] = true
		}
	}
	var missing []string
	for _, agent := range configured {
		if agent.materialized(materialized) {
			continue
		}
		missing = append(missing, agent.displayName())
	}
	if len(missing) == 0 {
		return
	}
	sort.Strings(missing)
	fmt.Fprintf(o.stderr,
		"warning: %d configured agent(s) have no session home yet (%s); "+
			"a home created later carries untrusted hooks - re-run `ao gc prepare` after the city starts and before dispatching\n",
		len(missing), strings.Join(missing, ", "))
}

// configuredAgents lists both identity forms for each unsuspended agent. Gas
// City may return a dotted qualified name (for example `gastown.mayor`) while
// storing the session at `.gc/agents/mayor`; nested providers may instead use
// a slash-qualified home such as `.gc/agents/agentops/witness`.
func (o *ops) configuredAgents() ([]configuredAgent, error) {
	var payload struct {
		Agents []struct {
			Name          string `json:"name"`
			QualifiedName string `json:"qualified_name"`
			Suspended     bool   `json:"suspended"`
		} `json:"agents"`
	}
	if err := o.gcJSON(&payload, "cannot list configured agents", "--city", o.city, "agent", "list", "--json"); err != nil {
		return nil, err
	}
	var agents []configuredAgent
	for _, agent := range payload.Agents {
		if agent.Suspended {
			continue
		}
		if agent.Name != "" || agent.QualifiedName != "" {
			agents = append(agents, configuredAgent{
				name:          agent.Name,
				qualifiedName: agent.QualifiedName,
			})
		}
	}
	return agents, nil
}

// tomlQuote renders value as a TOML basic string.
func tomlQuote(value string) string {
	var out strings.Builder
	out.WriteByte('"')
	for _, r := range value {
		switch r {
		case '\\':
			out.WriteString(`\\`)
		case '"':
			out.WriteString(`\"`)
		case '\n':
			out.WriteString(`\n`)
		case '\t':
			out.WriteString(`\t`)
		case '\r':
			out.WriteString(`\r`)
		default:
			out.WriteRune(r)
		}
	}
	out.WriteByte('"')
	return out.String()
}

// appendCodexConfig appends whole table blocks to the trust store, creating it
// when absent.
//
// The operator's config is never edited in place. The merged content is built
// and parsed in memory; a merge that would not round-trip is rejected before
// the real file is touched. The repository's canonical atomic writer then
// installs the validated bytes durably while preserving the existing mode. No
// failure path can leave the operator with a partially written Codex config.
func appendCodexConfig(path, block string) error {
	existing, err := os.ReadFile(path) // #nosec G304 -- operator-owned codex trust store resolved from CODEX_HOME.
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read codex trust store %s: %w", path, err)
	}
	mode := os.FileMode(0o600)
	if info, statErr := os.Stat(path); statErr == nil {
		mode = info.Mode().Perm()
	}

	merged := string(existing)
	if len(existing) > 0 && !strings.HasSuffix(merged, "\n") {
		merged += "\n"
	}
	merged += block

	// Validate the replacement bytes before the operator's file is at risk.
	var candidate trustStore
	if err := toml.Unmarshal([]byte(merged), &candidate); err != nil {
		return fmt.Errorf("refusing to update %s: the merged trust store would not be parsable: %w", path, err)
	}
	if err := storage.AtomicWriteFile(path, []byte(merged), mode); err != nil {
		return fmt.Errorf("install codex trust store %s: %w", path, err)
	}
	return nil
}
