// Package pool: ingest_orchestrator.go provides a shared file-iteration loop
// used by both `ao pool ingest` and `ao flywheel close-loop`. The two paths
// previously inlined the same per-file ReadFile + parse-header + parse-blocks
// + per-block ingest + per-file processed-file tracking pattern. This helper
// extracts the outer loop while keeping all parsing/scoring/move helpers in
// cmd/ao via injected callbacks — the orchestrator is purely structural and
// has no dependency on the candidate-building or pool-package internals.
package pool

import (
	"os"
)

// IngestFileFn ingests every learning block from a single file's bytes.
// It is implemented in cmd/ao using the existing parsePendingFileHeader,
// parseLearningBlocks, and ingestFileBlocks helpers. It must update any
// caller-owned counters via closure capture (the orchestrator does not
// inspect the result struct itself).
//
// Returns true when the file produced an ingest error that should suppress
// downstream processed-file move behavior.
type IngestFileFn func(path string, data []byte) (hadError bool)

// MoveProcessedFn moves successfully processed files to the processed
// directory. Implemented in cmd/ao via moveIngestedFiles. Called once at
// the end of the run with the full list of files that completed without
// errors (and only when GetDryRun() is false at the call site).
type MoveProcessedFn func(processed []string)

// IngestOrchestratorOpts wires the cmd/ao-side helpers into the shared loop.
type IngestOrchestratorOpts struct {
	// IngestFile must be non-nil. Called once per input file.
	IngestFile IngestFileFn

	// TrackProcessed enables processed-file accumulation. When true, each
	// file that ingests without errors is appended to the processed list and
	// passed to MoveProcessed at the end. Set to false for callers that
	// must not move files (e.g., the in-process flywheel close-loop path,
	// which historically left files in pending/ for separate orchestration).
	TrackProcessed bool

	// MoveProcessed is invoked once with the accumulated processed-files list.
	// Required when TrackProcessed is true; ignored otherwise.
	MoveProcessed MoveProcessedFn

	// OnReadError is called when os.ReadFile fails for a file. The orchestrator
	// always skips the file in that case; the callback lets the caller record
	// the error in its own result struct (e.g., bumping res.Errors and emitting
	// a verbose-print warning). May be nil.
	OnReadError func(path string, err error)
}

// IterateIngestFiles drives the shared outer file-iteration loop. For each
// path it reads the file once, hands the bytes to IngestFile, and tracks
// processed-file state when configured. Read errors do not stop sibling
// files — every file is independently processed.
//
// This function deliberately accepts no pool reference and returns no error:
// all behavior is observable through the callbacks and the optional
// MoveProcessed terminator.
func IterateIngestFiles(files []string, opts IngestOrchestratorOpts) {
	if opts.IngestFile == nil || len(files) == 0 {
		return
	}

	var processedFiles []string
	for _, f := range files {
		data, rerr := os.ReadFile(f)
		if rerr != nil {
			if opts.OnReadError != nil {
				opts.OnReadError(f, rerr)
			}
			continue
		}
		hadError := opts.IngestFile(f, data)
		if opts.TrackProcessed && !hadError {
			processedFiles = append(processedFiles, f)
		}
	}

	if opts.TrackProcessed && opts.MoveProcessed != nil && len(processedFiles) > 0 {
		opts.MoveProcessed(processedFiles)
	}
}
