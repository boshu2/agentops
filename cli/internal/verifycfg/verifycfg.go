// Package verifycfg loads the per-repo AgentOps verify/pawl policy from an
// optional config file at the repository root and resolves the effective
// configuration under a single precedence: env > file > default.
//
// # The missing middle layer
//
// The verify/pawl engine is tuned by ~30 PAWL_* environment variables. That
// surface is fine for an operator with muscle memory, but hostile for a
// stranger repo that wants a durable, reviewable, committed policy. This
// package adds the layer between "env var you export per shell" and "built-in
// default": a committed file the whole team can read and diff.
//
// # File name and location
//
// The file is [ConfigFileName] (".aoverify.yaml") at the repository root.
// Location rationale (this is a deliberate, load-bearing choice):
//
//   - It must be committable EVERYWHERE. In the AgentOps repo the entire
//     repo-root ".agents/" tree is gitignored (see .gitignore "/.agents/**/*"),
//     so a verify.yaml placed under that tree would be silently untracked here — the exact
//     opposite of a "checked-in, reviewable policy". A top-level dotfile is not
//     caught by any such rule, in this repo or a stranger's.
//   - It sits at the repo root next to other policy dotfiles, so it is
//     discoverable and its diffs are obvious in review.
//
// # Format
//
// YAML, parsed by Go (gopkg.in/yaml.v3 is already a CLI dependency). YAML wins
// over a flat POSIX KEY=value file for two reasons:
//
//   - The committed file uses CLEAN semantic keys (review_timeout, autobind, …),
//     never the internal PAWL_* names. Leaking 30 PAWL_* names into a committed
//     policy would re-import the very surface this layer exists to hide.
//   - Comments and structure (for future nesting) are first-class in YAML.
//
// The shell (bead age-rk3r.17) consumes this WITHOUT re-parsing YAML: a Go
// bridge, [Resolved.ExportEnv], resolves precedence in one place and emits
// `export PAWL_*=…` lines the shell evals. See ExportEnv for the one-line hook.
//
// # Root resolution
//
// [LoadDir] walks up from the start directory to the nearest ancestor that
// contains a ".git" entry (file OR dir) and uses that as the repo root. This is
// the `git rev-parse --show-toplevel` equivalent: it returns the same directory
// (including inside a linked worktree, where ".git" is a file), needs no git
// subprocess, and is therefore hermetically testable. If no ".git" ancestor is
// found, the start directory itself is used.
package verifycfg

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ConfigFileName is the fixed, committable config file name resolved at the
// repository root.
const ConfigFileName = ".aoverify.yaml"

// Source identifies where a resolved value came from, in precedence order.
type Source string

const (
	// SourceDefault means the built-in default was used (no env, no file).
	SourceDefault Source = "default"
	// SourceFile means the value came from the .aoverify.yaml file.
	SourceFile Source = "file"
	// SourceEnv means an environment variable overrode the file and default.
	SourceEnv Source = "env"
)

// keySpec is the static description of one configurable key: its YAML name, the
// canonical environment variable that both overrides it AND is the export-env
// target, its type, and its default. The env var names deliberately map clean
// file keys onto the PAWL_* names the shell already understands (age-rk3r.17
// wires the ones not yet read).
type keySpec struct {
	yamlKey string
	envVar  string
	kind    kind
}

type kind int

const (
	kindString kind = iota
	kindInt
	kindBool
)

// keyOrder is the canonical display + iteration order (stable for tests and
// for --show-config output).
var keyOrder = []keySpec{
	{"reviewer_chain", "PAWL_REVIEWER_CHAIN", kindString},
	{"review_timeout", "PAWL_REVIEW_TIMEOUT", kindInt},
	{"strict", "PAWL_STRICT", kindBool},
	{"smoke", "PAWL_SMOKE_CMD", kindString},
	{"autobind", "PAWL_AUTOBIND", kindBool},
	{"author_family", "PAWL_AUTHOR_FAMILY", kindString},
}

// Defaults for each key. review_timeout mirrors pawl-review.sh's
// `${PAWL_REVIEW_TIMEOUT:-300}`; autobind mirrors pawl-verdict.sh's
// `${PAWL_AUTOBIND:-1}` (ON); author_family mirrors the `--author-family`
// default of "claude". reviewer_chain/smoke are empty (engine default);
// strict is off.
const (
	defReviewerChain = ""
	defReviewTimeout = 300
	defStrict        = false
	defSmoke         = ""
	defAutobind      = true
	defAuthorFamily  = "claude"
)

