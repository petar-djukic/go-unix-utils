// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/factor: prime factorization.
// Implements srd065-factor R1.1-R1.4, R2.1-R2.4, R3.1-R3.4.
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

// helpText is printed when --help is given.
// R3.1: --help prints usage to stdout and exits 0.
const helpText = `Usage: factor [NUMBER]...
  or:  factor [OPTION]
Print the prime factors of each specified integer NUMBER.  If none
are specified on the command line, they are read from standard input.

      --help        display this help and exit
      --version     output version information and exit
`

// versionText is printed when --version is given.
// R3.2: --version prints version to stdout and exits 0.
const versionText = "factor (go-unix-utils) 1.0\n"

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run executes the factor logic and returns the exit code.
// R2.1: when no arguments, reads from stdin.
// R3.1, R3.2: check first argument for --help/--version.
func run(args []string) int {
	if len(args) > 0 {
		switch args[0] {
		case "--help":
			fmt.Print(helpText)
			return 0
		case "--version":
			fmt.Print(versionText)
			return 0
		}
	}
	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()
	if len(args) == 0 {
		return processStdin(w)
	}
	return processArgs(w, args)
}

// processArgs factorizes each command-line argument.
// R1.4: each argument produces one output line.
// R3.4: errors go to stderr, processing continues.
func processArgs(w *bufio.Writer, args []string) int {
	exitCode := 0
	for _, arg := range args {
		if err := factorArg(w, arg); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
			exitCode = 1
		}
	}
	return exitCode
}

// processStdin reads integers from stdin, one per line.
// R2.1: factorizes each line. R2.3: skips blank lines.
// R2.4, R3.4: prints error on invalid input to stderr, continues.
func processStdin(w *bufio.Writer) int {
	scanner := bufio.NewScanner(os.Stdin)
	exitCode := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if err := factorArg(w, line); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
			exitCode = 1
		}
		w.Flush()
	}
	return exitCode
}

// factorArg parses a single argument and prints its factorization.
// R2.2: accepts integers up to at least 2^63-1.
// R2.4: returns error for non-integer or negative input.
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
// R3.3: output goes to stdout via the buffered writer.
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
