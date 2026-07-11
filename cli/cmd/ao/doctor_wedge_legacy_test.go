package main

import (
	"context"
	"time"

	doctoradapter "github.com/boshu2/agentops/cli/internal/adapters/doctor"
	revieweradapter "github.com/boshu2/agentops/cli/internal/adapters/reviewerhealth"
	"github.com/boshu2/agentops/cli/internal/reviewerhealth"
)

const wedgeReviewerTimeout = reviewerProbeTimeout

type reviewerProbe struct {
	name       string
	installCmd string
}

var wedgeReviewers = func() []reviewerProbe {
	catalog := reviewerhealth.DefaultCatalog()
	out := make([]reviewerProbe, 0, len(catalog))
	for _, reviewer := range catalog {
		out = append(out, reviewerProbe{name: reviewer.Name, installCmd: reviewer.InstallCommand})
	}
	return out
}()

func reviewerReachabilityChecks(timeout time.Duration) ([]doctorCheck, []string) {
	return reviewerHealthService.Check(context.Background(), timeout)
}

func checkReviewerReachable(reviewer reviewerProbe, timeout time.Duration) (doctorCheck, bool) {
	service := reviewerhealth.NewService([]reviewerhealth.Reviewer{{
		Name: reviewer.name, InstallCommand: reviewer.installCmd,
	}}, revieweradapter.SystemProbe())
	checks, live := service.Check(context.Background(), timeout)
	return checks[0], len(live) == 1
}

const agentopsModuleLine = "module github.com/boshu2/agentops/cli"

func wedgeDoctorChecks() []doctorCheck {
	return newLegacyDoctorService().Checks(context.Background())[11:]
}
func crossFamilyCheck(live []string) doctorCheck { return doctoradapter.CrossFamilyCheck(live) }
func binaryFreshnessCheck(dir, running string) doctorCheck {
	return doctoradapter.BinaryFreshnessCheck(dir, running)
}
func checkLedgerHealth(path string) doctorCheck {
	return doctoradapter.CheckLedgerHealth(path, time.Now)
}
func checkLaw0Guard(environment []string) doctorCheck {
	return doctoradapter.CheckLaw0Guard(environment)
}
