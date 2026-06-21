// practices: [hexagonal-architecture, fail-closed]
package checks

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/boshu2/agentops/cli/internal/gates"
	"github.com/boshu2/agentops/cli/internal/ports"
	"github.com/boshu2/agentops/cli/internal/search"
)

// The constraint-enforcement gate is the MECHANICAL half of the self-improving
// membrane (EM-ENF, age-membrane-memory-arch-tz2s.2.7): an escape compiled into
// an active constraint must mechanically FAIL a change that re-introduces its
// class. Loading known-risks into a prompt is advisory, not enforcement; this
// check is the windshield that catches a confident re-introduction the producer
// would otherwise narrate past.
//
// Contract (docs/contracts/finding-compiler.md, "Constraint Index Contract"):
//   - .agents/constraints/index.json is the canonical executable surface.
//   - The runtime reads ONLY the index; it never sources <id>.sh.
//   - Each ACTIVE constraint routes by applies_to.path_globs and is enforced by
//     its inline detector against the changed files.
//
// Fail-closed is the load-bearing property: a malformed index, or an active
// constraint this gate cannot fully evaluate, FAILS — it never silently skips.
// A safe membrane that occasionally cries wolf beats a membrane that waves a
// re-introduced escape through. The fail-open holes a cross-family review found
// (2026-06-21) are pinned by the constraints_test.go matrix: full-mode repo
// scan, strict index decode, present-but-unreadable file, unsupported glob.
func init() {
	gates.Register(gates.Check{
		ID:       "constraints.enforce",
		Tiers:    gates.Fast | gates.Full,
		Blocking: true,
		// No Match globs: the index can target any path, so the check always
		// runs and routes internally via each constraint's applies_to.path_globs.
		// The empty/parsed-fast path keeps the no-constraints case ~free.
		Run:        runConstraintEnforceGate,
		RepairHint: "ao constraint list — fix the change to satisfy the active constraint, or `ao constraint retire <id>` if the rule is wrong",
	})
}

// constraintSchemaVersion is the only index schema this gate knows how to
// enforce. A different (or absent) version fails closed: we cannot safely
// enforce a schema we do not understand.
const constraintSchemaVersion = 1

// errFailClosed marks an active constraint that could not be fully evaluated.
// It is distinct from a plain violation: both FAIL the gate, but the message
// distinguishes "this rule is broken" from "this change broke the rule".
var errFailClosed = errors.New("constraint could not be evaluated")

func runConstraintEnforceGate(_ context.Context, rc gates.RunContext) (ports.GateVerdict, error) {
	idx, missing, parseErr := loadConstraintIndexAt(rc.RepoRoot)
	if missing {
		return ports.GateVerdict{
			Status: ports.GateStatusPass,
			Reason: "no constraint index — nothing to enforce",
		}, nil
	}
	if parseErr != nil {
		// Parse error / unknown schema = gate fail, never skip (EM-ENF acceptance).
		return ports.GateVerdict{
			Status:  ports.GateStatusFail,
			Reason:  "constraint index malformed — failing closed",
			LogTail: fmt.Sprintf("%s: %v", search.ConstraintIndexPath(), parseErr),
		}, nil
	}

	// Candidate file set. Fast mode routes to the changed files; full mode gets
	// changed=nil from the orchestrator (it ignores routing), so we must
	// enumerate the repo ourselves or full/CI runs would enforce zero files.
	files := rc.ChangedFiles
	if rc.Mode == gates.Full {
		walked, err := walkRepoFiles(rc.RepoRoot)
		if err != nil {
			return ports.GateVerdict{
				Status:  ports.GateStatusFail,
				Reason:  "could not enumerate repo files for full-mode enforcement — failing closed",
				LogTail: err.Error(),
			}, nil
		}
		files = walked
	}

	var (
		active     int
		violations []string
		broken     []string
	)
	for i := range idx.Constraints {
		c := idx.Constraints[i]
		if c.Status != "active" {
			continue
		}
		active++
		hits, err := evalConstraint(c, rc.RepoRoot, files)
		if err != nil {
			broken = append(broken, fmt.Sprintf("%s (%s): %v", c.ID, c.Title, err))
			continue
		}
		violations = append(violations, hits...)
	}

	// Fail-closed takes precedence: a constraint we cannot evaluate is a hole in
	// the membrane, reported even if no clean violation fired.
	if len(broken) > 0 {
		return ports.GateVerdict{
			Status:  ports.GateStatusFail,
			Reason:  fmt.Sprintf("%d active constraint(s) could not be evaluated — failing closed", len(broken)),
			LogTail: strings.Join(append(broken, violations...), "\n"),
		}, nil
	}
	if len(violations) > 0 {
		return ports.GateVerdict{
			Status:  ports.GateStatusFail,
			Reason:  fmt.Sprintf("%d constraint violation(s) in changed files", len(violations)),
			LogTail: strings.Join(violations, "\n"),
		}, nil
	}
	return ports.GateVerdict{
		Status: ports.GateStatusPass,
		Reason: fmt.Sprintf("%d active constraint(s) enforced, no violations", active),
	}, nil
}

