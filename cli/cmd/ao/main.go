// Package main is the entry point for the ao CLI.
// practices: [pragmatic-programmer, twelve-factor-app]
package main

// version is set at build time via ldflags (goreleaser: -X main.version={{ .Version }}).
// The fallback "3.2.0-rc" identifies pre-tag source builds on the 3.2 line; the published
// release binary overrides this to "3.2.0" via the v3.2.0 git tag. Bump this fallback as
// part of every release prep (listed in skills/release/references/release-cut-and-bump.md).
var version = "3.2.0-rc"

func main() {
	Execute()
}
