// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/sleep implements GNU sleep: pause for a specified duration.
// Implements prd061-sleep R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4,
// R3.1, R3.2, R3.3, R3.4.
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	suffixSeconds = 's'
	suffixMinutes = 'm'
	suffixHours   = 'h'
	suffixDays    = 'd'
	progName      = "sleep"
)

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// D1: check --version and --help before parsing duration arguments.
	if len(args) > 0 {
		handleFlag(args[0])
	}

	// R2.2: no arguments → usage error to stderr, exit 1.
	if len(args) == 0 {
		exitWithError("missing operand")
	}

	total, err := sumDurations(args)
	if err != nil {
		// R2.3, R3.4: report first invalid argument and exit 1.
		exitWithError(err.Error())
	}

	// R1.1, R2.1: sleep for the computed duration (zero exits immediately).
	time.Sleep(total)
}

// handleFlag checks for --version and --help as the first argument.
// R3.3: --help prints usage to stdout, exits 0.
// R3.4: --version prints version to stdout, exits 0.
func handleFlag(arg string) {
	switch arg {
	case "--help":
		printHelp()
		os.Exit(0)
	case "--version":
		printVersion()
		os.Exit(0)
	}
}

// printHelp outputs usage information to stdout.
func printHelp() {
	fmt.Printf("Usage: %s NUMBER[SUFFIX]...\n", progName)
	fmt.Printf("  or:  %s OPTION\n", progName)
	fmt.Println("Pause for NUMBER seconds.")
	fmt.Println("SUFFIX may be 's' for seconds (default), 'm' for minutes,")
	fmt.Println("'h' for hours or 'd' for days.")
	fmt.Println("")
	fmt.Println("NUMBER may be 'infinity' or 'inf' to sleep indefinitely.")
	fmt.Println("")
	fmt.Println("      --help     display this help and exit")
	fmt.Println("      --version  output version information and exit")
}

// printVersion outputs version information to stdout.
func printVersion() {
	fmt.Printf("%s (go-unix-utils)\n", progName)
}

// exitWithError prints an error with the "Try --help" hint and exits 1.
// D3: matches GNU format exactly.
func exitWithError(msg string) {
	fmt.Fprintf(os.Stderr, "%s: %s\n", progName, msg)
	fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
	os.Exit(1)
}

// sumDurations parses each argument as a duration and returns their sum.
// R1.4: multiple arguments are summed. R2.1: zero is valid.
// R3.4: stops at first invalid argument.
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

// parseDuration parses a single duration argument.
// R1.2: fractional seconds supported. R1.3: suffix s/m/h/d.
func parseDuration(arg string) (time.Duration, error) {
	if arg == "" {
		return 0, fmt.Errorf("invalid time interval '%s'", arg)
	}

	multiplier := time.Second
	numStr := arg

	// R1.3: check for suffix multiplier.
	last := arg[len(arg)-1]
	if suffix, ok := suffixMultiplier(last); ok {
		multiplier = suffix
		numStr = arg[:len(arg)-1]
	}

	// R2.4: support infinity/inf (case-insensitive, D2).
	lower := strings.ToLower(numStr)
	if lower == "inf" || lower == "infinity" {
		selectForever()
		return 0, nil // unreachable
	}

	// R2.3: reject non-numeric and negative values.
	seconds, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid time interval '%s'", arg)
	}
	if seconds < 0 {
		return 0, fmt.Errorf("invalid time interval '%s'", arg)
	}

	// Convert to nanoseconds via the multiplier.
	nanos := seconds * float64(multiplier)
	return time.Duration(nanos), nil
}

// suffixMultiplier returns the duration multiplier for a suffix character.
func suffixMultiplier(c byte) (time.Duration, bool) {
	switch c {
	case byte(suffixSeconds):
		return time.Second, true
	case byte(suffixMinutes):
		return time.Minute, true
	case byte(suffixHours):
		return time.Hour, true
	case byte(suffixDays):
		return 24 * time.Hour, true
	default:
		return 0, false
	}
}

// selectForever blocks indefinitely, used for infinity duration.
func selectForever() {
	select {}
}
