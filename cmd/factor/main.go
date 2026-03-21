// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd065-factor R1.1–R1.4, R2.1–R2.4, R3.1–R3.4: integer
// factorization with large number support, stdin mode, error handling,
// and --help/--version support.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "factor"

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run parses arguments and factorizes each integer.
// R1.4: processes multiple arguments in order.
// R2.1: reads from stdin when no arguments given.
// R3.1: --help prints usage to stdout and exits 0.
// R3.2: --version prints version info to stdout and exits 0.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "--help":
			return printHelp(stdout)
		case "--version":
			return printVersion(stdout)
		}
		return factorArgs(args, stdout, stderr)
	}
	return factorStdin(stdin, stdout, stderr)
}

// printHelp writes usage information to stdout. R3.1.
func printHelp(w io.Writer) int {
	fmt.Fprintln(w, "Usage: factor [OPTION]... [NUMBER]...")
	fmt.Fprintln(w, "Print the prime factors of each specified integer NUMBER.  If none")
	fmt.Fprintln(w, "are specified on the command line, they are read from standard input.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "      --help     display this help and exit")
	fmt.Fprintln(w, "      --version  output version information and exit")
	return 0
}

// printVersion writes version information to stdout. R3.2.
func printVersion(w io.Writer) int {
	fmt.Fprintln(w, "factor (go-unix-utils)")
	return 0
}

// factorArgs processes each command-line argument as an integer to factorize.
// R1.4: one factorization line per argument, in order.
// R2.4: errors go to stderr without stopping processing.
func factorArgs(args []string, stdout, stderr io.Writer) int {
	bw := bufio.NewWriter(stdout)
	exitCode := 0
	for _, arg := range args {
		if err := factorOne(arg, bw); err != nil {
			fmt.Fprintf(stderr, "%s: %s\n", progName, err)
			exitCode = 1
		}
	}
	if err := bw.Flush(); err != nil {
		fmt.Fprintf(stderr, "%s: write error: %s\n", progName, err)
		return 1
	}
	return exitCode
}

// factorStdin reads integers from stdin, one per line.
// R2.1: stdin mode. R2.3: blank lines are skipped.
// R2.4: errors go to stderr without stopping processing.
func factorStdin(stdin io.Reader, stdout, stderr io.Writer) int {
	scanner := bufio.NewScanner(stdin)
	bw := bufio.NewWriter(stdout)
	exitCode := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if err := factorOne(line, bw); err != nil {
			fmt.Fprintf(stderr, "%s: %s\n", progName, err)
			exitCode = 1
		}
	}
	if err := bw.Flush(); err != nil {
		fmt.Fprintf(stderr, "%s: write error: %s\n", progName, err)
		return 1
	}
	return exitCode
}

// factorOne parses a string as a non-negative integer and writes its
// factorization. R1.1: format is "NUMBER: FACTOR FACTOR ...".
// R1.2: for 1, prints "1:" with no factors.
// R2.2: handles numbers up to 2^64-1 via uint64.
// R2.4: returns error for non-numeric or negative input.
func factorOne(s string, w io.Writer) error {
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return fmt.Errorf("'%s' is not a valid positive integer", s)
	}
	factors := trialDivision(n)
	return writeLine(strconv.FormatUint(n, 10), factors, w)
}

// writeLine formats and writes a single factorization line.
// R1.1: "NUMBER: FACTOR FACTOR ..."
func writeLine(number string, factors []uint64, w io.Writer) error {
	var sb strings.Builder
	sb.WriteString(number)
	sb.WriteByte(':')
	for _, f := range factors {
		sb.WriteByte(' ')
		sb.WriteString(strconv.FormatUint(f, 10))
	}
	sb.WriteByte('\n')
	_, err := io.WriteString(w, sb.String())
	return err
}

// trialDivision returns the prime factors of n in ascending order with
// multiplicity. For n <= 1, returns an empty slice.
// R1.2: n=1 yields empty factors. R1.3: primes yield [n].
// R2.2: uses native uint64 arithmetic for performance on large numbers.
func trialDivision(n uint64) []uint64 {
	if n < 2 {
		return nil
	}
	var factors []uint64
	for n%2 == 0 {
		factors = append(factors, 2)
		n /= 2
	}
	for d := uint64(3); d*d <= n; d += 2 {
		for n%d == 0 {
			factors = append(factors, d)
			n /= d
		}
	}
	if n > 1 {
		factors = append(factors, n)
	}
	return factors
}
