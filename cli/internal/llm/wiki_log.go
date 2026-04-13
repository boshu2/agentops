package llm

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// AppendToLog appends a single entry to .agents/LOG.md. The log is
// append-only per the Karpathy wiki pattern. Non-fatal: errors are
// returned but should not abort the caller's main operation.
func AppendToLog(agentsDir, actor, verb, subject, wikilink string) error {
	logPath := filepath.Join(agentsDir, "LOG.md")
	entry := fmt.Sprintf("%s | %s | %s | %s | [[%s]]\n",
		time.Now().UTC().Format("2006-01-02 15:04"),
		actor, verb, subject, wikilink)
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open LOG.md: %w", err)
	}
	defer f.Close()
	_, err = f.WriteString(entry)
	return err
}

// AppendToIndex appends a single wikilink entry to .agents/INDEX.md under
// the appropriate section header. If the section doesn't exist, appends at
// the end. Lightweight alternative to re-running generate-index.sh.
func AppendToIndex(agentsDir, section, wikilink, description string) error {
	indexPath := filepath.Join(agentsDir, "INDEX.md")
	entry := fmt.Sprintf("- [[%s]] — %s\n", wikilink, description)

	// Just append at end — generate-index.sh will reorganize on next full run.
	f, err := os.OpenFile(indexPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open INDEX.md: %w", err)
	}
	defer f.Close()
	_, err = f.WriteString(entry)
	return err
}
