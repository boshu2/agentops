package bridge

import (
	"strconv"
	"strings"
)

// ParseSemverParts extracts major, minor, patch integers from a version string.
func ParseSemverParts(v string) [3]int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	parts := strings.SplitN(v, ".", 3)
	var result [3]int
	for i := 0; i < 3 && i < len(parts); i++ {
		num := strings.SplitN(parts[i], "-", 2)[0]
		result[i], _ = strconv.Atoi(num)
	}
	return result
}

// CompareSemver returns -1, 0, or 1 comparing two semver strings.
func CompareSemver(a, b string) int {
	aParts := ParseSemverParts(a)
	bParts := ParseSemverParts(b)
	for i := 0; i < 3; i++ {
		if aParts[i] < bParts[i] {
			return -1
		}
		if aParts[i] > bParts[i] {
			return 1
		}
	}
	return 0
}
