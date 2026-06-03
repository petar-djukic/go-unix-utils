// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd046-nproc R1.1-R1.4, R2.1-R2.3.
package main

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()

	allFlag := false
	ignoreN := 0
	args := os.Args[1:]

	for len(args) > 0 {
		arg := args[0]
		args = args[1:]

		if arg == "--" {
			break
		}
		if arg == "--all" {
			allFlag = true
			continue
		}
		if arg == "--ignore" {
			if len(args) == 0 {
				fmt.Fprintf(os.Stderr, "nproc: option '--ignore' requires an argument\n")
				fmt.Fprintln(os.Stderr, "Try 'nproc --help' for more information.")
				os.Exit(1)
			}
			if !parseIgnore(args[0], &ignoreN) {
				os.Exit(1)
			}
			args = args[1:]
			continue
		}
		if strings.HasPrefix(arg, "--ignore=") {
			val := arg[len("--ignore="):]
			if !parseIgnore(val, &ignoreN) {
				os.Exit(1)
			}
			continue
		}
		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			fmt.Fprintf(os.Stderr, "nproc: unrecognized option '%s'\n", arg)
			fmt.Fprintln(os.Stderr, "Try 'nproc --help' for more information.")
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "nproc: extra operand '%s'\n", arg)
		fmt.Fprintln(os.Stderr, "Try 'nproc --help' for more information.")
		os.Exit(1)
	}

	for _, arg := range args {
		fmt.Fprintf(os.Stderr, "nproc: extra operand '%s'\n", arg)
		fmt.Fprintln(os.Stderr, "Try 'nproc --help' for more information.")
		os.Exit(1)
	}

	count := runtime.NumCPU()
	_ = allFlag

	count -= ignoreN
	if count < 1 {
		count = 1
	}

	if _, err := fmt.Fprintln(os.Stdout, count); err != nil {
		os.Exit(1)
	}
}

func parseIgnore(val string, n *int) bool {
	v, err := strconv.Atoi(val)
	if err != nil || v < 0 {
		fmt.Fprintf(os.Stderr, "nproc: invalid number: '%s'\n", val)
		return false
	}
	*n = v
	return true
}