// loadConstraintIndexAt reads the index relative to root. It separates the three
// outcomes the gate must treat differently: missing file (no feature in use ->
// PASS), parse/structure error (-> FAIL CLOSED), and a valid index. Decoding is
// STRICT (DisallowUnknownFields + schema-version + no trailing data) so a
// JSON-valid but structurally-wrong index (e.g. a typo'd "constraint" key that
// drops every constraint) fails closed instead of silently passing. It does not
// use search.LoadConstraintIndex because that resolves a CWD-relative path and
// is permissive.
func loadConstraintIndexAt(root string) (idx *search.ConstraintIndex, missing bool, parseErr error) {
	path := filepath.Join(root, search.ConstraintIndexPath())
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, true, nil
		}
		// Unreadable (permissions, IO) is not "no feature in use" — fail closed.
		return nil, false, fmt.Errorf("reading index: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var parsed search.ConstraintIndex
	if err := dec.Decode(&parsed); err != nil {
		return nil, false, err
	}
	// Reject trailing data: the next read must be clean EOF. dec.More() only
	// covers array/object iteration, so a trailing "}" slips past it; a second
	// Decode that is not io.EOF means there is garbage after the index value.
	var rest json.RawMessage
	if err := dec.Decode(&rest); err != io.EOF {
		if err == nil {
			return nil, false, fmt.Errorf("trailing data after index JSON")
		}
		return nil, false, fmt.Errorf("trailing data after index JSON: %w", err)
	}
	if err := validateIndexStructure(data, &parsed); err != nil {
		return nil, false, err
	}
	return &parsed, false, nil
}

var knownConstraintStatuses = map[string]bool{"active": true, "draft": true, "retired": true}

// validateIndexStructure enforces the fields the gate depends on, so a
// JSON-valid but structurally-incomplete index fails closed instead of silently
// producing zero enforced constraints. It rejects: a wrong/absent schema
// version, an absent "constraints" array (a truncated write), and any entry
// missing an id or carrying an unknown status (status routes active-vs-not, so a
// blank/garbage status would silently drop a would-be-active constraint).
func validateIndexStructure(raw []byte, parsed *search.ConstraintIndex) error {
	if parsed.SchemaVersion != constraintSchemaVersion {
		return fmt.Errorf("unsupported schema_version %d (want %d)", parsed.SchemaVersion, constraintSchemaVersion)
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return err
	}
	// Reject duplicate top-level keys: JSON last-wins means a duplicate
	// "constraints" key could hide the real (active) array behind an empty one,
	// and the map probe above would also only see the last. Both decode "cleanly"
	// — so detect the ambiguity at the token level and fail closed.
	if dupKey, err := duplicateKey(raw); err != nil {
		return err
	} else if dupKey != "" {
		return fmt.Errorf("duplicate top-level key %q in index", dupKey)
	}
	craw, ok := probe["constraints"]
	if !ok {
		return fmt.Errorf("index missing required \"constraints\" array")
	}
	// The key must hold an actual JSON array. encoding/json silently accepts
	// `null` (and would for any non-array it can coerce) into a nil slice, so a
	// truncated `"constraints":null` would otherwise pass as an empty index.
	if trimmed := bytes.TrimSpace(craw); len(trimmed) == 0 || trimmed[0] != '[' {
		return fmt.Errorf("\"constraints\" must be a JSON array")
	}
	for i := range parsed.Constraints {
		c := parsed.Constraints[i]
		if strings.TrimSpace(c.ID) == "" {
			return fmt.Errorf("constraint #%d missing id", i)
		}
		if !knownConstraintStatuses[c.Status] {
			return fmt.Errorf("constraint %q has invalid status %q (want active|draft|retired)", c.ID, c.Status)
		}
	}
	return nil
}

