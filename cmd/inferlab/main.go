// Command inferlab is the command-line entry point for InferLab.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/gaurav-gs7/InferLab/internal/buildinfo"
	"github.com/gaurav-gs7/InferLab/pkg/change"
)

const usage = `InferLab builds pre-production safety evidence for LLM inference changes.

Usage:
  inferlab <command>

Commands:
  change     Validate or digest an inference-change document
  policies   List built-in scheduling policies
  version    Print build version information
  help       Show this help
`

const changeUsage = `Usage:
  inferlab change validate <path>
  inferlab change digest <path>
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
	case "policies":
		fmt.Fprintln(stdout, "round-robin")
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n%s", args[0], usage)
		return 2
	}
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
