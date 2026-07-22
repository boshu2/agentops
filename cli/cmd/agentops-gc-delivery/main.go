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
		fmt.Fprintln(os.Stderr, "usage: agentops-gc-delivery step --root DIR --certificate FILE --subject-manifest FILE --subject-manifest-digest SHA256 --native-context FILE --native-context-digest SHA256 --semantic-bead ID --terminal-ref REF --rig ID --repository OWNER/REPO --remote NAME --epoch 1 [--delivery-bead ID] --mode auto|manual --deadline RFC3339 --prepared-at RFC3339 --committed-at RFC3339 --base-ref REF --base-oid SHA --gc-bin /absolute/gc --beads-bin /absolute/bd --git-bin /absolute/git --gh-bin /absolute/gh")
		os.Exit(2)
	}
	flags := flag.NewFlagSet("step", flag.ExitOnError)
	root, certificatePath := flags.String("root", "", "immutable evidence root"), flags.String("certificate", "", "exact admission certificate bytes")
	subjectPath, subjectDigest := flags.String("subject-manifest", "", "canonical subject-manifest.v1"), flags.String("subject-manifest-digest", "", "exact subject-manifest sha256")
	nativePath, nativeDigest := flags.String("native-context", "", "canonical native delivery context"), flags.String("native-context-digest", "", "exact native context sha256")
	semantic, terminalRef, rig, repository, remote := flags.String("semantic-bead", "", "semantic bead"), flags.String("terminal-ref", "", "terminal semantic reference"), flags.String("rig", "", "rig identity"), flags.String("repository", "", "repository identity"), flags.String("remote", "", "remote identity")
	epoch := flags.Int("epoch", 0, "initial delivery epoch (must be 1 without --delivery-bead)")
	deliveryBead := flags.String("delivery-bead", "", "explicit ready delivery bead selected by the sweep")
	mode, deadline, preparedAt, committedAt := flags.String("mode", "", "mode"), flags.String("deadline", "", "deadline"), flags.String("prepared-at", "", "prepared timestamp"), flags.String("committed-at", "", "committed timestamp")
	baseRef, baseOID := flags.String("base-ref", "", "base ref"), flags.String("base-oid", "", "base oid")
	observedAt := flags.String("observed-at", "", "caller-attested observation time used only for deadline reduction")
	gcBin := flags.String("gc-bin", "", "deployment-pinned Gas City binary")
	beadsBin := flags.String("beads-bin", "", "deployment-pinned Beads binary")
	gitBin := flags.String("git-bin", "", "pinned Git binary")
	ghBin := flags.String("gh-bin", "", "pinned GitHub CLI binary")
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
	subject, subjectBytes, err := delivery.ReadExactSubjectManifest(*subjectPath, *subjectDigest)
	if err != nil {
		fail(err)
	}
	native, nativeBytes, err := delivery.ReadExactNativeContext(*nativePath, *nativeDigest)
	if err != nil {
		fail(err)
	}
	sum := sha256.Sum256(bytes)
	request := delivery.Request{Root: *root, Certificate: certificate, CertificateBytes: bytes, CertificateDigest: hex.EncodeToString(sum[:]), Target: delivery.Target{DeliveryBeadID: *deliveryBead, SemanticBeadID: *semantic, SemanticTerminalRef: *terminalRef, RigID: *rig, Repository: *repository, Remote: *remote, Epoch: *epoch, Mode: *mode, Deadline: *deadline, PreparedAt: *preparedAt, CommittedAt: *committedAt, BaseRef: *baseRef, BaseOID: *baseOID, ObservedAt: *observedAt}, SubjectManifest: subject, SubjectBytes: subjectBytes, SubjectDigest: *subjectDigest, NativeContext: native, NativeBytes: nativeBytes, NativeDigest: *nativeDigest}
	deliveryBin, err := os.Executable()
	if err != nil {
		fail(err)
	}
	providers, err := delivery.NewNativeProviders(delivery.NativeBinaries{GC: *gcBin, Beads: *beadsBin, Git: *gitBin, GH: *ghBin, Delivery: deliveryBin}, native)
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
