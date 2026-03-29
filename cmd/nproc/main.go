// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/nproc prints the number of available processing units.
// Implements prd046-nproc R1.1–R1.4, R2.1–R2.3, R3.1.
package main

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// programName is the name used in error messages.
const programName = "nproc"

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	sys.InstallSIGPIPEHandler()

	allFlag, ignore, exit, done := parseArgs(os.Args[1:])
	if done {
		os.Exit(exit)
	}

	count := cpuCount(allFlag)
	count -= ignore
	if count < 1 {
		count = 1
	}
	fmt.Println(count)
}

// cpuCount returns the processor count.
// R1.1: available count by default.
// R1.2: --all returns installed count (same as available on Darwin).
func cpuCount(_ bool) int {
	return runtime.NumCPU()
}

// parseArgs processes command-line arguments for --all and --ignore=N.
// Returns (allFlag, ignoreN, exitCode, shouldExit).
func parseArgs(args []string) (bool, int, int, bool) {
	allFlag := false
	ignore := 0

	for _, arg := range args {
		switch {
		case arg == "--version":
			fmt.Printf("%s (go-unix-utils) %s\n", programName, version)
			return false, 0, 0, true
		case arg == "--help":
			printHelp()
			return false, 0, 0, true
		case arg == "--all":
			allFlag = true
		case arg == "--ignore":
			fmt.Fprintf(os.Stderr,
				"%s: option '--ignore' requires an argument\n", programName)
			return false, 0, 1, true
		case strings.HasPrefix(arg, "--ignore="):
			val := arg[len("--ignore="):]
			n, err := strconv.Atoi(val)
			if err != nil {
				fmt.Fprintf(os.Stderr,
					"%s: invalid number: '%s'\n", programName, val)
				return false, 0, 1, true
			}
			ignore = n
		case strings.HasPrefix(arg, "-") && arg != "-":
			fmt.Fprintf(os.Stderr,
				"%s: unrecognized option '%s'\n", programName, arg)
			return false, 0, 1, true
		default:
			fmt.Fprintf(os.Stderr,
				"%s: extra operand '%s'\n", programName, arg)
			return false, 0, 1, true
		}
	}
	return allFlag, ignore, 0, false
}

// printHelp writes usage information to stdout.
func printHelp() {
	fmt.Printf("Usage: %s [OPTION]...\n", programName)
	fmt.Println("Print the number of processing units available.")
	fmt.Println()
	fmt.Println("      --all       print the number of installed processors")
	fmt.Println("      --ignore=N  if possible, exclude N processing units")
	fmt.Println("      --help      display this help and exit")
	fmt.Println("      --version   output version information and exit")
}