// knownKeys is derived from keyOrder for unknown-key detection.
var knownKeys = func() map[string]bool {
	m := make(map[string]bool, len(keyOrder))
	for _, k := range keyOrder {
		m[k.yamlKey] = true
	}
	return m
}()

// Resolved is the effective verify configuration after applying env > file >
// default precedence, plus the provenance of each value.
type Resolved struct {
	// ReviewerChain is the ordered reviewer family chain (engine default when empty).
	ReviewerChain string
	// ReviewTimeout is the per-review timeout in seconds.
	ReviewTimeout int
	// Strict makes advisory findings fail-closed.
	Strict bool
	// Smoke is an optional smoke-test command run as part of verification.
	Smoke string
	// Autobind controls whether a CONFIRMED verdict auto-binds to the commit.
	Autobind bool
	// AuthorFamily is the default value for --author-family.
	AuthorFamily string

	// ConfigPath is the resolved .aoverify.yaml path, or "" when no file was found.
	ConfigPath string
	// FileFound reports whether a config file was present and read.
	FileFound bool
	// Warnings holds non-fatal notices (unknown keys, unparseable values, a
	// malformed file). Consumers must also inspect ValidationError before using
	// values from a committed file.
	Warnings []string
	// InvalidReason is non-empty when a present committed policy could not be
	// read or parsed. Such a policy must HOLD verification rather than silently
	// inherit permissive defaults.
	InvalidReason string

	// sources maps each yaml key to where its value came from.
	sources map[string]Source
}

// ValidationError reports whether a present committed policy is unusable.
// An absent file is valid and preserves the zero-config defaults.
func (r *Resolved) ValidationError() error {
	if r == nil || r.InvalidReason == "" {
		return nil
	}
	return fmt.Errorf("invalid committed verify policy: %s", r.InvalidReason)
}

// Entry is one resolved setting rendered for display (--show-config).
type Entry struct {
	// Key is the clean YAML key name.
	Key string
	// EnvVar is the canonical environment variable for the key.
	EnvVar string
	// Value is the canonical string form of the effective value.
	Value string
	// Source is where the effective value came from.
	Source Source
}

// Source returns where the value for the given YAML key came from. Unknown keys
// report SourceDefault.
func (r *Resolved) Source(yamlKey string) Source {
	if s, ok := r.sources[yamlKey]; ok {
		return s
	}
	return SourceDefault
}

// Entries returns every setting in canonical order for display.
func (r *Resolved) Entries() []Entry {
	out := make([]Entry, 0, len(keyOrder))
	for _, k := range keyOrder {
		out = append(out, Entry{
			Key:    k.yamlKey,
			EnvVar: k.envVar,
			Value:  r.valueString(k.yamlKey),
			Source: r.Source(k.yamlKey),
		})
	}
	return out
}

// valueString renders the effective value of a key as its canonical string.
// Bools render as "true"/"false" for human display; ExportEnv renders "1"/"0"
// for the shell.
func (r *Resolved) valueString(yamlKey string) string {
	switch yamlKey {
	case "reviewer_chain":
		return r.ReviewerChain
	case "review_timeout":
		return strconv.Itoa(r.ReviewTimeout)
	case "strict":
		return strconv.FormatBool(r.Strict)
	case "smoke":
		return r.Smoke
	case "autobind":
		return strconv.FormatBool(r.Autobind)
	case "author_family":
		return r.AuthorFamily
	}
	return ""
}

// ExportEnv renders the shell bridge that bead age-rk3r.17 evals in one line:
//
//	eval "$(ao verify --export-env)"
//
// It emits `export PAWL_*=…` ONLY for keys whose source is env or file — never
// for a key left at its default. This is load-bearing: emitting a default (e.g.
// PAWL_REVIEW_TIMEOUT=300) would make pawl-review.sh believe the operator
// pinned the timeout and disable its own diff-size auto-scaling. Omitting
// default-sourced keys keeps the zero-config path byte-identical after the hook
// lands. Env-sourced keys are re-emitted idempotently (already in the shell's
// env) so the clean file key -> PAWL_* name translation is uniform.
//
// Values are shell-quoted; bools render as 1/0 (the form the PAWL_* shell reads).
func (r *Resolved) ExportEnv() string {
	var b strings.Builder
	for _, k := range keyOrder {
		if r.Source(k.yamlKey) == SourceDefault {
			continue
		}
		b.WriteString("export ")
		b.WriteString(k.envVar)
		b.WriteString("=")
		b.WriteString(shellQuote(r.exportValue(k.yamlKey, k.kind)))
		b.WriteString("\n")
	}
	return b.String()
}

