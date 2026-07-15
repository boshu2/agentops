// Package main is the entry point for the ao CLI.
// practices: [pragmatic-programmer, twelve-factor-app]
package main

// version is set at build time via ldflags (goreleaser: -X main.version={{ .Version }}).
// The fallback identifies untagged source builds for the next breaking release;
// published binaries override it from the release tag via GoReleaser.
var version = "4.0.0-rc"

func main() {
	Execute()
}
