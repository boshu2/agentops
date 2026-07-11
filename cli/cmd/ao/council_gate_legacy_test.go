package main

import verdictparse "github.com/boshu2/agentops/cli/internal/verdict"

const tickVerdictLineCap = verdictparse.LineCap

type tickVerdictIdentityInfo = verdictparse.IdentityInfo

func tickScanVerdictLines(text string) ([]string, error) { return verdictparse.ScanLines(text) }
func tickVerdictHasCommandsRun(text string) bool         { return verdictparse.HasCommandsRun(text) }
func tickVerdictIdentity(text string) (tickVerdictIdentityInfo, []string) {
	return verdictparse.Identity(text)
}
func tickVerdictMetadataValue(text string, keys ...string) string {
	return verdictparse.MetadataValue(text, keys...)
}
func tickNormalizeMetadataKey(key string) string { return verdictparse.NormalizeMetadataKey(key) }
func tickNormalizeIdentityValue(value string) string {
	return verdictparse.NormalizeIdentityValue(value)
}
func tickNormalizeModelFamily(value string) string         { return verdictparse.NormalizeModelFamily(value) }
func tickUnknownModelFamily(family string) bool            { return verdictparse.UnknownModelFamily(family) }
func tickVerdictTokenCounts(text string) (int, int)        { return verdictparse.TokenCounts(text) }
func tickCouncilGate(rt tickRuntime, paths []string) error { return runCouncilGate(rt, paths) }
