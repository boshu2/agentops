package main

import "strings"

// shellQuote wraps a string in single quotes, escaping embedded single quotes
// so the result is safe to pass to a POSIX shell.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
