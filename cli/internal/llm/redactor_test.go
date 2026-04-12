package llm

import (
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
