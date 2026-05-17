// This file implements CorpusLocator — the single resolver for the
// .agents/ corpus directory relative to a given base. It is the wiki
// bounded context's home for path resolution that previously lived as
// agentsDirIn() in cli/cmd/ao/maturity.go.
package wiki

import (
	"os"
	"path/filepath"
	"strings"
)

// CorpusLocator resolves the AgentsDir (the .agents/ corpus directory)
// relative to a given base directory, honoring the AO_AGENTS_DIR / AO_HOME
// environment overrides.
//
// Resolution precedence (byte-identical to the legacy agentsDirIn shim):
//  1. AO_AGENTS_DIR — if set and non-empty (after trimming), used verbatim.
//  2. AO_HOME — if set and non-empty (after trimming), used verbatim.
//  3. filepath.Join(base, ".agents") — the legacy default.
//
// The zero value is usable: an empty CorpusLocator resolves exactly as the
// legacy agentsDirIn function did, so it can be substituted at call sites
// without behavior change.
type CorpusLocator struct{}

// AgentsDir returns the AgentsDir resolved relative to base, honoring the
// AO_AGENTS_DIR / AO_HOME env overrides (the same precedence implemented by
// lib/ao-paths.sh and cli/internal/paths). When neither env is set, the
// result is the legacy filepath.Join(base, ".agents").
//
// Callers already know the *root* (cwd or $HOME for --global), so this
// provides $base/.agents semantics rather than the paths package's repo-root
// auto-detect, while still threading the env overrides through.
func (CorpusLocator) AgentsDir(base string) string {
	if v := strings.TrimSpace(os.Getenv("AO_AGENTS_DIR")); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("AO_HOME")); v != "" {
		return v
	}
	return filepath.Join(base, ".agents")
}

// AgentsDirIn is a package-level convenience that resolves the AgentsDir for
// base using the zero-value CorpusLocator. It exists so callers can migrate
// to the wiki bounded context without threading a locator value.
func AgentsDirIn(base string) string {
	return CorpusLocator{}.AgentsDir(base)
}
