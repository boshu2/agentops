// Package embedded provides lib helpers and skill files embedded in the ao binary.
// These are used as a fallback when the agentops repo checkout is not available
// (e.g., Homebrew or npx installs).
package embedded

import "embed"

// HooksFS contains embedded lib helpers and skill files.
// Use fs.WalkDir to extract files to disk.
//
//go:embed all:lib all:skills
var HooksFS embed.FS

// TemplatesFS contains embedded goal template YAML files for ao goals init.
//
//go:embed all:templates
var TemplatesFS embed.FS
