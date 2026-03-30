// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/sleep implements GNU sleep: pause for a specified duration.
//
// Implements prd061-sleep R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4,
// R3.1, R3.2, R3.3, R3.4, R4.1, R4.2.
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

// Version is set at build time via -ldflags "-X main.Version=<tag>".
var Version = "dev"

const helpText = `Usage: sleep NUMBER[SUFFIX]...
  or:  sleep OPTION
Pause for NUMBER seconds.  SUFFIX may be 's' for seconds (the default),
'm' for minutes, 'h' for hours or 'd' for days.  NUMBER may be an
arbitrary floating point number.  Given two or more arguments, pause
for the amount of time specified by the sum of their values.

      --help     display this help and exit
      --version  output version information and exit
`

const progName = "sleep"

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run implements the sleep logic. Returns exit code.
func run(args []string) int {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", progName)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
		return 1
	}

	if args[0] == "--help" {
		fmt.Fprint(os.Stdout, helpText) //nolint:errcheck // best-effort
		return 0
	}
	if args[0] == "--version" {
		fmt.Fprintf(os.Stdout, "%s (go-unix-utils) %s\n", progName, Version) //nolint:errcheck // best-effort
		return 0
	}

	total, err := sumDurations(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		return 1
	}

	// R1.1, R1.2: pause for the computed duration.
	time.Sleep(total)
	return 0
}

// sumDurations parses all arguments, applies suffix multipliers, and
// returns the total duration. R1.4: multiple arguments are summed.
func sumDurations(args []string) (time.Duration, error) {
	var total float64
	for _, arg := range args {
		secs, err := parseDuration(arg)
		if err != nil {
			return 0, err
		}
		total += secs
	}
	if total < 0 {
		return 0, fmt.Errorf("invalid time interval '%s'", args[0])
	}
	if math.IsInf(total, 1) {
		// R2.4: sleep indefinitely.
		return time.Duration(math.MaxInt64), nil
	}
	return time.Duration(total * float64(time.Second)), nil
}

// parseDuration parses a single argument with optional suffix.
// R1.3: suffix multipliers s, m, h, d (case-sensitive, lowercase).
func parseDuration(arg string) (float64, error) {
	multiplier := 1.0
	numStr := arg

	if len(arg) > 0 {
		last := arg[len(arg)-1]
		if m, ok := suffixMultiplier(last); ok {
			multiplier = m
			numStr = arg[:len(arg)-1]
		}
	}

	// R2.4: support infinity/inf.
	lower := strings.ToLower(numStr)
	if lower == "infinity" || lower == "inf" {
		return math.Inf(1), nil
	}

	val, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid time interval '%s'", arg)
	}
	if val < 0 {
		return 0, fmt.Errorf("invalid time interval '%s'", arg)
	}
	return val * multiplier, nil
}

// suffixMultiplier returns the multiplier for a duration suffix.
func suffixMultiplier(ch byte) (float64, bool) {
	switch ch {
	case 's':
		return 1, true
	case 'm':
		return 60, true
	case 'h':
		return 3600, true
	case 'd':
		return 86400, true
	default:
		return 0, false
	}
}
