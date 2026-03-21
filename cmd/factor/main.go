// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd065-factor R1.1–R1.4: core integer factorization from arguments.
package main

import (
	"bufio"
	"fmt"
	"io"
	"math/big"
	"os"
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
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		return factorArgs(args, stdout, stderr)
	}
	return factorStdin(stdin, stdout, stderr)
}

// factorArgs processes each command-line argument as an integer to factorize.
// R1.4: one factorization line per argument, in order.
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

// factorOne parses a string as an integer and writes its factorization.
// R1.1: format is "NUMBER: FACTOR FACTOR ..." with ascending factors.
// R1.2: for 1, prints "1:" with no factors.
// R1.3: primes print the number itself as the sole factor.
func factorOne(s string, w io.Writer) error {
	n := new(big.Int)
	if _, ok := n.SetString(s, 10); !ok {
		return fmt.Errorf("'%s' is not a valid positive integer", s)
	}
	if n.Sign() < 0 {
		return fmt.Errorf("'%s' is not a valid positive integer", s)
	}
	factors := trialDivision(n)
	return writeLine(s, factors, w)
}

// writeLine formats and writes a single factorization line.
// R1.1: "NUMBER: FACTOR FACTOR ..."
func writeLine(number string, factors []*big.Int, w io.Writer) error {
	var sb strings.Builder
	sb.WriteString(number)
	sb.WriteByte(':')
	for _, f := range factors {
		sb.WriteByte(' ')
		sb.WriteString(f.String())
	}
	sb.WriteByte('\n')
	_, err := io.WriteString(w, sb.String())
	return err
}

// trialDivision returns the prime factors of n in ascending order with
// multiplicity. For n <= 1, returns an empty slice. R1.2, R1.3.
func trialDivision(n *big.Int) []*big.Int {
	if n.Cmp(big.NewInt(2)) < 0 {
		return nil
	}
	var factors []*big.Int
	val := new(big.Int).Set(n)
	factors = extractFactor(val, big.NewInt(2), factors)
	d := big.NewInt(3)
	two := big.NewInt(2)
	for {
		dSq := new(big.Int).Mul(d, d)
		if dSq.Cmp(val) > 0 {
			break
		}
		factors = extractFactor(val, d, factors)
		d = new(big.Int).Add(d, two)
	}
	if val.Cmp(big.NewInt(1)) > 0 {
		factors = append(factors, new(big.Int).Set(val))
	}
	return factors
}

// extractFactor divides val by divisor as many times as possible,
// appending each occurrence to factors. Modifies val in place.
func extractFactor(val, divisor *big.Int, factors []*big.Int) []*big.Int {
	mod := new(big.Int)
	for {
		mod.Mod(val, divisor)
		if mod.Sign() != 0 {
			break
		}
		factors = append(factors, new(big.Int).Set(divisor))
		val.Div(val, divisor)
	}
	return factors
}
