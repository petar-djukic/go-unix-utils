// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/factor: prime factorization.
// Implements srd065-factor R1.1-R1.4.
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

// progName is used in diagnostic messages.
const progName = "factor"

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run executes the factor logic and returns the exit code.
func run(args []string) int {
	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()
	exitCode := 0
	for _, arg := range args {
		if err := factorArg(w, arg); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
			exitCode = 1
		}
	}
	return exitCode
}

// factorArg parses a single argument and prints its factorization.
// R1.4: each argument produces one output line.
func factorArg(w *bufio.Writer, arg string) error {
	n, err := strconv.ParseUint(strings.TrimSpace(arg), 10, 64)
	if err != nil {
		return fmt.Errorf("'%s' is not a valid positive integer", arg)
	}
	printFactors(w, n)
	return nil
}

// printFactors writes the factorization line for n.
// R1.1: format is 'NUMBER: FACTOR FACTOR ...' with ascending factors.
// R1.2: for 1, prints '1:' with no factors.
// R1.3: primes print the number as the sole factor.
func printFactors(w *bufio.Writer, n uint64) {
	fmt.Fprintf(w, "%d:", n)
	if n > 1 {
		writeFactors(w, n)
	}
	w.WriteByte('\n')
}

// writeFactors appends prime factors of n to the writer.
// Uses trial division, which suffices for uint64 range per SRD non-goals.
func writeFactors(w *bufio.Writer, n uint64) {
	n = extractFactor(w, n, 2)
	limit := uint64(math.Sqrt(float64(n)))
	for d := uint64(3); d <= limit; d += 2 {
		n = extractFactor(w, n, d)
		limit = uint64(math.Sqrt(float64(n)))
	}
	if n > 1 {
		fmt.Fprintf(w, " %d", n)
	}
}

// extractFactor divides n by d repeatedly, writing each occurrence.
func extractFactor(w *bufio.Writer, n, d uint64) uint64 {
	for n%d == 0 {
		fmt.Fprintf(w, " %d", d)
		n /= d
	}
	return n
}
