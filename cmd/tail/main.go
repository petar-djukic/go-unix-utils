// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd055-tail: Print the last lines or bytes of files.
// R1 (line-count mode), R2 (byte-count mode), R3 (multi-file headers),
// R4 (exit codes and differential testing).
package main

import (
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// countMode distinguishes between line-count and byte-count modes.
// R1: line mode is the default; R2: byte mode is selected by -c.
type countMode int

const (
	modeLines countMode = iota
	modeBytes
)

// config holds the parsed command-line flags for tail.
// R1.1–R1.4: line-count mode fields.
// R2.1–R2.3: byte-count mode fields.
// R3.1–R3.4: header control fields.
type config struct {
	// mode selects line-count vs byte-count (R1 vs R2).
	mode countMode
	// count is the number of lines or bytes to output (R1.2, R2.1).
	count int64
	// fromStart is true when the +N prefix is used (R1.3, R2.2).
	fromStart bool
	// quiet suppresses headers even for multiple files (R3.3).
	quiet bool
	// verbose forces headers even for a single file (R3.4).
	verbose bool
	// zeroTerminated uses NUL instead of newline as line delimiter.
	zeroTerminated bool

	// TODO: prd055-tail non_goals: follow mode (-f/--follow), --pid,
	// --retry, and -F are excluded. Config fields for follow-name vs
	// follow-descriptor, sleep interval, pid, retry, and
	// max-unchanged-stats are not defined per E6 (non-goals enforcement).
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run parses arguments and dispatches to the appropriate tail function.
// Returns the process exit code (R4.1, R4.2).
func run(args []string) int {
	panic("not implemented")
}

// parseArgs parses command-line arguments into a config and file list.
// R1.2: -n/--lines, R2.1: -c/--bytes, R2.3: multiplier suffixes,
// R3.3: -q/--quiet/--silent, R3.4: -v/--verbose.
func parseArgs(args []string) (config, []string) {
	panic("not implemented")
}

// tailFile prints the last lines or bytes of a named file.
// R1.1–R1.3: line-count extraction, R2.1–R2.2: byte-count extraction,
// R3.1–R3.2: header printing, R4.2/R4.4: error handling for unreadable files.
func tailFile(cfg config, filename string, printHeader bool) error {
	panic("not implemented")
}

// tailStdin prints the last lines or bytes of stdin.
// R1.4: reads from stdin when no file arguments are given or file is "-".
func tailStdin(cfg config) error {
	panic("not implemented")
}

// TODO: prd055-tail non_goals: followFile is not defined because -f/--follow
// is excluded from the implementation per E6 (non-goals enforcement).
