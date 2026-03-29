// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/pwd implements GNU pwd: print the current working directory.
//
// Implements prd051-pwd R1.1, R1.2, R1.3, R1.4, R2.1, R2.2.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "pwd"

// pwdMode represents the logical vs physical path resolution mode.
type pwdMode int

const (
	// R1.1, R1.3: physical mode is the default — resolve symlinks.
	modePhysical pwdMode = iota
	// R1.2: logical mode — use PWD environment variable.
	modeLogical
)

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run parses arguments and prints the working directory. Returns exit code.
func run(args []string, stdout, stderr *os.File) int {
	m, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err) //nolint:errcheck
		return 1
	}
	return printWorkingDir(m, stdout, stderr)
}

// parseArgs extracts the mode from command-line arguments.
// R1.4: when both -L and -P are given, the last one wins.
// R2.1: positional operands are rejected.
// R2.2: unknown flags produce an error.
func parseArgs(args []string) (pwdMode, error) {
	m := modePhysical // R1.1: default is physical.
	for _, arg := range args {
		if arg == "--" {
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			return m, fmt.Errorf("extra operand '%s'", arg)
		}
		parsed, err := parseFlag(arg, m)
		if err != nil {
			return m, err
		}
		m = parsed
	}
	return m, nil
}

// parseFlag parses a single flag argument and returns the updated mode.
func parseFlag(arg string, current pwdMode) (pwdMode, error) {
	if strings.HasPrefix(arg, "--") {
		return parseLongFlag(arg, current)
	}
	return parseShortFlags(arg, current)
}

// parseLongFlag handles --logical and --physical.
func parseLongFlag(arg string, current pwdMode) (pwdMode, error) {
	switch arg {
	case "--logical":
		return modeLogical, nil
	case "--physical":
		return modePhysical, nil
	default:
		return current, fmt.Errorf("unrecognized option '%s'", arg)
	}
}

// parseShortFlags handles short flag bundles like -L, -P, -LP.
// R1.4: last flag in the bundle wins.
func parseShortFlags(arg string, current pwdMode) (pwdMode, error) {
	m := current
	for _, ch := range arg[1:] {
		switch ch {
		case 'L':
			m = modeLogical
		case 'P':
			m = modePhysical
		default:
			return m, fmt.Errorf("invalid option -- '%c'", ch)
		}
	}
	return m, nil
}

// printWorkingDir prints the current working directory according to mode.
// TODO(prd051): implement R1.1, R1.2, R1.3 — full pwd logic.
func printWorkingDir(_ pwdMode, _ *os.File, stderr *os.File) int {
	fmt.Fprintln(stderr, progName+": not yet implemented") //nolint:errcheck
	return 1
}
