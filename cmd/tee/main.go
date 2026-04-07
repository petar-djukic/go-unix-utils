// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/tee: read stdin and write to stdout and files.
// Implements srd017-tee R1.1-R1.5, R2.1-R2.3, R3.1-R3.4.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is used in diagnostic messages.
const progName = "tee"

// TODO: -p flag and --output-error=MODE (warn, warn-nopipe, exit, exit-nopipe)
// are listed in srd017 non_goals. Skipped per execution constitution E6.

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:])
	os.Exit(exitCode)
}

// run executes the tee logic and returns the exit code.
// R1.2: with no file arguments, acts as passthrough (stdin to stdout).
// R1.1: writes stdin bytes to stdout and all named files simultaneously.
// R3.1: returns 0 when all writes succeed.
// R3.2, R3.4: returns 1 when any file or stdout write fails.
func run(args []string) int {
	appendMode, ignoreInt, fileArgs := parseFlags(args)

	// R2.2: when -i is specified, ignore SIGINT.
	if ignoreInt {
		signal.Ignore(syscall.SIGINT)
	}

	files, exitCode := openFiles(fileArgs, appendMode)
	defer closeFiles(files)

	// R3.3: use resilient writer that continues writing to remaining
	// destinations when one fails.
	rw := newResilientWriter(files)
	if err := copyToAll(os.Stdin, rw); err != nil {
		exitCode = 1
	}
	if rw.failed {
		exitCode = 1
	}
	return exitCode
}

// parseFlags extracts -a and -i flags from args, returning remaining file args.
// R2.3: -a and -i may be combined; their effects are independent.
func parseFlags(args []string) (appendMode, ignoreInt bool, fileArgs []string) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			fileArgs = append(fileArgs, args[i+1:]...)
			return
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
			parseShortFlags(arg[1:], &appendMode, &ignoreInt)
			continue
		}
		fileArgs = append(fileArgs, arg)
	}
	return
}

// parseShortFlags processes combined short flags like -ai.
func parseShortFlags(flags string, appendMode, ignoreInt *bool) {
	for _, c := range flags {
		switch c {
		case 'a':
			*appendMode = true
		case 'i':
			*ignoreInt = true
		default:
			fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\n", progName, c)
		}
	}
}

// openFiles opens each file argument for writing.
// R1.3: creates files that do not exist.
// R2.1: when appendMode is true, opens with O_APPEND; otherwise truncates.
// R1.4: "-" is treated as stdout (not opened as a file).
// R3.2: reports open failures to stderr and sets exit code 1.
func openFiles(args []string, appendMode bool) ([]*os.File, int) {
	var files []*os.File
	exitCode := 0

	flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	if appendMode {
		flags = os.O_WRONLY | os.O_CREATE | os.O_APPEND
	}

	for _, name := range args {
		if name == "-" {
			// R1.4: "-" means stdout; no additional file to open.
			continue
		}
		f, err := os.OpenFile(name, flags, 0o666)
		if err != nil {
			reportError(name, err)
			exitCode = 1
			continue
		}
		files = append(files, f)
	}
	return files, exitCode
}

// reportError prints a GNU-compatible diagnostic to stderr.
// R3.2: format is "tee: <filename>: <reason>".
func reportError(name string, err error) {
	var pe *os.PathError
	if errors.As(err, &pe) {
		fmt.Fprintf(os.Stderr, "%s: %s: %s\n", progName, name, pe.Err)
		return
	}
	fmt.Fprintf(os.Stderr, "%s: %s: %s\n", progName, name, err)
}

// dest tracks a named write destination for resilient multi-write.
type dest struct {
	writer io.Writer
	name   string
	dead   bool
}

// resilientWriter writes to stdout and all files, continuing past failures.
// R3.3: a single file failure does not stop output to other destinations.
type resilientWriter struct {
	dests  []dest
	failed bool
}

// newResilientWriter builds a writer list with stdout first, then files.
func newResilientWriter(files []*os.File) *resilientWriter {
	rw := &resilientWriter{
		dests: make([]dest, 0, 1+len(files)),
	}
	rw.dests = append(rw.dests, dest{writer: os.Stdout, name: "stdout"})
	for _, f := range files {
		rw.dests = append(rw.dests, dest{writer: f, name: f.Name()})
	}
	return rw
}

// Write writes p to all live destinations.
// R3.3: skips destinations that have previously failed.
// R3.2: reports write errors to stderr on first occurrence.
// R3.4: tracks stdout write failures.
func (rw *resilientWriter) Write(p []byte) (int, error) {
	var stdoutErr error
	for i := range rw.dests {
		if rw.dests[i].dead {
			continue
		}
		_, err := rw.dests[i].writer.Write(p)
		if err != nil {
			rw.dests[i].dead = true
			rw.failed = true
			if i == 0 {
				// R3.4: stdout write error.
				stdoutErr = err
			}
			reportWriteError(rw.dests[i].name, err)
		}
	}
	// R3.4: propagate stdout error to stop io.Copy when stdout is broken.
	if stdoutErr != nil {
		return 0, stdoutErr
	}
	return len(p), nil
}

// reportWriteError prints a diagnostic for a write failure.
func reportWriteError(name string, err error) {
	fmt.Fprintf(os.Stderr, "%s: %s: %s\n", progName, name, err)
}

// copyToAll reads from r and writes every byte to the resilient writer.
// R1.5: output order matches stdin order.
func copyToAll(r io.Reader, w io.Writer) error {
	_, err := io.Copy(w, r)
	return err
}

// closeFiles closes all open file handles.
func closeFiles(files []*os.File) {
	for _, f := range files {
		f.Close() // best-effort close
	}
}
