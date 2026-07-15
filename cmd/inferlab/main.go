// Command inferlab is the command-line entry point for InferLab.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/gaurav-gs7/InferLab/internal/buildinfo"
	"github.com/gaurav-gs7/InferLab/pkg/adapter"
	"github.com/gaurav-gs7/InferLab/pkg/change"
	"github.com/gaurav-gs7/InferLab/pkg/evidence"
	"github.com/gaurav-gs7/InferLab/pkg/gate"
)

const usage = `InferLab builds pre-production safety evidence for LLM inference changes.

Usage:
  inferlab <command>

Commands:
  adapter    List adapters, inspect capabilities, or normalize evidence
  change     Validate or digest an inference-change document
  evidence   Validate or digest an evidence envelope
  gate       Evaluate an evidence gate or inspect its documents
  runtime    Validate or digest a runtime signature
  policies   List built-in scheduling policies
  version    Print build version information
  help       Show this help
`

const adapterUsage = `Usage:
  inferlab adapter list
  inferlab adapter capabilities <name>
  inferlab adapter normalize <name> <input-path>
  inferlab adapter validate <normalized-report-path>
  inferlab adapter digest <normalized-report-path>
`

const changeUsage = `Usage:
  inferlab change validate <path>
  inferlab change digest <path>
`

const evidenceUsage = `Usage:
  inferlab evidence validate <path>
  inferlab evidence digest <path>
`

const gateUsage = `Usage:
  inferlab gate evaluate <evaluation-path>
  inferlab gate evaluation validate <evaluation-path>
  inferlab gate result validate <result-path>
  inferlab gate result digest <result-path>

Exit codes for gate evaluate:
  0  PASS
  3  BLOCK
  4  INCONCLUSIVE
  1  invalid input or execution failure
  2  usage error
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
	case "adapter":
		return runAdapter(args[1:], stdout, stderr)
	case "evidence":
		return runEvidence(args[1:], stdout, stderr)
	case "gate":
		return runGate(args[1:], stdout, stderr)
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

func runGate(args []string, stdout, stderr io.Writer) int {
	if len(args) == 2 && args[0] == "evaluate" {
		file, err := os.Open(args[1])
		if err != nil {
			fmt.Fprintf(stderr, "open gate evaluation: %v\n", err)
			return 1
		}
		defer file.Close()
		evaluation, err := gate.DecodeEvaluation(file)
		if err != nil {
			fmt.Fprintf(stderr, "invalid gate evaluation: %v\n", err)
			return 1
		}
		result, err := gate.Evaluate(evaluation)
		if err != nil {
			fmt.Fprintf(stderr, "evaluate gate: %v\n", err)
			return 1
		}
		encoded, err := gate.CanonicalResultJSON(result)
		if err != nil {
			fmt.Fprintf(stderr, "encode gate result: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, string(encoded))
		switch result.Decision {
		case gate.DecisionPass:
			return 0
		case gate.DecisionBlock:
			return 3
		case gate.DecisionInconclusive:
			return 4
		default:
			return 1
		}
	}
	if len(args) == 3 && args[0] == "evaluation" && args[1] == "validate" {
		file, err := os.Open(args[2])
		if err != nil {
			fmt.Fprintf(stderr, "open gate evaluation: %v\n", err)
			return 1
		}
		defer file.Close()
		evaluation, err := gate.DecodeEvaluation(file)
		if err != nil {
			fmt.Fprintf(stderr, "invalid gate evaluation: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "valid %s\n", evaluation.Name)
		return 0
	}
	if len(args) == 3 && args[0] == "result" && (args[1] == "validate" || args[1] == "digest") {
		file, err := os.Open(args[2])
		if err != nil {
			fmt.Fprintf(stderr, "open gate result: %v\n", err)
			return 1
		}
		defer file.Close()
		result, err := gate.DecodeResult(file)
		if err != nil {
			fmt.Fprintf(stderr, "invalid gate result: %v\n", err)
			return 1
		}
		digest, err := gate.ResultDigest(result)
		if err != nil {
			fmt.Fprintf(stderr, "digest gate result: %v\n", err)
			return 1
		}
		if args[1] == "digest" {
			fmt.Fprintln(stdout, digest)
		} else {
			fmt.Fprintf(stdout, "valid %s %s %s\n", result.Evaluation, digest, result.Decision)
		}
		return 0
	}
	fmt.Fprint(stderr, gateUsage)
	return 2
}

func runAdapter(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && args[0] == "list" {
		for _, implementation := range adapter.Builtins() {
			capabilities := implementation.Capabilities()
			classes := make([]string, 0, len(capabilities.Classifications))
			for _, classification := range capabilities.Classifications {
				classes = append(classes, string(classification))
			}
			fmt.Fprintf(stdout, "%s\t%s@%s\t%s\n", capabilities.Adapter.Name, capabilities.Producer.Tool, capabilities.Producer.ToolVersion, strings.Join(classes, ","))
		}
		return 0
	}
	if len(args) == 2 && args[0] == "capabilities" {
		implementation, ok := adapter.Builtin(args[1])
		if !ok {
			fmt.Fprintf(stderr, "unknown adapter %q\n", args[1])
			return 1
		}
		encoded, err := adapter.MarshalCapabilities(implementation.Capabilities())
		if err != nil {
			fmt.Fprintf(stderr, "encode capabilities: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, string(encoded))
		return 0
	}
	if len(args) == 3 && args[0] == "normalize" {
		implementation, ok := adapter.Builtin(args[1])
		if !ok {
			fmt.Fprintf(stderr, "unknown adapter %q\n", args[1])
			return 1
		}
		file, err := os.Open(args[2])
		if err != nil {
			fmt.Fprintf(stderr, "open adapter input: %v\n", err)
			return 1
		}
		defer file.Close()
		input, err := adapter.DecodeInput(file)
		if err != nil {
			fmt.Fprintf(stderr, "invalid adapter input: %v\n", err)
			return 1
		}
		report, err := implementation.Normalize(input)
		if err != nil {
			fmt.Fprintf(stderr, "normalize evidence: %v\n", err)
			return 1
		}
		encoded, err := adapter.CanonicalJSON(report)
		if err != nil {
			fmt.Fprintf(stderr, "encode normalized report: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, string(encoded))
		return 0
	}
	if len(args) == 2 && (args[0] == "validate" || args[0] == "digest") {
		file, err := os.Open(args[1])
		if err != nil {
			fmt.Fprintf(stderr, "open normalized report: %v\n", err)
			return 1
		}
		defer file.Close()
		report, err := adapter.DecodeNormalizedReport(file)
		if err != nil {
			fmt.Fprintf(stderr, "invalid normalized report: %v\n", err)
			return 1
		}
		digest, err := adapter.Digest(report)
		if err != nil {
			fmt.Fprintf(stderr, "digest normalized report: %v\n", err)
			return 1
		}
		if args[0] == "digest" {
			fmt.Fprintln(stdout, digest)
			return 0
		}
		fmt.Fprintf(stdout, "valid %s %s %s\n", report.Adapter.Name, digest, report.Envelope.Classification)
		return 0
	}
	fmt.Fprint(stderr, adapterUsage)
	return 2
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
