// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd065-factor: Prime Factorization.
// Covers R1.1-R1.4 (core factorization, output format, multiple arguments),
// R2.1-R2.4 (stdin mode, large integers, blank lines, error handling),
// R3.1-R3.4 (help, version, stdout/stderr routing),
// R4.1-R4.2 (exit codes).
package main

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]
	exitCode := dispatch(args)
	os.Exit(exitCode)
}

// dispatch routes to help, version, argument mode, or stdin mode.
func dispatch(args []string) int {
	if len(args) > 0 && args[0] == "--help" {
		return printHelp()
	}
	if len(args) > 0 && args[0] == "--version" {
		return printVersion()
	}
	if len(args) > 0 {
		return factorArgs(args)
	}
	return factorStdin()
}

// factorArgs factorizes each command-line argument.
// R1.4: processes multiple arguments in order.
// R4.1/R4.2: exits 0 on success, 1 if any input is invalid.
func factorArgs(args []string) int {
	w := bufio.NewWriter(os.Stdout)
	defer w.Flush() // best-effort flush
	exitCode := 0
	for _, arg := range args {
		if err := factorOne(w, arg); err != nil {
			fmt.Fprintf(os.Stderr, "factor: %v\n", err)
			exitCode = 1
		}
	}
	return exitCode
}

// factorStdin reads numbers from stdin, one per line.
// R2.1: factorizes each line. R2.3: skips blank lines.
// R2.4/R3.4: prints errors to stderr and continues.
func factorStdin() int {
	w := bufio.NewWriter(os.Stdout)
	defer w.Flush() // best-effort flush
	scanner := bufio.NewScanner(os.Stdin)
	exitCode := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue // R2.3: skip blank lines
		}
		if err := factorOne(w, line); err != nil {
			fmt.Fprintf(os.Stderr, "factor: %v\n", err)
			exitCode = 1
		}
	}
	return exitCode
}

// factorOne parses a single input string and writes its factorization.
// R2.2: accepts integers up to 2^63-1.
// R2.4: returns error for non-integer or negative input.
func factorOne(w *bufio.Writer, input string) error {
	input = strings.TrimSpace(input)
	n, err := parseUint(input)
	if err != nil {
		return err
	}
	printFactors(w, n)
	return nil
}

// parseUint parses a non-negative integer string.
// R2.2: handles up to math.MaxUint64 for parity with GNU factor.
func parseUint(s string) (uint64, error) {
	if s == "" {
		return 0, fmt.Errorf("'%s' is not a valid positive integer", s)
	}
	// Reject negative numbers and explicit signs
	if s[0] == '-' {
		return 0, fmt.Errorf("'%s' is not a valid positive integer", s)
	}
	if s[0] == '+' {
		s = s[1:]
	}
	// Reject floating-point input
	if strings.ContainsAny(s, ".eE") {
		return 0, fmt.Errorf("'%s' is not a valid positive integer", s)
	}
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("'%s' is not a valid positive integer", s)
	}
	return n, nil
}

// printFactors writes "N: p1 p2 p3 ..." to w.
// R1.1: factors in ascending order with multiplicity.
// R1.2: number 1 prints "1:" with no factors.
// R1.3: primes print the number itself as the sole factor.
func printFactors(w *bufio.Writer, n uint64) {
	fmt.Fprintf(w, "%d:", n)
	if n > 1 {
		writeFactors(w, n)
	}
	fmt.Fprintln(w)
}

// writeFactors performs trial division and writes each factor.
// D2: trial division is sufficient for the uint64 range.
func writeFactors(w *bufio.Writer, n uint64) {
	n = extractFactor(w, n, 2)
	limit := isqrt(n)
	for d := uint64(3); d <= limit; d += 2 {
		n = extractFactor(w, n, d)
		limit = isqrt(n)
	}
	if n > 1 {
		fmt.Fprintf(w, " %d", n)
	}
}

// isqrt returns the integer square root of n (largest x where x*x <= n).
// Uses float64 sqrt as initial estimate, then corrects for precision loss
// on large values near 2^63-1. R2.2/R3.1: required for correct factorization
// of numbers up to 2^63-1.
func isqrt(n uint64) uint64 {
	if n < 2 {
		return n
	}
	x := uint64(math.Sqrt(float64(n)))
	if x > 0xFFFFFFFF {
		x = 0xFFFFFFFF
	}
	for x*x > n {
		x--
	}
	for x < 0xFFFFFFFF && (x+1)*(x+1) <= n {
		x++
	}
	return x
}

// extractFactor divides n by d as many times as possible, writing each.
func extractFactor(w *bufio.Writer, n, d uint64) uint64 {
	for n%d == 0 {
		fmt.Fprintf(w, " %d", d)
		n /= d
	}
	return n
}

// printHelp writes usage information to stdout and returns exit code.
// R3.1: --help prints usage and exits 0.
func printHelp() int {
	fmt.Print(`Usage: factor [NUMBER]...
  or:  factor [OPTION]
Print the prime factors of each specified integer NUMBER.  If none
are specified on the command line, they are read from standard input.

      --help     display this help and exit
      --version  output version information and exit
`)
	return 0
}

// printVersion writes version information to stdout and returns exit code.
// R3.2: --version prints version and exits 0.
func printVersion() int {
	fmt.Fprintf(os.Stdout, "factor (go-unix-utils) %s\n", version)
	return 0
}