// exportValue renders a key's value in the form the shell expects (bools as 1/0).
func (r *Resolved) exportValue(yamlKey string, kd kind) string {
	if kd == kindBool {
		if r.valueString(yamlKey) == "true" {
			return "1"
		}
		return "0"
	}
	return r.valueString(yamlKey)
}

// Load resolves the effective config using the current working directory as the
// start point. Callers that authorize verification effects must check
// ValidationError before using the resolved values.
func Load() *Resolved {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	return LoadDir(cwd)
}

// LoadDir resolves the effective config relative to the repo root discovered by
// walking up from startDir (see package doc, "Root resolution"). An absent file
// is valid; a present unreadable, malformed, or incorrectly typed file is
// recorded for fail-closed consumers through ValidationError.
func LoadDir(startDir string) *Resolved {
	root := findRoot(startDir)
	path := filepath.Join(root, ConfigFileName)

	r := &Resolved{
		ReviewerChain: defReviewerChain,
		ReviewTimeout: defReviewTimeout,
		Strict:        defStrict,
		Smoke:         defSmoke,
		Autobind:      defAutobind,
		AuthorFamily:  defAuthorFamily,
		sources:       make(map[string]Source, len(keyOrder)),
	}
	for _, k := range keyOrder {
		r.sources[k.yamlKey] = SourceDefault
	}

	file := loadFile(r, path)

	// Precedence per key: env > file > default (default already set above).
	applyString(r, "reviewer_chain", "PAWL_REVIEWER_CHAIN", file.reviewerChain, &r.ReviewerChain)
	applyInt(r, "review_timeout", "PAWL_REVIEW_TIMEOUT", file.reviewTimeout, &r.ReviewTimeout)
	applyBool(r, "strict", "PAWL_STRICT", file.strict, &r.Strict)
	applyString(r, "smoke", "PAWL_SMOKE_CMD", file.smoke, &r.Smoke)
	applyBool(r, "autobind", "PAWL_AUTOBIND", file.autobind, &r.Autobind)
	applyString(r, "author_family", "PAWL_AUTHOR_FAMILY", file.authorFamily, &r.AuthorFamily)
	if r.InvalidReason != "" {
		// Defense in depth for consumers that display the resolved values before
		// checking ValidationError: an invalid policy never looks permissive.
		r.Strict = true
		r.Autobind = false
	}

	return r
}

// fileValues holds the file-present values for each key. A nil pointer means the
// key was absent (or unparseable) in the file.
type fileValues struct {
	reviewerChain *string
	reviewTimeout *int
	strict        *bool
	smoke         *string
	autobind      *bool
	authorFamily  *string
}

// loadFile reads and decodes the config file at path, recording ConfigPath,
// FileFound, warnings, and committed-policy validity on r. Missing file is not
// an error. Unknown keys remain advisory for forward compatibility. An
// unreadable, malformed, or incorrectly typed committed field is invalid and
// yields a fail-closed ValidationError.
func loadFile(r *Resolved, path string) fileValues {
	var fv fileValues

	data, err := os.ReadFile(path) // #nosec G304 -- path is the repo-root config file, operator-owned.
	if err != nil {
		if !os.IsNotExist(err) {
			r.warn("cannot read %s: %v (using env/defaults)", path, err)
			r.InvalidReason = fmt.Sprintf("cannot read %s: %v", path, err)
		}
		return fv
	}
	r.ConfigPath = path
	r.FileFound = true

	var root map[string]yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		r.warn("cannot parse %s: %v (using env/defaults)", path, err)
		r.InvalidReason = fmt.Sprintf("cannot parse %s: %v", path, err)
		return fv
	}

	// Unknown-key detection: warn ONCE with the full list (forward-compatible).
	var unknown []string
	for key := range root {
		if !knownKeys[key] {
			unknown = append(unknown, key)
		}
	}
	if len(unknown) > 0 {
		sortStrings(unknown)
		r.warn("%s: unknown key(s) ignored: %s", path, strings.Join(unknown, ", "))
	}

	// Per-field decode with error isolation.
	fv.reviewerChain = decodeString(r, root, path, "reviewer_chain")
	fv.reviewTimeout = decodeInt(r, root, path, "review_timeout")
	fv.strict = decodeBool(r, root, path, "strict")
	fv.smoke = decodeString(r, root, path, "smoke")
	fv.autobind = decodeBool(r, root, path, "autobind")
	fv.authorFamily = decodeString(r, root, path, "author_family")
	return fv
}

