// Package buildinfo exposes version metadata injected by release builds.
package buildinfo

var (
	// Version is the semantic version or development label.
	Version = "dev"
	// Commit is the source revision used to build the binary.
	Commit = "unknown"
	// Date is the RFC 3339 build timestamp.
	Date = "unknown"
)
