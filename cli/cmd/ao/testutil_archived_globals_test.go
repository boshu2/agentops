// practices: [tdd]
package main

import "testing"

// The Cathedral Cut removed archived lifecycle command globals. These no-op
// helpers keep shared test setup simple without reviving an alternate build.
func snapshotArchivedCommandGlobals() func() { return func() {} }

func resetArchivedCommandGlobals(_ *testing.T) {}
