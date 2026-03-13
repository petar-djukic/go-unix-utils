// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd015-basename R1.1–R1.4
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// programName is the name used in error and --help output.
const programName = "basename"

func main() {
	// D1: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// R1.4: handle --help and --version before argument validation.
	if len(args) == 1 {
		switch args[0] {
		case "--help":
			printHelp()
			return
		case "--version":
			printVersion()
			return
		}
	}

	// R3.3, R3.4: validate argument count.
	if len(args) == 0 || len(args) > 2 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", programName)
		if len(args) == 0 {
			fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
		} else {
			fmt.Fprintf(os.Stderr, "%s: extra operand '%s'\n", programName, args[2])
			fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
		}
		os.Exit(1)
	}

	name := args[0]
	result := basename(name)

	// R1.2: strip SUFFIX if second operand provided.
	if len(args) == 2 {
		suffix := args[1]
		if suffix != "" && result != suffix && strings.HasSuffix(result, suffix) {
			result = result[:len(result)-len(suffix)]
		}
	}

	// R1.3: print result followed by a newline. R3.2: exit 0 on success.
	fmt.Println(result)
}

// basename strips the directory component from name, implementing the POSIX
// basename algorithm.
//
// R1.1: Strip the longest prefix ending in '/'.
// R1.3: Strip trailing slashes before removing the directory component.
// R1.4: If name consists entirely of slashes, return "/".
// R1.5: If name is empty, return "".
func basename(name string) string {
	// R1.5: empty string returns empty.
	if name == "" {
		return ""
	}

	// R1.3: strip trailing slashes.
	stripped := strings.TrimRight(name, "/")

	// R1.4: all slashes → "/".
	if stripped == "" {
		return "/"
	}

	// R1.1: find the last '/' and take everything after it.
	if i := strings.LastIndex(stripped, "/"); i >= 0 {
		return stripped[i+1:]
	}
	return stripped
}

// printHelp writes usage information to stdout and exits 0.
// R1.4: --help flag support.
func printHelp() {
	fmt.Print(`Usage: basename NAME [SUFFIX]
  or:  basename OPTION... NAME...
Print NAME with any leading directory components removed.
If specified, also remove a trailing SUFFIX.

      --help     display this help and exit
      --version  output version information and exit
`)
}

// printVersion writes version information to stdout and exits 0.
// R1.4: --version flag support.
func printVersion() {
	fmt.Println("basename (go-unix-utils) 0.1")
}
