// Package buildinfo carries build-time version metadata for the binaries.
package buildinfo

// Values are overridden at build time via -ldflags "-X ...".
var (
	// Version is the semantic version of the build.
	Version = "dev"
	// Commit is the source revision the build was produced from.
	Commit = "unknown"
	// Date is the UTC build timestamp (RFC3339).
	Date = "unknown"
)
