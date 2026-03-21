// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd061-sleep R1.1-R1.4: core sleep behavior with duration parsing,
// fractional seconds, suffix multipliers, and multiple argument summing.
package main

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// programName is used in error messages.
const programName = "sleep"

// helpText is the usage message printed for --help.
const helpText = `Usage: sleep NUMBER[SUFFIX]...
  or:  sleep OPTION
Pause for NUMBER seconds.  SUFFIX may be 's' for seconds (the default),
'm' for minutes, 'h' for hours or 'd' for days.  NUMBER need not be an
integer.  Given two or more arguments, pause for the amount of time
specified by the sum of their values.

      --help        display this help and exit
      --version     output version information and exit
`

// versionText is the version message printed for --version.
const versionText = `sleep (go-unix-utils) 1.0
`

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]
	if len(args) == 0 {
		exitWithError("missing operand")
	}

	if args[0] == "--help" {
		printAndExit(helpText)
	}
	if args[0] == "--version" {
		printAndExit(versionText)
	}

	total, err := sumDurations(args)
	if err != nil {
		exitWithError(err.Error())
	}

	time.Sleep(total)
}

// sumDurations parses each argument as a duration and returns the sum.
// R1.4: multiple arguments are summed.
func sumDurations(args []string) (time.Duration, error) {
	var total time.Duration
	for _, arg := range args {
		d, err := parseDuration(arg)
		if err != nil {
			return 0, err
		}
		total += d
	}
	return total, nil
}

// parseDuration parses a single duration argument with optional suffix.
// R1.1: numeric seconds. R1.2: fractional seconds. R1.3: suffix multipliers.
func parseDuration(arg string) (time.Duration, error) {
	multiplier := 1.0
	numStr := arg

	if len(arg) > 0 {
		suffix := arg[len(arg)-1]
		multiplier, numStr = applySuffix(arg, suffix)
	}

	if strings.EqualFold(numStr, "infinity") || strings.EqualFold(numStr, "inf") {
		return time.Duration(math.MaxInt64), nil
	}

	seconds, err := strconv.ParseFloat(numStr, 64)
	if err != nil || seconds < 0 {
		return 0, fmt.Errorf("invalid time interval %q", arg)
	}

	return time.Duration(seconds * multiplier * float64(time.Second)), nil
}

// applySuffix extracts the suffix multiplier from the argument.
// Returns the multiplier and the numeric portion of the string.
func applySuffix(arg string, suffix byte) (float64, string) {
	switch suffix {
	case 's':
		return 1.0, arg[:len(arg)-1]
	case 'm':
		return 60.0, arg[:len(arg)-1]
	case 'h':
		return 3600.0, arg[:len(arg)-1]
	case 'd':
		return 86400.0, arg[:len(arg)-1]
	default:
		return 1.0, arg
	}
}

// exitWithError prints an error message to stderr and exits 1.
func exitWithError(msg string) {
	fmt.Fprintf(os.Stderr, "%s: %s\n", programName, msg)
	fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
	os.Exit(1)
}

// printAndExit writes text to stdout and exits 0 on success or 1 on write error.
func printAndExit(text string) {
	_, err := fmt.Fprint(os.Stdout, text)
	if err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}
