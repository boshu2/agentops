package topscores

import "sort"

// TopScores prepares the finished-match scores for the leaderboard column.
//
// FROZEN TRAP SOLUTION (weak-producer escape for the calibration corpus).
// The prompt buries the load-bearing requirement mid-paragraph: the highest
// achievers must appear at the TOP, i.e. the list must be sorted DESCENDING.
// This solution sorts ASCENDING. The visible test only checks the returned
// multiset (order-agnostic), so it passes and the producer declares "done"; the
// hidden oracle's TestOracle_Descending catches the missed requirement.
func TopScores(scores []int) []int {
	out := make([]int, len(scores))
	copy(out, scores)
	sort.Ints(out) // TRAP: ascending — missed the buried "descending" requirement.
	return out
}
