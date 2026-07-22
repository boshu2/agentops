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
	"time"

	"github.com/boshu2/agentops/cli/internal/gcadapter/delivery"
)

func main() {
	if len(os.Args) < 2 || (os.Args[1] != "step" && os.Args[1] != "sweep" && os.Args[1] != "status") {
		fmt.Fprintln(os.Stderr, "usage: agentops-gc-delivery step|sweep|status --native-context FILE --native-context-digest SHA256 --gc-bin /absolute/gc --beads-bin /absolute/bd --git-bin /absolute/git --gh-bin /absolute/gh --bash-bin /absolute/bash [step/sweep delivery inputs] [status --observed-at RFC3339]")
		os.Exit(2)
	}
	options, err := parseOptions(os.Args[1], os.Args[2:])
	if err != nil {
		fail(err)
	}
	output, err := runCommand(context.Background(), os.Args[1], options)
	if err != nil {
		fail(err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(output); err != nil {
		fail(err)
	}
}

type commandOptions struct {
	root, certificatePath, subjectPath, subjectDigest, nativePath, nativeDigest string
	semantic, terminalRef, rig, repository, remote, deliveryBead                string
	mode, deadline, preparedAt, committedAt, baseRef, baseOID, observedAt       string
	gcBin, beadsBin, gitBin, ghBin, bashBin                                     string
	epoch                                                                       int
}

func parseOptions(command string, args []string) (commandOptions, error) {
	var options commandOptions
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.StringVar(&options.root, "root", "", "immutable evidence root")
	flags.StringVar(&options.certificatePath, "certificate", "", "exact admission certificate bytes")
	flags.StringVar(&options.subjectPath, "subject-manifest", "", "canonical subject-manifest.v1")
	flags.StringVar(&options.subjectDigest, "subject-manifest-digest", "", "exact subject-manifest sha256")
	flags.StringVar(&options.nativePath, "native-context", "", "canonical native delivery context")
	flags.StringVar(&options.nativeDigest, "native-context-digest", "", "exact native context sha256")
	flags.StringVar(&options.semantic, "semantic-bead", "", "semantic bead")
	flags.StringVar(&options.terminalRef, "terminal-ref", "", "terminal semantic reference")
	flags.StringVar(&options.rig, "rig", "", "rig identity")
	flags.StringVar(&options.repository, "repository", "", "repository identity")
	flags.StringVar(&options.remote, "remote", "", "remote identity")
	flags.IntVar(&options.epoch, "epoch", 0, "initial delivery epoch (must be 1 without --delivery-bead)")
	flags.StringVar(&options.deliveryBead, "delivery-bead", "", "explicit ready delivery bead selected by the sweep")
	flags.StringVar(&options.mode, "mode", "", "mode")
	flags.StringVar(&options.deadline, "deadline", "", "deadline")
	flags.StringVar(&options.preparedAt, "prepared-at", "", "prepared timestamp")
	flags.StringVar(&options.committedAt, "committed-at", "", "committed timestamp")
	flags.StringVar(&options.baseRef, "base-ref", "", "base ref")
	flags.StringVar(&options.baseOID, "base-oid", "", "base oid")
	flags.StringVar(&options.observedAt, "observed-at", "", "caller-attested observation time used only for deadline reduction")
	flags.StringVar(&options.gcBin, "gc-bin", "", "deployment-pinned Gas City binary")
	flags.StringVar(&options.beadsBin, "beads-bin", "", "deployment-pinned Beads binary")
	flags.StringVar(&options.gitBin, "git-bin", "", "pinned Git binary")
	flags.StringVar(&options.ghBin, "gh-bin", "", "pinned GitHub CLI binary")
	flags.StringVar(&options.bashBin, "bash-bin", "", "pinned Bash binary for check-only gates")
	return options, flags.Parse(args)
}

func runCommand(ctx context.Context, command string, options commandOptions) (any, error) {
	switch command {
	case "status":
		return runStatus(ctx, options)
	case "sweep":
		return runSweep(ctx, options)
	default:
		return runStep(ctx, options)
	}
}

func providersFor(options commandOptions, native delivery.NativeContext) (*delivery.NativeProviders, error) {
	deliveryBin, err := os.Executable()
	if err != nil {
		return nil, err
	}
	return delivery.NewNativeProviders(delivery.NativeBinaries{GC: options.gcBin, Beads: options.beadsBin, Git: options.gitBin, GH: options.ghBin, Bash: options.bashBin, Delivery: deliveryBin}, native)
}

func runStatus(ctx context.Context, options commandOptions) (any, error) {
	observation, err := statusObservation(options.observedAt)
	if err != nil {
		return nil, err
	}
	native, _, err := delivery.ReadExactNativeContext(options.nativePath, options.nativeDigest)
	if err != nil {
		return nil, err
	}
	providers, err := providersFor(options, native)
	if err != nil {
		return nil, err
	}
	return providers.DeliveryStatus(ctx, observation)
}

func runSweep(ctx context.Context, options commandOptions) (any, error) {
	native, _, err := delivery.ReadExactNativeContext(options.nativePath, options.nativeDigest)
	if err != nil {
		return nil, err
	}
	if native.RigID != options.rig || native.Repository != options.repository || native.Remote != options.remote {
		return nil, fmt.Errorf("sweep controller identity does not match native context")
	}
	providers, err := providersFor(options, native)
	if err != nil {
		return nil, err
	}
	controller := delivery.SweepController{Root: options.root, RigID: options.rig, Repository: options.repository, Remote: options.remote}
	return delivery.SweepReady(ctx, providers, providers, controller)
}

func runStep(ctx context.Context, options commandOptions) (any, error) {
	bytes, err := os.ReadFile(options.certificatePath)
	if err != nil {
		return nil, err
	}
	var certificate delivery.AdmissionCertificate
	if err := json.Unmarshal(bytes, &certificate); err != nil {
		return nil, err
	}
	subject, subjectBytes, err := delivery.ReadExactSubjectManifest(options.subjectPath, options.subjectDigest)
	if err != nil {
		return nil, err
	}
	native, nativeBytes, err := delivery.ReadExactNativeContext(options.nativePath, options.nativeDigest)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(bytes)
	request := delivery.Request{Root: options.root, Certificate: certificate, CertificateBytes: bytes, CertificateDigest: hex.EncodeToString(sum[:]), Target: delivery.Target{DeliveryBeadID: options.deliveryBead, SemanticBeadID: options.semantic, SemanticTerminalRef: options.terminalRef, RigID: options.rig, Repository: options.repository, Remote: options.remote, Epoch: options.epoch, Mode: options.mode, Deadline: options.deadline, PreparedAt: options.preparedAt, CommittedAt: options.committedAt, BaseRef: options.baseRef, BaseOID: options.baseOID, ObservedAt: options.observedAt}, SubjectManifest: subject, SubjectBytes: subjectBytes, SubjectDigest: options.subjectDigest, NativeContext: native, NativeBytes: nativeBytes, NativeDigest: options.nativeDigest}
	providers, err := providersFor(options, native)
	if err != nil {
		return nil, err
	}
	return delivery.NewReducer(providers, nil).Step(ctx, request)
}
func fail(err error) { fmt.Fprintln(os.Stderr, "agentops-gc-delivery:", err); os.Exit(1) }

func statusObservation(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, fmt.Errorf("status requires --observed-at RFC3339")
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil || parsed.UTC().Format(time.RFC3339) != value {
		return time.Time{}, fmt.Errorf("status --observed-at must be canonical RFC3339 UTC")
	}
	return parsed, nil
}
