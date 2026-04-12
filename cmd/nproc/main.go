// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/nproc: print number of available processing units.
// Implements srd046-nproc R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3.
package main

import (
	"errors"
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

// usageError indicates an error that should include a "Try --help" hint.
type usageError struct {
	msg string
}

func (e *usageError) Error() string { return e.msg }

func main() {
	sys.InstallSIGPIPEHandler()

	count, err := run(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		var ue *usageError
		if errors.As(err, &ue) {
			fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
		}
		os.Exit(1)
	}

	fmt.Println(count)
}

// run parses flags and computes the processor count.
// R1.1: default prints available count.
// R1.2: --ignore=N subtracts N, clamping to 1.
// R1.3: --all prints installed count.
// R1.4: --all and --ignore=N may be combined.
// R2.1: positional operands produce an error.
// R2.2: non-numeric --ignore produces an error.
// R2.3: unknown flags produce an error.
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
		n, ok, err := parseIgnore(arg)
		if err != nil {
			return 0, err
		}
		if ok {
			ignoreVal = n
			continue
		}
		if strings.HasPrefix(arg, "--") || strings.HasPrefix(arg, "-") {
			return 0, &usageError{msg: fmt.Sprintf("unrecognized option '%s'", arg)}
		}
		// R2.1: positional operands are rejected.
		return 0, &usageError{msg: fmt.Sprintf("extra operand '%s'", arg)}
	}

	count := processorCount(allFlag)
	count -= ignoreVal
	if count < 1 {
		count = 1
	}
	return count, nil
}

// parseIgnore checks if arg is --ignore=N and returns (N, true, nil) if so.
// R2.2: returns an error if the value is not a valid number.
func parseIgnore(arg string) (int, bool, error) {
	val, found := strings.CutPrefix(arg, "--ignore=")
	if !found {
		return 0, false, nil
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return 0, false, fmt.Errorf("invalid number: '%s'", val)
	}
	return n, true, nil
}

// processorCount returns the CPU count based on the --all flag.
// When OMP_NUM_THREADS is set and valid, it is used as the base count
// instead of runtime.NumCPU() (unless --all is specified).
// R1.1: available count via runtime.NumCPU().
// R1.3: --all also uses runtime.NumCPU() since Go does not distinguish
// installed vs available on macOS (per srd046 non_goals).
func processorCount(all bool) int {
	if all {
		return runtime.NumCPU()
	}
	// OMP_NUM_THREADS overrides the available count when not using --all.
	if env := os.Getenv("OMP_NUM_THREADS"); env != "" {
		// OMP_NUM_THREADS may contain a comma-separated list; use first value.
		parts := strings.SplitN(env, ",", 2)
		if n, err := strconv.Atoi(strings.TrimSpace(parts[0])); err == nil && n > 0 {
			return n
		}
	}
	return runtime.NumCPU()
}
