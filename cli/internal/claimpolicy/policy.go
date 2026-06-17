// Package claimpolicy holds shared policy helpers for public claim citation.
package claimpolicy

import (
	"path"
	"path/filepath"
	"strings"
)

// NormalizeTier returns the registry tier used for policy decisions.
func NormalizeTier(tier string) string {
	tier = strings.TrimSpace(strings.ToUpper(tier))
	if tier == "" {
		return "UNPROVEN"
	}
	return tier
}

// SurfaceAllowed reports whether a claim at surface may be cited by a tier
// with the provided cite_allowed patterns. It supports exact paths, shell-style
// path.Match globs, "**" for every surface, and "dir/**" prefix globs.
func SurfaceAllowed(patterns []string, surface string) bool {
	surface = filepath.ToSlash(strings.TrimSpace(surface))
	for _, pattern := range patterns {
		pattern = filepath.ToSlash(strings.TrimSpace(pattern))
		if pattern == "" {
			continue
		}
		if pattern == "**" {
			return true
		}
		if strings.HasSuffix(pattern, "/**") {
			prefix := strings.TrimSuffix(pattern, "/**")
			if surface == prefix || strings.HasPrefix(surface, prefix+"/") {
				return true
			}
			continue
		}
		if pattern == surface {
			return true
		}
		if matched, err := path.Match(pattern, surface); err == nil && matched {
			return true
		}
	}
	return false
}
