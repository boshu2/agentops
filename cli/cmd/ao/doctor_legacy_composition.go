package main

import (
	doctoradapter "github.com/boshu2/agentops/cli/internal/adapters/doctor"
	doctorapp "github.com/boshu2/agentops/cli/internal/doctor"
)

func newLegacyDoctorService() doctorapp.LegacyService {
	checks := doctoradapter.SystemLegacyChecks(version, IndexDir, IndexFileName, resolveLedgerPath, reviewerHealthService)
	return doctorapp.NewLegacyService(version, checks)
}

var doctorReadService = doctorapp.NewReadService(version, doctoradapter.ReadRuntime{ToolVersion: version}, doctoradapter.ReadGateway{})
