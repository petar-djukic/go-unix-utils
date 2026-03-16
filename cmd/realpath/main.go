// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd049-realpath R1.1-R1.4, R3.1, R3.3:
// cmd/realpath resolves each command-line path argument to its canonical
// absolute pathname with all symlinks resolved, prints one per line,
// and reports errors for nonexistent paths. Installs SIGPIPE handler
// for clean exit on broken pipe.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is the name used in diagnostic output.
const progName = "realpath"

func main() {
	// D1: install SIGPIPE handler per shared protocol.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// R1.3 (task R3): no operands → usage error to stderr, exit 1.
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", progName)                   //nolint:errcheck // best-effort diagnostic
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName) //nolint:errcheck // best-effort diagnostic
		os.Exit(1)
	}

	exitCode := 0

	// D4: process arguments left to right; on error, print error and continue.
	for _, arg := range args {
		resolved, err := resolve(arg)
		if err != nil {
			// R1.2/R1.4 (task R4): print error to stderr, set exit code 1.
			fmt.Fprintf(os.Stderr, "%s: %s: %s\n", progName, arg, err) //nolint:errcheck // best-effort diagnostic
			exitCode = 1
			continue
		}
		// R1.1/R1.2 (task R1, R2): print resolved path on its own line.
		fmt.Fprintln(os.Stdout, resolved) //nolint:errcheck // best-effort output
	}

	os.Exit(exitCode)
}

// resolve canonicalizes path to its absolute form with symlinks resolved.
// R1.1: GNU realpath default behavior requires all parent components to exist
// but allows the final component to be missing. It resolves symlinks in the
// existing prefix and appends the remaining base name.
func resolve(path string) (string, error) {
	// Try full resolution first — works when the entire path exists.
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return filepath.Abs(resolved)
	}

	// R1.1/R1.2: if the full path doesn't exist, resolve the parent directory
	// (which must exist) and append the base name. This matches GNU realpath
	// default behavior where the last component may be missing.
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	resolvedDir, dirErr := filepath.EvalSymlinks(dir)
	if dirErr != nil {
		// Parent doesn't exist — return the original error.
		return "", err
	}

	absDir, dirErr := filepath.Abs(resolvedDir)
	if dirErr != nil {
		return "", dirErr
	}

	return filepath.Join(absDir, base), nil
}
