// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/sleep: pause for a specified duration.
// Implements srd061-sleep R1.1, R1.2, R1.3, R1.4.
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "sleep"

const tryHelp = "Try 'sleep --help' for more information."

// suffixMultipliers maps duration suffix characters to their
// multiplier in seconds. R1.3: s, m, h, d suffixes.
var suffixMultipliers = map[byte]float64{
	's': 1,
	'm': 60,
	'h': 3600,
	'd': 86400,
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run executes the sleep logic and returns the exit code.
// R1.1: pause for NUMBER seconds. R1.4: sum multiple arguments.
func run(args []string) int {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n%s\n", progName, tryHelp)
		return 1
	}
	if err := checkOptions(args); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n%s\n", progName, err, tryHelp)
		return 1
	}
	total, err := sumDurations(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n%s\n", progName, err, tryHelp)
		return 1
	}
	time.Sleep(total)
	return 0
}

// checkOptions detects arguments that look like flags (start with '-')
// and reports them as invalid options, matching GNU sleep behavior.
// GNU sleep does not accept negative numbers; -1 is parsed as option '1'.
func checkOptions(args []string) error {
	for _, arg := range args {
		if len(arg) > 1 && arg[0] == '-' {
			return fmt.Errorf("invalid option -- '%s'", arg[1:])
		}
	}
	return nil
}

// sumDurations parses each argument as a duration and returns their sum.
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

// parseDuration parses a single duration argument.
// R1.1: numeric seconds. R1.2: fractional seconds. R1.3: suffix multipliers.
func parseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, fmt.Errorf("invalid time interval '%s'", s)
	}
	multiplier := 1.0
	numStr := s
	last := s[len(s)-1]
	if m, ok := suffixMultipliers[last]; ok {
		multiplier = m
		numStr = s[:len(s)-1]
	}
	if strings.EqualFold(numStr, "inf") || strings.EqualFold(numStr, "infinity") {
		return time.Duration(1<<63 - 1), nil
	}
	val, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid time interval '%s'", s)
	}
	if val < 0 {
		return 0, fmt.Errorf("invalid time interval '%s'", s)
	}
	seconds := val * multiplier
	return time.Duration(seconds * float64(time.Second)), nil
}
