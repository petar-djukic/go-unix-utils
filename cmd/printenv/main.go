// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the printenv utility for printing environment variables.
//
// Implements prd040-printenv: default behavior (R1), output formatting and exit codes (R2).
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	nullTerminate := false
	var names []string

	for _, arg := range args {
		if arg == "-0" || arg == "--null" {
			nullTerminate = true
		} else if strings.HasPrefix(arg, "-") && arg != "-" {
			fmt.Fprintf(os.Stderr, "printenv: invalid option -- '%s'\n", arg[1:])
			os.Exit(2)
		} else {
			names = append(names, arg)
		}
	}

	terminator := byte('\n')
	if nullTerminate {
		terminator = 0
	}

	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()

	if len(names) == 0 {
		for _, env := range os.Environ() {
			w.WriteString(env)
			w.WriteByte(terminator)
		}
		return
	}

	exitCode := 0
	for _, name := range names {
		val, ok := os.LookupEnv(name)
		if !ok {
			exitCode = 1
			continue
		}
		w.WriteString(val)
		w.WriteByte(terminator)
	}
	w.Flush()
	os.Exit(exitCode)
}
