package cli

import (
	"fmt"
	"io"
)

// runPkg implements `pr pkg ...` for the purepkg registry. The actual
// registry logic lives in internal/purepkg; this file is a thin wrapper
// that wires it into the CLI.
func runPkg(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: pr pkg <publish|info|search>")
		return 1
	}
	switch args[0] {
	case "publish":
		return runPkgPublish(args[1:], stdout, stderr)
	case "info":
		return runPkgInfo(args[1:], stdout, stderr)
	case "search":
		return runPkgSearch(args[1:], stdout, stderr)
	}
	fmt.Fprintf(stderr, "unknown pkg subcommand %q\n", args[0])
	return 1
}

// Default no-op stubs; replaced in pkg_purepkg.go once the registry is wired in.
var (
	runPkgPublish = func(args []string, stdout, stderr io.Writer) int {
		fmt.Fprintln(stderr, "purepkg is not configured")
		return 1
	}
	runPkgInfo = func(args []string, stdout, stderr io.Writer) int {
		fmt.Fprintln(stderr, "purepkg is not configured")
		return 1
	}
	runPkgSearch = func(args []string, stdout, stderr io.Writer) int {
		fmt.Fprintln(stderr, "purepkg is not configured")
		return 1
	}
)
