// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd048-hostid: Print Numeric Host Identifier.
// Covers R1.1-R1.2 (default behavior, gethostid syscall),
// R2.1-R2.2 (extra operand/unknown flag errors),
// R3.1-R3.2 (differential testing).
package main

/*
#include <unistd.h>
*/
import "C"

import (
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:])
	os.Exit(exitCode)
}

// run parses arguments and prints the host identifier. Returns exit code.
func run(args []string) int {
	for _, arg := range args {
		switch arg {
		case "--help":
			return printHelp()
		case "--version":
			return printVersion()
		case "--":
			// Ignore, but any following args are extra operands.
		default:
			if len(arg) > 1 && arg[0] == '-' {
				// R2.2: unknown flags produce an error.
				fmt.Fprintf(os.Stderr, "hostid: unrecognized option '%s'\n", arg)
				fmt.Fprintln(os.Stderr, "Try 'hostid --help' for more information.")
				return 1
			}
			// R2.1: extra operand.
			fmt.Fprintf(os.Stderr, "hostid: extra operand '%s'\n", arg)
			fmt.Fprintln(os.Stderr, "Try 'hostid --help' for more information.")
			return 1
		}
	}

	// R1.1, R1.2: print 32-bit host identifier as 8-digit lowercase hex.
	hostID := getHostID()

	// R1.1: 8-digit lowercase hexadecimal followed by newline.
	if _, err := fmt.Printf("%08x\n", uint32(hostID)); err != nil {
		return 1
	}
	return 0
}

// getHostID calls the gethostid(3) libc function via cgo.
// R1.2: value derived from gethostid(3) syscall.
func getHostID() int64 {
	return int64(C.gethostid())
}

// printHelp writes usage information to stdout and returns the exit code.
func printHelp() int {
	_, err := fmt.Fprint(os.Stdout, `Usage: hostid [OPTION]
Print the numeric identifier (in hexadecimal) for the current host.

      --help     display this help and exit
      --version  output version information and exit
`)
	if err != nil {
		return 1
	}
	return 0
}

// printVersion writes version information to stdout and returns the exit code.
func printVersion() int {
	_, err := fmt.Fprintf(os.Stdout, "hostid (go-unix-utils) %s\n", version)
	if err != nil {
		return 1
	}
	return 0
}
