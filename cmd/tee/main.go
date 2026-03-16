// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd017-tee R1.1-R1.5, R2.1-R2.3, R3.1-R3.4: cmd/tee reads
// stdin and writes to stdout and named output files simultaneously. Supports
// append mode (-a), SIGINT suppression (-i), --output-error modes (warn,
// warn-nopipe, exit, exit-nopipe), and -p shorthand for --output-error=warn.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is the name used in error messages to match GNU tee format.
const progName = "tee"

// outputErrorMode controls how tee handles write errors.
type outputErrorMode int

const (
	// outputErrorDefault: SIGPIPE exits 0, write errors diagnosed, exit 1.
	outputErrorDefault outputErrorMode = iota
	// outputErrorWarn: diagnose all write errors, continue processing.
	outputErrorWarn
	// outputErrorWarnNopipe: diagnose non-pipe write errors, silently ignore EPIPE.
	outputErrorWarnNopipe
	// outputErrorExit: diagnose and exit on any write error.
	outputErrorExit
	// outputErrorExitNopipe: diagnose and exit on non-pipe write errors, ignore EPIPE.
	outputErrorExitNopipe
)

// destination represents a single output target for tee.
type destination struct {
	w      io.Writer
	name   string
	failed bool
}

func main() {
	appendMode := false
	ignoreInterrupts := false
	mode := outputErrorDefault

	// Parse flags manually to match GNU tee flag handling.
	var files []string
	for _, arg := range os.Args[1:] {
		switch {
		case arg == "-a" || arg == "--append":
			appendMode = true
		case arg == "-i" || arg == "--ignore-interrupts":
			ignoreInterrupts = true
		case arg == "-p":
			// R3.2: -p is shorthand for --output-error=warn.
			mode = outputErrorWarn
		case arg == "--output-error":
			// --output-error without =MODE defaults to warn.
			mode = outputErrorWarn
		case strings.HasPrefix(arg, "--output-error="):
			val := strings.TrimPrefix(arg, "--output-error=")
			parsed, err := parseOutputErrorMode(val)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
				os.Exit(1)
			}
			mode = parsed
		default:
			files = append(files, arg)
		}
	}

	// R3.1: Install SIGPIPE handler based on mode.
	// Default mode: exit 0 on SIGPIPE (standard GNU behavior).
	// Any --output-error mode: ignore SIGPIPE signal so we can handle
	// EPIPE write errors individually in the write loop.
	if mode == outputErrorDefault {
		sys.InstallSIGPIPEHandler()
	} else {
		signal.Ignore(syscall.SIGPIPE)
	}

	// R2.2: ignore SIGINT when -i is specified.
	if ignoreInterrupts {
		signal.Ignore(os.Interrupt)
	}

	exitCode := 0

	// Build destination list: stdout first, then each file operand.
	dests := []destination{{w: os.Stdout, name: "standard output"}}
	var closers []*os.File

	for _, name := range files {
		// R1.4: "-" is an additional reference to stdout; already included.
		if name == "-" {
			continue
		}

		// R1.3, R2.1: create/truncate or append depending on -a flag.
		flag := os.O_WRONLY | os.O_CREATE
		if appendMode {
			flag |= os.O_APPEND
		} else {
			flag |= os.O_TRUNC
		}
		f, err := os.OpenFile(name, flag, 0o666)
		if err != nil {
			// R3.2: report failed file and continue.
			fmt.Fprintf(os.Stderr, "%s: %s: %v\n", progName, name, unwrapPathError(err))
			exitCode = 1
			continue
		}
		dests = append(dests, destination{w: f, name: name})
		closers = append(closers, f)
	}

	// R1.1, R1.5: read stdin and write to all destinations in order.
	buf := make([]byte, 8192)
	shouldExit := false
	for !shouldExit {
		n, readErr := os.Stdin.Read(buf)
		if n > 0 {
			for i := range dests {
				if dests[i].failed {
					continue
				}
				if _, werr := dests[i].w.Write(buf[:n]); werr != nil {
					shouldExit = handleWriteError(mode, dests[i].name, werr, &exitCode)
					dests[i].failed = true
					if shouldExit {
						break
					}
				}
			}
		}
		if readErr != nil {
			if readErr != io.EOF {
				fmt.Fprintf(os.Stderr, "%s: read error: %v\n", progName, readErr)
				exitCode = 1
			}
			break
		}
	}

	// Close all opened files.
	for _, f := range closers {
		f.Close() // best-effort close
	}

	os.Exit(exitCode)
}

// handleWriteError processes a write error according to the current output
// error mode. It prints diagnostics as appropriate and sets the exit code.
// Returns true if tee should stop processing immediately (exit modes).
func handleWriteError(mode outputErrorMode, name string, err error, exitCode *int) bool {
	pipe := isPipeError(err)

	switch mode {
	case outputErrorDefault:
		// R3.3: diagnose all errors, continue to other destinations, exit 1.
		fmt.Fprintf(os.Stderr, "%s: %s: %v\n", progName, name, err)
		*exitCode = 1
		return false

	case outputErrorWarn:
		// R3.2: diagnose all write errors, continue processing.
		fmt.Fprintf(os.Stderr, "%s: %s: %v\n", progName, name, err)
		*exitCode = 1
		return false

	case outputErrorWarnNopipe:
		// R3.3: silently ignore pipe errors, diagnose others.
		if pipe {
			return false
		}
		fmt.Fprintf(os.Stderr, "%s: %s: %v\n", progName, name, err)
		*exitCode = 1
		return false

	case outputErrorExit:
		// R3.4: diagnose and exit on any write error.
		fmt.Fprintf(os.Stderr, "%s: %s: %v\n", progName, name, err)
		*exitCode = 1
		return true

	case outputErrorExitNopipe:
		// R3.4: silently ignore pipe errors, exit on others.
		if pipe {
			return false
		}
		fmt.Fprintf(os.Stderr, "%s: %s: %v\n", progName, name, err)
		*exitCode = 1
		return true
	}

	return false
}

// parseOutputErrorMode parses the value of --output-error=MODE.
func parseOutputErrorMode(val string) (outputErrorMode, error) {
	switch val {
	case "warn":
		return outputErrorWarn, nil
	case "warn-nopipe":
		return outputErrorWarnNopipe, nil
	case "exit":
		return outputErrorExit, nil
	case "exit-nopipe":
		return outputErrorExitNopipe, nil
	default:
		return outputErrorDefault, fmt.Errorf("ambiguous argument %q for '--output-error'\nValid arguments are:\n  - 'warn'\n  - 'warn-nopipe'\n  - 'exit'\n  - 'exit-nopipe'", val)
	}
}

// isPipeError reports whether err is a broken pipe error (EPIPE).
func isPipeError(err error) bool {
	return errors.Is(err, syscall.EPIPE)
}

// unwrapPathError extracts the inner error from an *os.PathError to produce
// messages like "No such file or directory" instead of "open foo: no such ...".
func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}
