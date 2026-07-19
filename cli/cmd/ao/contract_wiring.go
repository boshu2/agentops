package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/clicontract"
)

// attachChildContract binds a CommandContract to a named direct subcommand of
// parent. It is the seam for giving a leaf command (e.g. `gate check`,
// `eval run`) its own real capabilities contract while the parent keeps its
// own. A missing child or an invalid contract is a construction-time
// programming error and panics, matching the parent-attach sites.
func attachChildContract(parent *cobra.Command, name string, contract clicontract.CommandContract) {
	for _, child := range parent.Commands() {
		if child.Name() == name {
			if err := clicontract.Attach(child, contract); err != nil {
				panic(fmt.Sprintf("attach contract to %q: %v", parent.Name()+" "+name, err))
			}
			return
		}
	}
	panic(fmt.Sprintf("attach contract: %q has no subcommand %q", parent.Name(), name))
}
