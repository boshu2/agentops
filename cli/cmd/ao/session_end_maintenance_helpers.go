// practices: [event-sourcing-cqrs, distributed-tracing]
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type aoMaintenanceGlobals struct {
	dryRun            bool
	output            string
	dedupMerge        bool
	maturityApply     bool
	maturityScan      bool
	maturityCurate    bool
	maturityExpire    bool
	maturityArchive   bool
	maturityEvict     bool
	maturityGlobal    bool
	maturityMigrateMd bool
	maturityRecalc    bool
	uncitedDays       int
}

func snapshotAOMaintenanceGlobals() aoMaintenanceGlobals {
	return aoMaintenanceGlobals{
		dryRun:            dryRun,
		output:            output,
		dedupMerge:        dedupMerge,
		maturityApply:     maturityApply,
		maturityScan:      maturityScan,
		maturityCurate:    maturityCurate,
		maturityExpire:    maturityExpire,
		maturityArchive:   maturityArchive,
		maturityEvict:     maturityEvict,
		maturityGlobal:    maturityGlobal,
		maturityMigrateMd: maturityMigrateMd,
		maturityRecalc:    maturityRecalibrate,
		uncitedDays:       maturityUncitedDays,
	}
}

func restoreAOMaintenanceGlobals(state aoMaintenanceGlobals) {
	dryRun = state.dryRun
	output = state.output
	dedupMerge = state.dedupMerge
	maturityApply = state.maturityApply
	maturityScan = state.maturityScan
	maturityCurate = state.maturityCurate
	maturityExpire = state.maturityExpire
	maturityArchive = state.maturityArchive
	maturityEvict = state.maturityEvict
	maturityGlobal = state.maturityGlobal
	maturityMigrateMd = state.maturityMigrateMd
	maturityRecalibrate = state.maturityRecalc
	maturityUncitedDays = state.uncitedDays
}

func withWorkingDir(dir string, fn func() error) error {
	origDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	if err := os.Chdir(dir); err != nil {
		return fmt.Errorf("chdir %s: %w", dir, err)
	}
	defer func() { _ = os.Chdir(origDir) }()
	return fn()
}

func withSuppressedOutput(fn func() error) error {
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		return fn()
	}
	defer func() { _ = devNull.Close() }()

	oldStdout := os.Stdout
	oldStderr := os.Stderr
	os.Stdout = devNull
	os.Stderr = devNull
	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}()

	return fn()
}

func bestEffortPruneAgents(cwd string) {
	if os.Getenv("AGENTOPS_AUTO_PRUNE") == "0" {
		return
	}
	script := filepath.Join(cwd, "scripts", "prune-agents.sh")
	if _, err := os.Stat(script); err != nil {
		return
	}
	cmd := exec.Command("bash", script, "--execute", "--quiet")
	cmd.Dir = cwd
	cmd.Stdout = nil
	cmd.Stderr = nil
	_ = cmd.Run()
}

func performHooklessSessionEndMaintenance(cwd string) error {
	previous := snapshotAOMaintenanceGlobals()
	defer restoreAOMaintenanceGlobals(previous)

	return withWorkingDir(cwd, func() error {
		return withSuppressedOutput(func() error {
			dryRun = false
			output = "table"
			dedupMerge = true
			maturityApply = false
			maturityScan = false
			maturityGlobal = false
			maturityMigrateMd = false
			maturityRecalibrate = false
			maturityUncitedDays = 60

			if err := runDedup(nil, nil); err != nil {
				return fmt.Errorf("dedup maintenance: %w", err)
			}
			if err := runContradict(nil, nil); err != nil {
				return fmt.Errorf("contradiction maintenance: %w", err)
			}

			if os.Getenv("AGENTOPS_EVICTION_DISABLED") != "1" {
				maturityArchive = true

				maturityExpire = true
				maturityEvict = false
				maturityCurate = false
				if err := runMaturityExpire(nil); err != nil {
					return fmt.Errorf("expiry maintenance: %w", err)
				}

				maturityExpire = false
				maturityEvict = true
				maturityCurate = false
				if err := runMaturityEvict(nil); err != nil {
					return fmt.Errorf("eviction maintenance: %w", err)
				}

				maturityExpire = false
				maturityEvict = false
				maturityCurate = true
				if err := runMaturityCurate(nil); err != nil {
					return fmt.Errorf("curation maintenance: %w", err)
				}
			}

			bestEffortRefreshFindingCompiler(cwd)
			bestEffortPruneAgents(cwd)
			return nil
		})
	})
}
