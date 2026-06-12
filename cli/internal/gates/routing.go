package gates

import "strings"

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
