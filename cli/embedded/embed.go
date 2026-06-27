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

// SchemasFS contains embedded JSON Schemas enforced at runtime (Codex task
// packet and run receipt). Canonical copies live at the repo root `schemas/`;
// a parity test asserts the embedded copies stay byte-identical.
//
//go:embed all:schemas
var SchemasFS embed.FS

// PawlFS contains the cross-family pawl review scripts + the verdict schema,
// embedded so `ao pawl review` runs zero-config on a user's OWN repo when there
// is no AgentOps checkout to resolve them from (the stranger path). The bundle
// preserves the scripts/ + schemas/ sibling layout pawl-verdict.sh depends on
// (it reads its schema script-relative as $SCRIPT_DIR/../schemas/...). Canonical
// copies live at the repo root scripts/ + schemas/; `make sync-hooks` re-copies
// them and a parity test asserts the embedded copies stay byte-identical.
//
//go:embed all:pawl
var PawlFS embed.FS
