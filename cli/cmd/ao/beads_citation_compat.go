package main

import (
	beadsadapter "github.com/boshu2/agentops/cli/internal/adapters/beads"
	beadsapp "github.com/boshu2/agentops/cli/internal/beads"
)

// Citation aliases keep the older BC1 CitationPort adapter source-stable while
// its effects are owned by the beads repository adapter.
type Citation = beadsapp.Citation
type CitationStatus = beadsapp.CitationStatus

const (
	CitationFresh   = beadsapp.CitationFresh
	CitationStale   = beadsapp.CitationStale
	CitationUnknown = beadsapp.CitationUnknown
)

func verifyFileCitation(citation *Citation, cwd string) {
	verifyCitationWithRepository(citation, cwd)
}

func verifyFunctionCitation(citation *Citation, cwd string) {
	verifyCitationWithRepository(citation, cwd)
}

func verifySymbolCitation(citation *Citation, cwd string) {
	verifyCitationWithRepository(citation, cwd)
}

func verifyCitationWithRepository(citation *Citation, cwd string) {
	repository := beadsadapter.NewKnowledgeRepositoryWithWorkingDirectory(func() (string, error) { return cwd, nil })
	verified := repository.VerifyCitations([]beadsapp.Citation{*citation})
	*citation = verified[0]
}
