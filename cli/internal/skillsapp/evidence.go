package skillsapp

import (
	"encoding/json"
	"os"

	"github.com/boshu2/agentops/cli/internal/skillshealth"
)

// WriteAuditEvidence uses the same protected destination contract as build receipts.
// Existing reports are not overwritten; callers can always consume stdout instead.
func WriteAuditEvidence(repo, path string, report *skillshealth.EvidenceReport) error {
	if err := checkBuildReport(repo, path); err != nil {
		return err
	}
	if err := checkBuildReport(report.Target, path); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")
	err = enc.Encode(report)
	closeErr := file.Close()
	if err != nil {
		return err
	}
	return closeErr
}
