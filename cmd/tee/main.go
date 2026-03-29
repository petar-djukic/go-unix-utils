// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/tee implements GNU tee: read stdin and write to stdout and files.
//
// Implements prd017-tee R1.1-R1.5, R2.1-R2.3, R3.1-R3.4.
//
// TODO: --output-error modes (warn, warn-nopipe, exit, exit-nopipe) are listed
// in prd017 non_goals and must not be implemented per article E6.
package main

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const helpText = `Usage: tee [OPTION]... [FILE]...
Copy standard input to each FILE, and also to standard output.

  -a, --append             append to the given FILEs, do not overwrite
  -i, --ignore-interrupts  ignore interrupt signals
      --help        display this help and exit
      --version     output version information and exit
`

const versionText = "tee (go-unix-utils) 0.1\n"

// stdoutName is the display name for stdout in error diagnostics.
const stdoutName = "standard output"

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run parses arguments and executes tee logic.
func run(args []string, stdin io.Reader, stdout, stderr *os.File) int {
	opts, result := parseArgs(args)
	switch result {
	case parseHelp:
		fmt.Fprint(stdout, helpText) //nolint:errcheck // best-effort
		return 0
	case parseVer:
		fmt.Fprint(stdout, versionText) //nolint:errcheck // best-effort
		return 0
	}

	// R2.2: ignore SIGINT when -i is set.
	if opts.ignoreInterrupts {
		signal.Ignore(syscall.SIGINT)
	}

	return copyToFiles(opts, stdin, stdout, stderr)
}

// parseResult signals how argument parsing concluded.
type parseResult int

const (
	parseOK   parseResult = iota
	parseHelp             // --help requested
	parseVer              // --version requested
)

// options holds parsed command-line flags.
type options struct {
	appendMode       bool
	ignoreInterrupts bool
	files            []string
}

// parseArgs parses GNU-style tee arguments.
func parseArgs(args []string) (*options, parseResult) {
	opts := &options{}
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--help" {
			return opts, parseHelp
		}
		if arg == "--version" {
			return opts, parseVer
		}
		if arg == "--" {
			opts.files = append(opts.files, args[i+1:]...)
			return opts, parseOK
		}
		if arg == "--append" {
			opts.appendMode = true
			i++
			continue
		}
		if arg == "--ignore-interrupts" {
			opts.ignoreInterrupts = true
			i++
			continue
		}
		if len(arg) > 1 && arg[0] == '-' && arg[1] != '-' {
			if parseShortFlags(arg, opts) {
				i++
				continue
			}
		}
		// Not a flag -- this and all remaining args are file names.
		opts.files = append(opts.files, args[i:]...)
		return opts, parseOK
	}
	return opts, parseOK
}

// parseShortFlags handles short flags like -a, -i, -ai.
// Returns true if the arg was handled as flags.
func parseShortFlags(arg string, opts *options) bool {
	for j := 1; j < len(arg); j++ {
		switch arg[j] {
		case 'a':
			opts.appendMode = true
		case 'i':
			opts.ignoreInterrupts = true
		default:
			return false
		}
	}
	return true
}

// dest represents a single output destination with error tracking.
type dest struct {
	name    string   // display name for diagnostics
	file    *os.File // underlying writer
	errored bool     // true after first write error
}

// copyToFiles reads stdin and writes to stdout and all output files.
// R1.5: writes output in the order received from stdin.
// R3.3: continues writing to remaining destinations when one fails.
func copyToFiles(opts *options, stdin io.Reader, stdout, stderr *os.File) int {
	dests, exitCode := openDests(opts, stdout, stderr)
	defer closeDests(dests)

	buf := make([]byte, 32*1024)
	for {
		n, readErr := stdin.Read(buf)
		if n > 0 {
			if writeErr := writeAll(dests, buf[:n], stderr); writeErr {
				exitCode = 1
			}
		}
		if readErr != nil {
			break
		}
	}

	return exitCode
}

// openDests opens all output files and builds the destination list.
// Stdout is always the first destination.
// R3.2: open failures print a diagnostic and skip the file.
func openDests(opts *options, stdout, stderr *os.File) ([]dest, int) {
	flag := os.O_WRONLY | os.O_CREATE
	if opts.appendMode {
		flag |= os.O_APPEND
	} else {
		flag |= os.O_TRUNC
	}

	exitCode := 0
	dests := make([]dest, 0, 1+len(opts.files))
	dests = append(dests, dest{name: stdoutName, file: stdout})

	for _, name := range opts.files {
		f, err := os.OpenFile(name, flag, 0o666)
		if err != nil {
			// R3.1: GNU format 'tee: FILE: REASON'
			printDiag(stderr, name, err)
			exitCode = 1
			continue
		}
		dests = append(dests, dest{name: name, file: f})
	}
	return dests, exitCode
}

// writeAll writes data to every non-errored destination.
// Returns true if any write error occurred.
func writeAll(dests []dest, data []byte, stderr *os.File) bool {
	hadError := false
	for i := range dests {
		if dests[i].errored {
			continue
		}
		if _, err := dests[i].file.Write(data); err != nil {
			// R3.1: GNU format 'tee: FILE: REASON'
			printDiag(stderr, dests[i].name, err)
			dests[i].errored = true
			hadError = true
		}
	}
	return hadError
}

// printDiag writes a GNU-compatible diagnostic to stderr.
// R3.1: format is 'tee: NAME: REASON'.
// R3.4: always writes to stderr regardless of other flags.
func printDiag(stderr *os.File, name string, err error) {
	fmt.Fprintf(stderr, "tee: %s: %s\n", name, unwrapMsg(err)) //nolint:errcheck
}

// closeDests closes all opened file destinations (skips stdout).
func closeDests(dests []dest) {
	for i := 1; i < len(dests); i++ {
		dests[i].file.Close() // best-effort close
	}
}

// unwrapMsg extracts the inner message from a *os.PathError.
func unwrapMsg(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}
