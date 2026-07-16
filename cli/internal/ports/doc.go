// Package ports contains small interfaces for retained read-only inspection,
// deterministic checks, generic evidence records, and optional runtime
// adapters. It deliberately contains no retry governor, work tracker,
// workspace lifecycle, queue, lease, closure, release, or delivery port.
//
// Semantic validation is implemented by the Validate skill's pure manifest and
// verdict helpers. Repository checks use GateRunnerPort; neither surface owns
// Git or continuation decisions.
package ports