func decodeString(r *Resolved, root map[string]yaml.Node, path, key string) *string {
	node, ok := root[key]
	if !ok {
		return nil
	}
	var v string
	if err := node.Decode(&v); err != nil {
		r.warn("%s: ignoring %s: %v", path, key, err)
		invalidateCommittedField(r, path, key, err)
		return nil
	}
	return &v
}

func decodeInt(r *Resolved, root map[string]yaml.Node, path, key string) *int {
	node, ok := root[key]
	if !ok {
		return nil
	}
	var v int
	if err := node.Decode(&v); err != nil {
		r.warn("%s: ignoring %s: %v", path, key, err)
		invalidateCommittedField(r, path, key, err)
		return nil
	}
	return &v
}

func decodeBool(r *Resolved, root map[string]yaml.Node, path, key string) *bool {
	node, ok := root[key]
	if !ok {
		return nil
	}
	var v bool
	if err := node.Decode(&v); err != nil {
		r.warn("%s: ignoring %s: %v", path, key, err)
		invalidateCommittedField(r, path, key, err)
		return nil
	}
	return &v
}

func invalidateCommittedField(r *Resolved, path, key string, err error) {
	if r.InvalidReason == "" {
		r.InvalidReason = fmt.Sprintf("cannot decode %s field %s: %v", path, key, err)
	}
}

// applyString sets *dst and the key's source from env (highest) then file.
func applyString(r *Resolved, key, envVar string, fileVal *string, dst *string) {
	if v, ok := os.LookupEnv(envVar); ok {
		*dst = v
		r.sources[key] = SourceEnv
		return
	}
	if fileVal != nil {
		*dst = *fileVal
		r.sources[key] = SourceFile
	}
}

// applyInt sets *dst and the key's source from env (highest) then file. A
// non-integer env value warns and is ignored (falls through to file/default).
func applyInt(r *Resolved, key, envVar string, fileVal *int, dst *int) {
	if v, ok := os.LookupEnv(envVar); ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			*dst = n
			r.sources[key] = SourceEnv
			return
		}
		r.warn("ignoring %s=%q: not an integer", envVar, v)
	}
	if fileVal != nil {
		*dst = *fileVal
		r.sources[key] = SourceFile
	}
}

// applyBool sets *dst and the key's source from env (highest) then file. An
// unrecognized env value warns and is ignored (falls through to file/default).
func applyBool(r *Resolved, key, envVar string, fileVal *bool, dst *bool) {
	if v, ok := os.LookupEnv(envVar); ok {
		if b, ok := parseBool(v); ok {
			*dst = b
			r.sources[key] = SourceEnv
			return
		}
		r.warn("ignoring %s=%q: not a boolean", envVar, v)
	}
	if fileVal != nil {
		*dst = *fileVal
		r.sources[key] = SourceFile
	}
}

// parseBool accepts the common shell + YAML truthy/falsy spellings.
func parseBool(s string) (val, ok bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true, true
	case "0", "false", "no", "off":
		return false, true
	}
	return false, false
}

// warn appends a formatted non-fatal notice.
func (r *Resolved) warn(format string, args ...any) {
	r.Warnings = append(r.Warnings, fmt.Sprintf(format, args...))
}

// findRoot walks up from startDir to the nearest ancestor containing a ".git"
// entry (file or dir) and returns it. Falls back to startDir when none is found.
func findRoot(startDir string) string {
	dir := startDir
	if dir == "" {
		dir = "."
	}
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return startDir
		}
		dir = parent
	}
}

// shellQuote single-quotes s for safe eval in POSIX sh.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// sortStrings sorts in place (small n; avoids importing sort at call sites).
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}
