// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/sleep implements GNU sleep: pause for a specified duration.
// Implements prd061-sleep R1.1, R1.2, R1.3, R1.4, R2.1, R2.2.
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
)

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]
	// R2.2: no arguments → usage error to stderr, exit 1.
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "sleep: missing operand\n")
		fmt.Fprintf(os.Stderr, "Try 'sleep --help' for more information.\n")
		os.Exit(1)
	}

	total, err := sumDurations(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sleep: %s\n", err)
		os.Exit(1)
	}

	// R1.1, R2.1: sleep for the computed duration (zero exits immediately).
	time.Sleep(total)
}

// sumDurations parses each argument as a duration and returns their sum.
// R1.4: multiple arguments are summed. R2.1: zero is valid.
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

	// R2.4: support infinity/inf.
	lower := strings.ToLower(numStr)
	if lower == "inf" || lower == "infinity" {
		// Sleep in a loop that won't overflow time.Duration.
		selectForever()
		return 0, nil // unreachable
	}

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
