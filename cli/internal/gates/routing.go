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
	for _, p := range patterns {
		if matchGlob(p, path) {
			return true
		}
	}
	return false
}

// affected reports whether a check is selected by the changed-file set: a check
// with no globs always runs, otherwise it runs if any changed file matches.
func (c Check) affected(changed []string) bool {
	if c.AlwaysRun() {
		return true
	}
	for _, f := range changed {
		if matchAny(c.Match, f) {
			return true
		}
	}
	return false
}

// invalidatesAll reports whether any changed file forces a full run regardless
// of routing. Routing is a speed optimization on top of a correct full run; if
// module deps or the gate's OWN source/config changed, a stale router could
// silently skip the very checks that guard them (the #634 false-negative
// class), so we run everything.
func invalidatesAll(changed []string) bool {
	for _, f := range changed {
		switch {
		case f == "go.mod" || f == "go.sum" || f == "cli/go.mod" || f == "cli/go.sum":
			return true
		case strings.HasPrefix(f, "cli/internal/gates/"):
			return true
		case strings.HasPrefix(f, "cli/cmd/ao/gate"):
			return true
		}
	}
	return false
}
