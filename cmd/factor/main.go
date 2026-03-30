// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/factor implements GNU factor: print prime factorizations of integers.
//
// Implements prd065-factor R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4,
// R3.1, R3.2, R3.3, R3.4, R4.1, R4.2.
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

const programName = "factor"

const usageText = `Usage: factor [NUMBER]...
Print the prime factors of each specified integer.
If none are specified on the command line, read them from standard input.
`

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run processes arguments or stdin and prints factorizations.
// R4.1: returns 0 when all inputs are valid.
// R4.2: returns 1 when any input is invalid.
// R3.1: --help prints usage to stdout, exits 0.
// R3.2: --version prints version info to stdout, exits 0.
func run(args []string, stdin *os.File, stdout, stderr *os.File) int {
	if len(args) > 0 {
		switch args[0] {
		case "--help":
			fmt.Fprint(stdout, usageText)
			return 0
		case "--version":
			fmt.Fprintf(stdout, "%s (go-unix-utils)\n", programName)
			return 0
		}
		return factorArgs(args, stdout, stderr)
	}
	return factorStdin(stdin, stdout, stderr)
}

// factorArgs factorizes each argument. R1.4: multiple args in order.
func factorArgs(args []string, stdout, stderr *os.File) int {
	w := bufio.NewWriter(stdout)
	defer w.Flush() // best-effort flush
	hadError := false
	for _, arg := range args {
		if err := processInput(arg, w, stderr); err != nil {
			hadError = true
		}
	}
	if hadError {
		return 1
	}
	return 0
}

// factorStdin reads integers from stdin, one per line.
// R2.1: stdin mode when no arguments given.
// R2.3: blank lines are skipped without output or error.
func factorStdin(stdin *os.File, stdout, stderr *os.File) int {
	scanner := bufio.NewScanner(stdin)
	w := bufio.NewWriter(stdout)
	defer w.Flush() // best-effort flush
	hadError := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if err := processInput(line, w, stderr); err != nil {
			hadError = true
		}
	}
	if hadError {
		return 1
	}
	return 0
}

// processInput parses a single input string and writes its factorization.
// R2.2: ParseInt with 64-bit accepts up to 2^63-1.
// R2.4, R3.4: errors on non-integer or negative input to stderr,
// does not stop processing.
func processInput(input string, w *bufio.Writer, stderr *os.File) error {
	n, err := strconv.ParseInt(input, 10, 64)
	if err != nil || n < 0 {
		fmt.Fprintf(stderr, "%s: '%s' is not a valid positive integer\n", programName, input)
		return fmt.Errorf("invalid input")
	}
	// R3.3: factorization output goes to stdout.
	printFactors(w, n)
	return nil
}

// printFactors writes "N: f1 f2 ..." for the given number.
// R1.1: ascending order with multiplicity.
// R1.2: for 1, prints "1:" with no factors.
// R1.3: primes appear as their own sole factor.
func printFactors(w *bufio.Writer, n int64) {
	fmt.Fprintf(w, "%d:", n)
	factors := trialDivision(n)
	for _, f := range factors {
		fmt.Fprintf(w, " %d", f)
	}
	fmt.Fprintln(w)
}

// trialDivision returns the prime factors of n in ascending order.
// For n <= 1, returns an empty slice.
func trialDivision(n int64) []int64 {
	if n <= 1 {
		return nil
	}
	var factors []int64
	for n%2 == 0 {
		factors = append(factors, 2)
		n /= 2
	}
	limit := int64(math.Sqrt(float64(n)))
	for d := int64(3); d <= limit; d += 2 {
		for n%d == 0 {
			factors = append(factors, d)
			n /= d
		}
		limit = int64(math.Sqrt(float64(n)))
	}
	if n > 1 {
		factors = append(factors, n)
	}
	return factors
}
