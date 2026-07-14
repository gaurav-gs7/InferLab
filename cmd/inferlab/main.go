// Command inferlab is the command-line entry point for InferLab.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/gaurav-gs7/InferLab/internal/buildinfo"
)

const usage = `InferLab develops and validates production LLM inference schedulers.

Usage:
  inferlab <command>

Commands:
  policies   List built-in scheduling policies
  version    Print build version information
  help       Show this help
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
	case "policies":
		fmt.Fprintln(stdout, "round-robin")
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}
