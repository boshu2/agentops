// Package main is the entry point for the ao CLI.
// practices: [pragmatic-programmer, twelve-factor-app]
package main

// version is set at build time via ldflags (goreleaser: -X main.version={{ .Version }}).
// The fallback "3.1.0-rc" identifies pre-tag builds on the 3.1 branch; the published
// release binary overrides this to "3.1.0" via the v3.1.0 git tag.
var version = "3.1.0-rc"

func main() {
	Execute()
}
