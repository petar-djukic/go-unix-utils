// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd017-tee R1.1–R1.5, R2.1–R2.3, R3.1–R3.4
package main

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// stdoutName is the display name for stdout in error diagnostics,
// matching GNU tee format.
const stdoutName = "standard output"

func main() {
	// R1.5: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	// Parse flags: -a/--append, -i/--ignore-interrupts.
	appendMode, ignoreInt, fileArgs := parseArgs(os.Args[1:])

	// R2.2: When -i is given, ignore SIGINT so tee continues until EOF or
	// write error, matching GNU tee behavior.
	if ignoreInt {
		signal.Ignore(syscall.SIGINT)
	}

	exitCode := 0

	// R1.3, R2.1: Choose open flags based on -a.
	openFlags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	if appendMode {
		openFlags = os.O_WRONLY | os.O_CREATE | os.O_APPEND
	}

	// R1.1, R1.2: Build destinations: always include stdout.
	mw := &resilientMultiWriter{
		dests: []dest{{name: stdoutName, w: os.Stdout}},
	}
	var files []*os.File

	for _, arg := range fileArgs {
		// R1.4: "-" is an additional reference to stdout; data is not
		// duplicated because the writer handles it.
		if arg == "-" {
			continue
		}
		f, err := os.OpenFile(arg, openFlags, 0o666)
		if err != nil {
			// R3.2: diagnostic for failed open.
			reportError(arg, err)
			exitCode = 1
			continue
		}
		files = append(files, f)
		mw.dests = append(mw.dests, dest{name: arg, w: f})
	}

	// R1.1, R1.5, R3.3: Read all of stdin and write to all destinations.
	// The resilient writer continues to remaining destinations when one fails.
	if _, err := io.Copy(mw, os.Stdin); err != nil {
		fmt.Fprintf(os.Stderr, "tee: %v\n", err)
		exitCode = 1
	}
	// R3.1, R3.2, R3.4: Set exit code if any write failed.
	if mw.hadError {
		exitCode = 1
	}

	// Close all opened files.
	for _, f := range files {
		if err := f.Close(); err != nil {
			reportError(f.Name(), err)
			exitCode = 1
		}
	}

	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

// dest is a named output destination.
type dest struct {
	name   string
	w      io.Writer
	failed bool
}

// resilientMultiWriter writes to multiple destinations. When a write fails on
// one destination, it reports the error to stderr and skips that destination
// for future writes. R3.3: a single file failure does not stop output to
// other destinations.
type resilientMultiWriter struct {
	dests    []dest
	hadError bool
}

// Write writes p to all non-failed destinations.
func (m *resilientMultiWriter) Write(p []byte) (int, error) {
	for i := range m.dests {
		if m.dests[i].failed {
			continue
		}
		if _, err := m.dests[i].w.Write(p); err != nil {
			m.dests[i].failed = true
			m.hadError = true
			// R3.2, R3.4: diagnostic to stderr for file or stdout write error.
			reportError(m.dests[i].name, err)
		}
	}
	return len(p), nil
}

// reportError prints a diagnostic to stderr in the format:
// tee: <name>: <message>, matching GNU tee output.
func reportError(name string, err error) {
	fmt.Fprintf(os.Stderr, "tee: %s: %v\n", name, unwrapError(err))
}

// unwrapError extracts the innermost error message, stripping os.PathError
// wrapping so the diagnostic omits the verb prefix (open/write).
func unwrapError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}

// parseArgs extracts -a/--append and -i/--ignore-interrupts flags from args,
// returning the flag states and remaining file arguments. Supports combined
// short flags (e.g., -ai) and -- to end flag parsing.
func parseArgs(args []string) (appendMode, ignoreInt bool, files []string) {
	endFlags := false
	for _, arg := range args {
		if endFlags {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			endFlags = true
			continue
		}
		if arg == "--append" {
			appendMode = true
			continue
		}
		if arg == "--ignore-interrupts" {
			ignoreInt = true
			continue
		}
		if len(arg) > 1 && arg[0] == '-' && arg[1] != '-' {
			// Parse combined short flags (e.g., -ai, -ia).
			knownFlags := true
			for _, ch := range arg[1:] {
				switch ch {
				case 'a':
					appendMode = true
				case 'i':
					ignoreInt = true
				default:
					knownFlags = false
				}
			}
			if knownFlags {
				continue
			}
		}
		files = append(files, arg)
	}
	return appendMode, ignoreInt, files
}
