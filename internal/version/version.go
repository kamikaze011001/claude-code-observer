// Package version holds the build-time version metadata. Values are set via
// -ldflags at build time; the defaults below apply for `go run` and tests.
package version

var (
	// Version is the human-readable version string (e.g. "v0.3.1" or a git
	// describe output). Defaults to "dev".
	Version = "dev"
	// Commit is the short git SHA the binary was built from. Defaults to
	// "none".
	Commit = "none"
)
