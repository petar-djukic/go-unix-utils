// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd061-sleep R1.1-R4.4.
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

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	if len(args) > 0 && args[0] == "--help" {
		fmt.Println("Usage: sleep NUMBER[SUFFIX]...")
		fmt.Println("Pause for NUMBER seconds.")
		fmt.Println("SUFFIX may be 's' for seconds (default), 'm' for minutes,")
		fmt.Println("'h' for hours or 'd' for days.")
		fmt.Println()
		fmt.Println("NUMBER may be an arbitrary floating point number.")
		fmt.Println("Given two or more arguments, pause for the amount of time")
		fmt.Println("specified by the sum of their values.")
		os.Exit(0)
	}

	if len(args) > 0 && args[0] == "--version" {
		fmt.Println("sleep (go-unix-utils) 1.0")
		os.Exit(0)
	}

	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "sleep: missing operand")
		os.Exit(1)
	}

	var total float64
	for _, arg := range args {
		secs, err := parseDuration(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "sleep: invalid time interval %q\n", arg)
			os.Exit(1)
		}
		total += secs
	}

	if math.IsInf(total, 1) {
		select {}
	}

	if total > 0 {
		time.Sleep(time.Duration(total * float64(time.Second)))
	}
}

func parseDuration(arg string) (float64, error) {
	lower := strings.ToLower(arg)
	if lower == "infinity" || lower == "inf" {
		return math.Inf(1), nil
	}

	multiplier := 1.0
	s := arg
	if len(s) > 0 {
		switch s[len(s)-1] {
		case 's':
			multiplier = 1
			s = s[:len(s)-1]
		case 'm':
			multiplier = 60
			s = s[:len(s)-1]
		case 'h':
			multiplier = 3600
			s = s[:len(s)-1]
		case 'd':
			multiplier = 86400
			s = s[:len(s)-1]
		}
	}

	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	if val < 0 {
		return 0, fmt.Errorf("negative duration")
	}

	return val * multiplier, nil
}
