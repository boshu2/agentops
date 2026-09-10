package leases_test

import (
	"example.invalid/eval/leases/candidate_a"
	"example.invalid/eval/leases/candidate_b"
	"example.invalid/eval/leases/candidate_c"
	"testing"
)

func TestSuppliedSmoke(t *testing.T) {
	for name, expired := range map[string]func(int64, int64) bool{
		"candidate-a": candidate_a.Expired, "candidate-b": candidate_b.Expired, "candidate-c": candidate_c.Expired,
	} {
		if expired(9, 10) || !expired(11, 10) {
			t.Fatalf("%s smoke behavior failed", name)
		}
	}
}
