package resteer

import "fmt"

// rationalef formats a Recommendation.Rationale string. It is a thin wrapper
// over fmt.Sprintf kept as a named helper so rationale construction has a
// single, greppable callsite.
func rationalef(format string, args ...any) string {
	return fmt.Sprintf(format, args...)
}
