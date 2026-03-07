// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the tee utility for copying stdin to stdout and files.
//
// Implements prd017-tee: core copy behavior (R1), append mode and SIGINT
// suppression (R2), error handling and exit codes (R3).
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const version = "tee (go-unix-utils) 1.0"

func main() {
	sys.InstallSIGPIPEHandler()

	appendMode, ignoreInterrupts, files := parseArgs(os.Args[1:])

	// R2.2: Ignore SIGINT when -i is set.
	if ignoreInterrupts {
		signal.Ignore(syscall.SIGINT)
	}

	exitCode := 0

	// Open output files.
	var writers []io.Writer
	writers = append(writers, os.Stdout)

	var closers []*os.File
	for _, name := range files {
		// R1.4: "-" is an additional reference to stdout.
		if name == "-" {
			// stdout is already in writers; skip to avoid duplicate writes.
			continue
		}

		flags := os.O_WRONLY | os.O_CREATE
		if appendMode {
			// R2.1: Append mode.
			flags |= os.O_APPEND
		} else {
			// R1.3: Truncate existing files.
			flags |= os.O_TRUNC
		}

		f, err := os.OpenFile(name, flags, 0o644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tee: %s: %v\n", name, err)
			exitCode = 1
			continue
		}
		writers = append(writers, f)
		closers = append(closers, f)
	}

	// R1.1: Read stdin and write to all destinations simultaneously.
	mw := io.MultiWriter(writers...)
	r := bufio.NewReader(os.Stdin)

	if _, err := io.Copy(mw, r); err != nil {
		fmt.Fprintf(os.Stderr, "tee: %v\n", err)
		exitCode = 1
	}

	// Close output files.
	for _, f := range closers {
		if err := f.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "tee: %s: %v\n", f.Name(), err)
			exitCode = 1
		}
	}

	os.Exit(exitCode)
}

// parseArgs extracts flags and file names from command-line arguments.
func parseArgs(args []string) (appendMode, ignoreInterrupts bool, files []string) {
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			i++
			break
		}
		if arg == "--help" {
			fmt.Fprintln(os.Stdout, "Usage: tee [OPTION]... [FILE]...\nCopy standard input to each FILE, and also to standard output.\n\n  -a, --append             append to the given FILEs, do not overwrite\n  -i, --ignore-interrupts  ignore interrupt signals\n      --help     display this help and exit\n      --version  output version information and exit")
			os.Exit(0)
		}
		if arg == "--version" {
			fmt.Fprintln(os.Stdout, version)
			os.Exit(0)
		}
		if arg == "--append" {
			appendMode = true
			i++
			continue
		}
		if arg == "--ignore-interrupts" {
			ignoreInterrupts = true
			i++
			continue
		}
		if strings.HasPrefix(arg, "--") {
			fmt.Fprintf(os.Stderr, "tee: unrecognized option '%s'\n", arg)
			os.Exit(1)
		}
		if len(arg) > 1 && arg[0] == '-' {
			// Short flags.
			for _, ch := range arg[1:] {
				switch ch {
				case 'a':
					appendMode = true
				case 'i':
					ignoreInterrupts = true
				default:
					fmt.Fprintf(os.Stderr, "tee: invalid option -- '%c'\n", ch)
					os.Exit(1)
				}
			}
			i++
			continue
		}
		break
	}

	files = args[i:]
	return appendMode, ignoreInterrupts, files
}