// duplicateKey scans the ENTIRE JSON tree and returns the first object key that
// repeats within the same object (or "" if none). Go's last-wins decode would
// otherwise let a duplicate hide the real value at any depth — a second
// top-level "constraints":[] dropping the real array, or a nested second
// "status":"draft" downgrading an active constraint. Recursion closes the whole
// class, not just the top level. Returns a non-nil error only on a malformed
// token stream (which the strict decode also rejects).
func duplicateKey(raw []byte) (string, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	return scanDuplicateKey(dec)
}

// scanDuplicateKey consumes exactly one JSON value from dec, recursing into
// objects/arrays, and returns the first within-object duplicate key it finds.
func scanDuplicateKey(dec *json.Decoder) (string, error) {
	t, err := dec.Token()
	if err != nil {
		return "", err
	}
	d, ok := t.(json.Delim)
	if !ok {
		return "", nil // scalar value
	}
	switch d {
	case '{':
		seen := map[string]bool{}
		for dec.More() {
			keyTok, err := dec.Token()
			if err != nil {
				return "", err
			}
			key, ok := keyTok.(string)
			if !ok {
				return "", fmt.Errorf("malformed object key")
			}
			if seen[key] {
				return key, nil
			}
			seen[key] = true
			if dup, err := scanDuplicateKey(dec); err != nil || dup != "" {
				return dup, err
			}
		}
	case '[':
		for dec.More() {
			if dup, err := scanDuplicateKey(dec); err != nil || dup != "" {
				return dup, err
			}
		}
	}
	// consume the matching close delimiter
	if _, err := dec.Token(); err != nil {
		return "", err
	}
	return "", nil
}

