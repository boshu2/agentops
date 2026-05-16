// practices: [hexagonal-architecture, tdd]
package ports

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestInMemorySafetyPolicy_ReturnsConfiguredDecision(t *testing.T) {
	policy := NewInMemorySafetyPolicy(map[SafetyPolicyName]SafetyDecision{
		"git.destructive": {Status: SafetyDecisionBlock, Reason: "reset hard blocked"},
	})
	decision, err := policy.Evaluate(context.Background(), SafetyPolicyRequest{Policy: "git.destructive"})
	if err != nil {
		t.Fatal(err)
	}
	if decision.Status != SafetyDecisionBlock {
		t.Fatalf("Status = %q, want BLOCK", decision.Status)
	}
	if decision.Reason != "reset hard blocked" {
		t.Fatalf("Reason = %q, want configured reason", decision.Reason)
	}
}

func TestInMemorySafetyPolicy_UnknownPolicyFailsClosed(t *testing.T) {
	policy := NewInMemorySafetyPolicy(nil)
	decision, err := policy.Evaluate(context.Background(), SafetyPolicyRequest{Policy: "scope.edit"})
	if err != nil {
		t.Fatal(err)
	}
	if decision.Status != SafetyDecisionBlock {
		t.Fatalf("Status = %q, want BLOCK", decision.Status)
	}
	if !strings.Contains(decision.Reason, "scope.edit") {
		t.Fatalf("Reason = %q, want policy name", decision.Reason)
	}
}

func TestInMemorySafetyPolicy_RejectsEmptyPolicy(t *testing.T) {
	policy := NewInMemorySafetyPolicy(nil)
	_, err := policy.Evaluate(context.Background(), SafetyPolicyRequest{})
	if err == nil {
		t.Fatal("expected error for empty policy")
	}
	if !strings.Contains(err.Error(), "policy required") {
		t.Fatalf("error = %v, want policy required", err)
	}
}

func TestInMemorySafetyPolicy_HonorsContextCancellation(t *testing.T) {
	policy := NewInMemorySafetyPolicy(nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := policy.Evaluate(ctx, SafetyPolicyRequest{Policy: "x"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}
