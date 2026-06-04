// practices: [agile-manifesto]
package main

import "strings"

// shellQuote wraps a string in POSIX single quotes, escaping any embedded
// single quotes, so it is safe to embed in a /bin/sh command line.
//
// Relocated out of the rpi command surface (was rpi_parallel.go) so it survives
// the ao-rpi removal: handoff.go depends on it. (ag-362v / ag-llni)
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
