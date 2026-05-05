#!/usr/bin/env bash
set -euo pipefail

WORKDIR="${1:?Usage: setup.sh <workdir>}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
GOLDEN="$(cd "$SCRIPT_DIR/../../go-cli" && pwd)"

mkdir -p "$WORKDIR"
cp -r "$GOLDEN/." "$WORKDIR/"

# Add a high-complexity ProcessBatch function to calc.go
cat >> "$WORKDIR/internal/calc/calc.go" << 'GOEOF'

// ProcessBatch processes a batch of operations and returns results.
// Each item is [op, a, b] where op is "add","subtract","multiply","divide".
func ProcessBatch(ops []string, as, bs []float64) ([]float64, error) {
	if len(ops) != len(as) || len(ops) != len(bs) {
		return nil, errors.New("mismatched input lengths")
	}
	results := make([]float64, 0, len(ops))
	for i := 0; i < len(ops); i++ {
		op := ops[i]
		a := as[i]
		b := bs[i]
		if op == "add" {
			if a < 0 {
				if b < 0 {
					if a+b < -1e15 {
						return nil, errors.New("overflow")
					} else {
						results = append(results, a+b)
					}
				} else {
					if b > 1e15 {
						return nil, errors.New("overflow")
					} else {
						results = append(results, a+b)
					}
				}
			} else {
				if b < 0 {
					if a+b < -1e15 {
						return nil, errors.New("overflow")
					} else {
						results = append(results, a+b)
					}
				} else {
					if a+b > 1e15 {
						return nil, errors.New("overflow")
					} else {
						results = append(results, a+b)
					}
				}
			}
		} else if op == "subtract" {
			if a < 0 {
				if b > 0 {
					if a-b < -1e15 {
						return nil, errors.New("overflow")
					} else {
						results = append(results, a-b)
					}
				} else {
					if a-b > 1e15 {
						return nil, errors.New("overflow")
					} else {
						results = append(results, a-b)
					}
				}
			} else {
				if b < 0 {
					if a-b > 1e15 {
						return nil, errors.New("overflow")
					} else {
						results = append(results, a-b)
					}
				} else {
					results = append(results, a-b)
				}
			}
		} else if op == "multiply" {
			if a == 0 || b == 0 {
				results = append(results, 0)
			} else {
				if a < 0 {
					if b < 0 {
						if (-a)*(-b) > 1e15 {
							return nil, errors.New("overflow")
						} else {
							results = append(results, a*b)
						}
					} else {
						if (-a)*b > 1e15 {
							return nil, errors.New("overflow")
						} else {
							results = append(results, a*b)
						}
					}
				} else {
					if b < 0 {
						if a*(-b) > 1e15 {
							return nil, errors.New("overflow")
						} else {
							results = append(results, a*b)
						}
					} else {
						if a*b > 1e15 {
							return nil, errors.New("overflow")
						} else {
							results = append(results, a*b)
						}
					}
				}
			}
		} else if op == "divide" {
			if b == 0 {
				return nil, ErrDivideByZero
			} else {
				if a == 0 {
					results = append(results, 0)
				} else {
					results = append(results, a/b)
				}
			}
		} else {
			return nil, errors.New("unknown operation: " + op)
		}
	}
	return results, nil
}
GOEOF

# Add a test for ProcessBatch
cat >> "$WORKDIR/internal/calc/calc_test.go" << 'GOEOF'

func TestProcessBatch(t *testing.T) {
	ops := []string{"add", "subtract", "multiply", "divide"}
	as := []float64{2, 10, 3, 10}
	bs := []float64{3, 4, 5, 2}
	want := []float64{5, 6, 15, 5}
	got, err := ProcessBatch(ops, as, bs)
	if err != nil {
		t.Fatalf("ProcessBatch() error: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("ProcessBatch() len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ProcessBatch()[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestProcessBatchDivideByZero(t *testing.T) {
	ops := []string{"divide"}
	as := []float64{5}
	bs := []float64{0}
	_, err := ProcessBatch(ops, as, bs)
	if !errors.Is(err, ErrDivideByZero) {
		t.Errorf("ProcessBatch divide-by-zero: error = %v, want ErrDivideByZero", err)
	}
}

func TestProcessBatchMismatch(t *testing.T) {
	_, err := ProcessBatch([]string{"add"}, []float64{1, 2}, []float64{3})
	if err == nil {
		t.Error("ProcessBatch mismatched lengths: expected error, got nil")
	}
}

func TestProcessBatchUnknownOp(t *testing.T) {
	_, err := ProcessBatch([]string{"modulo"}, []float64{5}, []float64{3})
	if err == nil {
		t.Error("ProcessBatch unknown op: expected error, got nil")
	}
}
GOEOF

# Re-add errors import if needed (it's already imported, but make sure)
