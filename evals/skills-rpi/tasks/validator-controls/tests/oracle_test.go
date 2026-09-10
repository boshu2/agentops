package leases_test

import (
	"example.invalid/eval/leases/candidate_a"
	"example.invalid/eval/leases/candidate_b"
	"example.invalid/eval/leases/candidate_c"
	"testing"
)

// This guards the seeded ground truth; it is not the worker's validator.
func TestEvalControlTruth(t *testing.T) {
	if candidate_a.Expired(10, 10) {
		t.Fatal("faulty control unexpectedly repaired")
	}
	for name, expired := range map[string]func(int64, int64) bool{
		"candidate-b": candidate_b.Expired, "candidate-c": candidate_c.Expired,
	} {
		if expired(9, 10) || !expired(10, 10) || !expired(11, 10) || !expired(-1, -1) {
			t.Fatalf("clean source control %s is invalid", name)
		}
	}
}
