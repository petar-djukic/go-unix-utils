// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// factor prints the prime factorization of each integer.
//
// Specification: srd065-factor R1.1-R1.4, R2.1-R2.4, R3.1-R3.4
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

const helpText = `Usage: factor [NUMBER]...
Print the prime factors of each specified integer.
`

const versionText = `factor (go-unix-utils) 0.1.0`

func main() {
	sys.InstallSIGPIPEHandler()

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--help":
			fmt.Print(helpText)
			return
		case "--version":
			fmt.Println(versionText)
			return
		}
	}

	exitCode := 0
	if len(os.Args) > 1 {
		exitCode = processArgs(os.Args[1:])
	} else {
		exitCode = processStdin()
	}
	os.Exit(exitCode)
}

func processArgs(args []string) int {
	exitCode := 0
	for _, arg := range args {
		if err := factorize(arg); err != nil {
			fmt.Fprintf(os.Stderr, "factor: %s\n", err)
			exitCode = 1
		}
	}
	return exitCode
}

func processStdin() int {
	exitCode := 0
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if err := factorize(line); err != nil {
			fmt.Fprintf(os.Stderr, "factor: %s\n", err)
			exitCode = 1
		}
	}
	return exitCode
}

func factorize(input string) error {
	n, err := strconv.ParseInt(input, 10, 64)
	if err != nil {
		return fmt.Errorf("'%s' is not a valid positive integer", input)
	}
	if n < 0 {
		return fmt.Errorf("'%s' is not a valid positive integer", input)
	}

	factors := primeFactors(n)
	printFactors(n, factors)
	return nil
}

func primeFactors(n int64) []int64 {
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

func printFactors(n int64, factors []int64) {
	var b strings.Builder
	fmt.Fprintf(&b, "%d:", n)
	for _, f := range factors {
		fmt.Fprintf(&b, " %d", f)
	}
	fmt.Println(b.String())
}
