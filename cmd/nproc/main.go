// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd046-nproc R1.1-R1.4, R2.1-R2.3:
// cmd/nproc prints the number of available processing units.
// Supports --all flag for installed processors, --ignore=N to reserve cores,
// and OMP_NUM_THREADS environment variable override.
// Installs SIGPIPE handler per ARCHITECTURE.yaml.
package main

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
	"github.com/petar-djukic/go-unix-utils/pkg/version"
)

// nprocOpts holds the parsed command-line options.
type nprocOpts struct {
	all          bool // --all: print installed processor count
	ignore       int  // --ignore=N: subtract N from count
	numericError bool // true when the error is an invalid number (no "Try --help" hint)
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", os.Args[0], err) //nolint:errcheck // best-effort diagnostic
		// GNU nproc prints "Try --help" for flag/operand errors but not for
		// invalid --ignore numeric values. Match that behavior.
		if !opts.numericError {
			fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", os.Args[0]) //nolint:errcheck // best-effort diagnostic
		}
		os.Exit(1)
	}

	count := cpuCount(opts)
	fmt.Println(count)
}

// cpuCount returns the processor count adjusted for flags and environment.
// R1.1: default returns available processing units.
// R1.2: --all returns installed processors (same as available on most systems).
// R1.3: --ignore=N subtracts N, with a floor of 1.
// R1.4: OMP_NUM_THREADS overrides the count unless --all is specified.
func cpuCount(opts *nprocOpts) int {
	count := runtime.NumCPU()

	// R1.4: OMP_NUM_THREADS overrides the default count unless --all is set.
	if !opts.all {
		if ompStr, ok := os.LookupEnv("OMP_NUM_THREADS"); ok && ompStr != "" {
			// GNU nproc takes the first comma-separated value.
			parts := strings.SplitN(ompStr, ",", 2)
			if n, err := strconv.Atoi(parts[0]); err == nil && n > 0 {
				count = n
			}
		}
	}

	// R1.3, R1.4: subtract ignore count, floor at 1.
	count -= opts.ignore
	if count < 1 {
		count = 1
	}

	return count
}

// parseArgs parses command-line arguments for nproc. Returns the parsed options
// or an error for unknown flags, invalid --ignore values, or extra operands.
func parseArgs(args []string) (*nprocOpts, error) {
	opts := &nprocOpts{}

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if arg == "--" {
			// Everything after -- is a positional operand.
			if i+1 < len(args) {
				return opts, fmt.Errorf("extra operand '%s'", args[i+1])
			}
			break
		}

		// --help prints usage to stdout and exits 0.
		if arg == "--help" {
			fmt.Fprintf(os.Stdout, //nolint:errcheck // best-effort output
				"Usage: %s [OPTION]...\n"+
					"Print the number of processing units available to the current process,\n"+
					"which may be less than the number of online processors.\n\n"+
					"      --all       print the number of installed processors\n"+
					"      --ignore=N  if possible, exclude N processing units\n"+
					"      --help      display this help and exit\n"+
					"      --version   output version information and exit\n",
				os.Args[0],
			)
			os.Exit(0)
		}

		// --version prints version info to stdout and exits 0.
		if arg == "--version" {
			fmt.Fprintf(os.Stdout, "%s (%s) %s\n", //nolint:errcheck // best-effort output
				os.Args[0], "go-unix-utils", version.Version,
			)
			os.Exit(0)
		}

		if arg == "--all" {
			opts.all = true
			continue
		}

		// --ignore=N: parse the numeric value.
		if arg == "--ignore" || strings.HasPrefix(arg, "--ignore=") {
			var valStr string
			if arg == "--ignore" {
				// Next argument is the value.
				if i+1 >= len(args) {
					return opts, fmt.Errorf("option '--ignore' requires an argument")
				}
				i++
				valStr = args[i]
			} else {
				valStr = arg[len("--ignore="):]
			}
			n, err := strconv.Atoi(valStr)
			if err != nil || n < 0 {
				opts.numericError = true
				return opts, fmt.Errorf("invalid number: '%s'", valStr)
			}
			opts.ignore = n
			continue
		}

		// Unknown long option.
		if strings.HasPrefix(arg, "--") {
			return opts, fmt.Errorf("unrecognized option '%s'", arg)
		}

		// Unknown short flag.
		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			return opts, fmt.Errorf("invalid option -- '%c'", arg[1])
		}

		// R2.1: no positional operands accepted.
		return opts, fmt.Errorf("extra operand '%s'", arg)
	}

	return opts, nil
}
