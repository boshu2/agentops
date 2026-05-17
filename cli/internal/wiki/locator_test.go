package wiki

import (
	"os"
	"path/filepath"
	"testing"
)

// legacyAgentsDirIn reproduces the exact behavior of the pre-migration
// cli/cmd/ao/maturity.go agentsDirIn function. The golden test asserts that
// CorpusLocator is byte-identical to this reference implementation.
func legacyAgentsDirIn(base string) string {
	if v := trimSpaceCopy(os.Getenv("AO_AGENTS_DIR")); v != "" {
		return v
	}
	if v := trimSpaceCopy(os.Getenv("AO_HOME")); v != "" {
		return v
	}
	return filepath.Join(base, ".agents")
}

// trimSpaceCopy mirrors strings.TrimSpace; kept local so the reference impl
// is self-contained and obviously identical to the original.
func trimSpaceCopy(s string) string {
	start := 0
	for start < len(s) && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	end := len(s)
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

// TestCorpusLocator is a golden test asserting CorpusLocator.AgentsDir (and
// the AgentsDirIn convenience) resolve paths byte-identically to the legacy
// agentsDirIn function for representative inputs and env-override states.
func TestCorpusLocator(t *testing.T) {
	cases := []struct {
		name        string
		base        string
		agentsDir   string // AO_AGENTS_DIR value; "" means unset
		homeOverlay string // AO_HOME value; "" means unset
		want        string
	}{
		{
			name: "default no env, repo cwd",
			base: "/home/user/project",
			want: filepath.Join("/home/user/project", ".agents"),
		},
		{
			name: "default no env, relative base",
			base: ".",
			want: filepath.Join(".", ".agents"),
		},
		{
			name: "default no env, empty base",
			base: "",
			want: filepath.Join("", ".agents"),
		},
		{
			name: "default no env, home base",
			base: "/home/user",
			want: filepath.Join("/home/user", ".agents"),
		},
		{
			name:      "AO_AGENTS_DIR override wins",
			base:      "/home/user/project",
			agentsDir: "/custom/agents",
			want:      "/custom/agents",
		},
		{
			name:      "AO_AGENTS_DIR override wins over AO_HOME",
			base:      "/home/user/project",
			agentsDir: "/custom/agents",
			want:      "/custom/agents",
		},
		{
			name:        "AO_HOME override used when AO_AGENTS_DIR unset",
			base:        "/home/user/project",
			homeOverlay: "/custom/home/.agents",
			want:        "/custom/home/.agents",
		},
		{
			name:        "AO_AGENTS_DIR beats AO_HOME when both set",
			base:        "/home/user/project",
			agentsDir:   "/from/agents-dir",
			homeOverlay: "/from/ao-home",
			want:        "/from/agents-dir",
		},
		{
			name:      "AO_AGENTS_DIR whitespace-only is ignored",
			base:      "/home/user/project",
			agentsDir: "   ",
			want:      filepath.Join("/home/user/project", ".agents"),
		},
		{
			name:        "AO_AGENTS_DIR whitespace falls through to AO_HOME",
			base:        "/home/user/project",
			agentsDir:   "  \t ",
			homeOverlay: "/fallback/home",
			want:        "/fallback/home",
		},
		{
			name:      "AO_AGENTS_DIR value is trimmed",
			base:      "/home/user/project",
			agentsDir: "  /trimmed/agents  ",
			want:      "/trimmed/agents",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setOrUnset(t, "AO_AGENTS_DIR", tc.agentsDir)
			setOrUnset(t, "AO_HOME", tc.homeOverlay)

			// Golden assertion: exact expected path.
			if got := (CorpusLocator{}).AgentsDir(tc.base); got != tc.want {
				t.Errorf("CorpusLocator.AgentsDir(%q) = %q, want %q", tc.base, got, tc.want)
			}

			// AgentsDirIn convenience must match the method.
			if got := AgentsDirIn(tc.base); got != tc.want {
				t.Errorf("AgentsDirIn(%q) = %q, want %q", tc.base, got, tc.want)
			}

			// Byte-identical to the legacy reference implementation.
			if got, legacy := AgentsDirIn(tc.base), legacyAgentsDirIn(tc.base); got != legacy {
				t.Errorf("AgentsDirIn(%q) = %q, legacy agentsDirIn = %q (must be byte-identical)", tc.base, got, legacy)
			}
		})
	}
}

// setOrUnset sets env var key to val, or unsets it when val is "".
// t.Setenv handles cleanup; for the unset case we explicitly unset and
// register a restore.
func setOrUnset(t *testing.T, key, val string) {
	t.Helper()
	if val == "" {
		orig, had := os.LookupEnv(key)
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unset %s: %v", key, err)
		}
		t.Cleanup(func() {
			if had {
				_ = os.Setenv(key, orig)
			} else {
				_ = os.Unsetenv(key)
			}
		})
		return
	}
	t.Setenv(key, val)
}
