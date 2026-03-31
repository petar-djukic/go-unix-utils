// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/link implements GNU link: create a hard link via link(2).
//
// Implements prd084-link R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3.
package main

import (
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "link"

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stderr))
}

// run executes the link logic and returns the exit code.
// R1.1: calls os.Link to create a hard link from FILE2 to FILE1.
// R1.2: uses raw link(2) semantics — no symlink following or directory handling.
// R1.3: exits 1 on wrong number of arguments.
// R1.4: exits 1 on link(2) failure.
func run(args []string, stderr *os.File) int {
	if len(args) == 0 {
		// R1.3: missing operand error.
		fmt.Fprintf(stderr, "%s: missing operand\n", progName)                   //nolint:errcheck
		fmt.Fprintf(stderr, "Try '%s --help' for more information.\n", progName) //nolint:errcheck
		return 1
	}

	if len(args) == 1 {
		// R1.3: missing destination operand.
		fmt.Fprintf(stderr, "%s: missing operand after '%s'\n", progName, args[0]) //nolint:errcheck
		fmt.Fprintf(stderr, "Try '%s --help' for more information.\n", progName)   //nolint:errcheck
		return 1
	}

	if len(args) > 2 {
		// R1.3: extra operand error.
		fmt.Fprintf(stderr, "%s: extra operand '%s'\n", progName, args[2])       //nolint:errcheck
		fmt.Fprintf(stderr, "Try '%s --help' for more information.\n", progName) //nolint:errcheck
		return 1
	}

	// R1.1, R1.2: create hard link using os.Link which calls link(2).
	if err := os.Link(args[0], args[1]); err != nil {
		// R1.4: report link(2) failure.
		fmt.Fprintf(stderr, "%s: cannot create link '%s' to '%s': %s\n", progName, args[1], args[0], unwrapErr(err)) //nolint:errcheck
		return 1
	}

	// R2.1: exit 0 on success.
	return 0
}

// unwrapErr extracts the innermost error message, stripping os.LinkError wrapper.
func unwrapErr(err error) string {
	if le, ok := err.(*os.LinkError); ok {
		return le.Err.Error()
	}
	return err.Error()
}
