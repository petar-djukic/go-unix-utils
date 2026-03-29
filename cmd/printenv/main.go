// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/printenv implements GNU printenv: print environment variables.
//
// Implements prd040-printenv R1.1, R1.2, R1.3, R2.1, R2.2, R2.3, R2.4, R3.1, R3.2, R3.3.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:], os.Stdout)
	os.Exit(exitCode)
}

// run parses arguments and prints environment variables.
// Returns 0 if all requested variables are found, 1 otherwise.
func run(args []string, stdout *os.File) int {
	nullTerminated, vars := parseArgs(args)
	terminator := "\n"
	if nullTerminated {
		terminator = "\x00"
	}

	if len(vars) == 0 {
		return printAll(stdout, terminator)
	}
	return printVars(vars, stdout, terminator)
}

// parseArgs extracts the -0/--null flag and remaining variable names.
func parseArgs(args []string) (nullTerm bool, vars []string) {
	for _, arg := range args {
		switch arg {
		case "-0", "--null":
			nullTerm = true
		default:
			vars = append(vars, arg)
		}
	}
	return nullTerm, vars
}

// printAll prints every environment variable in NAME=VALUE format.
// R1.1: prints all variables, one per line, exits 0.
func printAll(stdout *os.File, terminator string) int {
	for _, entry := range os.Environ() {
		fmt.Fprint(stdout, entry+terminator) //nolint:errcheck // best-effort
	}
	return 0
}

// printVars prints the value of each named variable.
// R1.2: prints only the value (no NAME= prefix), one per line.
// R1.3: missing variables produce no output and no error.
func printVars(vars []string, stdout *os.File, terminator string) int {
	exitCode := 0
	for _, name := range vars {
		val, ok := lookupEnv(name)
		if !ok {
			exitCode = 1
			continue
		}
		fmt.Fprint(stdout, val+terminator) //nolint:errcheck // best-effort
	}
	return exitCode
}

// lookupEnv scans the process environment for the named variable.
// R1.3: distinguishes unset from empty.
func lookupEnv(name string) (string, bool) {
	for _, entry := range os.Environ() {
		if k, v, ok := strings.Cut(entry, "="); ok && k == name {
			return v, true
		}
	}
	return "", false
}
