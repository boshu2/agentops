// Package cliapp owns profile selection and deterministic root assembly.
package cliapp

import (
	"fmt"

	"github.com/boshu2/agentops/cli/internal/clicontract"
)

type Profile string

const (
	ProfileDefault  Profile = "default"
	ProfileFlywheel Profile = "flywheel"
	ProfileLegacy   Profile = "legacy"
	ProfileCombined Profile = "combined"
)

func ParseProfile(value string) (Profile, error) {
	profile := Profile(value)
	if _, err := profile.contractProfile(); err != nil {
		return "", err
	}
	return profile, nil
}

func (profile Profile) contractProfile() (clicontract.ProfileSet, error) {
	switch profile {
	case ProfileDefault:
		return clicontract.ProfileDefault, nil
	case ProfileFlywheel:
		return clicontract.ProfileFlywheel, nil
	case ProfileLegacy:
		return clicontract.ProfileLegacy, nil
	case ProfileCombined:
		return clicontract.ProfileCombined, nil
	default:
		return 0, fmt.Errorf("unknown CLI profile %q", profile)
	}
}
