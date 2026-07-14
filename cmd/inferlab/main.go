// Command inferlab is the command-line entry point for InferLab.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/gaurav-gs7/InferLab/internal/buildinfo"
	"github.com/gaurav-gs7/InferLab/pkg/change"
	"github.com/gaurav-gs7/InferLab/pkg/evidence"
)

const usage = `InferLab builds pre-production safety evidence for LLM inference changes.

Usage:
  inferlab <command>

Commands:
  change     Validate or digest an inference-change document
  evidence   Validate or digest an evidence envelope
  runtime    Validate or digest a runtime signature
  policies   List built-in scheduling policies
  version    Print build version information
  help       Show this help
`

const changeUsage = `Usage:
  inferlab change validate <path>
  inferlab change digest <path>
`

const evidenceUsage = `Usage:
  inferlab evidence validate <path>
  inferlab evidence digest <path>
`

const runtimeUsage = `Usage:
  inferlab runtime validate <path>
  inferlab runtime digest <path>
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}

	switch args[0] {
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return 0
	case "version":
		fmt.Fprintf(stdout, "inferlab %s (commit=%s, built=%s)\n", buildinfo.Version, buildinfo.Commit, buildinfo.Date)
		return 0
	case "change":
		return runChange(args[1:], stdout, stderr)
	case "evidence":
		return runEvidence(args[1:], stdout, stderr)
	case "runtime":
		return runRuntime(args[1:], stdout, stderr)
	case "policies":
		fmt.Fprintln(stdout, "round-robin")
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}

func runEvidence(args []string, stdout, stderr io.Writer) int {
	if len(args) != 2 || (args[0] != "validate" && args[0] != "digest") {
		fmt.Fprint(stderr, evidenceUsage)
		return 2
	}
	file, err := os.Open(args[1])
	if err != nil {
		fmt.Fprintf(stderr, "open evidence envelope: %v\n", err)
		return 1
	}
	defer file.Close()
	envelope, err := evidence.Decode(file)
	if err != nil {
		fmt.Fprintf(stderr, "invalid evidence envelope: %v\n", err)
		return 1
	}
	digest, err := evidence.Digest(envelope)
	if err != nil {
		fmt.Fprintf(stderr, "digest evidence envelope: %v\n", err)
		return 1
	}
	if args[0] == "digest" {
		fmt.Fprintln(stdout, digest)
		return 0
	}
	fmt.Fprintf(stdout, "valid %s %s %s %s\n", envelope.Name, digest, envelope.Classification, envelope.Completeness)
	return 0
}

func runRuntime(args []string, stdout, stderr io.Writer) int {
	if len(args) != 2 || (args[0] != "validate" && args[0] != "digest") {
		fmt.Fprint(stderr, runtimeUsage)
		return 2
	}
	file, err := os.Open(args[1])
	if err != nil {
		fmt.Fprintf(stderr, "open runtime signature: %v\n", err)
		return 1
	}
	defer file.Close()
	signature, err := evidence.DecodeRuntimeSignature(file)
	if err != nil {
		fmt.Fprintf(stderr, "invalid runtime signature: %v\n", err)
		return 1
	}
	digest, err := evidence.RuntimeSignatureDigest(signature)
	if err != nil {
		fmt.Fprintf(stderr, "digest runtime signature: %v\n", err)
		return 1
	}
	if args[0] == "digest" {
		fmt.Fprintln(stdout, digest)
		return 0
	}
	unknown, err := evidence.UnknownDimensions(signature)
	if err != nil {
		fmt.Fprintf(stderr, "inspect runtime signature: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "valid %s %s unknown=%d\n", digest, signature.Origin, len(unknown))
	return 0
}

func runChange(args []string, stdout, stderr io.Writer) int {
	if len(args) != 2 || (args[0] != "validate" && args[0] != "digest") {
		fmt.Fprint(stderr, changeUsage)
		return 2
	}

	file, err := os.Open(args[1])
	if err != nil {
		fmt.Fprintf(stderr, "open inference change: %v\n", err)
		return 1
	}
	defer file.Close()

	document, err := change.Decode(file)
	if err != nil {
		fmt.Fprintf(stderr, "invalid inference change: %v\n", err)
		return 1
	}
	digest, err := change.Digest(document)
	if err != nil {
		fmt.Fprintf(stderr, "digest inference change: %v\n", err)
		return 1
	}

	if args[0] == "digest" {
		fmt.Fprintln(stdout, digest)
		return 0
	}
	fmt.Fprintf(stdout, "valid %s %s\n", document.Name, digest)
	return 0
}
