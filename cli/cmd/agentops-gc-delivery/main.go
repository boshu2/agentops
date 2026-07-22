// Command agentops-gc-delivery is the optional, pack-selected GC33 delivery
// reducer. It is intentionally separate from the ao command tree.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/boshu2/agentops/cli/internal/gcadapter/delivery"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "step" {
		fmt.Fprintln(os.Stderr, "usage: agentops-gc-delivery step --root DIR --certificate FILE --semantic-bead ID --terminal-ref REF --rig ID --repository OWNER/REPO --remote NAME --epoch N --mode auto|manual --deadline RFC3339 --prepared-at RFC3339 --committed-at RFC3339 --base-ref REF --base-oid SHA --fake-terminal-ref REF")
		os.Exit(2)
	}
	flags := flag.NewFlagSet("step", flag.ExitOnError)
	root, certificatePath := flags.String("root", "", "immutable evidence root"), flags.String("certificate", "", "exact admission certificate bytes")
	semantic, terminalRef, rig, repository, remote := flags.String("semantic-bead", "", "semantic bead"), flags.String("terminal-ref", "", "terminal semantic reference"), flags.String("rig", "", "rig identity"), flags.String("repository", "", "repository identity"), flags.String("remote", "", "remote identity")
	epoch := flags.Int("epoch", 0, "delivery epoch")
	mode, deadline, preparedAt, committedAt := flags.String("mode", "", "mode"), flags.String("deadline", "", "deadline"), flags.String("prepared-at", "", "prepared timestamp"), flags.String("committed-at", "", "committed timestamp")
	baseRef, baseOID, fakeRef := flags.String("base-ref", "", "base ref"), flags.String("base-oid", "", "base oid"), flags.String("fake-terminal-ref", "", "explicit fake terminal ref for this thin-slice boundary")
	fixtureState := flags.String("fixture-state", "", "explicit offline fake-provider state path")
	if err := flags.Parse(os.Args[2:]); err != nil {
		fail(err)
	}
	bytes, err := os.ReadFile(*certificatePath)
	if err != nil {
		fail(err)
	}
	var certificate delivery.AdmissionCertificate
	if err := json.Unmarshal(bytes, &certificate); err != nil {
		fail(err)
	}
	sum := sha256.Sum256(bytes)
	request := delivery.Request{Root: *root, Certificate: certificate, CertificateBytes: bytes, CertificateDigest: hex.EncodeToString(sum[:]), Target: delivery.Target{SemanticBeadID: *semantic, SemanticTerminalRef: *terminalRef, RigID: *rig, Repository: *repository, Remote: *remote, Epoch: *epoch, Mode: *mode, Deadline: *deadline, PreparedAt: *preparedAt, CommittedAt: *committedAt, BaseRef: *baseRef, BaseOID: *baseOID}}
	// GC33-6 deliberately exposes only the fake boundary. GC33-7 can add a
	// caller-owned real adapter after this exact transition contract is proven.
	if *fakeRef == "" || *fixtureState == "" {
		fail(fmt.Errorf("--fake-terminal-ref and --fixture-state are required by the GC33-6 offline thin slice"))
	}
	providers, err := delivery.OpenFixtureProviders(*fixtureState, delivery.Terminal{BeadID: *semantic, Ref: *fakeRef, Verdict: "PASS", CertificateDigest: request.CertificateDigest})
	if err != nil {
		fail(err)
	}
	result, err := delivery.NewReducer(providers, nil).Step(context.Background(), request)
	if err != nil {
		fail(err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fail(err)
	}
}
func fail(err error) { fmt.Fprintln(os.Stderr, "agentops-gc-delivery:", err); os.Exit(1) }
