//go:build flywheel

package main

// Compiled only with `-tags flywheel`. Marks the binary as carrying the archived
// corpus/flywheel command set (ADR-0012). The default build omits this file.
func init() {
	archiveBuildTags = append(archiveBuildTags, "flywheel")
}
