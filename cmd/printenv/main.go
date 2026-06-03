// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()

	nullTerminate := false
	args := os.Args[1:]

	var positional []string
	for i := range len(args) {
		a := args[i]
		if a == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if a == "--null" {
			nullTerminate = true
			continue
		}
		if a == "--help" {
			fmt.Fprintf(os.Stdout, "Usage: printenv [OPTION]... [VARIABLE]...\nPrint the values of the specified environment VARIABLE(s).\nIf no VARIABLE is specified, print name and value pairs for them all.\n\n  -0, --null     end each output line with NUL, not newline\n      --help     display this help and exit\n      --version  output version information and exit\n")
			os.Exit(0)
		}
		if a == "--version" {
			fmt.Fprintf(os.Stdout, "printenv (go-unix-utils) 0.1\n")
			os.Exit(0)
		}
		if strings.HasPrefix(a, "-") && a != "-" {
			for _, ch := range a[1:] {
				switch ch {
				case '0':
					nullTerminate = true
				default:
					fmt.Fprintf(os.Stderr, "printenv: invalid option -- '%c'\n", ch)
					os.Exit(2)
				}
			}
			continue
		}
		positional = append(positional, a)
	}

	terminator := "\n"
	if nullTerminate {
		terminator = "\x00"
	}

	if len(positional) == 0 {
		for _, e := range os.Environ() {
			fmt.Fprintf(os.Stdout, "%s%s", e, terminator)
		}
		os.Exit(0)
	}

	exitCode := 0
	for _, name := range positional {
		val, ok := os.LookupEnv(name)
		if ok {
			fmt.Fprintf(os.Stdout, "%s%s", val, terminator)
		} else {
			exitCode = 1
		}
	}
	os.Exit(exitCode)
}
