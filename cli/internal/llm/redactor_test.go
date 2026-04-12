package llm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRedact_AWSAccessKey(t *testing.T) {
	in := "here is my key AKIAIOSFODNN7EXAMPLE which you should not see"
	out := Redact(in)
	if strings.Contains(out, "AKIAIOSFODNN7EXAMPLE") {
		t.Errorf("AWS key leaked: %q", out)
	}
	if !strings.Contains(out, "[REDACTED]") {
		t.Errorf("expected [REDACTED] marker, got %q", out)
	}
}

func TestRedact_GitHubToken(t *testing.T) {
	in := "the token is ghp_abcdefghijklmnopqrstuvwxyz0123456789XY here"
	out := Redact(in)
	if strings.Contains(out, "ghp_abcdefghijklmnopqrstuvwxyz") {
		t.Errorf("GH token leaked: %q", out)
	}
}

func TestRedact_AnthropicKey(t *testing.T) {
	in := "anthropic key: sk-ant-api03-abcdefghijklmnop_qrstuvwxyz0123456789ABCDEFGHIJ"
	out := Redact(in)
	if strings.Contains(out, "sk-ant-api03-") {
		t.Errorf("Anthropic key leaked: %q", out)
	}
}

func TestRedact_OpenAIKey(t *testing.T) {
	in := "openai key is sk-abcdefghijklmnopqrstuvwxyz1234567890ABCDEF"
	out := Redact(in)
	if strings.Contains(out, "sk-abcdefghijklmnopqrstuvwxyz") {
		t.Errorf("OpenAI key leaked: %q", out)
	}
}

func TestRedact_PrivateKeyBlock(t *testing.T) {
	in := `some log
-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEAfakekeymaterialthatmustneverleak
-----END RSA PRIVATE KEY-----
more log`
	out := Redact(in)
	if strings.Contains(out, "fakekeymaterial") {
		t.Errorf("private key material leaked: %q", out)
	}
}

func TestRedact_ExtendedProviderTokens(t *testing.T) {
	cases := []struct {
		name string
		in   string
		leak string
	}{
		{
			name: "aws secret assignment",
			in:   `export AWS_SECRET_ACCESS_KEY="wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"`,
			leak: "wJalrXUtnFEMI",
		},
		{
			name: "github session token",
			in:   "session token ghs_abcdefghijklmnopqrstuvwxyz0123456789ABCD",
			leak: "ghs_abcdef",
		},
		{
			name: "gitlab token",
			in:   "gitlab token glpat-abcdefghijklmnopQRST123456",
			leak: "glpat-abcdefgh",
		},
		{
			name: "slack token",
			in:   "slack token " + "xoxb-" + "123456789012" + "-ABCDEFGHIJKLMNO",
			leak: "xoxb-" + "123456",
		},
		{
			name: "google oauth token",
			in:   "google access ya29.a0AfH6SMAabcdefghijklmnopqrstuvwxyz1234567890",
			leak: "ya29.a0Af",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := Redact(tc.in)
			if strings.Contains(out, tc.leak) {
				t.Errorf("token leaked: %q", out)
			}
			if !strings.Contains(out, "[REDACTED]") {
				t.Errorf("expected [REDACTED] marker, got %q", out)
			}
		})
	}
}

func TestRedact_JWTBearerAndConnectionStrings(t *testing.T) {
	in := strings.Join([]string{
		"jwt eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.signatureVALUE12345",
		"auth bearer Zm9vYmFyYmF6cXV4cXV1eGNvcnB0b2tlbjEyMzQ1Njc4OTA",
		"dsn postgres://agent:supersecret@db.internal/agentops",
		"blob dGhpcy1pcy1hLXZlcnktbG9uZy1vcGFxdWUtdG9rZW4tdGhhdC1zaG91bGQtYmUtcmVkYWN0ZWQ=",
	}, "\n")

	out := Redact(in)
	for _, leak := range []string{"eyJhbGci", "Zm9vYmFy", "supersecret", "dGhpcy1pcy"} {
		if strings.Contains(out, leak) {
			t.Fatalf("secret fragment %q leaked in %q", leak, out)
		}
	}
	if count := strings.Count(out, "[REDACTED]"); count < 4 {
		t.Fatalf("redaction count = %d, want at least 4 in %q", count, out)
	}
}

func TestRedact_DenylistFile(t *testing.T) {
	denylist := filepath.Join(t.TempDir(), "denylist.txt")
	if err := os.WriteFile(denylist, []byte("# comment\ninternal-codename-777\nliteral.with.regex*chars\n\n"), 0o600); err != nil {
		t.Fatalf("write denylist: %v", err)
	}
	t.Setenv(redactionDenylistEnv, denylist)

	out := Redact("ship internal-codename-777 and literal.with.regex*chars in notes")
	for _, leak := range []string{"internal-codename-777", "literal.with.regex*chars"} {
		if strings.Contains(out, leak) {
			t.Fatalf("denylist literal %q leaked in %q", leak, out)
		}
	}
}

func TestRedact_HomePathScrubbing(t *testing.T) {
	t.Setenv("HOME", "/Users/fullerbt")
	in := "see /Users/fullerbt/gt/agentops/crew/nami/cli/forge.go for details"
	out := Redact(in)
	if strings.Contains(out, "/Users/fullerbt") {
		t.Errorf("home path leaked: %q", out)
	}
	if !strings.Contains(out, "forge.go") {
		t.Errorf("non-sensitive path suffix lost: %q", out)
	}
}

func TestRedact_KeepsNonSensitiveContent(t *testing.T) {
	in := "the assistant suggested running make build and then make test for the cli package"
	out := Redact(in)
	if out != in {
		t.Errorf("non-sensitive content mutated:\n in:  %q\n out: %q", in, out)
	}
}

func TestRedactBytes_ScrubsSecrets(t *testing.T) {
	msgs := []byte("ghp_abcdefghijklmnopqrstuvwxyz0123456789XY")
	out := RedactBytes(msgs)
	if strings.Contains(string(out), "ghp_abcdefghijk") {
		t.Errorf("RedactBytes leaked: %q", out)
	}
}

func TestRedact_CredentialAtChunkBoundary(t *testing.T) {
	// A chunk ends with a credential that WAS half-split by an upstream
	// truncator — redactor runs before chunking so it should still catch
	// the whole secret at its full length.
	in := "log line ending with AKIAIOSFODNN7EXAMPLE and more"
	out := Redact(in)
	if strings.Contains(out, "AKIAIOSFODNN7EXAMPLE") {
		t.Errorf("boundary credential leaked: %q", out)
	}
}
