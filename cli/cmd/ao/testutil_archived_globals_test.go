// practices: [tdd]
package main

import "testing"

// The Cathedral Cut removed archived lifecycle command globals. This no-op
// helper keeps shared test setup simple without reviving an alternate build.
func resetArchivedCommandGlobals(_ *testing.T) {}
