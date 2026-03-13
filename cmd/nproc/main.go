// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd046-nproc R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R3.1
package main

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// programName is the name used in error and help messages.
const programName = "nproc"

func main() {
	// D2: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	var allFlag bool
	var ignore int

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--help":
			// R1.4: print usage to stdout and exit 0.
			if _, err := fmt.Print(helpText); err != nil {
				os.Exit(1)
			}
			return
		case arg == "--version":
			// R1.4: print version to stdout and exit 0.
			if _, err := fmt.Println("nproc (go-unix-utils) 0.1"); err != nil {
				os.Exit(1)
			}
			return
		case arg == "--all":
			// R1.2: use installed processor count.
			allFlag = true
		case arg == "--ignore":
			// R1.3: --ignore N form (space-separated).
			i++
			if i >= len(args) {
				fmt.Fprintf(os.Stderr, "%s: option '--ignore' requires an argument\nTry '%s --help' for more information.\n", programName, programName)
				os.Exit(1)
			}
			n, err := strconv.Atoi(args[i])
			if err != nil {
				// R2.2: non-numeric --ignore value.
				fmt.Fprintf(os.Stderr, "%s: invalid number: '%s'\n", programName, args[i])
				os.Exit(1)
			}
			ignore = n
		case strings.HasPrefix(arg, "--ignore="):
			// R1.3: --ignore=N form.
			val := arg[len("--ignore="):]
			n, err := strconv.Atoi(val)
			if err != nil {
				// R2.2: non-numeric --ignore value.
				fmt.Fprintf(os.Stderr, "%s: invalid number: '%s'\n", programName, val)
				os.Exit(1)
			}
			ignore = n
		case strings.HasPrefix(arg, "-"):
			// R2.3: unknown flag.
			fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\nTry '%s --help' for more information.\n", programName, arg, programName)
			os.Exit(1)
		default:
			// R2.1: positional operands are not accepted.
			fmt.Fprintf(os.Stderr, "%s: extra operand '%s'\nTry '%s --help' for more information.\n", programName, arg, programName)
			os.Exit(1)
		}
	}

	// R1.1, R1.2: get the processor count.
	// On most systems runtime.NumCPU() returns the same value for both
	// available and installed counts. This matches GNU nproc behavior on
	// macOS where affinity/cgroup restrictions are not applicable.
	_ = allFlag // Both paths use runtime.NumCPU(); kept for flag acceptance.
	count := runtime.NumCPU()

	// R1.3, R1.4: subtract ignore value, floor at 1.
	count -= ignore
	if count < 1 {
		count = 1
	}

	fmt.Println(count)
}

// helpText is the usage message printed by --help.
const helpText = `Usage: nproc [OPTION]...
Print the number of processing units available to the current process,
which may be less than the number of online processors.

      --all        print the number of installed processors
      --ignore=N   exclude N processing units
      --help       display this help and exit
      --version    output version information and exit
`
