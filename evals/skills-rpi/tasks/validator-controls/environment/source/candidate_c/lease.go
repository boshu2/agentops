package candidate_c

// Expired reports whether a lease has expired at the inclusive deadline.
func Expired(now, deadline int64) bool { return now >= deadline }
