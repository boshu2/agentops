package ports

// DetectorPrecisionEvidence summarizes observations gathered while a detector
// ran in non-blocking shadow mode.
type DetectorPrecisionEvidence struct {
	EvidenceRef    string `json:"evidence_ref"`
	Samples        int    `json:"samples"`
	TruePositives  int    `json:"true_positives"`
	FalsePositives int    `json:"false_positives"`
}

// DetectorReplayResult is the safe persisted projection of replay evidence. It
// carries references and counts, never fixture contents.
type DetectorReplayResult struct {
	PositiveRefs        []string
	NegativeControlRefs []string
	Precision           *DetectorPrecisionEvidence
}
