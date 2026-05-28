// practices: [pragmatic-programmer, design-patterns]
package main

import (
	"os"
	"path/filepath"
)

func detectTemplateFromProjectRoot(root string) string {
	stat := func(rel string) bool {
		_, err := os.Stat(filepath.Join(root, rel))
		return err == nil
	}

	switch {
	case stat("go.mod") || stat("cli/go.mod"):
		return "go-cli"
	case stat("package.json"):
		return "web-app"
	case stat("pyproject.toml"):
		return "python-lib"
	case stat("Cargo.toml"):
		return "rust-cli"
	default:
		return "generic"
	}
}
