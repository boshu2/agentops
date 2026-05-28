// Package main is the entry point for the ao CLI.
// practices: [pragmatic-programmer, twelve-factor-app]
package main

// version is set at build time via ldflags.
var version = "dev"

func main() {
	Execute()
}
