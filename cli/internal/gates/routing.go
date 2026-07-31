package gates

import "strings"

// installedSkillCopyPrefixes are the repo-relative roots where an agent
// runtime materializes INSTALLED copies of skill packages (`ao skills link`,
// a plugin install, or a vendored copy). Files under them are distribution
// artifacts owned by their upstream source, never first-party source of the
// repository being gated: a user who installs AgentOps into their own project
// gets AgentOps' own shell scripts on disk, and the shellcheck gate then fails
// the user's clean commit on OUR files with no repair the user can perform.
//
// The agentops repository itself tracks nothing under these prefixes (skills
// live in skills/), so excluding them cannot hide a first-party change from a
// gate — the #634 silent-drop hazard does not apply here.
var installedSkillCopyPrefixes = []string{
	".agents/skills/",
	".claude/skills/",
	".codex/skills/",
	".gemini/skills/",
	".cursor/skills/",
	".pi/skills/",
	"agent/skills/",
}

// IsInstalledSkillCopy reports whether a repo-relative path is an installed
// copy of a skill package rather than repository source.
func IsInstalledSkillCopy(path string) bool {
	for _, prefix := range installedSkillCopyPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// FilterInstalledSkillCopies drops installed skill copies from a change set.
// Every change set the gates consume passes through it — the orchestrator's
// routed set and the native checks' own fallback set — so a check can never
// see a path that routing already declared out of scope.
func FilterInstalledSkillCopies(paths []string) []string {
	kept := make([]string, 0, len(paths))
	for _, p := range paths {
		if IsInstalledSkillCopy(p) {
			continue
		}
		kept = append(kept, p)
	}
	if len(kept) == 0 {
		return nil
	}
	return kept
}

// matchGlob reports whether a repo-relative path matches a single glob pattern.
// It supports the small set of forms the seed registry actually uses (ported
// faithfully from the bash gate's anchored regexes):
//
//	exact:        "go.mod"            -> path == "go.mod"
//	dir prefix:   "cli/**"            -> path starts with "cli/"
//	ext suffix:   "**/*.go", "*.go"   -> path ends with ".go"
//	segment glob: "schemas/eval-*"    -> prefix before '*' + suffix after it
func matchGlob(pattern, path string) bool {
	switch {
	case pattern == path:
		return true
	case strings.HasSuffix(pattern, "/**"):
		return strings.HasPrefix(path, strings.TrimSuffix(pattern, "**"))
	case strings.HasPrefix(pattern, "**/*"):
		return strings.HasSuffix(path, strings.TrimPrefix(pattern, "**/*"))
	case strings.HasPrefix(pattern, "*."):
		return strings.HasSuffix(path, pattern[1:])
	case strings.Contains(pattern, "*"):
		i := strings.IndexByte(pattern, '*')
		return strings.HasPrefix(path, pattern[:i]) && strings.HasSuffix(path, pattern[i+1:])
	default:
		return false
	}
}

// matchAny reports whether path matches any of the patterns.
func matchAny(patterns []string, path string) bool {
	_, ok := matchAnyPattern(patterns, path)
	return ok
}

// PathMatchesAny reports whether a repo-relative path matches any of the glob
// patterns, using the gate's own glob semantics (matchGlob). Exported so the
// constraint-enforcement check (package checks) routes constraint
// applies_to.path_globs against changed files with the identical semantics the
// orchestrator uses to route check Match globs — one source of glob truth.
func PathMatchesAny(patterns []string, path string) bool {
	return matchAny(patterns, path)
}

// matchAnyPattern reports the first pattern matched by path.
func matchAnyPattern(patterns []string, path string) (string, bool) {
	for _, p := range patterns {
		if matchGlob(p, path) {
			return p, true
		}
	}
	return "", false
}

// affected reports whether a check is selected by the changed-file set: a check
// with no globs always runs, otherwise it runs if any changed file matches.
func (c Check) affected(changed []string) bool {
	if c.AlwaysRun() {
		return true
	}
	for _, f := range changed {
		if _, ok := matchAnyPattern(c.Match, f); ok {
			return true
		}
	}
	return false
}

// affectedReason reports the first changed-file/path-glob reason selecting c.
func (c Check) affectedReason(changed []string) (file string, pattern string, ok bool) {
	for _, f := range changed {
		if p, matched := matchAnyPattern(c.Match, f); matched {
			return f, p, true
		}
	}
	return "", "", false
}

// invalidatesAll reports whether any changed file forces a full run regardless
// of routing. Routing is a speed optimization on top of a correct full run; if
// module deps or the gate's OWN source/config changed, a stale router could
// silently skip the very checks that guard them (the #634 false-negative
// class), so we run everything.
func invalidatesAll(changed []string) bool {
	for _, f := range changed {
		if invalidatesAllFile(f) {
			return true
		}
	}
	return false
}

func firstInvalidatingFile(changed []string) string {
	for _, f := range changed {
		if invalidatesAllFile(f) {
			return f
		}
	}
	return ""
}

func invalidatesAllFile(f string) bool {
	switch {
	case f == "go.mod" || f == "go.sum" || f == "cli/go.mod" || f == "cli/go.sum":
		return true
	case strings.HasPrefix(f, "cli/internal/gates/"):
		return true
	case strings.HasPrefix(f, "cli/cmd/ao/gate"):
		return true
	}
	return false
}
