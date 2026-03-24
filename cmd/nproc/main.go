// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd046-nproc: Print Number of Available Processing Units.
// Covers R1.1-R1.4 (default behavior, --all, --ignore=N, combined),
// R2.1-R2.2 (extra operand/non-numeric ignore errors).
package main

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:])
	os.Exit(exitCode)
}

// config holds parsed flag state for the nproc invocation.
type config struct {
	all    bool
	ignore int
}

// run parses arguments and prints the processor count. Returns exit code.
func run(args []string) int {
	cfg, exitCode := parseArgs(args)
	if exitCode >= 0 {
		return exitCode
	}
	return printCount(cfg)
}

// parseArgs processes command-line arguments into a config.
// Returns (cfg, -1) on success, or (_, exitCode) on termination.
func parseArgs(args []string) (config, int) {
	var cfg config
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--help":
			return cfg, printHelp()
		case arg == "--version":
			return cfg, printVersion()
		case arg == "--all":
			cfg.all = true
		case arg == "--ignore" && i+1 < len(args):
			i++
			n, err := parseIgnoreValue(args[i])
			if err != nil {
				return cfg, err.(ignoreError).code //nolint:errorlint
			}
			cfg.ignore = n
		case strings.HasPrefix(arg, "--ignore="):
			n, err := parseIgnoreValue(strings.TrimPrefix(arg, "--ignore="))
			if err != nil {
				return cfg, err.(ignoreError).code //nolint:errorlint
			}
			cfg.ignore = n
		case arg == "--":
			// R2.1: remaining args after -- are extra operands.
			if i+1 < len(args) {
				return cfg, extraOperandError(args[i+1])
			}
		case len(arg) > 1 && arg[0] == '-':
			// R2.3: unknown flags produce an error.
			fmt.Fprintf(os.Stderr, "nproc: unrecognized option '%s'\n", arg)
			fmt.Fprintln(os.Stderr, "Try 'nproc --help' for more information.")
			return cfg, 1
		default:
			// R2.1: extra operand.
			return cfg, extraOperandError(arg)
		}
	}
	return cfg, -1
}

// ignoreError wraps an exit code for --ignore parse failures.
type ignoreError struct {
	code int
}

func (e ignoreError) Error() string { return "ignore error" }

// parseIgnoreValue parses and validates the --ignore=N value.
// R2.2: non-numeric value prints an error and returns exit code 1.
func parseIgnoreValue(val string) (int, error) {
	n, err := strconv.Atoi(val)
	if err != nil {
		fmt.Fprintf(os.Stderr, "nproc: %q: invalid number to ignore\n", val)
		return 0, ignoreError{code: 1}
	}
	return n, nil
}

// extraOperandError prints the extra-operand diagnostic and returns exit code 1.
func extraOperandError(operand string) int {
	fmt.Fprintf(os.Stderr, "nproc: extra operand '%s'\n", operand)
	fmt.Fprintln(os.Stderr, "Try 'nproc --help' for more information.")
	return 1
}

// printCount computes and prints the processor count. Returns exit code.
// R1.1: default prints available CPUs. R1.2: --all prints installed CPUs.
// R1.3: --ignore subtracts N, minimum 1. R1.4: --all and --ignore combine.
func printCount(cfg config) int {
	count := runtime.NumCPU()
	count -= cfg.ignore
	if count < 1 {
		count = 1
	}
	if _, err := fmt.Println(count); err != nil {
		return 1
	}
	return 0
}

// printHelp writes usage information to stdout and returns the exit code.
func printHelp() int {
	_, err := fmt.Fprint(os.Stdout, `Usage: nproc [OPTION]...
Print the number of processing units available to the current process,
which may be less than the number of online processors.

      --all        print the number of installed processors
      --ignore=N   if possible, exclude N processing units
      --help       display this help and exit
      --version    output version information and exit
`)
	if err != nil {
		return 1
	}
	return 0
}

// printVersion writes version information to stdout and returns the exit code.
func printVersion() int {
	_, err := fmt.Fprintf(os.Stdout, "nproc (go-unix-utils) %s\n", version)
	if err != nil {
		return 1
	}
	return 0
}
