// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/nproc: print number of available processing units.
// Implements srd046-nproc R1.1, R1.2, R1.3, R1.4.
package main

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is used in error messages.
const progName = "nproc"

// versionText is printed when --version is passed.
const versionText = progName + " (go-unix-utils)"

// helpText is the usage message printed when --help is passed.
const helpText = `Usage: nproc [OPTION]...
Print the number of processing units available to the current process,
which may be less than the number of online processors.

      --all      print the number of installed processors
      --ignore=N  if possible, exclude N processing units
      --help     display this help and exit
      --version  output version information and exit
`

func main() {
	sys.InstallSIGPIPEHandler()

	count, err := run(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		os.Exit(1)
	}

	fmt.Println(count)
}

// run parses flags and computes the processor count.
// R1.1: default prints available count.
// R1.2: --ignore=N subtracts N, clamping to 1.
// R1.3: --all prints installed count.
// R1.4: --all and --ignore=N may be combined.
func run(args []string) (int, error) {
	allFlag := false
	ignoreVal := 0

	for _, arg := range args {
		if arg == "--help" {
			fmt.Print(helpText)
			os.Exit(0)
		}
		if arg == "--version" {
			fmt.Println(versionText)
			os.Exit(0)
		}
		if arg == "--all" {
			allFlag = true
			continue
		}
		n, ok := parseIgnore(arg)
		if ok {
			ignoreVal = n
			continue
		}
		if strings.HasPrefix(arg, "--") {
			return 0, fmt.Errorf("unrecognized option '%s'", arg)
		}
		return 0, fmt.Errorf("extra operand '%s'", arg)
	}

	count := processorCount(allFlag)
	count -= ignoreVal
	if count < 1 {
		count = 1
	}
	return count, nil
}

// parseIgnore checks if arg is --ignore=N and returns (N, true) if so.
func parseIgnore(arg string) (int, bool) {
	val, found := strings.CutPrefix(arg, "--ignore=")
	if !found {
		return 0, false
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s: invalid number to ignore\n", progName, val)
		os.Exit(1)
	}
	return n, true
}

// processorCount returns the CPU count based on the --all flag.
// R1.1: available count via runtime.NumCPU().
// R1.3: --all also uses runtime.NumCPU() since Go does not distinguish
// installed vs available on macOS (per srd046 non_goals).
func processorCount(_ bool) int {
	return runtime.NumCPU()
}
