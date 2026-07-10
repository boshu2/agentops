package ports

import "testing"

func TestAgentMailContractsRequireScopedIdentityAndCoordination(t *testing.T) {
	identity := AgentMailIdentityRequest{Project: "/repo", Agent: "worker-1", Program: "codex-cli", Model: "gpt-5", Task: "C09"}
	if err := identity.Validate(); err != nil {
		t.Fatalf("valid identity: %v", err)
	}
	reservation := AgentMailReservationRequest{Project: "/repo", Agent: "worker-1", Paths: []string{"cli/internal/adapters/**"}, TTLSeconds: 900, Exclusive: true, Reason: "C09"}
	if err := reservation.Validate(); err != nil {
		t.Fatalf("valid reservation: %v", err)
	}
	message := AgentMailMessageRequest{Project: "/repo", Sender: "worker-1", Recipients: []string{"orchestrator"}, Subject: "C09 handoff", Body: "evidence: /tmp/c09.json", ThreadID: "C09", AckRequired: true}
	if err := message.Validate(); err != nil {
		t.Fatalf("valid message: %v", err)
	}

	bad := reservation
	bad.Paths = nil
	if err := bad.Validate(); err == nil {
		t.Fatal("empty reservation must fail")
	}
	badMessage := message
	badMessage.Recipients = nil
	if err := badMessage.Validate(); err == nil {
		t.Fatal("recipient-less handoff must fail")
	}
}