// walkRepoFiles returns every regular file under root as a repo-relative,
// slash-separated path, skipping the .git directory. Used only in full mode to
// reconstruct the candidate set the orchestrator does not compute.
func walkRepoFiles(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return fs.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// evalConstraint enforces one active constraint against the candidate files,
// returning violation messages. A returned error means the constraint itself is
// malformed/unsupported (fail-closed), distinct from a clean violation.
func evalConstraint(c search.ConstraintEntry, repoRoot string, files []string) ([]string, error) {
	globs := c.AppliesTo.PathGlobs
	if len(globs) == 0 {
		// path_globs IS the routing; an active mechanical constraint with none is
		// malformed (it cannot say where it applies). Fail closed.
		return nil, fmt.Errorf("%w: active constraint has empty applies_to.path_globs", errFailClosed)
	}
	for _, g := range globs {
		if err := validateGlobSupported(g); err != nil {
			// A glob the gate's matcher cannot faithfully evaluate would route to
			// zero files and silently pass — fail closed instead.
			return nil, fmt.Errorf("%w: %v", errFailClosed, err)
		}
	}

	// Require an explicit, supported detector kind. Consistent with every other
	// field, an active constraint whose detector kind is blank/unknown is
	// ambiguous and fails closed rather than being silently assumed-regex.
	if c.Detector.Kind != "regex" {
		// Only regex detectors are enforced in this slice. Any other (or blank)
		// kind on an ACTIVE constraint must fail closed — never assume/skip.
		return nil, fmt.Errorf("%w: detector kind %q not enforceable (only regex)", errFailClosed, c.Detector.Kind)
	}
	if c.Detector.Pattern == "" {
		return nil, fmt.Errorf("%w: regex detector has empty pattern", errFailClosed)
	}
	re, err := regexp.Compile(c.Detector.Pattern)
	if err != nil {
		return nil, fmt.Errorf("%w: bad regex %q: %v", errFailClosed, c.Detector.Pattern, err)
	}
	var exclude *regexp.Regexp
	if c.Detector.Exclude != "" {
		exclude, err = regexp.Compile(c.Detector.Exclude)
		if err != nil {
			return nil, fmt.Errorf("%w: bad exclude regex %q: %v", errFailClosed, c.Detector.Exclude, err)
		}
	}

	mode := c.Detector.Mode
	if mode == "" {
		mode = "match"
	}

	var out []string
	for _, rel := range files {
		if !gates.PathMatchesAny(globs, rel) {
			continue
		}
		content, readErr := os.ReadFile(filepath.Join(repoRoot, rel))
		if readErr != nil {
			if os.IsNotExist(readErr) {
				// A genuine deletion cannot re-introduce a forbidden pattern, and an
				// absent-mode rule is moot on a file that no longer exists: safe skip.
				continue
			}
			// Present-but-unreadable (directory, permissions, symlink loop): we
			// cannot certify this file, so we cannot certify the constraint. Fail
			// closed rather than conflate it with a safe deletion.
			return nil, fmt.Errorf("%w: cannot read changed file %s: %v", errFailClosed, rel, readErr)
		}
		hits, evalErr := applyDetector(mode, re, exclude, c, rel, string(content))
		if evalErr != nil {
			return nil, evalErr
		}
		out = append(out, hits...)
	}
	return out, nil
}

// validateGlobSupported reports an error for any glob shape that the gate's
// matcher (gates.PathMatchesAny / matchGlob) does NOT faithfully evaluate, so an
// author who writes e.g. "cli/**/*.go" gets a loud fail-closed rather than a
// silent zero-file no-op. The accepted shapes mirror matchGlob exactly: exact,
// "base/**", "**/*suffix", "*.ext", and a single-'*' segment glob.
func validateGlobSupported(p string) error {
	if strings.TrimSpace(p) == "" {
		// A blank glob matches no repo-relative file, so it would route an active
		// constraint to zero files and pass vacuously. Fail closed.
		return fmt.Errorf("blank path glob")
	}
	if !strings.Contains(p, "*") {
		return nil // exact path
	}
	switch {
	case strings.HasSuffix(p, "/**") && !strings.Contains(strings.TrimSuffix(p, "/**"), "*"):
		return nil // dir prefix: base/**
	case strings.HasPrefix(p, "**/*") && !strings.Contains(p[len("**/*"):], "*"):
		return nil // **/*suffix
	case strings.HasPrefix(p, "*.") && strings.Count(p, "*") == 1:
		return nil // *.ext
	case strings.Count(p, "*") == 1:
		return nil // single-'*' segment glob (e.g. schemas/eval-*)
	default:
		return fmt.Errorf("unsupported glob shape %q (use base/**, **/*.ext, *.ext, prefix*suffix, or an exact path)", p)
	}
}

// applyDetector applies the compiled regex in the given mode to one file's
// content, returning violation messages. An unknown mode is fail-closed.
func applyDetector(mode string, re, exclude *regexp.Regexp, c search.ConstraintEntry, rel, content string) ([]string, error) {
	msg := c.Detector.Message
	if msg == "" {
		msg = c.Title
	}
	switch mode {
	case "match", "present", "forbid":
		var out []string
		for n, line := range strings.Split(content, "\n") {
			if !re.MatchString(line) {
				continue
			}
			if exclude != nil && exclude.MatchString(line) {
				continue
			}
			out = append(out, fmt.Sprintf("%s:%d [%s] %s", rel, n+1, c.ID, msg))
		}
		return out, nil
	case "absent", "require", "required":
		if !re.MatchString(content) {
			return []string{fmt.Sprintf("%s [%s] %s (required pattern absent)", rel, c.ID, msg)}, nil
		}
		return nil, nil
	default:
		return nil, fmt.Errorf("%w: unknown detector mode %q", errFailClosed, mode)
	}
}
