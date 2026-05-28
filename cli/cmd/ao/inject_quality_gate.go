// practices: [wiki-knowledge-surface, ai-assisted-dev]
package main

import "github.com/boshu2/agentops/cli/internal/search"

// Type alias — canonical type lives in internal/search.
type qualityReport = search.QualityReport

// Thin wrappers — delegate to search package, kept for test compatibility.
func scanLearningQuality(dir string) (*qualityReport, error) { return search.ScanLearningQuality(dir) }
func assessLearningFile(path string) (bool, bool, error)     { return search.AssessLearningFile(path) }
