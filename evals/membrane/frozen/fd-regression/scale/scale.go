package scale

// Scale multiplies v by factor.
//
// Contract (must be preserved):
//   - Scale(0, factor) == 0          (zero input stays zero)
//   - Scale(v, 1)      == v          (identity factor)
//
// FROZEN TRAP SOLUTION (weak-producer escape for the calibration corpus).
// Asked to "add support for negative factors", the producer rewrote Scale as a
// repeated-addition loop. The visible test (Scale(4,3)=12, Scale(0,5)=0) passes,
// so it declares "done" — but the loop never runs for a negative factor, so
// negative factors silently return 0 (Scale(4,-3)=0, not -12). The hidden
// oracle's full matrix catches the regression the buried loop introduced.
func Scale(v, factor int) int {
	result := 0
	for i := 0; i < factor; i++ { // TRAP: negative factor → loop body never runs → 0.
		result += v
	}
	return result
}
