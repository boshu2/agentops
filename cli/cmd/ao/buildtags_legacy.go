//go:build legacy

package main

// Compiled only with `-tags legacy`. Marks the binary as carrying the archived
// RPI/factory command set (ADR-0012). The default build omits this file.
func init() {
	archiveBuildTags = append(archiveBuildTags, "legacy")
}
