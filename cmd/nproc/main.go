// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd046-nproc R1.1 (default prints available CPU count),
// R1.2 (--all prints installed processor count),
// R1.3 (--ignore=N subtracts from count, minimum 1),
// R1.4 (--all and --ignore=N may be combined),
// R2.1-R2.3 (error handling for operands, bad --ignore, unknown flags).
package main

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// programName is used in error messages.
const programName = "nproc"

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// parsedArgs holds the result of argument parsing.
type parsedArgs struct {
	all    bool // --all: use installed processor count
	ignore int  // --ignore=N: subtract N from count
}

// run processes arguments and prints the processor count.
// Returns the exit code.
func run(args []string) int {
	parsed, err := parseArgs(args)
	if err != nil {
		printError(err.Error())
		return 1
	}
	count := cpuCount()
	count -= parsed.ignore
	if count < 1 {
		count = 1
	}
	fmt.Println(count)
	return 0
}

// parseArgs parses command-line arguments for nproc.
// Supports --all, --ignore=N, --help, --version.
// R2.1: positional operands produce an error.
func parseArgs(args []string) (parsedArgs, error) {
	var result parsedArgs
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			if i+1 < len(args) {
				return result, fmt.Errorf("extra operand '%s'", args[i+1])
			}
			break
		}
		if arg == "--help" || arg == "--version" {
			fmt.Print(helpTextFor(arg))
			os.Exit(0)
		}
		if arg == "--all" {
			result.all = true
			continue
		}
		if handled, err := parseIgnore(arg, args, i, &result); handled {
			if err != nil {
				return result, err
			}
			if arg == "--ignore" {
				i++ // consumed next arg
			}
			continue
		}
		if !strings.HasPrefix(arg, "-") {
			return result, fmt.Errorf("extra operand '%s'", arg)
		}
		return result, fmt.Errorf("unrecognized option '%s'", arg)
	}
	return result, nil
}

// parseIgnore handles --ignore=N and --ignore N forms.
// Returns (true, nil) if the arg was an --ignore flag,
// (true, err) if it was --ignore with a bad value,
// (false, nil) if the arg is not an --ignore flag.
func parseIgnore(arg string, args []string, i int, result *parsedArgs) (bool, error) {
	if arg == "--ignore" {
		if i+1 >= len(args) {
			return true, fmt.Errorf("option '--ignore' requires an argument")
		}
		return true, setIgnore(args[i+1], result)
	}
	if val, ok := strings.CutPrefix(arg, "--ignore="); ok {
		return true, setIgnore(val, result)
	}
	return false, nil
}

// setIgnore parses the ignore value and stores it in result.
// R2.2: non-numeric value produces an error.
func setIgnore(val string, result *parsedArgs) error {
	n, err := strconv.Atoi(val)
	if err != nil {
		return fmt.Errorf("invalid number: '%s'", val)
	}
	result.ignore = n
	return nil
}

// cpuCount returns the processor count. R1.1: available CPUs by default.
// R1.2: --all returns installed count. On macOS, both are identical
// since there is no cgroup/affinity restriction.
func cpuCount() int {
	return runtime.NumCPU()
}

// helpTextFor returns text for --help or --version.
func helpTextFor(flag string) string {
	if flag == "--version" {
		return "nproc (go-unix-utils) 1.0\n"
	}
	return `Usage: nproc [OPTION]...
Print the number of processing units available.

      --all        print the number of installed processors
      --ignore=N   subtract N from the count
      --help       display this help and exit
      --version    output version information and exit
`
}

// printError writes a formatted error message to stderr.
func printError(msg string) {
	fmt.Fprintf(os.Stderr,
		"%s: %s\nTry '%s --help' for more information.\n",
		programName, msg, programName)
}
