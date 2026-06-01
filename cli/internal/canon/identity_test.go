package canon

import "testing"

func TestParseActor(t *testing.T) {
	tests := []struct {
		name      string
		spec      string
		wantName  string
		wantEmail string
		wantOK    bool
	}{
		{"empty", "", "", "", false},
		{"whitespace", "   ", "", "", false},
		{"name only", "apollo", "apollo", "", true},
		{"name and email", "apollo <apollo@fleet>", "apollo", "apollo@fleet", true},
		{"name with spaces and email", "Apollo Agent <a@x.com>", "Apollo Agent", "a@x.com", true},
		{"angle without close is name", "apollo <oops", "apollo <oops", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, ok := parseActor(tt.spec)
			if ok != tt.wantOK || id.Name != tt.wantName || id.Email != tt.wantEmail {
				t.Errorf("parseActor(%q) = (%+v, %v), want ({%q %q}, %v)", tt.spec, id, ok, tt.wantName, tt.wantEmail, tt.wantOK)
			}
		})
	}
}

func TestResolveIdentityPrecedence(t *testing.T) {
	// Explicit beats env beats git.
	t.Setenv(envActor, "envagent <env@x>")
	t.Setenv(envActorEmail, "")

	id, src := ResolveIdentity("explicitagent <ex@x>")
	if src != SourceExplicit || id.Name != "explicitagent" || id.Email != "ex@x" {
		t.Errorf("explicit override = (%+v, %s), want explicitagent/ex@x/explicit", id, src)
	}

	id, src = ResolveIdentity("")
	if src != SourceEnv || id.Name != "envagent" || id.Email != "env@x" {
		t.Errorf("env tier = (%+v, %s), want envagent/env@x/env", id, src)
	}
}

func TestResolveIdentityEnvEmailSupplements(t *testing.T) {
	t.Setenv(envActor, "witness") // no inline email
	t.Setenv(envActorEmail, "witness@fleet")
	id, src := ResolveIdentity("")
	if src != SourceEnv || id.Name != "witness" || id.Email != "witness@fleet" {
		t.Errorf("env + email supplement = (%+v, %s), want witness/witness@fleet/env", id, src)
	}
}

// TestCrossAgentGuard is the load-bearing test: distinct agent identities,
// injected via AGENTOPS_ACTOR, satisfy the cross-actor guard, while an agent
// attesting its own authored entry does not.
func TestCrossAgentGuard(t *testing.T) {
	dir := t.TempDir()
	// Entry authored by agent "apollo".
	entry := writeLearning(t, dir, "l.md", "apollo", "apollo@fleet")
	cl := NewCitationLedger(joinTmp(dir, "c.jsonl"))
	vl := NewVerificationLedger(joinTmp(dir, "v.jsonl"))

	apollo := Identity{Name: "apollo", Email: "apollo@fleet"}
	witness := Identity{Name: "witness", Email: "witness@fleet"}

	// apollo (author) cites + verifies its own work → self, must not count.
	if _, err := cl.Record("e", entry, "", "", apollo, clock); err != nil {
		t.Fatal(err)
	}
	if _, err := vl.Record("e", entry, "manual", "r", VerdictConfirmed, apollo, clock); err != nil {
		t.Fatal(err)
	}
	// witness (a different agent) cites + verifies → counts.
	if _, err := cl.Record("e", entry, "", "", witness, clock); err != nil {
		t.Fatal(err)
	}
	if _, err := vl.Record("e", entry, "council", "gate.log:L1", VerdictConfirmed, witness, clock); err != nil {
		t.Fatal(err)
	}

	d, err := DefaultGate().Evaluate("e", cl, vl)
	if err != nil {
		t.Fatal(err)
	}
	if !d.Eligible || d.Citations != 1 || d.Verifications != 1 {
		t.Errorf("cross-agent attestation: %+v, want citations=1 verifications=1 eligible (apollo's self-attestations excluded)", d)
	}
}

func joinTmp(dir, name string) string { return dir + "/" + name }
