// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd016-dirname R1.1–R1.4
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// programName is the name used in error output.
const programName = "dirname"

func main() {
	// D2: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// R3.2: no arguments is an error.
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", programName)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
		os.Exit(1)
	}

	// R1.5: process each argument and print one result per line.
	for _, name := range args {
		result := dirname(name)
		fmt.Println(result)
	}
}

// dirname extracts the directory component from name, implementing the POSIX
// dirname algorithm.
//
// R1.1: Strip trailing slashes, then remove the last component.
// R1.2: If name contains no '/' after trailing-slash removal, return ".".
// R1.3: Trailing slashes are stripped before extracting the directory component.
// R1.4: Handle root path (/) and all-slash inputs by returning "/".
func dirname(name string) string {
	// R1.3: strip trailing slashes.
	stripped := strings.TrimRight(name, "/")

	// R1.4: all slashes or empty → "/". Empty input per GNU dirname also yields ".".
	if stripped == "" {
		if name == "" {
			return "."
		}
		return "/"
	}

	// R1.2: no slash means no directory component → ".".
	i := strings.LastIndex(stripped, "/")
	if i < 0 {
		return "."
	}

	// R1.1: take everything before the last '/'.
	dir := stripped[:i]

	// R1.4: strip trailing slashes from result; if empty, return "/".
	dir = strings.TrimRight(dir, "/")
	if dir == "" {
		return "/"
	}
	return dir
}
